package productflow

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BatchControlResult struct {
	Batch         CandidateAnalysisBatch `json:"batch"`
	AffectedItems int64                  `json:"affectedItems"`
}

func (s *Service) ListAnalysisBatches(ctx context.Context, tenantID int64, q ListQuery) (*PageResult[CandidateAnalysisBatch], error) {
	q = normalizePage(q)
	db := s.DB.WithContext(ctx).Model(&CandidateAnalysisBatch{}).Where("tenant_id = ?", tenantID)
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var rows []CandidateAnalysisBatch
	if err := db.Order("created_at DESC, id DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error; err != nil {
		return nil, err
	}
	return &PageResult[CandidateAnalysisBatch]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, nil
}

func (s *Service) ListAnalysisBatchItems(ctx context.Context, tenantID int64, batchID uuid.UUID, q ListQuery) (*PageResult[BatchItemDetail], error) {
	if _, err := s.GetAnalysisBatch(ctx, tenantID, batchID, false); err != nil {
		return nil, err
	}
	q = normalizePage(q)
	db := s.DB.WithContext(ctx).Model(&CandidateAnalysisBatchItem{}).Where("tenant_id = ? AND batch_id = ?", tenantID, batchID)
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	var total int64
	if err := db.Count(&total).Error; err != nil {
		return nil, err
	}
	var items []CandidateAnalysisBatchItem
	if err := db.Order("rank NULLS LAST, created_at, id").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&items).Error; err != nil {
		return nil, err
	}
	candidateIDs := make([]uuid.UUID, 0, len(items))
	analysisIDs := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		candidateIDs = append(candidateIDs, item.CandidateID)
		if item.AnalysisID != nil {
			analysisIDs = append(analysisIDs, *item.AnalysisID)
		}
	}
	var candidates []Candidate
	if len(candidateIDs) > 0 {
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, candidateIDs).Find(&candidates).Error; err != nil {
			return nil, err
		}
		if err := s.attachCandidateSources(ctx, tenantID, candidates); err != nil {
			return nil, err
		}
	}
	var analyses []CandidateAnalysis
	if len(analysisIDs) > 0 {
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, analysisIDs).Find(&analyses).Error; err != nil {
			return nil, err
		}
	}
	candidateByID := map[uuid.UUID]*Candidate{}
	for i := range candidates {
		candidateByID[candidates[i].ID] = &candidates[i]
	}
	analysisByID := map[uuid.UUID]*CandidateAnalysis{}
	for i := range analyses {
		analysisByID[analyses[i].ID] = &analyses[i]
	}
	rows := make([]BatchItemDetail, 0, len(items))
	for _, item := range items {
		row := BatchItemDetail{CandidateAnalysisBatchItem: item, Candidate: candidateByID[item.CandidateID]}
		if item.AnalysisID != nil {
			row.Analysis = analysisByID[*item.AnalysisID]
		}
		rows = append(rows, row)
	}
	return &PageResult[BatchItemDetail]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, nil
}

func (s *Service) PauseAnalysisBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*BatchControlResult, error) {
	return s.controlBatch(ctx, tenantID, id, "pause")
}

func (s *Service) ResumeAnalysisBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*BatchControlResult, error) {
	result, err := s.controlBatch(ctx, tenantID, id, "resume")
	if err != nil {
		return nil, err
	}
	if result.Batch.Pending > 0 {
		if err := s.enqueueBatch(ctx, id); err != nil {
			_ = s.DB.WithContext(ctx).Model(&CandidateAnalysisBatch{}).Where("tenant_id = ? AND id = ? AND status IN ?", tenantID, id, []string{BatchStatusPending, BatchStatusRunning}).Updates(map[string]any{"status": BatchStatusPaused, "control_version": gorm.Expr("control_version + 1")}).Error
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) CancelAnalysisBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*BatchControlResult, error) {
	return s.controlBatch(ctx, tenantID, id, "cancel")
}

func (s *Service) RetryFailedAnalysisBatch(ctx context.Context, tenantID int64, id uuid.UUID) (*BatchControlResult, error) {
	result, err := s.controlBatch(ctx, tenantID, id, "retry_failed")
	if err != nil {
		return nil, err
	}
	if result.AffectedItems > 0 {
		if err := s.enqueueBatch(ctx, id); err != nil {
			_ = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				items := tx.Model(&CandidateAnalysisBatchItem{}).Where("batch_id = ? AND status = ? AND attempts = 0 AND error_message = ''", id, BatchItemStatusPending).Updates(map[string]any{"status": BatchItemStatusFailed, "error_type": BatchErrorRetryable, "error_message": "queue wake-up failed; retry remains available", "completed_at": time.Now().UTC()})
				if items.Error != nil {
					return items.Error
				}
				return tx.Model(&CandidateAnalysisBatch{}).Where("tenant_id = ? AND id = ?", tenantID, id).Updates(map[string]any{"status": BatchStatusPartialFailed, "pending": gorm.Expr("pending - ?", items.RowsAffected), "failed": gorm.Expr("failed + ?", items.RowsAffected), "completed_at": time.Now().UTC(), "control_version": gorm.Expr("control_version + 1")}).Error
			})
			return nil, err
		}
	}
	return result, nil
}

func (s *Service) controlBatch(ctx context.Context, tenantID int64, id uuid.UUID, action string) (*BatchControlResult, error) {
	var result BatchControlResult
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch CandidateAnalysisBatch
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&batch).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrNotFound
			}
			return err
		}
		now := time.Now().UTC()
		switch action {
		case "pause":
			if batch.Status != BatchStatusPending && batch.Status != BatchStatusRunning {
				return fmt.Errorf("%w: batch cannot be paused from %s", ErrInvalidTransition, batch.Status)
			}
			status := BatchStatusPausing
			if batch.Processing == 0 {
				status = BatchStatusPaused
			}
			if err := tx.Model(&batch).Updates(map[string]any{"status": status, "control_version": gorm.Expr("control_version + 1")}).Error; err != nil {
				return err
			}
			batch.Status = status
		case "resume":
			if batch.Status != BatchStatusPaused && batch.Status != BatchStatusPausing {
				return fmt.Errorf("%w: batch cannot be resumed from %s", ErrInvalidTransition, batch.Status)
			}
			if batch.Processing > 0 {
				return fmt.Errorf("%w: wait for in-flight items before resume", ErrConflict)
			}
			status := BatchStatusRunning
			if batch.StartedAt == nil {
				status = BatchStatusPending
			}
			if err := tx.Model(&batch).Updates(map[string]any{"status": status, "completed_at": nil, "control_version": gorm.Expr("control_version + 1")}).Error; err != nil {
				return err
			}
			batch.Status = status
		case "cancel":
			if batch.Status == BatchStatusCancelled {
				result.Batch = batch
				return nil
			}
			if batch.Status == BatchStatusCompleted || batch.Status == BatchStatusPartialFailed || batch.Status == BatchStatusFailed {
				return fmt.Errorf("%w: terminal batch cannot be cancelled", ErrInvalidTransition)
			}
			items := tx.Model(&CandidateAnalysisBatchItem{}).Where("batch_id = ? AND status = ?", id, BatchItemStatusPending).Updates(map[string]any{"status": BatchItemStatusCancelled, "completed_at": now, "error_message": "cancelled before processing", "error_type": ""})
			if items.Error != nil {
				return items.Error
			}
			status := BatchStatusCancelling
			completedAt := any(nil)
			if batch.Processing == 0 {
				status = BatchStatusCancelled
				completedAt = now
			}
			if err := tx.Model(&batch).Updates(map[string]any{"status": status, "pending": gorm.Expr("pending - ?", items.RowsAffected), "completed_at": completedAt, "control_version": gorm.Expr("control_version + 1")}).Error; err != nil {
				return err
			}
			batch.Status = status
			batch.Pending -= int(items.RowsAffected)
			result.AffectedItems = items.RowsAffected
		case "retry_failed":
			if batch.Status != BatchStatusFailed && batch.Status != BatchStatusPartialFailed {
				return fmt.Errorf("%w: retry is only available for failed batches", ErrInvalidTransition)
			}
			items := tx.Model(&CandidateAnalysisBatchItem{}).Where("batch_id = ? AND status = ? AND error_type = ?", id, BatchItemStatusFailed, BatchErrorRetryable).Updates(map[string]any{"status": BatchItemStatusPending, "attempts": 0, "error_message": "", "error_type": "", "worker_id": "", "started_at": nil, "completed_at": nil, "analysis_id": nil, "rank": nil, "ranking_score": 0})
			if items.Error != nil {
				return items.Error
			}
			if items.RowsAffected == 0 {
				return fmt.Errorf("%w: no retryable failed items", ErrValidation)
			}
			if err := tx.Model(&batch).Updates(map[string]any{"status": BatchStatusRunning, "pending": gorm.Expr("pending + ?", items.RowsAffected), "failed": gorm.Expr("failed - ?", items.RowsAffected), "completed_at": nil, "control_version": gorm.Expr("control_version + 1")}).Error; err != nil {
				return err
			}
			batch.Status = BatchStatusRunning
			batch.Pending += int(items.RowsAffected)
			batch.Failed -= int(items.RowsAffected)
			result.AffectedItems = items.RowsAffected
		default:
			return fmt.Errorf("%w: unsupported batch action", ErrValidation)
		}
		batch.ControlVersion++
		result.Batch = batch
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}
