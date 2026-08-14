package selectionengine

import (
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"testing"
	"time"
)

func baseInput() Input {
	return Input{Pricing: pricingengine.PricingResult{EstimatedProfit: 2000, EstimatedMarginBPS: 4500, SuggestedSalePrice: 4500, BreakEvenPrice: 2000}, Product: ProductData{Title: "完整收纳盒", Description: "清晰描述", ImageCount: 5, SKUCount: 2, CompleteSKUCount: 2, HasPurchaseCost: true, HasFreight: true, Supplier: "供应商", SourceURL: "https://example.com", Category: "家居", MOQ: 1, HasStockData: true, CollectedAt: time.Now()}}
}
func TestScoreHighProfitAndUnknownMarketDimensions(t *testing.T) {
	r := Score(baseInput(), DefaultConfig())
	require.Equal(t, RecommendationWatch, r.Recommendation)
	require.Nil(t, r.Dimensions["demand"].Score)
	require.Nil(t, r.Dimensions["competition"].Score)
	require.Nil(t, r.MarketOpportunityScore)
	require.Zero(t, r.EvidenceCoverageBPS)
	require.Contains(t, r.MissingDimensions, "demand")
	require.Contains(t, r.MissingDimensions, "competition")
	require.Equal(t, r.BaseQualityScore, r.OverallScore)
	require.Less(t, r.ConfidenceScore, int64(100))
}
func TestOverallScoreIsCompatibilityAliasForBaseQuality(t *testing.T) {
	in := baseInput()
	cfg := DefaultConfig()
	r := Score(in, cfg)
	require.Equal(t, r.BaseQualityScore, r.OverallScore)
	require.Nil(t, r.MarketOpportunityScore)
}
func TestStrongRecommendRequiresBothReliableMarketDimensions(t *testing.T) {
	in := baseInput()
	in.Market = map[string]MarketDimension{"demand": {Score: 95, ConfidenceBPS: 9000, Freshness: "fresh"}}
	oneSignal := Score(in, DefaultConfig())
	require.Equal(t, RecommendationRecommend, oneSignal.Recommendation)
	require.Equal(t, int64(5000), oneSignal.ConfidenceBreakdown.MarketCoverageBPS)
	require.NotNil(t, oneSignal.MarketOpportunityScore)
	require.NotEqual(t, RecommendationStrong, oneSignal.Recommendation)

	in.Market["competition"] = MarketDimension{Score: 90, ConfidenceBPS: 9000, Freshness: "fresh"}
	twoSignals := Score(in, DefaultConfig())
	require.Equal(t, RecommendationStrong, twoSignals.Recommendation)
	require.Equal(t, int64(10000), twoSignals.ConfidenceBreakdown.MarketCoverageBPS)
}
func TestStrongRecommendRequiresConfidenceAndConfiguredCoverage(t *testing.T) {
	in := baseInput()
	in.Market = map[string]MarketDimension{
		"demand":      {Score: 95, ConfidenceBPS: 9000, Freshness: "fresh"},
		"competition": {Score: 90, ConfidenceBPS: 9000, Freshness: "fresh"},
	}
	cfg := DefaultConfig()
	result := Score(in, cfg)
	require.Equal(t, RecommendationStrong, result.Recommendation)
	require.Equal(t, int64(10000), result.EvidenceCoverageBPS)
}
func TestLowConfidenceOrExpiredMarketSignalStaysUnknown(t *testing.T) {
	in := baseInput()
	in.Market = map[string]MarketDimension{
		"demand":      {Score: 100, ConfidenceBPS: 2999, Freshness: "fresh"},
		"competition": {Score: 100, ConfidenceBPS: 10000, Freshness: "expired"},
	}
	r := Score(in, DefaultConfig())
	require.Nil(t, r.Dimensions["demand"].Score)
	require.Nil(t, r.Dimensions["competition"].Score)
	require.Equal(t, int64(0), r.ConfidenceBreakdown.MarketCoverageBPS)
	require.NotEqual(t, RecommendationStrong, r.Recommendation)
}
func TestMissingDataLowersConfidence(t *testing.T) {
	full := Score(baseInput(), DefaultConfig())
	in := baseInput()
	in.Product = ProductData{Title: "少资料", CollectedAt: time.Now()}
	missing := Score(in, DefaultConfig())
	require.Less(t, missing.ConfidenceScore, full.ConfidenceScore)
	require.Less(t, missing.Dimensions["data_quality"].ScoreValue(), full.Dimensions["data_quality"].ScoreValue())
}
func TestUnknownFreightLowersCostConfidenceAndWarns(t *testing.T) {
	known := Score(baseInput(), DefaultConfig())
	in := baseInput()
	in.Product.HasFreight = false
	in.Product.FreightConfidenceBPS = 0
	unknown := Score(in, DefaultConfig())
	require.Less(t, unknown.ConfidenceBreakdown.CostDataBPS, known.ConfidenceBreakdown.CostDataBPS)
	require.Contains(t, unknown.Warnings, "采购运费待确认，当前利润为不含可靠采购运费的初筛结果")
}
func TestLossAndBlockerForceReject(t *testing.T) {
	in := baseInput()
	in.Pricing.EstimatedProfit = -100
	in.Pricing.SuggestedSalePrice = 1000
	in.Pricing.BreakEvenPrice = 1200
	r := Score(in, DefaultConfig())
	require.Equal(t, RecommendationReject, r.Recommendation)
	require.NotEmpty(t, r.Blockers)
}
func TestRiskKeywordIsWarningNotAutomaticBlocker(t *testing.T) {
	in := baseInput()
	in.Product.Title = "复刻风格收纳盒"
	r := Score(in, DefaultConfig())
	require.NotEmpty(t, r.Warnings)
	require.Empty(t, r.Blockers)
}
func TestNoKnownRiskIsNotVerifiedLowRisk(t *testing.T) {
	in := baseInput()
	result := Score(in, DefaultConfig())
	require.Equal(t, RiskEvidenceNoKnown, result.Dimensions["risk"].EvidenceStatus)
	require.Less(t, result.Dimensions["risk"].ScoreValue(), int64(100))
	in.Product.RiskVerified = true
	verified := Score(in, DefaultConfig())
	require.Equal(t, RiskEvidenceVerifiedLow, verified.Dimensions["risk"].EvidenceStatus)
	require.Equal(t, int64(100), verified.Dimensions["risk"].ScoreValue())
}
func TestPlatformFitDistinguishesOperationalEvidenceAndSKUComplexity(t *testing.T) {
	simple := baseInput()
	complex := baseInput()
	complex.Product.SKUCount = 161
	complex.Product.CompleteSKUCount = 161
	complex.Product.Description = ""
	complex.Product.Category = ""
	complexFit := Score(complex, DefaultConfig()).Dimensions["platform_fit"].ScoreValue()
	suitable := true
	requiresQualification := false
	simple.Product.AfterSaleComplexity = "low"
	simple.Product.DeliveryMode = "standard_shipping"
	simple.Product.SmallSellerSuitable = &suitable
	simple.Product.RequiresQualification = &requiresQualification
	simpleFit := Score(simple, DefaultConfig()).Dimensions["platform_fit"].ScoreValue()
	require.Greater(t, simpleFit, complexFit)
	require.Less(t, complexFit, int64(100))
}
func TestFreshMarketDimensionsParticipateAndExpiredAreExcludedByProvider(t *testing.T) {
	in := baseInput()
	in.Market = map[string]MarketDimension{"demand": {Score: 82, ConfidenceBPS: 8000, Freshness: "fresh"}, "competition": {Score: 60, ConfidenceBPS: 6000, Freshness: "stale"}}
	r := Score(in, DefaultConfig())
	require.NotNil(t, r.Dimensions["demand"].Score)
	require.NotNil(t, r.Dimensions["competition"].Score)
	require.Empty(t, r.MissingDimensions)
	require.Greater(t, r.ConfidenceBreakdown.MarketCoverageBPS, int64(0))
}
func (d Dimension) ScoreValue() int64 {
	if d.Score == nil {
		return 0
	}
	return *d.Score
}
