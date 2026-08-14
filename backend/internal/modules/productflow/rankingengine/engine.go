package rankingengine

import "github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"

const ConfigVersion = "ranking-v2"

type Config struct {
	OverallWeight    int64               `json:"overallWeight"`
	ConfidenceWeight int64               `json:"confidenceWeight"`
	ProfitWeight     int64               `json:"profitWeight"`
	MarginWeight     int64               `json:"marginWeight"`
	RiskWeight       int64               `json:"riskWeight"`
	ProfitCap        pricingengine.Money `json:"profitCap"`
	MarginCapBPS     int64               `json:"marginCapBps"`
}

func DefaultConfig() Config {
	return Config{OverallWeight: 50, ConfidenceWeight: 15, ProfitWeight: 10, MarginWeight: 15, RiskWeight: 10, ProfitCap: 5000, MarginCapBPS: 6000}
}

type Input struct {
	OverallScore           int64
	BaseQualityScore       int64
	MarketOpportunityScore *int64
	EvidenceCoverageBPS    int64
	Confidence             int64
	Profit                 pricingengine.Money
	MarginBPS              int64
	RiskScore              int64
	Blocked                bool
}

type Result struct {
	Score   int64    `json:"score"`
	Reasons []string `json:"reasons"`
}

func clamp(value, maximum int64) int64 {
	if value < 0 {
		return 0
	}
	if value > maximum {
		return maximum
	}
	return value
}

// Score is deterministic and batch-relative only in ordering; it never changes the product's overall score.
func Score(in Input, cfg Config) Result {
	if in.Blocked {
		return Result{Score: 0, Reasons: []string{"存在阻断项，不能进入自动推荐榜"}}
	}
	profitScore := int64(0)
	if cfg.ProfitCap > 0 {
		profitScore = clamp(int64(in.Profit)*100/int64(cfg.ProfitCap), 100)
	}
	marginScore := int64(0)
	if cfg.MarginCapBPS > 0 {
		marginScore = clamp(in.MarginBPS*100/cfg.MarginCapBPS, 100)
	}
	weight := cfg.OverallWeight + cfg.ConfidenceWeight + cfg.ProfitWeight + cfg.MarginWeight + cfg.RiskWeight
	if weight <= 0 {
		return Result{}
	}
	baseQuality := in.BaseQualityScore
	if baseQuality == 0 {
		baseQuality = in.OverallScore
	}
	baseRank := (clamp(baseQuality, 100)*cfg.OverallWeight + clamp(in.Confidence, 100)*cfg.ConfidenceWeight + profitScore*cfg.ProfitWeight + marginScore*cfg.MarginWeight + clamp(in.RiskScore, 100)*cfg.RiskWeight) / weight
	reasons := []string{"排序先使用基础质量、分析可信度、预计利润、利润率与基础风险进行初筛", "排序分不包含 LLM 自由判断"}
	score := baseRank * 80 / 100
	if in.MarketOpportunityScore != nil {
		coverage := clamp(in.EvidenceCoverageBPS, 10000)
		marketShare := int64(10)
		if coverage >= 10000 {
			marketShare = 20
		}
		score = baseRank*(100-marketShare)/100 + clamp(*in.MarketOpportunityScore, 100)*marketShare/100
		reasons = append(reasons, "市场机会仅在存在可靠证据时参与排序，并保留证据覆盖差异")
	} else {
		reasons = append(reasons, "暂无可靠市场证据，当前排名仅代表基础经营条件初筛")
	}
	return Result{Score: clamp(score, 100), Reasons: reasons}
}
