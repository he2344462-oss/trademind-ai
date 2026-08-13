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
	require.Contains(t, []string{RecommendationStrong, RecommendationRecommend}, r.Recommendation)
	require.Nil(t, r.Dimensions["demand"].Score)
	require.Nil(t, r.Dimensions["competition"].Score)
	require.Contains(t, r.MissingDimensions, "demand")
	require.Greater(t, r.OverallScore, int64(70))
	require.Less(t, r.ConfidenceScore, int64(100))
}
func TestDynamicWeightDoesNotTreatUnknownAsFifty(t *testing.T) {
	in := baseInput()
	cfg := DefaultConfig()
	r := Score(in, cfg)
	knownSum, knownWeight := int64(0), int64(0)
	for _, d := range r.Dimensions {
		if d.Score != nil && d.Reliable {
			knownSum += *d.Score * d.Weight
			knownWeight += d.Weight
		}
	}
	require.Equal(t, (knownSum+knownWeight/2)/knownWeight, r.OverallScore)
}
func TestMissingDataLowersConfidence(t *testing.T) {
	full := Score(baseInput(), DefaultConfig())
	in := baseInput()
	in.Product = ProductData{Title: "少资料", CollectedAt: time.Now()}
	missing := Score(in, DefaultConfig())
	require.Less(t, missing.ConfidenceScore, full.ConfidenceScore)
	require.Less(t, missing.Dimensions["data_quality"].ScoreValue(), full.Dimensions["data_quality"].ScoreValue())
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
