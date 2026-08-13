package productflow

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/calibration"
)

func TestProviderCredentialNeverLeaksAndHealthDegrades(t *testing.T) {
	svc := newTestService(t)
	t.Setenv("TRADEMIND_TEST_PROVIDER_TOKEN", "super-secret-abcd")
	view, err := svc.SaveMarketProviderConfig(context.Background(), 0, nil, MarketProviderConfigInput{ProviderType: "official-placeholder", Name: "官方占位", Platform: "xianyu", SourceLevel: SignalOriginOfficial, Config: json.RawMessage(`{"endpoint":"https://example.invalid"}`), CredentialReference: "env:TRADEMIND_TEST_PROVIDER_TOKEN"})
	require.NoError(t, err)
	require.True(t, view.CredentialConfigured)
	payload, err := json.Marshal(view)
	require.NoError(t, err)
	require.NotContains(t, string(payload), "super-secret-abcd")
	require.NotContains(t, string(payload), "TRADEMIND_TEST_PROVIDER_TOKEN")

	_, err = svc.SaveMarketProviderConfig(context.Background(), 0, nil, MarketProviderConfigInput{ProviderType: "official-placeholder", Name: "泄露", Platform: "taobao", SourceLevel: SignalOriginAuthorized, Config: json.RawMessage(`{"apiToken":"plain-text"}`)})
	require.Error(t, err)

	view, err = svc.SetMarketProviderEnabled(context.Background(), 0, view.ID, true)
	require.NoError(t, err)
	view, err = svc.HealthCheckMarketProvider(context.Background(), 0, view.ID)
	require.NoError(t, err)
	require.Equal(t, "not_configured", view.Status)
	require.NotContains(t, view.LastError, "super-secret-abcd")
}

func TestCSVPreviewMappingFormulaAndPartialErrors(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("csv-s5"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	csv := fmt.Sprintf("商品ID,平台,类型,值,时间,来源\n%s,xianyu,demand_score,80,%s,manual-study\n%s,xianyu,demand_score,70,%s,=DANGEROUS", candidate.ID, time.Now().UTC().Format(time.RFC3339), candidate.ID, time.Now().UTC().Format(time.RFC3339))
	body := CSVImportRequest{CSV: csv, Mapping: map[string]string{"商品ID": "candidate_id", "平台": "platform", "类型": "signal_type", "值": "value", "时间": "observed_at", "来源": "source"}}
	preview, err := svc.PreviewMarketSignalCSV(ctx, 0, body)
	require.NoError(t, err)
	require.Equal(t, 2, preview.TotalRows)
	require.Equal(t, 1, preview.ValidRows)
	require.Equal(t, 1, preview.InvalidRows)
	require.Contains(t, preview.Errors[0].Message, "unsafe")
	result, err := svc.ImportMappedMarketSignalCSV(ctx, 0, body)
	require.NoError(t, err)
	require.Equal(t, 1, result.Imported)
	require.Len(t, result.Failed, 1)
}

func TestCalibrationExcludesFixtureAndMarksInsufficient(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	for i, sourceLevel := range []string{SignalOriginImport, SignalOriginFixture} {
		source, err := svc.CreateSource(ctx, 0, testSourceBody(fmt.Sprintf("cal-%d", i)))
		require.NoError(t, err)
		candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
		require.NoError(t, err)
		analysis, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
		require.NoError(t, err)
		catalog, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
		require.NoError(t, err)
		catalogProduct, ok := catalog.Catalog.(product.Product)
		require.True(t, ok)
		views, orders, revenue, profit := int64(100), int64(2), "50.00", "12.00"
		_, err = svc.importPerformanceItem(ctx, 0, ImportPerformanceItem{CandidateID: candidate.ID, CatalogProductID: catalogProduct.ID, Platform: "xianyu", Source: sourceLevel, ObservedAt: time.Now().UTC(), PeriodStart: time.Now().UTC().Add(-24 * time.Hour), PeriodEnd: time.Now().UTC(), Views: &views, Orders: &orders, GrossRevenue: &revenue, RealizedProfit: &profit})
		require.NoError(t, err)
		require.NotEmpty(t, analysis.Analysis.SelectionConfigVersion)
	}
	report, err := svc.CalibrationReport(ctx, 0, false)
	require.NoError(t, err)
	require.Equal(t, int64(1), report.SampleCount)
	require.Equal(t, "insufficient", report.Readiness.Level)
	require.True(t, report.RecommendationGroups[0].InsufficientSample)
	require.True(t, report.Suggestions[0].SuggestionOnly)
	reportWithFixture, err := svc.CalibrationReport(ctx, 0, true)
	require.NoError(t, err)
	require.Equal(t, int64(2), reportWithFixture.SampleCount)
}

func TestSelectionConfigReviewActivateAndAnalysisVersionAudit(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("config-version"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	first, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
	require.NoError(t, err)
	cfg := defaultSelectionConfig()
	cfg.Weights["profit"] = 25
	cfg.Weights["data_quality"] = 25
	input := SelectionConfigInput{Version: "selection-s5-test", Weights: cfg.Weights, Thresholds: SelectionThresholds{StrongRecommend: cfg.StrongRecommendThreshold, Recommend: cfg.RecommendThreshold, Watch: cfg.WatchThreshold, MinimumMarginBPS: cfg.MinimumMarginBPS, MinimumProfit: int64(cfg.MinimumProfit), StaleAfterDays: cfg.StaleAfterDays}, Blockers: SelectionBlockers{SensitiveKeywords: cfg.SensitiveKeywords, BlockerKeywords: cfg.BlockerKeywords}}
	draft, err := svc.CreateSelectionConfigDraft(ctx, 0, input, nil)
	require.NoError(t, err)
	_, err = svc.ActivateSelectionConfig(ctx, 0, draft.ID)
	require.ErrorIs(t, err, ErrInvalidTransition)
	_, err = svc.ReviewSelectionConfig(ctx, 0, draft.ID)
	require.NoError(t, err)
	_, err = svc.ActivateSelectionConfig(ctx, 0, draft.ID)
	require.NoError(t, err)
	second, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
	require.NoError(t, err)
	require.Equal(t, DefaultSelectionVersion, first.Analysis.SelectionConfigVersion)
	require.Equal(t, "selection-s5-test", second.Analysis.SelectionConfigVersion)
	var stored CandidateAnalysis
	require.NoError(t, svc.DB.Where("id = ?", first.Analysis.ID).First(&stored).Error)
	require.Equal(t, DefaultSelectionVersion, stored.SelectionConfigVersion)
}

func TestCalibrationPureReportEnoughSamplesAndSuggestion(t *testing.T) {
	rows := make([]calibration.Observation, 0, 25)
	score := int64(90)
	for i := 0; i < 25; i++ {
		profit := int64(1000)
		orders := int64(1)
		if i%5 == 0 {
			profit = -100
			orders = 0
		}
		rows = append(rows, calibration.Observation{Recommendation: "recommend", OverallScore: 80, Dimensions: map[string]*int64{"profit": &score}, Views: 100, Orders: orders, Revenue: 2000, Profit: profit, ObservedAt: time.Now().UTC().AddDate(0, 0, -i)})
	}
	report := calibration.Build(rows, false)
	require.Equal(t, "low", report.Readiness.Level)
	require.False(t, report.RecommendationGroups[1].InsufficientSample)
	require.True(t, report.Suggestions[0].SuggestionOnly)
}
