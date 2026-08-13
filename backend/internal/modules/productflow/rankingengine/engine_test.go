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
