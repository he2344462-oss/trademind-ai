package selectionengine

import (
	"math"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
)

const (
	RecommendationStrong    = "strong_recommend"
	RecommendationRecommend = "recommend"
	RecommendationWatch     = "watch"
	RecommendationReject    = "reject"
)

type Config struct {
	Weights                  map[string]int64    `json:"weights"`
	StrongRecommendThreshold int64               `json:"strongRecommendThreshold"`
	RecommendThreshold       int64               `json:"recommendThreshold"`
	WatchThreshold           int64               `json:"watchThreshold"`
	MinimumMarginBPS         int64               `json:"minimumMarginBps"`
	MinimumProfit            pricingengine.Money `json:"minimumProfit"`
	StaleAfterDays           int                 `json:"staleAfterDays"`
	SensitiveKeywords        []string            `json:"sensitiveKeywords"`
	BlockerKeywords          []string            `json:"blockerKeywords"`
}

func DefaultConfig() Config {
	return Config{Weights: map[string]int64{"profit": 30, "data_quality": 20, "supply": 15, "risk": 15, "platform_fit": 10, "demand": 5, "competition": 5}, StrongRecommendThreshold: 85, RecommendThreshold: 70, WatchThreshold: 50, MinimumMarginBPS: 2000, MinimumProfit: 500, StaleAfterDays: 90, SensitiveKeywords: []string{"仿牌", "高仿", "复刻", "商标授权"}, BlockerKeywords: []string{"枪支", "毒品", "违禁药"}}
}

type ProductData struct {
	Title            string
	Description      string
	ImageCount       int
	SKUCount         int
	CompleteSKUCount int
	HasPurchaseCost  bool
	HasFreight       bool
	Supplier         string
	SourceURL        string
	Category         string
	MOQ              int
	HasStockData     bool
	CollectedAt      time.Time
	Platform         string
}

type Input struct {
	Pricing             pricingengine.PricingResult
	Product             ProductData
	MinimumSKUMarginBPS *int64
	Market              map[string]MarketDimension
}

// MarketDimension is normalized provider evidence on a 0..100 scale.
type MarketDimension struct {
	Score         int64    `json:"score"`
	ConfidenceBPS int64    `json:"confidenceBps"`
	Freshness     string   `json:"freshness"`
	Evidence      []string `json:"evidence"`
}

type Dimension struct {
	Score    *int64   `json:"score"`
	Weight   int64    `json:"weight"`
	Reliable bool     `json:"reliable"`
	Evidence []string `json:"evidence"`
}
type Result struct {
	Dimensions          map[string]Dimension `json:"dimensions"`
	OverallScore        int64                `json:"overallScore"`
	ConfidenceScore     int64                `json:"confidenceScore"`
	Recommendation      string               `json:"recommendation"`
	Reasons             []string             `json:"reasons"`
	Warnings            []string             `json:"warnings"`
	Blockers            []string             `json:"blockers"`
	MissingDimensions   []string             `json:"missingDimensions"`
	ConfidenceBreakdown ConfidenceBreakdown  `json:"confidenceBreakdown"`
}

type ConfidenceBreakdown struct {
	ProductDataBPS    int64 `json:"productDataBps"`
	CostDataBPS       int64 `json:"costDataBps"`
	MarketCoverageBPS int64 `json:"marketCoverageBps"`
}

func ptr(v int64) *int64 { return &v }
func clamp(v int64) int64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func Score(in Input, cfg Config) Result {
	dims := map[string]Dimension{}
	profit := int64(20)
	if in.Pricing.EstimatedProfit > 0 {
		profit = 50 + clamp(in.Pricing.EstimatedMarginBPS/100)
	}
	if in.Pricing.EstimatedMarginBPS >= 3000 {
		profit += 10
	}
	if in.Pricing.EstimatedMarginBPS >= 4500 {
		profit += 10
	}
	if in.MinimumSKUMarginBPS != nil && *in.MinimumSKUMarginBPS < cfg.MinimumMarginBPS {
		profit -= 25
	}
	dims["profit"] = Dimension{Score: ptr(clamp(profit)), Weight: cfg.Weights["profit"], Reliable: true, Evidence: []string{"由精确成本、利润率和最低 SKU 利润率计算"}}
	quality := int64(0)
	checks := []bool{strings.TrimSpace(in.Product.Title) != "", strings.TrimSpace(in.Product.Description) != "", in.Product.ImageCount > 0, in.Product.ImageCount >= 3, in.Product.SKUCount > 0, in.Product.SKUCount == in.Product.CompleteSKUCount, in.Product.HasPurchaseCost, strings.TrimSpace(in.Product.Supplier) != "", strings.TrimSpace(in.Product.SourceURL) != "", strings.TrimSpace(in.Product.Category) != "", in.Product.HasFreight}
	for _, ok := range checks {
		if ok {
			quality += 100 / int64(len(checks))
		}
	}
	dims["data_quality"] = Dimension{Score: ptr(clamp(quality)), Weight: cfg.Weights["data_quality"], Reliable: true, Evidence: []string{"按标题、描述、图片、SKU、成本、供应商、链接、类目和运费完整度计算"}}
	supply := int64(25)
	evidence := []string{}
	if in.Product.Supplier != "" {
		supply += 20
		evidence = append(evidence, "供应商信息明确")
	}
	if in.Product.HasPurchaseCost {
		supply += 20
		evidence = append(evidence, "采购价明确")
	}
	if in.Product.SKUCount > 0 && in.Product.SKUCount <= 50 {
		supply += 15
		evidence = append(evidence, "SKU 结构处于可管理范围")
	}
	if in.Product.MOQ > 0 && in.Product.MOQ <= 5 {
		supply += 10
		evidence = append(evidence, "MOQ 较低")
	}
	if in.Product.HasStockData {
		supply += 10
		evidence = append(evidence, "包含库存数据")
	}
	dims["supply"] = Dimension{Score: ptr(clamp(supply)), Weight: cfg.Weights["supply"], Reliable: true, Evidence: evidence}
	risk := int64(100)
	blockers := []string{}
	warnings := []string{}
	combined := strings.ToLower(in.Product.Title + " " + in.Product.Description)
	for _, kw := range cfg.SensitiveKeywords {
		if strings.Contains(combined, strings.ToLower(kw)) {
			risk -= 18
			warnings = append(warnings, "命中风险提示词："+kw)
		}
	}
	for _, kw := range cfg.BlockerKeywords {
		if strings.Contains(combined, strings.ToLower(kw)) {
			risk = 0
			blockers = append(blockers, "命中明确禁售规则："+kw)
		}
	}
	if !in.Product.HasPurchaseCost {
		risk = 0
		blockers = append(blockers, "缺少采购价，无法可靠计算利润")
	}
	if in.Pricing.EstimatedProfit <= 0 {
		risk = 0
		blockers = append(blockers, "预计利润不大于 0")
	}
	if in.Pricing.SuggestedSalePrice < in.Pricing.BreakEvenPrice {
		risk = 0
		blockers = append(blockers, "售价低于保本价")
	}
	if in.Product.CollectedAt.Before(time.Now().AddDate(0, 0, -cfg.StaleAfterDays)) {
		risk -= 15
		warnings = append(warnings, "货源数据采集时间过旧")
	}
	dims["risk"] = Dimension{Score: ptr(clamp(risk)), Weight: cfg.Weights["risk"], Reliable: true, Evidence: append([]string{"风险分越高表示当前已知风险越低"}, warnings...)}
	fit := int64(45)
	if in.Product.ImageCount >= 3 {
		fit += 15
	}
	if in.Product.SKUCount > 0 && in.Product.SKUCount <= 20 {
		fit += 15
	}
	if in.Pricing.EstimatedMarginBPS >= cfg.MinimumMarginBPS {
		fit += 15
	}
	if in.Pricing.SuggestedSalePrice >= 1000 && in.Pricing.SuggestedSalePrice <= 50000 {
		fit += 10
	}
	dims["platform_fit"] = Dimension{Score: ptr(clamp(fit)), Weight: cfg.Weights["platform_fit"], Reliable: true, Evidence: []string{"仅基于客单价、图片、SKU 复杂度和利润空间，不含平台流量数据"}}
	missing := []string{}
	for _, key := range []string{"demand", "competition"} {
		market, ok := in.Market[key]
		if !ok || market.ConfidenceBPS <= 0 {
			dims[key] = Dimension{Score: nil, Weight: cfg.Weights[key], Reliable: false, Evidence: []string{"暂无可验证的有效市场信号"}}
			missing = append(missing, key)
			continue
		}
		score := clamp(market.Score)
		evidence := append([]string{}, market.Evidence...)
		evidence = append(evidence, "市场信号新鲜度："+market.Freshness)
		dims[key] = Dimension{Score: &score, Weight: cfg.Weights[key], Reliable: true, Evidence: evidence}
	}
	weighted, totalWeight, knownWeight := int64(0), int64(0), int64(0)
	for key, d := range dims {
		totalWeight += cfg.Weights[key]
		if d.Score != nil && d.Reliable {
			weighted += *d.Score * cfg.Weights[key]
			knownWeight += cfg.Weights[key]
		}
	}
	overall := int64(0)
	if knownWeight > 0 {
		overall = int64(math.Round(float64(weighted) / float64(knownWeight)))
	}
	dataCoverage := int64(0)
	for _, ok := range checks {
		if ok {
			dataCoverage++
		}
	}
	productConfidenceBPS := dataCoverage * 10000 / int64(len(checks))
	costKnown := int64(0)
	for _, ok := range []bool{in.Product.HasPurchaseCost, in.Product.HasFreight, in.Pricing.EstimatedTotalCost >= 0, in.Pricing.SuggestedSalePrice > 0} {
		if ok {
			costKnown++
		}
	}
	costConfidenceBPS := costKnown * 10000 / 4
	marketConfidenceBPS := int64(0)
	for _, key := range []string{"demand", "competition"} {
		if item, ok := in.Market[key]; ok && item.ConfidenceBPS > 0 {
			marketConfidenceBPS += item.ConfidenceBPS / 2
		}
	}
	confidenceBPS := productConfidenceBPS*35/100 + costConfidenceBPS*45/100 + marketConfidenceBPS*20/100
	confidence := clamp(int64(math.Round(float64(confidenceBPS) / 100)))
	rec := RecommendationReject
	if len(blockers) == 0 {
		switch {
		case overall >= cfg.StrongRecommendThreshold && confidence >= 75:
			rec = RecommendationStrong
		case overall >= cfg.RecommendThreshold:
			rec = RecommendationRecommend
		case overall >= cfg.WatchThreshold:
			rec = RecommendationWatch
		}
	}
	reasons := []string{}
	if in.Pricing.EstimatedMarginBPS >= 3000 {
		reasons = append(reasons, "毛利空间较好")
	}
	if quality >= 75 {
		reasons = append(reasons, "商品资料较完整")
	}
	if supply >= 65 {
		reasons = append(reasons, "当前供应资料较完整")
	}
	if risk >= 75 {
		reasons = append(reasons, "暂无明显高风险因素")
	}
	if len(missing) > 0 {
		reasons = append(reasons, "当前市场信号覆盖不足，结论是规则初筛，不代表市场爆款概率")
	}
	return Result{Dimensions: dims, OverallScore: overall, ConfidenceScore: confidence, Recommendation: rec, Reasons: reasons, Warnings: warnings, Blockers: blockers, MissingDimensions: missing, ConfidenceBreakdown: ConfidenceBreakdown{ProductDataBPS: productConfidenceBPS, CostDataBPS: costConfidenceBPS, MarketCoverageBPS: marketConfidenceBPS}}
}
