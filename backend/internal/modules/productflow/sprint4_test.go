package productflow

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func TestBatchPauseResumeCancelStateMachine(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	batch := CandidateAnalysisBatch{TenantID: 0, Status: BatchStatusRunning, Total: 2, Pending: 1, Processing: 1, AnalysisMode: "rules_only", Platform: "xianyu", TopN: 20}
	require.NoError(t, svc.DB.Create(&batch).Error)
	pending := CandidateAnalysisBatchItem{TenantID: 0, BatchID: batch.ID, CandidateID: uuid.New(), Status: BatchItemStatusPending, MaxAttempts: 3}
	processing := CandidateAnalysisBatchItem{TenantID: 0, BatchID: batch.ID, CandidateID: uuid.New(), Status: BatchItemStatusProcessing, Attempts: 1, MaxAttempts: 3}
	require.NoError(t, svc.DB.Create(&[]CandidateAnalysisBatchItem{pending, processing}).Error)
	paused, err := svc.PauseAnalysisBatch(ctx, 0, batch.ID)
	require.NoError(t, err)
	require.Equal(t, BatchStatusPausing, paused.Batch.Status)
	_, _, err = svc.claimBatchItem(ctx, batch.ID, "another-worker")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.NoError(t, svc.DB.Model(&CandidateAnalysisBatchItem{}).Where("id = ?", processing.ID).Updates(map[string]any{"status": BatchItemStatusCompleted, "completed_at": time.Now().UTC()}).Error)
	require.NoError(t, svc.DB.Model(&CandidateAnalysisBatch{}).Where("id = ?", batch.ID).Updates(map[string]any{"processing": 0, "completed": 1}).Error)
	require.NoError(t, svc.finalizeBatch(ctx, batch.ID))
	detail, err := svc.GetAnalysisBatch(ctx, 0, batch.ID, false)
	require.NoError(t, err)
	require.Equal(t, BatchStatusPaused, detail.Status)
	svc.AnalysisQueueEnabled = false
	_, err = svc.ResumeAnalysisBatch(ctx, 0, batch.ID)
	require.Error(t, err)
	// A failed wake-up returns the batch to paused so the UI never claims it is running.
	detail, _ = svc.GetAnalysisBatch(ctx, 0, batch.ID, false)
	require.Equal(t, BatchStatusPaused, detail.Status)
	cancelled, err := svc.CancelAnalysisBatch(ctx, 0, batch.ID)
	require.NoError(t, err)
	require.Equal(t, BatchStatusCancelled, cancelled.Batch.Status)
	require.EqualValues(t, 1, cancelled.AffectedItems)
}

func TestRetryFailedOnlyResetsRetryableItems(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	batch := CandidateAnalysisBatch{TenantID: 0, Status: BatchStatusPartialFailed, Total: 3, Completed: 1, Failed: 2, AnalysisMode: "rules_only", Platform: "xianyu", TopN: 20}
	require.NoError(t, svc.DB.Create(&batch).Error)
	items := []CandidateAnalysisBatchItem{{TenantID: 0, BatchID: batch.ID, CandidateID: uuid.New(), Status: BatchItemStatusFailed, Attempts: 3, MaxAttempts: 3, ErrorType: BatchErrorRetryable}, {TenantID: 0, BatchID: batch.ID, CandidateID: uuid.New(), Status: BatchItemStatusFailed, Attempts: 1, MaxAttempts: 3, ErrorType: BatchErrorNonRetryable}}
	require.NoError(t, svc.DB.Create(&items).Error)
	svc.AnalysisQueueEnabled = false
	_, err := svc.RetryFailedAnalysisBatch(ctx, 0, batch.ID)
	require.Error(t, err)
	var retryable, nonRetryable CandidateAnalysisBatchItem
	require.NoError(t, svc.DB.First(&retryable, "id = ?", items[0].ID).Error)
	require.NoError(t, svc.DB.First(&nonRetryable, "id = ?", items[1].ID).Error)
	require.Equal(t, BatchItemStatusFailed, retryable.Status)
	require.Equal(t, BatchErrorRetryable, retryable.ErrorType)
	require.Equal(t, BatchItemStatusFailed, nonRetryable.Status)
}

func TestMarketSignalFreshnessAggregationDedupeAndFixtureExclusion(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("signals-v2"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	stale := now.Add(-5 * 24 * time.Hour)
	expired := now.Add(-20 * 24 * time.Hour)
	freshBody := CreateMarketSignalBody{Platform: "xianyu", SignalType: SignalDemandScore, Value: 80, Source: "manual-research", Origin: SignalOriginManual, ConfidenceBPS: 9000, ObservedAt: &now}
	fresh, err := svc.CreateMarketSignal(ctx, 0, candidate.ID, freshBody)
	require.NoError(t, err)
	require.Equal(t, int64(7000), fresh.EffectiveConfidence)
	require.True(t, fresh.Participates)
	_, err = svc.CreateMarketSignal(ctx, 0, candidate.ID, freshBody)
	require.ErrorIs(t, err, ErrConflict)
	_, err = svc.CreateMarketSignal(ctx, 0, candidate.ID, CreateMarketSignalBody{Platform: "xianyu", SignalType: SignalDemandScore, Value: 20, Source: "old-import", Origin: SignalOriginImport, ConfidenceBPS: 6500, ObservedAt: &stale})
	require.NoError(t, err)
	fixture, err := svc.CreateMarketSignal(ctx, 0, candidate.ID, CreateMarketSignalBody{Platform: "xianyu", SignalType: SignalCompetitionScore, Value: 99, Source: "test-only", Origin: SignalOriginFixture, ConfidenceBPS: 10000, ObservedAt: &now})
	require.NoError(t, err)
	require.False(t, fixture.Participates)
	expiredSignal, err := svc.CreateMarketSignal(ctx, 0, candidate.ID, CreateMarketSignalBody{Platform: "xianyu", SignalType: SignalCompetitionScore, Value: 10, Source: "expired", Origin: SignalOriginManual, ConfidenceBPS: 7000, ObservedAt: &expired})
	require.NoError(t, err)
	require.Equal(t, "expired", expiredSignal.Freshness)
	aggregated, _, err := svc.AggregateMarketSignals(ctx, 0, candidate.ID, "xianyu")
	require.NoError(t, err)
	require.NotNil(t, aggregated.Demand)
	require.Nil(t, aggregated.Competition)
	require.Equal(t, int64(5000), aggregated.CoverageBPS)
	out, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{})
	require.NoError(t, err)
	require.NotNil(t, out.Score.Dimensions["demand"].Score)
	require.Nil(t, out.Score.Dimensions["competition"].Score)
}

func TestMarketSignalProviderStatusDoesNotPretendRealProvider(t *testing.T) {
	svc := newTestService(t)
	statuses, err := svc.MarketSignalProviderStatuses(context.Background(), 0)
	require.NoError(t, err)
	require.Len(t, statuses, 3)
	var official MarketSignalProviderStatus
	for _, status := range statuses {
		if status.ID == "official-placeholder" {
			official = status
		}
	}
	require.False(t, official.Configured)
	require.Equal(t, "not_configured", official.Health.Status)
}

func TestPerformanceImportDedupeAndPredictionOutcome(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("performance"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	_, err = svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{})
	require.NoError(t, err)
	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	catalog := approved.Catalog.(product.Product)
	listing, _, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu"})
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	start := now.Add(-24 * time.Hour)
	views, inquiries, orders, refunds := int64(120), int64(12), int64(3), int64(1)
	revenue, profit := "119.70", "38.10"
	item := ImportPerformanceItem{ListingDraftID: &listing.ID, CatalogProductID: catalog.ID, CandidateID: candidate.ID, Platform: "xianyu", Source: SignalOriginFixture, ObservedAt: now, PeriodStart: start, PeriodEnd: now, Views: &views, Inquiries: &inquiries, Orders: &orders, RefundCount: &refunds, GrossRevenue: &revenue, RealizedProfit: &profit}
	result, err := svc.ImportPerformance(ctx, 0, ImportPerformanceBody{Items: []ImportPerformanceItem{item, item}})
	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Equal(t, 1, result.Duplicates)
	summary, err := svc.PerformanceSummary(ctx, 0)
	require.NoError(t, err)
	require.Equal(t, int64(120), summary.Views)
	require.Equal(t, int64(3810), summary.RealizedProfit)
	report, err := svc.SelectionPerformance(ctx, 0)
	require.NoError(t, err)
	require.Len(t, report.Evaluations, 1)
	require.Equal(t, int64(3), report.Evaluations[0].Orders)
	require.Greater(t, report.Evaluations[0].ActualMarginBPS, int64(0))
}

func TestPricingProfileCreatesImmutableRevisions(t *testing.T) {
	svc := newTestService(t)
	enabled := true
	profile, err := svc.CreatePricingProfile(context.Background(), 0, PricingProfileInput{Name: "revision-test", Platform: "xianyu", Currency: "CNY", Enabled: &enabled})
	require.NoError(t, err)
	require.Equal(t, 1, profile.Version)
	profile, err = svc.UpdatePricingProfile(context.Background(), 0, profile.ID, PricingProfileInput{Name: profile.Name, Platform: profile.Platform, Currency: profile.Currency, PlatformFeeBPS: 100, Enabled: &enabled})
	require.NoError(t, err)
	require.Equal(t, 2, profile.Version)
	var revisions []PricingProfileRevision
	require.NoError(t, svc.DB.Where("profile_id = ?", profile.ID).Order("version").Find(&revisions).Error)
	require.Len(t, revisions, 2)
	require.NotEqual(t, string(revisions[0].Snapshot), string(revisions[1].Snapshot))
}

func TestClassifyBatchError(t *testing.T) {
	require.Equal(t, BatchErrorNonRetryable, classifyBatchError(fmt.Errorf("wrapped: %w", ErrValidation)))
	require.Equal(t, BatchErrorRetryable, classifyBatchError(errors.New("temporary redis interruption")))
}
