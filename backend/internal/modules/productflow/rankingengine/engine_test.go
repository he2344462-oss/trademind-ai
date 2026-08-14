package rankingengine

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func TestRankingIsDeterministicAndBlockedIsZero(t *testing.T) {
	cfg := DefaultConfig()
	high := Score(Input{OverallScore: 88, Confidence: 80, Profit: 3000, MarginBPS: 4500, RiskScore: 90}, cfg)
	low := Score(Input{OverallScore: 60, Confidence: 55, Profit: 800, MarginBPS: 1800, RiskScore: 60}, cfg)
	require.Greater(t, high.Score, low.Score)
	require.Equal(t, high, Score(Input{OverallScore: 88, Confidence: 80, Profit: 3000, MarginBPS: 4500, RiskScore: 90}, cfg))
	require.Zero(t, Score(Input{OverallScore: 99, Blocked: true}, cfg).Score)
}

func TestRankingSeparatesBaseOnlyFromMarketEvidence(t *testing.T) {
	cfg := DefaultConfig()
	market := int64(88)
	baseOnly := Score(Input{BaseQualityScore: 88, Confidence: 80, Profit: 3000, MarginBPS: 4500, RiskScore: 75}, cfg)
	withMarket := Score(Input{BaseQualityScore: 88, MarketOpportunityScore: &market, EvidenceCoverageBPS: 10000, Confidence: 80, Profit: 3000, MarginBPS: 4500, RiskScore: 75}, cfg)
	require.Greater(t, withMarket.Score, baseOnly.Score)
	require.Contains(t, baseOnly.Reasons, "暂无可靠市场证据，当前排名仅代表基础经营条件初筛")
}
