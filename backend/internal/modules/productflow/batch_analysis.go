package productflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/rankingengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type analysisQueueMessage struct {
	BatchID string `json:"batchId"`
}

type BatchDetail struct {
	CandidateAnalysisBatch
	Items []CandidateAnalysisBatchItem `json:"items,omitempty"`
}

type BatchItemDetail struct {
	CandidateAnalysisBatchItem
	Candidate *Candidate         `json:"candidate,omitempty"`
	Analysis  *CandidateAnalysis `json:"analysis,omitempty"`
}

type RecommendationRow struct {
	CandidateAnalysisBatchItem
	Candidate Candidate         `json:"candidate"`
	Analysis  CandidateAnalysis `json:"analysis"`
}

func (s *Service) batchQueueName() string {
	if strings.TrimSpace(s.AnalysisQueueName) == "" {
		return "candidate:analysis:batches"
	}
	return strings.TrimSpace(s.AnalysisQueueName)
}

func (s *Service) enqueueBatch(ctx context.Context, id uuid.UUID) error {
	if !s.AnalysisQueueEnabled || s.Redis == nil || s.Redis.Client == nil {
		return fmt.Errorf("%w: candidate analysis queue unavailable", ErrConflict)
	}
	payload, _ := json.Marshal(analysisQueueMessage{BatchID: id.String()})
	workers := s.AnalysisConcurrency
	if workers < 1 {
		workers = 1
	}
	if workers > 32 {
		workers = 32
	}
	values := make([]any, workers)
	for i := range values {
		values[i] = string(payload)
	}
	return s.Redis.LPush(ctx, s.batchQueueName(), values...).Err()
}

func (s *Service) CreateAnalysisBatch(ctx context.Context, tenantID int64, actorID *uuid.UUID, body AnalyzeBatchBody) (*CandidateAnalysisBatch, error) {
	if !s.AnalysisQueueEnabled || s.Redis == nil || s.Redis.Client == nil {
		return nil, fmt.Errorf("%w: candidate analysis queue unavailable", ErrConflict)
	}
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	if platform == "" {
		platform = "xianyu"
	}
	if !SupportedDraftPlatform(platform) {
		return nil, fmt.Errorf("%w: unsupported platform", ErrValidation)
	}
	mode := strings.ToLower(strings.TrimSpace(body.AnalysisMode))
	if mode == "" {
		mode = "rules_only"
	}
	if mode != "rules_only" && mode != "ai_explanation" {
		return nil, fmt.Errorf("%w: unsupported analysis mode", ErrValidation)
	}
	if body.TopN == 0 {
		body.TopN = 20
	}
	if body.TopN < 1 || body.TopN > 50 {
		return nil, fmt.Errorf("%w: topN must be 1..50", ErrValidation)
	}
	minimumProfit, err := pricingengine.ParseMoney(body.MinimumProfit)
	if err != nil {
		return nil, fmt.Errorf("%w: minimumProfit", ErrValidation)
	}
	if body.PricingProfileID != nil {
		if _, err := s.GetPricingProfile(ctx, tenantID, *body.PricingProfileID); err != nil {
			return nil, err
		}
	}
	query := s.DB.WithContext(ctx).Model(&Candidate{}).Where("candidates.tenant_id = ? AND candidates.status <> ?", tenantID, CandidateStatusApproved)
	if len(body.CandidateIDs) > 0 {
		unique := make([]uuid.UUID, 0, len(body.CandidateIDs))
		seen := map[uuid.UUID]struct{}{}
		for _, id := range body.CandidateIDs {
			if id == uuid.Nil {
				continue
			}
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
		query = query.Where("candidates.id IN ?", unique)
	} else {
		if status := strings.TrimSpace(body.Filters.Status); status != "" {
			query = query.Where("candidates.status = ?", status)
		}
		if p := strings.TrimSpace(body.Filters.SourcePlatform); p != "" {
			query = query.Joins("JOIN source_products batch_source ON batch_source.id = candidates.source_product_id").Where("batch_source.source_platform = ?", strings.ToLower(p))
		}
		if body.Filters.UnanalyzedOnly {
			query = query.Where("candidates.analysis_version = 0")
		}
	}
	limit := body.Filters.Limit
	if limit <= 0 {
		limit = 500
	}
	if limit > 1000 {
		limit = 1000
	}
	var ids []uuid.UUID
	if err := query.Order("candidates.created_at ASC").Limit(limit).Pluck("candidates.id", &ids).Error; err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: no eligible candidates", ErrValidation)
	}
	filterJSON, _ := json.Marshal(body.Filters)
	exclude := true
	if body.ExcludeBlocked != nil {
		exclude = *body.ExcludeBlocked
	}
	batch := CandidateAnalysisBatch{TenantID: tenantID, Status: BatchStatusPending, Total: len(ids), Pending: len(ids), Filters: datatypes.JSON(filterJSON), AnalysisMode: mode, Platform: platform, PricingProfileID: body.PricingProfileID, MinimumScore: body.MinimumScore, MinimumMarginBPS: body.MinimumMarginBPS, MinimumProfit: int64(minimumProfit), MinimumConfidence: body.MinimumConfidence, ExcludeBlocked: exclude, TopN: body.TopN, CreatedBy: actorID, ActorType: "user"}
	maxAttempts := s.AnalysisMaxRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 3
	}
	if maxAttempts > 10 {
		maxAttempts = 10
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&batch).Error; err != nil {
			return err
		}
		items := make([]CandidateAnalysisBatchItem, 0, len(ids))
		for _, id := range ids {
			items = append(items, CandidateAnalysisBatchItem{TenantID: tenantID, BatchID: batch.ID, CandidateID: id, Status: BatchItemStatusPending, MaxAttempts: maxAttempts})
		}
		return tx.CreateInBatches(items, 100).Error
	})
	if err != nil {
		return nil, err
	}
	if err := s.enqueueBatch(ctx, batch.ID); err != nil {
		now := time.Now().UTC()
		_ = s.DB.Model(&batch).Updates(map[string]any{"status": BatchStatusFailed, "completed_at": now}).Error
		return nil, err
	}
	return &batch, nil
}

func (s *Service) GetAnalysisBatch(ctx context.Context, tenantID int64, id uuid.UUID, includeItems bool) (*BatchDetail, error) {
	var batch CandidateAnalysisBatch
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&batch).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	out := &BatchDetail{CandidateAnalysisBatch: batch}
	if includeItems {
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND batch_id = ?", tenantID, id).Order("rank NULLS LAST, created_at").Find(&out.Items).Error; err != nil {
			return nil, err
		}
	}
	return out, nil
}

func (s *Service) claimBatchItem(ctx context.Context, batchID uuid.UUID, workerID string) (*CandidateAnalysisBatchItem, *CandidateAnalysisBatch, error) {
	var item CandidateAnalysisBatchItem
	var batch CandidateAnalysisBatch
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", batchID).First(&batch).Error; err != nil {
			return err
		}
		if batch.Status != BatchStatusPending && batch.Status != BatchStatusRunning {
			return gorm.ErrRecordNotFound
		}
		q := tx.Where("batch_id = ? AND status = ?", batchID, BatchItemStatusPending).Order("created_at ASC")
		if tx.Dialector.Name() == "postgres" {
			q = q.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"})
		}
		if err := q.First(&item).Error; err != nil {
			return err
		}
		now := time.Now().UTC()
		if batch.Status == BatchStatusPending {
			if err := tx.Model(&batch).Updates(map[string]any{"status": BatchStatusRunning, "started_at": now}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&item).Updates(map[string]any{"status": BatchItemStatusProcessing, "attempts": gorm.Expr("attempts + 1"), "worker_id": workerID, "started_at": now, "error_message": ""}).Error; err != nil {
			return err
		}
		if err := tx.Model(&batch).Updates(map[string]any{"pending": gorm.Expr("pending - 1"), "processing": gorm.Expr("processing + 1")}).Error; err != nil {
			return err
		}
		item.Attempts++
		item.Status = BatchItemStatusProcessing
		return nil
	})
	return &item, &batch, err
}

func (s *Service) processBatchItem(ctx context.Context, batch *CandidateAnalysisBatch, item *CandidateAnalysisBatchItem) error {
	if s.BeforeBatchAnalyze != nil {
		if err := s.BeforeBatchAnalyze(ctx, item.CandidateID, item.Attempts); err != nil {
			return s.finishBatchItemError(ctx, batch, item, err)
		}
	}
	body := AnalyzeCandidateBody{AnalysisMode: "rules_only", Platform: batch.Platform, PricingProfileID: batch.PricingProfileID}
	out, err := s.AnalyzeCandidate(ctx, batch.TenantID, item.CandidateID, body)
	now := time.Now().UTC()
	if err != nil {
		return s.finishBatchItemError(ctx, batch, item, err)
	}
	risk := int64(0)
	if d, ok := out.Score.Dimensions["risk"]; ok && d.Score != nil {
		risk = *d.Score
	}
	ranked := rankingengine.Score(rankingengine.Input{OverallScore: out.Score.OverallScore, BaseQualityScore: out.Score.BaseQualityScore, MarketOpportunityScore: out.Score.MarketOpportunityScore, EvidenceCoverageBPS: out.Score.EvidenceCoverageBPS, Confidence: out.Score.ConfidenceScore, Profit: out.Cost.EstimatedProfit, MarginBPS: out.Cost.EstimatedMarginBPS, RiskScore: risk, Blocked: len(out.Score.Blockers) > 0}, rankingengine.DefaultConfig())
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Model(item).Updates(map[string]any{"status": BatchItemStatusCompleted, "analysis_id": out.Analysis.ID, "ranking_score": ranked.Score, "completed_at": now}).Error; e != nil {
			return e
		}
		return tx.Model(batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1"), "completed": gorm.Expr("completed + 1")}).Error
	})
}

func (s *Service) finishBatchItemError(ctx context.Context, batch *CandidateAnalysisBatch, item *CandidateAnalysisBatchItem, err error) error {
	now := time.Now().UTC()
	errorType := classifyBatchError(err)
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if errorType == BatchErrorRetryable && item.Attempts < item.MaxAttempts {
			if e := tx.Model(item).Updates(map[string]any{"status": BatchItemStatusPending, "error_message": truncateError(err), "error_type": errorType, "completed_at": nil}).Error; e != nil {
				return e
			}
			return tx.Model(batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1"), "pending": gorm.Expr("pending + 1")}).Error
		}
		if e := tx.Model(item).Updates(map[string]any{"status": BatchItemStatusFailed, "error_message": truncateError(err), "error_type": errorType, "completed_at": now}).Error; e != nil {
			return e
		}
		return tx.Model(batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1"), "failed": gorm.Expr("failed + 1")}).Error
	})
}

func classifyBatchError(err error) string {
	if errors.Is(err, ErrValidation) || errors.Is(err, ErrNotFound) || errors.Is(err, ErrInvalidTransition) {
		return BatchErrorNonRetryable
	}
	return BatchErrorRetryable
}

func truncateError(err error) string {
	value := err.Error()
	if len(value) > 1000 {
		return value[:1000]
	}
	return value
}

func (s *Service) finalizeBatch(ctx context.Context, batchID uuid.UUID) error {
	var batch CandidateAnalysisBatch
	if err := s.DB.WithContext(ctx).First(&batch, "id = ?", batchID).Error; err != nil {
		return err
	}
	if batch.Status == BatchStatusPausing && batch.Processing == 0 {
		return s.DB.WithContext(ctx).Model(&batch).Updates(map[string]any{"status": BatchStatusPaused, "control_version": gorm.Expr("control_version + 1")}).Error
	}
	if batch.Status == BatchStatusCancelling && batch.Processing == 0 {
		now := time.Now().UTC()
		return s.DB.WithContext(ctx).Model(&batch).Updates(map[string]any{"status": BatchStatusCancelled, "completed_at": now, "control_version": gorm.Expr("control_version + 1")}).Error
	}
	if batch.Pending > 0 || batch.Processing > 0 {
		return nil
	}
	if batch.Status == BatchStatusPaused || batch.Status == BatchStatusCancelled {
		return nil
	}
	var items []CandidateAnalysisBatchItem
	if err := s.DB.WithContext(ctx).Where("batch_id = ? AND status = ?", batchID, BatchItemStatusCompleted).Find(&items).Error; err != nil {
		return err
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].RankingScore == items[j].RankingScore {
			return items[i].CandidateID.String() < items[j].CandidateID.String()
		}
		return items[i].RankingScore > items[j].RankingScore
	})
	if batch.AnalysisMode == "ai_explanation" && !batch.AIExplanationDone && s.AIExplanationTopN > 0 {
		claimed := s.DB.WithContext(ctx).Model(&CandidateAnalysisBatch{}).Where("id = ? AND ai_explanation_done = ?", batchID, false).Update("ai_explanation_done", true)
		if claimed.Error != nil {
			return claimed.Error
		}
		if claimed.RowsAffected > 0 {
			limit := s.AIExplanationTopN
			if limit > batch.TopN {
				limit = batch.TopN
			}
			if limit > len(items) {
				limit = len(items)
			}
			for i := 0; i < limit; i++ {
				out, explainErr := s.AnalyzeCandidate(ctx, batch.TenantID, items[i].CandidateID, AnalyzeCandidateBody{AnalysisMode: "ai_explanation", Platform: batch.Platform, PricingProfileID: batch.PricingProfileID})
				if explainErr == nil {
					items[i].AnalysisID = &out.Analysis.ID
					_ = s.DB.WithContext(ctx).Model(&items[i]).Update("analysis_id", out.Analysis.ID).Error
				}
			}
		}
	}
	now := time.Now().UTC()
	status := BatchStatusCompleted
	if batch.Failed > 0 && batch.Completed > 0 {
		status = BatchStatusPartialFailed
	}
	if batch.Failed > 0 && batch.Completed == 0 {
		status = BatchStatusFailed
	}
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for i := range items {
			rank := i + 1
			if err := tx.Model(&items[i]).Update("rank", rank).Error; err != nil {
				return err
			}
		}
		updated := tx.Model(&CandidateAnalysisBatch{}).
			Where("id = ? AND control_version = ? AND pending = 0 AND processing = 0 AND status IN ?", batch.ID, batch.ControlVersion, []string{BatchStatusPending, BatchStatusRunning}).
			Updates(map[string]any{"status": status, "completed_at": now})
		if updated.Error != nil {
			return updated.Error
		}
		if updated.RowsAffected == 0 {
			// A pause/resume/cancel/retry command won the race. Its newer
			// control version is authoritative and must not be overwritten.
			return nil
		}
		return nil
	})
}

func (s *Service) RunAnalysisBatch(ctx context.Context, batchID uuid.UUID, workerID string) {
	for {
		item, batch, err := s.claimBatchItem(ctx, batchID, workerID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = s.finalizeBatch(ctx, batchID)
			return
		}
		if err != nil {
			return
		}
		_ = s.processBatchItem(ctx, batch, item)
		_ = s.finalizeBatch(ctx, batchID)
	}
}

// RecoverStaleAnalysisItems makes process crashes retryable while preserving the per-item max-attempt bound.
func (s *Service) RecoverStaleAnalysisItems(ctx context.Context, staleBefore time.Time) error {
	return s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var items []CandidateAnalysisBatchItem
		if err := tx.Where("status = ? AND started_at < ?", BatchItemStatusProcessing, staleBefore).Find(&items).Error; err != nil {
			return err
		}
		for _, item := range items {
			var batch CandidateAnalysisBatch
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", item.BatchID).First(&batch).Error; err != nil {
				return err
			}
			if batch.Status == BatchStatusCancelling || batch.Status == BatchStatusCancelled {
				if err := tx.Model(&item).Updates(map[string]any{"status": BatchItemStatusCancelled, "worker_id": "", "error_message": "cancelled after worker interruption", "error_type": "", "completed_at": time.Now().UTC()}).Error; err != nil {
					return err
				}
				if err := tx.Model(&batch).Updates(map[string]any{"processing": gorm.Expr("processing - 1")}).Error; err != nil {
					return err
				}
				continue
			}
			status := BatchItemStatusPending
			batchUpdates := map[string]any{"processing": gorm.Expr("processing - 1"), "pending": gorm.Expr("pending + 1")}
			if item.Attempts >= item.MaxAttempts {
				status = BatchItemStatusFailed
				batchUpdates = map[string]any{"processing": gorm.Expr("processing - 1"), "failed": gorm.Expr("failed + 1")}
			}
			if err := tx.Model(&item).Updates(map[string]any{"status": status, "worker_id": "", "error_message": "worker interrupted; recovered", "error_type": BatchErrorRetryable, "completed_at": nil}).Error; err != nil {
				return err
			}
			if err := tx.Model(&CandidateAnalysisBatch{}).Where("id = ?", item.BatchID).Updates(batchUpdates).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Service) ResumeAnalysisBatches(ctx context.Context) error {
	var ids []uuid.UUID
	if err := s.DB.WithContext(ctx).Model(&CandidateAnalysisBatch{}).Where("status IN ? AND pending > 0", []string{BatchStatusPending, BatchStatusRunning}).Pluck("id", &ids).Error; err != nil {
		return err
	}
	for _, id := range ids {
		if err := s.enqueueBatch(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) TopRecommendations(ctx context.Context, tenantID int64, batchID uuid.UUID, limit int) ([]RecommendationRow, error) {
	batch, err := s.GetAnalysisBatch(ctx, tenantID, batchID, false)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = batch.TopN
	}
	if limit > 50 {
		limit = 50
	}
	var items []CandidateAnalysisBatchItem
	q := s.DB.WithContext(ctx).Table("candidate_analysis_batch_items items").Select("items.*").Joins("JOIN candidates c ON c.id = items.candidate_id").Joins("JOIN candidate_analyses a ON a.id = items.analysis_id").Where("items.tenant_id = ? AND items.batch_id = ? AND items.status = ? AND a.overall_score >= ? AND a.confidence_score >= ?", tenantID, batchID, BatchItemStatusCompleted, batch.MinimumScore, batch.MinimumConfidence).Where("c.estimated_margin >= ? AND c.estimated_profit >= ?", float64(batch.MinimumMarginBPS)/10000, float64(batch.MinimumProfit)/100)
	if batch.ExcludeBlocked {
		q = q.Where("items.ranking_score > 0")
	}
	if err := q.Order("items.rank ASC").Limit(limit).Scan(&items).Error; err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []RecommendationRow{}, nil
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
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, candidateIDs).Find(&candidates).Error; err != nil {
		return nil, err
	}
	if err := s.attachCandidateSources(ctx, tenantID, candidates); err != nil {
		return nil, err
	}
	var analyses []CandidateAnalysis
	if len(analysisIDs) > 0 {
		if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, analysisIDs).Find(&analyses).Error; err != nil {
			return nil, err
		}
	}
	byCandidate := map[uuid.UUID]Candidate{}
	for _, row := range candidates {
		byCandidate[row.ID] = row
	}
	byAnalysis := map[uuid.UUID]CandidateAnalysis{}
	for _, row := range analyses {
		byAnalysis[row.ID] = row
	}
	rows := make([]RecommendationRow, 0, len(items))
	for _, item := range items {
		row := RecommendationRow{CandidateAnalysisBatchItem: item, Candidate: byCandidate[item.CandidateID]}
		if item.AnalysisID != nil {
			row.Analysis = byAnalysis[*item.AnalysisID]
		}
		rows = append(rows, row)
	}
	return rows, nil
}
