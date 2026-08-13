package productflow

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/testing/postgrestest"
)

// TestSprint5RealDataCalibrationIntegration demonstrates the complete CSV ->
// analysis -> performance -> calibration -> config-version audit on isolated
// PostgreSQL. It skips unless the explicitly safe TEST_DATABASE_URL is present.
func TestSprint5RealDataCalibrationIntegration(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") == "" {
		t.Skip("TEST_DATABASE_URL is required")
	}
	harness := postgrestest.Require(t)
	require.NoError(t, harness.DB.AutoMigrate(&SourceProduct{}, &Candidate{}, &CandidateAnalysis{}, &PricingProfile{}, &PricingProfileRevision{}, &MarketSignalSnapshot{}, &MarketSignalProviderConfig{}, &ListingPerformanceSnapshot{}, &SelectionOutcomeEvaluation{}, &SelectionConfig{}, &product.Product{}, &product.ProductImage{}, &product.ProductSKU{}))
	svc := &Service{DB: harness.DB}
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	type fixture struct {
		candidate Candidate
		catalog   product.Product
		first     CandidateAnalysis
	}
	fixtures := make([]fixture, 0, 25)
	for i := 0; i < 25; i++ {
		source, err := svc.CreateSource(ctx, 0, testSourceBody(fmt.Sprintf("s5-real-%02d", i)))
		require.NoError(t, err)
		candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
		require.NoError(t, err)
		analysis, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
		require.NoError(t, err)
		approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
		require.NoError(t, err)
		catalog, ok := approved.Catalog.(product.Product)
		require.True(t, ok)
		fixtures = append(fixtures, fixture{candidate: *candidate, catalog: catalog, first: analysis.Analysis})
	}
	marketCSV := "商品ID,平台,信号,数值,时间,可信度,来源\n"
	performanceCSV := "候选ID,商品ID,平台,观察时间,开始,结束,浏览,咨询,订单,收入,利润,退款\n"
	for i, row := range fixtures {
		marketCSV += fmt.Sprintf("%s,xianyu,demand_score,%d,%s,80,user-authorized-export\n", row.candidate.ID, 70+i%20, now.AddDate(0, 0, -i).Format(time.RFC3339))
		orders := 0
		profit := "-2.00"
		if i%4 != 0 {
			orders = 1 + i%3
			profit = "12.00"
		}
		performanceCSV += fmt.Sprintf("%s,%s,xianyu,%s,%s,%s,%d,%d,%d,49.90,%s,%d\n", row.candidate.ID, row.catalog.ID, now.Format(time.RFC3339), now.Add(-24*time.Hour).Format(time.RFC3339), now.Format(time.RFC3339), 100+i*10, orders, orders, ""+profit, i%7)
	}
	marketBody := CSVImportRequest{CSV: marketCSV, Mapping: map[string]string{"商品ID": "candidate_id", "平台": "platform", "信号": "signal_type", "数值": "value", "时间": "observed_at", "可信度": "confidence", "来源": "source"}}
	marketPreview, err := svc.PreviewMarketSignalCSV(ctx, 0, marketBody)
	require.NoError(t, err)
	require.Equal(t, 25, marketPreview.ValidRows)
	marketResult, err := svc.ImportMappedMarketSignalCSV(ctx, 0, marketBody)
	require.NoError(t, err)
	require.Equal(t, 25, marketResult.Imported)
	performanceBody := CSVImportRequest{CSV: performanceCSV, Mapping: map[string]string{"候选ID": "candidate_id", "商品ID": "catalog_product_id", "平台": "platform", "观察时间": "observed_at", "开始": "period_start", "结束": "period_end", "浏览": "views", "咨询": "inquiries", "订单": "orders", "收入": "gross_revenue", "利润": "realized_profit", "退款": "refund_count"}}
	performancePreview, err := svc.PreviewPerformanceCSV(ctx, 0, performanceBody)
	require.NoError(t, err)
	require.Equal(t, 25, performancePreview.ValidRows)
	performanceResult, err := svc.ImportMappedPerformanceCSV(ctx, 0, performanceBody)
	require.NoError(t, err)
	require.Equal(t, 25, performanceResult.Imported)
	report, err := svc.CalibrationReport(ctx, 0, false)
	require.NoError(t, err)
	require.Equal(t, int64(25), report.SampleCount)
	require.Equal(t, "low", report.Readiness.Level)
	require.NotEmpty(t, report.Suggestions)
	require.True(t, report.Suggestions[0].SuggestionOnly)
	cfg := defaultSelectionConfig()
	cfg.Weights["profit"] = 25
	cfg.Weights["data_quality"] = 25
	input := SelectionConfigInput{Version: "selection-s5-integration", Weights: cfg.Weights, Thresholds: SelectionThresholds{StrongRecommend: cfg.StrongRecommendThreshold, Recommend: cfg.RecommendThreshold, Watch: cfg.WatchThreshold, MinimumMarginBPS: cfg.MinimumMarginBPS, MinimumProfit: int64(cfg.MinimumProfit), StaleAfterDays: cfg.StaleAfterDays}, Blockers: SelectionBlockers{SensitiveKeywords: cfg.SensitiveKeywords, BlockerKeywords: cfg.BlockerKeywords}}
	draft, err := svc.CreateSelectionConfigDraft(ctx, 0, input, nil)
	require.NoError(t, err)
	_, err = svc.ReviewSelectionConfig(ctx, 0, draft.ID)
	require.NoError(t, err)
	_, err = svc.ActivateSelectionConfig(ctx, 0, draft.ID)
	require.NoError(t, err)
	newSource, err := svc.CreateSource(ctx, 0, testSourceBody("s5-after-activate"))
	require.NoError(t, err)
	newCandidate, _, err := svc.CreateCandidate(ctx, 0, newSource.ID)
	require.NoError(t, err)
	next, err := svc.AnalyzeCandidate(ctx, 0, newCandidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
	require.NoError(t, err)
	require.Equal(t, "selection-s5-integration", next.Analysis.SelectionConfigVersion)
	require.Equal(t, DefaultSelectionVersion, fixtures[0].first.SelectionConfigVersion)
	t.Logf("market CSV imported=%d; performance CSV imported=%d; calibration samples=%d readiness=%s; suggestion=%q", marketResult.Imported, performanceResult.Imported, report.SampleCount, report.Readiness.Level, report.Suggestions[0].Message)
}
