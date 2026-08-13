package productflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ListingOperationContent = "content_generation"
	ListingOperationPackage = "publish_package"
)

type listingBatchOptions struct {
	GenerationMode string `json:"generationMode"`
}
type listingQueueMessage struct {
	BatchID string `json:"batchId"`
}

func (s *Service) listingQueueName(operation string) string {
	base := strings.TrimSpace(s.ContentQueueName)
	if base == "" {
		base = "listing:operation:batches"
	}
	if operation == ListingOperationPackage {
		return base + ":package"
	}
	return base + ":content"
}

func (s *Service) enqueueListingBatch(ctx context.Context, batch *ListingOperationBatch) error {
	if !s.ContentQueueEnabled || s.Redis == nil || s.Redis.Client == nil {
		return fmt.Errorf("%w: listing operation queue unavailable", ErrConflict)
	}
	payload, _ := json.Marshal(listingQueueMessage{BatchID: batch.ID.String()})
	workers := s.ContentConcurrency
	if batch.Operation == ListingOperationPackage {
		workers = s.PackageConcurrency
	}
	if workers < 1 {
		workers = 1
	}
	if workers > 16 {
		workers = 16
	}
	values := make([]any, workers)
	for i := range values {
		values[i] = string(payload)
	}
	return s.Redis.LPush(ctx, s.listingQueueName(batch.Operation), values...).Err()
}

func (s *Service) CreateListingOperationBatch(ctx context.Context, tenantID int64, actor *uuid.UUID, body CreateListingOperationBatchBody) (*ListingOperationBatch, error) {
	op := strings.ToLower(strings.TrimSpace(body.Operation))
	if op != ListingOperationContent && op != ListingOperationPackage {
		return nil, fmt.Errorf("%w: invalid listing operation", ErrValidation)
	}
	if len(body.ListingDraftIDs) == 0 || len(body.ListingDraftIDs) > 500 {
		return nil, fmt.Errorf("%w: listingDraftIds must contain 1-500 items", ErrValidation)
	}
	seen := map[uuid.UUID]bool{}
	ids := make([]uuid.UUID, 0, len(body.ListingDraftIDs))
	for _, id := range body.ListingDraftIDs {
		if id != uuid.Nil && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	var count int64
	if err := s.DB.WithContext(ctx).Model(&ListingDraft{}).Where("tenant_id=? AND id IN ?", tenantID, ids).Count(&count).Error; err != nil {
		return nil, err
	}
	if count != int64(len(ids)) {
		return nil, fmt.Errorf("%w: one or more listings do not exist", ErrValidation)
	}
	mode := strings.ToLower(strings.TrimSpace(body.GenerationMode))
	if mode == "" {
		mode = "template_only"
	}
	if op == ListingOperationContent && mode != "template_only" && mode != "ai_generate" {
		return nil, fmt.Errorf("%w: invalid generation mode", ErrValidation)
	}
	options := jsonData(listingBatchOptions{GenerationMode: mode})
	batch := ListingOperationBatch{TenantID: tenantID, Operation: op, Status: BatchStatusPending, Total: len(ids), Pending: len(ids), Options: options, CreatedBy: actor}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&batch).Error; e != nil {
			return e
		}
		items := make([]ListingOperationBatchItem, 0, len(ids))
		for _, id := range ids {
			items = append(items, ListingOperationBatchItem{TenantID: tenantID, BatchID: batch.ID, ListingDraftID: id, Status: BatchItemStatusPending, MaxAttempts: 3})
		}
		return tx.CreateInBatches(items, 100).Error
	})
	if err != nil {
		return nil, err
	}
	if err = s.enqueueListingBatch(ctx, &batch); err != nil {
		now := time.Now().UTC()
		_ = s.DB.Model(&batch).Updates(map[string]any{"status": BatchStatusFailed, "completed_at": now}).Error
		return nil, err
	}
	return &batch, nil
}

func (s *Service) ListListingOperationBatches(ctx context.Context, tenantID int64, limit int) ([]ListingOperationBatch, error) {
	if limit < 1 || limit > 100 {
		limit = 20
	}
	var rows []ListingOperationBatch
	err := s.DB.WithContext(ctx).Where("tenant_id=?", tenantID).Order("created_at DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
func (s *Service) GetListingOperationBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingOperationBatch, error) {
	var row ListingOperationBatch
	err := s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}
func (s *Service) ListListingOperationBatchItems(ctx context.Context, tenantID int64, id uuid.UUID) ([]ListingOperationBatchItem, error) {
	if _, err := s.GetListingOperationBatch(ctx, tenantID, id); err != nil {
		return nil, err
	}
	var rows []ListingOperationBatchItem
	err := s.DB.WithContext(ctx).Where("tenant_id=? AND batch_id=?", tenantID, id).Order("created_at").Find(&rows).Error
	return rows, err
}
func (s *Service) RetryFailedListingOperationBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingOperationBatch, error) {
	batch, err := s.GetListingOperationBatch(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if batch.Failed == 0 {
		return nil, fmt.Errorf("%w: batch has no failed items", ErrInvalidTransition)
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&ListingOperationBatchItem{}).Where("tenant_id=? AND batch_id=? AND status=?", tenantID, id, BatchItemStatusFailed).Updates(map[string]any{"status": BatchItemStatusPending, "attempts": 0, "error_message": "", "completed_at": nil})
		if res.Error != nil {
			return res.Error
		}
		return tx.Model(batch).Updates(map[string]any{"status": BatchStatusPending, "pending": gorm.Expr("pending + ?", res.RowsAffected), "failed": 0, "completed_at": nil}).Error
	})
	if err != nil {
		return nil, err
	}
	if err = s.enqueueListingBatch(ctx, batch); err != nil {
		return nil, err
	}
	return s.GetListingOperationBatch(ctx, tenantID, id)
}
func (s *Service) CancelListingOperationBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingOperationBatch, error) {
	batch, err := s.GetListingOperationBatch(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if batch.Status != BatchStatusPending && batch.Status != BatchStatusRunning {
		return nil, ErrInvalidTransition
	}
	now := time.Now().UTC()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Model(&ListingOperationBatchItem{}).Where("batch_id=? AND status=?", id, BatchItemStatusPending).Updates(map[string]any{"status": BatchItemStatusCancelled, "completed_at": now}).Error; e != nil {
			return e
		}
		return tx.Model(batch).Updates(map[string]any{"status": BatchStatusCancelled, "pending": 0, "completed_at": now}).Error
	})
	if err != nil {
		return nil, err
	}
	return s.GetListingOperationBatch(ctx, tenantID, id)
}

func (s *Service) claimListingBatchItem(ctx context.Context, batchID uuid.UUID, workerID string) (*ListingOperationBatchItem, *ListingOperationBatch, error) {
	var item ListingOperationBatchItem
	var batch ListingOperationBatch
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&batch, "id=?", batchID).Error; e != nil {
			return e
		}
		if batch.Status != BatchStatusPending && batch.Status != BatchStatusRunning {
			return gorm.ErrRecordNotFound
		}
		q := tx.Where("batch_id=? AND status=?", batchID, BatchItemStatusPending).Order("created_at")
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if e := q.First(&item).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		if batch.Status == BatchStatusPending {
			if e := tx.Model(&batch).Updates(map[string]any{"status": BatchStatusRunning, "started_at": now}).Error; e != nil {
				return e
			}
		}
		if e := tx.Model(&item).Updates(map[string]any{"status": BatchItemStatusProcessing, "attempts": gorm.Expr("attempts + 1"), "worker_id": workerID, "started_at": now, "error_message": ""}).Error; e != nil {
			return e
		}
		if e := tx.Model(&batch).Updates(map[string]any{"pending": gorm.Expr("pending - 1"), "processing": gorm.Expr("processing + 1")}).Error; e != nil {
			return e
		}
		item.Attempts++
		return nil
	})
	return &item, &batch, err
}
func (s *Service) processListingBatchItem(ctx context.Context, batch *ListingOperationBatch, item *ListingOperationBatchItem) error {
	var options listingBatchOptions
	_ = json.Unmarshal(batch.Options, &options)
	var resultID uuid.UUID
	var err error
	if batch.Operation == ListingOperationContent {
		var out *ListingContentVersion
		out, err = s.generateListingContent(ctx, batch.TenantID, item.ListingDraftID, batch.CreatedBy, GenerateListingContentBody{GenerationMode: options.GenerationMode}, &item.ID)
		if out != nil {
			resultID = out.ID
		}
	} else {
		var out *ListingPublishPackage
		out, err = s.generatePublishPackage(ctx, batch.TenantID, item.ListingDraftID, batch.CreatedBy, &item.ID)
		if out != nil {
			resultID = out.ID
		}
	}
	now := time.Now().UTC()
	if err != nil {
		return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(item).Updates(map[string]any{"status": BatchItemStatusFailed, "error_message": truncateError(err), "completed_at": now}).Error; e != nil {
				return e
			}
			return tx.Model(batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1"), "failed": gorm.Expr("failed + 1")}).Error
		})
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Model(item).Updates(map[string]any{"status": BatchItemStatusCompleted, "result_id": resultID, "completed_at": now}).Error; e != nil {
			return e
		}
		return tx.Model(batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1"), "completed": gorm.Expr("completed + 1")}).Error
	})
}
func (s *Service) finalizeListingBatch(ctx context.Context, id uuid.UUID) error {
	var b ListingOperationBatch
	if e := s.DB.WithContext(ctx).First(&b, "id=?", id).Error; e != nil {
		return e
	}
	if b.Pending > 0 || b.Processing > 0 {
		return nil
	}
	if b.Status == BatchStatusCancelled {
		return nil
	}
	status := BatchStatusCompleted
	if b.Failed > 0 && b.Completed > 0 {
		status = BatchStatusPartialFailed
	}
	if b.Failed > 0 && b.Completed == 0 {
		status = BatchStatusFailed
	}
	now := time.Now().UTC()
	return s.DB.WithContext(ctx).Model(&b).Updates(map[string]any{"status": status, "completed_at": now}).Error
}
func (s *Service) RunListingOperationBatch(ctx context.Context, id uuid.UUID, workerID string) {
	for {
		item, batch, err := s.claimListingBatchItem(ctx, id, workerID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.finalizeListingBatch(ctx, id)
			return
		}
		if err != nil {
			return
		}
		_ = s.processListingBatchItem(ctx, batch, item)
		_ = s.finalizeListingBatch(ctx, id)
	}
}

func (s *Service) RecoverListingOperationBatches(ctx context.Context) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var items []ListingOperationBatchItem
		if err := tx.Where("status=?", BatchItemStatusProcessing).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			if err := tx.Model(&item).Updates(map[string]any{"status": BatchItemStatusPending, "worker_id": "", "error_message": "worker interrupted; recovered", "completed_at": nil}).Error; err != nil {
				return err
			}
			if err := tx.Model(&ListingOperationBatch{}).Where("id=?", item.BatchID).Updates(map[string]any{"status": BatchStatusPending, "processing": gorm.Expr("CASE WHEN processing > 0 THEN processing - 1 ELSE 0 END"), "pending": gorm.Expr("pending + 1"), "completed_at": nil}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ResumeListingOperationBatches(ctx context.Context) error {
	var rows []ListingOperationBatch
	if err := s.DB.WithContext(ctx).Where("status IN ? AND pending > 0", []string{BatchStatusPending, BatchStatusRunning}).Find(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		if err := s.enqueueListingBatch(ctx, &rows[i]); err != nil {
			return err
		}
	}
	return nil
}
