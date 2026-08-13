package productflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/rdb"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
	"github.com/trademind-ai/trademind/backend/internal/testing/safeenv"
)

// TestCandidateAnalysisBatchOperationsIntegration exercises the real PostgreSQL
// row-locking and Redis worker path. It is fail-closed and skips unless the two
// explicitly isolated TEST_* URLs are provided.
func TestCandidateAnalysisBatchOperationsIntegration(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" || os.Getenv("TEST_REDIS_URL") == "" {
		t.Skip("TEST_DATABASE_URL and TEST_REDIS_URL are required")
	}
	harness := postgrestest.Require(t)
	require.NoError(t, harness.DB.AutoMigrate(
		&SourceProduct{}, &Candidate{}, &CandidateAnalysis{}, &PricingProfile{}, &PricingProfileRevision{},
		&MarketSignalSnapshot{}, &CandidateAnalysisBatch{}, &CandidateAnalysisBatchItem{}, &ListingDraft{}, &ListingPerformanceSnapshot{}, &SelectionOutcomeEvaluation{},
		&product.Product{}, &product.ProductImage{}, &product.ProductSKU{},
	))

	redisCfg, ok, err := safeenv.TestRedisURLFromEnv()
	require.NoError(t, err)
	require.True(t, ok)
	options, err := goredis.ParseURL(redisCfg.URL)
	require.NoError(t, err)
	redisClient := goredis.NewClient(options)
	require.NoError(t, redisClient.Ping(context.Background()).Err())
	t.Cleanup(func() { _ = redisClient.Close() })

	queue := "test:candidate:analysis:operations"
	require.NoError(t, redisClient.Del(context.Background(), queue).Err())
	t.Cleanup(func() { _ = redisClient.Del(context.Background(), queue).Err() })
	svc := &Service{DB: harness.DB, Redis: &rdb.Client{Client: redisClient}, AnalysisQueueEnabled: true, AnalysisQueueName: queue, AnalysisConcurrency: 1, AnalysisMaxRetries: 2}

	ids := seedBatchCandidates(t, svc, 100)
	now := time.Now().UTC().Truncate(time.Second)
	_, err = svc.ImportMarketSignals(context.Background(), 0, ImportMarketSignalsBody{Items: []CreateMarketSignalBody{
		{CandidateID: ids[0], Platform: "xianyu", SignalType: SignalDemandScore, Value: 78, Unit: "score", Source: "user-market-research", Origin: SignalOriginManual, ConfidenceBPS: 8000, ObservedAt: &now},
		{CandidateID: ids[0], Platform: "xianyu", SignalType: SignalCompetitionScore, Value: 64, Unit: "score", Source: "user-authorized-csv", Origin: SignalOriginImport, ConfidenceBPS: 7500, ObservedAt: &now},
	}})
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	var workers sync.WaitGroup
	StartAnalysisWorker(ctx, &workers, slog.New(slog.NewTextHandler(os.Stderr, nil)), svc, nil)
	t.Cleanup(func() { cancel(); workers.Wait() })

	var delayMillis atomic.Int64
	delayMillis.Store(12)
	var failureMode atomic.Bool
	failures := map[string]struct{}{ids[0].String(): {}, ids[1].String(): {}, ids[2].String(): {}}
	svc.BeforeBatchAnalyze = func(_ context.Context, id uuid.UUID, _ int) error {
		if delay := delayMillis.Load(); delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
		if failureMode.Load() {
			if _, shouldFail := failures[id.String()]; shouldFail {
				return errors.New("temporary upstream dependency unavailable")
			}
		}
		return nil
	}

	start := time.Now()
	pauseBatch, err := svc.CreateAnalysisBatch(ctx, 0, nil, AnalyzeBatchBody{CandidateIDs: ids, Platform: "xianyu", AnalysisMode: "rules_only", TopN: 20})
	require.NoError(t, err)
	waitForBatch(t, svc, pauseBatch.ID, 15*time.Second, func(batch *BatchDetail) bool { return batch.Completed >= 2 })
	_, err = svc.PauseAnalysisBatch(ctx, 0, pauseBatch.ID)
	require.NoError(t, err)
	paused := waitForBatch(t, svc, pauseBatch.ID, 5*time.Second, func(batch *BatchDetail) bool { return batch.Status == BatchStatusPaused })
	completedAtPause := paused.Completed
	time.Sleep(150 * time.Millisecond)
	stable, err := svc.GetAnalysisBatch(ctx, 0, pauseBatch.ID, false)
	require.NoError(t, err)
	require.Equal(t, completedAtPause, stable.Completed, "paused worker must not claim new items")
	_, err = svc.ResumeAnalysisBatch(ctx, 0, pauseBatch.ID)
	require.NoError(t, err)
	completed := waitForBatch(t, svc, pauseBatch.ID, 20*time.Second, func(batch *BatchDetail) bool { return batch.Status == BatchStatusCompleted })
	require.Equal(t, 100, completed.Completed)
	require.Zero(t, completed.Failed)
	var marketAnalysis CandidateAnalysis
	require.NoError(t, svc.DB.Where("tenant_id = ? AND candidate_id = ?", 0, ids[0]).Order("analysis_version DESC").First(&marketAnalysis).Error)
	var dimensions map[string]struct {
		Score *int64 `json:"score"`
	}
	require.NoError(t, json.Unmarshal(marketAnalysis.ScoreBreakdown, &dimensions))
	require.NotNil(t, dimensions["demand"].Score)
	require.NotNil(t, dimensions["competition"].Score)
	require.NotEmpty(t, marketAnalysis.MarketSignalSnapshot)
	t.Logf("pause/resume 100 candidates completed in %s (paused after %d)", time.Since(start).Round(time.Millisecond), completedAtPause)

	cancelBatch, err := svc.CreateAnalysisBatch(ctx, 0, nil, AnalyzeBatchBody{CandidateIDs: ids, Platform: "taobao", AnalysisMode: "rules_only", TopN: 20})
	require.NoError(t, err)
	waitForBatch(t, svc, cancelBatch.ID, 10*time.Second, func(batch *BatchDetail) bool { return batch.Completed >= 1 })
	_, err = svc.CancelAnalysisBatch(ctx, 0, cancelBatch.ID)
	require.NoError(t, err)
	cancelled := waitForBatch(t, svc, cancelBatch.ID, 5*time.Second, func(batch *BatchDetail) bool { return batch.Status == BatchStatusCancelled })
	require.Less(t, cancelled.Completed, 100)
	var cancelledItems int64
	require.NoError(t, harness.DB.Model(&CandidateAnalysisBatchItem{}).Where("batch_id = ? AND status = ?", cancelBatch.ID, BatchItemStatusCancelled).Count(&cancelledItems).Error)
	require.Equal(t, int64(100-cancelled.Completed), cancelledItems)
	t.Logf("cancel stopped remaining work: completed=%d cancelled=%d", cancelled.Completed, cancelledItems)

	delayMillis.Store(0)
	failureMode.Store(true)
	retryBatch, err := svc.CreateAnalysisBatch(ctx, 0, nil, AnalyzeBatchBody{CandidateIDs: ids, Platform: "xianyu", AnalysisMode: "rules_only", TopN: 20})
	require.NoError(t, err)
	failed := waitForBatch(t, svc, retryBatch.ID, 20*time.Second, func(batch *BatchDetail) bool { return batch.Status == BatchStatusPartialFailed })
	require.Equal(t, 97, failed.Completed)
	require.Equal(t, 3, failed.Failed)
	failureMode.Store(false)
	retried, err := svc.RetryFailedAnalysisBatch(ctx, 0, retryBatch.ID)
	require.NoError(t, err)
	require.Equal(t, int64(3), retried.AffectedItems)
	recovered := waitForBatch(t, svc, retryBatch.ID, 30*time.Second, func(batch *BatchDetail) bool { return batch.Status == BatchStatusCompleted })
	require.Equal(t, 100, recovered.Completed)
	require.Zero(t, recovered.Failed)
	var analysisCount int64
	require.NoError(t, harness.DB.Model(&CandidateAnalysis{}).Where("candidate_id IN ?", ids).Count(&analysisCount).Error)
	require.Equal(t, int64(200+cancelled.Completed), analysisCount, "successful items should produce one immutable analysis per batch")
	t.Logf("three retryable failures recovered; total immutable analyses=%d", analysisCount)

	performanceItems := make([]ImportPerformanceItem, 0, 20)
	periodStart := now.Add(-24 * time.Hour)
	for i := 0; i < 20; i++ {
		approved, approveErr := svc.ApproveCandidate(ctx, 0, ids[i])
		require.NoError(t, approveErr)
		catalog := approved.Catalog.(product.Product)
		listing, _, listingErr := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu"})
		require.NoError(t, listingErr)
		views := int64(20 + i*10)
		orders := int64(i % 5)
		inquiries := orders + int64(i%3)
		refunds := int64(0)
		profit := fmt.Sprintf("%.2f", float64(orders*1200-int64(i%4)*500)/100)
		if i%5 == 4 {
			refunds = orders
			profit = "-5.00"
		}
		revenue := fmt.Sprintf("%.2f", float64(orders*3990)/100)
		performanceItems = append(performanceItems, ImportPerformanceItem{ListingDraftID: &listing.ID, CatalogProductID: catalog.ID, CandidateID: ids[i], Platform: "xianyu", Source: SignalOriginFixture, ObservedAt: now.Add(time.Duration(i) * time.Second), PeriodStart: periodStart, PeriodEnd: now, Views: &views, Inquiries: &inquiries, Orders: &orders, RefundCount: &refunds, GrossRevenue: &revenue, RealizedProfit: &profit})
	}
	performanceResult, err := svc.ImportPerformance(ctx, 0, ImportPerformanceBody{Items: performanceItems})
	require.NoError(t, err)
	require.Equal(t, 20, performanceResult.Imported)
	report, err := svc.SelectionPerformance(ctx, 0)
	require.NoError(t, err)
	require.Len(t, report.Evaluations, 20)
	require.NotEmpty(t, report.Groups)
	t.Logf("20 fixture-labelled performance snapshots evaluated across %d recommendation groups", len(report.Groups))
}

func seedBatchCandidates(t *testing.T, svc *Service, count int) []uuid.UUID {
	t.Helper()
	ids := make([]uuid.UUID, 0, count)
	for i := 0; i < count; i++ {
		price := float64(10 + i%20)
		freight := float64(i % 4)
		source, err := svc.CreateSource(context.Background(), 0, CreateSourceProductBody{
			SourcePlatform: "1688", SourceProductID: fmt.Sprintf("sprint4-test-%03d", i), SourceURL: fmt.Sprintf("https://detail.1688.com/offer/sprint4-test-%03d.html", i),
			SupplierName: "集成测试供应商", OriginalTitle: fmt.Sprintf("Sprint 4 测试商品 %03d", i), OriginalDescription: "仅用于隔离数据库任务运维测试",
			OriginalImages: json.RawMessage(`["https://example.invalid/test.jpg"]`), SKUData: json.RawMessage(`[{"skuCode":"DEFAULT","stock":100}]`), SourcePrice: &price, Freight: &freight,
		})
		require.NoError(t, err)
		candidate, _, err := svc.CreateCandidate(context.Background(), 0, source.ID)
		require.NoError(t, err)
		ids = append(ids, candidate.ID)
	}
	return ids
}

func waitForBatch(t *testing.T, svc *Service, id uuid.UUID, timeout time.Duration, done func(*BatchDetail) bool) *BatchDetail {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var last *BatchDetail
	for time.Now().Before(deadline) {
		batch, err := svc.GetAnalysisBatch(context.Background(), 0, id, false)
		require.NoError(t, err)
		last = batch
		if done(batch) {
			return batch
		}
		time.Sleep(10 * time.Millisecond)
	}
	if last != nil {
		t.Fatalf("batch %s did not reach expected state: status=%s pending=%d processing=%d completed=%d failed=%d", id, last.Status, last.Pending, last.Processing, last.Completed, last.Failed)
	}
	t.Fatalf("batch %s did not reach expected state", id)
	return nil
}
