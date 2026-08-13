package calibration

import "time"

const MinimumReliableSample = 20

type Observation struct {
	Recommendation string
	OverallScore   int64
	Dimensions     map[string]*int64
	Warnings       []string
	Blockers       []string
	Views          int64
	Inquiries      int64
	Orders         int64
	Refunds        int64
	Revenue        int64
	Profit         int64
	ObservedAt     time.Time
}

type Metrics struct {
	Samples               int64 `json:"samples"`
	WithExposure          int64 `json:"withExposure"`
	WithInquiries         int64 `json:"withInquiries"`
	WithOrders            int64 `json:"withOrders"`
	PositiveProfit        int64 `json:"positiveProfit"`
	WithRefunds           int64 `json:"withRefunds"`
	AverageProfit         int64 `json:"averageProfit"`
	AverageMarginBPS      int64 `json:"averageMarginBps"`
	OrderRateBPS          int64 `json:"orderRateBps"`
	PositiveProfitRateBPS int64 `json:"positiveProfitRateBps"`
	InsufficientSample    bool  `json:"insufficientSample"`
}

type RecommendationGroup struct {
	Recommendation string `json:"recommendation"`
	Metrics
}
type ScoreBucket struct {
	Range   string `json:"range"`
	Minimum int64  `json:"minimum"`
	Maximum int64  `json:"maximum"`
	Metrics
}
type DimensionReport struct {
	Dimension          string `json:"dimension"`
	HighScoreThreshold int64  `json:"highScoreThreshold"`
	Metrics
	Note string `json:"note"`
}
type RuleReport struct {
	RuleType string `json:"ruleType"`
	Rule     string `json:"rule"`
	Metrics
	Note string `json:"note"`
}
type Suggestion struct {
	Dimension      string `json:"dimension,omitempty"`
	Message        string `json:"message"`
	SuggestionOnly bool   `json:"suggestionOnly"`
	SampleSize     int64  `json:"sampleSize"`
}
type Readiness struct {
	Level        string `json:"level"`
	RealSamples  int64  `json:"realSamples"`
	TimeSpanDays int    `json:"timeSpanDays"`
	Message      string `json:"message"`
}
type Report struct {
	GeneratedAt          time.Time             `json:"generatedAt"`
	IncludeTestData      bool                  `json:"includeTestData"`
	SampleCount          int64                 `json:"sampleCount"`
	RecommendationGroups []RecommendationGroup `json:"recommendationGroups"`
	ScoreBuckets         []ScoreBucket         `json:"scoreBuckets"`
	DimensionReports     []DimensionReport     `json:"dimensionReports"`
	RuleEffectiveness    []RuleReport          `json:"ruleEffectiveness"`
	Suggestions          []Suggestion          `json:"suggestions"`
	Readiness            Readiness             `json:"readiness"`
}

func metrics(rows []Observation) Metrics {
	m := Metrics{Samples: int64(len(rows)), InsufficientSample: len(rows) < MinimumReliableSample}
	var marginCount int64
	for _, row := range rows {
		if row.Views > 0 {
			m.WithExposure++
		}
		if row.Inquiries > 0 {
			m.WithInquiries++
		}
		if row.Orders > 0 {
			m.WithOrders++
		}
		if row.Profit > 0 {
			m.PositiveProfit++
		}
		if row.Refunds > 0 {
			m.WithRefunds++
		}
		m.AverageProfit += row.Profit
		if row.Revenue != 0 {
			m.AverageMarginBPS += row.Profit * 10000 / row.Revenue
			marginCount++
		}
	}
	if m.Samples > 0 {
		m.AverageProfit /= m.Samples
		m.OrderRateBPS = m.WithOrders * 10000 / m.Samples
		m.PositiveProfitRateBPS = m.PositiveProfit * 10000 / m.Samples
	}
	if marginCount > 0 {
		m.AverageMarginBPS /= marginCount
	}
	return m
}

func Build(rows []Observation, includeTestData bool) Report {
	report := Report{GeneratedAt: time.Now().UTC(), IncludeTestData: includeTestData, SampleCount: int64(len(rows)), RecommendationGroups: []RecommendationGroup{}, ScoreBuckets: []ScoreBucket{}, DimensionReports: []DimensionReport{}, RuleEffectiveness: []RuleReport{}, Suggestions: []Suggestion{}}
	for _, rec := range []string{"strong_recommend", "recommend", "watch", "reject"} {
		subset := []Observation{}
		for _, row := range rows {
			if row.Recommendation == rec {
				subset = append(subset, row)
			}
		}
		report.RecommendationGroups = append(report.RecommendationGroups, RecommendationGroup{Recommendation: rec, Metrics: metrics(subset)})
	}
	buckets := []struct {
		label    string
		min, max int64
	}{{"0-49", 0, 49}, {"50-59", 50, 59}, {"60-69", 60, 69}, {"70-79", 70, 79}, {"80-89", 80, 89}, {"90-100", 90, 100}}
	for _, bucket := range buckets {
		subset := []Observation{}
		for _, row := range rows {
			if row.OverallScore >= bucket.min && row.OverallScore <= bucket.max {
				subset = append(subset, row)
			}
		}
		report.ScoreBuckets = append(report.ScoreBuckets, ScoreBucket{Range: bucket.label, Minimum: bucket.min, Maximum: bucket.max, Metrics: metrics(subset)})
	}
	for _, dimension := range []string{"profit", "data_quality", "supply", "risk", "platform_fit", "demand", "competition"} {
		subset := []Observation{}
		for _, row := range rows {
			if score := row.Dimensions[dimension]; score != nil && *score >= 70 {
				subset = append(subset, row)
			}
		}
		report.DimensionReports = append(report.DimensionReports, DimensionReport{Dimension: dimension, HighScoreThreshold: 70, Metrics: metrics(subset), Note: "观察性相关，不代表因果关系"})
	}
	rules := map[string][]Observation{}
	for _, row := range rows {
		for _, rule := range row.Blockers {
			rules["blocker\x00"+rule] = append(rules["blocker\x00"+rule], row)
		}
		for _, rule := range row.Warnings {
			rules["warning\x00"+rule] = append(rules["warning\x00"+rule], row)
		}
	}
	for key, subset := range rules {
		kind, rule := "warning", key
		if len(key) > 8 && key[:8] == "blocker\x00" {
			kind = "blocker"
			rule = key[8:]
		} else if len(key) > 8 {
			rule = key[8:]
		}
		report.RuleEffectiveness = append(report.RuleEffectiveness, RuleReport{RuleType: kind, Rule: rule, Metrics: metrics(subset), Note: "仅供规则复核，不自动删除或修改规则"})
	}
	span := 0
	if len(rows) > 0 {
		min, max := rows[0].ObservedAt, rows[0].ObservedAt
		for _, row := range rows {
			if row.ObservedAt.Before(min) {
				min = row.ObservedAt
			}
			if row.ObservedAt.After(max) {
				max = row.ObservedAt
			}
		}
		span = int(max.Sub(min).Hours() / 24)
	}
	level := "insufficient"
	message := "真实样本不足，暂不建议调整选品权重。"
	if len(rows) >= 20 {
		level = "low"
		message = "已有初步样本，建议继续积累并人工复核。"
	}
	if len(rows) >= 50 && span >= 14 {
		level = "medium"
		message = "样本与时间跨度可支持人工校准复核。"
	}
	if len(rows) >= 100 && span >= 30 {
		level = "high"
		message = "校准数据准备度较高，仍需人工审查任何规则调整。"
	}
	report.Readiness = Readiness{Level: level, RealSamples: int64(len(rows)), TimeSpanDays: span, Message: message}
	if len(rows) < MinimumReliableSample {
		report.Suggestions = append(report.Suggestions, Suggestion{Message: "当前样本不足，建议先补充真实 Performance 数据，不调整权重。", SuggestionOnly: true, SampleSize: int64(len(rows))})
	} else {
		best := report.DimensionReports[0]
		for _, item := range report.DimensionReports {
			if item.Samples >= MinimumReliableSample && item.OrderRateBPS > best.OrderRateBPS {
				best = item
			}
		}
		report.Suggestions = append(report.Suggestions, Suggestion{Dimension: best.Dimension, Message: "该维度高分组的订单表现值得人工复核；这是相关性观察，不建议自动调权。", SuggestionOnly: true, SampleSize: best.Samples})
	}
	return report
}
