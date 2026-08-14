package selectionengine

import (
	"math"
	"strings"
	"time"

	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
)

const (
	ConfigVersion                  = "selection-score-v4"
	RecommendationStrong           = "strong_recommend"
	RecommendationRecommend        = "recommend"
	RecommendationWatch            = "watch"
	RecommendationReject           = "reject"
	minimumMarketConfidence        = int64(3000)
	defaultStrongMarketCoverageBPS = int64(10000)
	RiskEvidenceVerifiedLow        = "verified_low_risk"
	RiskEvidenceNoKnown            = "no_known_risk"
	RiskEvidenceInsufficient       = "insufficient_risk_evidence"
	RiskEvidenceWarning            = "risk_warning"
	RiskEvidenceBlocker            = "known_blocker"
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
	StrongMarketCoverageBPS  int64               `json:"strongMarketCoverageBps"`
}

func DefaultConfig() Config {
	return Config{Weights: map[string]int64{"profit": 30, "data_quality": 20, "supply": 15, "risk": 15, "platform_fit": 10, "demand": 5, "competition": 5}, StrongRecommendThreshold: 85, RecommendThreshold: 70, WatchThreshold: 50, MinimumMarginBPS: 2000, MinimumProfit: 500, StaleAfterDays: 90, SensitiveKeywords: []string{"仿牌", "高仿", "复刻", "商标授权"}, BlockerKeywords: []string{"枪支", "毒品", "违禁药"}, StrongMarketCoverageBPS: defaultStrongMarketCoverageBPS}
}

type ProductData struct {
	Title                 string
	Description           string
	ImageCount            int
	SKUCount              int
	CompleteSKUCount      int
	HasPurchaseCost       bool
	HasFreight            bool
	FreightConfidenceBPS  int64
	Supplier              string
	SourceURL             string
	Category              string
	MOQ                   int
	HasStockData          bool
	CollectedAt           time.Time
	Platform              string
	RiskVerified          bool
	AfterSaleComplexity   string
	DeliveryMode          string
	SmallSellerSuitable   *bool
	RequiresQualification *bool
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
	Score          *int64   `json:"score"`
	Weight         int64    `json:"weight"`
	Reliable       bool     `json:"reliable"`
	Evidence       []string `json:"evidence"`
	EvidenceStatus string   `json:"evidenceStatus,omitempty"`
}
type Result struct {
	Dimensions             map[string]Dimension `json:"dimensions"`
	OverallScore           int64                `json:"overallScore"`
	BaseQualityScore       int64                `json:"baseQualityScore"`
	MarketOpportunityScore *int64               `json:"marketOpportunityScore"`
	EvidenceCoverageBPS    int64                `json:"evidenceCoverageBps"`
	ConfidenceScore        int64                `json:"confidenceScore"`
	Recommendation         string               `json:"recommendation"`
	Reasons                []string             `json:"reasons"`
	Warnings               []string             `json:"warnings"`
	Blockers               []string             `json:"blockers"`
	MissingDimensions      []string             `json:"missingDimensions"`
	ConfidenceBreakdown    ConfidenceBreakdown  `json:"confidenceBreakdown"`
}

type ConfidenceBreakdown struct {
	ProductDataBPS               int64  `json:"productDataBps"`
	CostDataBPS                  int64  `json:"costDataBps"`
	MarketCoverageBPS            int64  `json:"marketCoverageBps"`
	MarketSignalConfidenceBPS    int64  `json:"marketSignalConfidenceBps"`
	OverallAnalysisConfidenceBPS int64  `json:"overallAnalysisConfidenceBps"`
	BaseQualityScore             int64  `json:"baseQualityScore"`
	MarketOpportunityScore       *int64 `json:"marketOpportunityScore"`
	EvidenceCoverageBPS          int64  `json:"evidenceCoverageBps"`
	RiskEvidenceStatus           string `json:"riskEvidenceStatus"`
	PlatformFitEvidenceStatus    string `json:"platformFitEvidenceStatus"`
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
	if cfg.StrongMarketCoverageBPS <= 0 {
		cfg.StrongMarketCoverageBPS = defaultStrongMarketCoverageBPS
	}
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
	risk := int64(75)
	riskEvidenceStatus := RiskEvidenceNoKnown
	blockers := []string{}
	warnings := []string{}
	if !in.Product.HasFreight {
		warnings = append(warnings, "采购运费待确认，当前利润为不含可靠采购运费的初筛结果")
	}
	combined := strings.ToLower(in.Product.Title + " " + in.Product.Description)
	for _, kw := range cfg.SensitiveKeywords {
		if strings.Contains(combined, strings.ToLower(kw)) {
			risk = 55
			riskEvidenceStatus = RiskEvidenceWarning
			warnings = append(warnings, "命中风险提示词："+kw)
		}
	}
	for _, kw := range cfg.BlockerKeywords {
		if strings.Contains(combined, strings.ToLower(kw)) {
			risk = 0
			riskEvidenceStatus = RiskEvidenceBlocker
			blockers = append(blockers, "命中明确禁售规则："+kw)
		}
	}
	if !in.Product.HasPurchaseCost {
		risk = 0
		riskEvidenceStatus = RiskEvidenceBlocker
		blockers = append(blockers, "缺少采购价，无法可靠计算利润")
	}
	if in.Pricing.EstimatedProfit <= 0 {
		risk = 0
		riskEvidenceStatus = RiskEvidenceBlocker
		blockers = append(blockers, "预计利润不大于 0")
	}
	if in.Pricing.SuggestedSalePrice < in.Pricing.BreakEvenPrice {
		risk = 0
		riskEvidenceStatus = RiskEvidenceBlocker
		blockers = append(blockers, "售价低于保本价")
	}
	if in.Product.CollectedAt.Before(time.Now().AddDate(0, 0, -cfg.StaleAfterDays)) {
		if riskEvidenceStatus == RiskEvidenceNoKnown {
			risk = 50
			riskEvidenceStatus = RiskEvidenceInsufficient
		}
		warnings = append(warnings, "货源数据采集时间过旧")
	}
	if riskEvidenceStatus == RiskEvidenceNoKnown && (strings.TrimSpace(in.Product.Description) == "" || strings.TrimSpace(in.Product.Category) == "" || strings.TrimSpace(in.Product.SourceURL) == "") {
		risk = 50
		riskEvidenceStatus = RiskEvidenceInsufficient
	}
	if riskEvidenceStatus == RiskEvidenceNoKnown && in.Product.RiskVerified {
		risk = 100
		riskEvidenceStatus = RiskEvidenceVerifiedLow
	}
	dims["risk"] = Dimension{Score: ptr(clamp(risk)), Weight: cfg.Weights["risk"], Reliable: true, EvidenceStatus: riskEvidenceStatus, Evidence: append([]string{"风险状态区分已验证低风险、暂无已知风险与证据不足；未命中规则不等于风险已充分验证"}, warnings...)}

	fit, fitEvidenceStatus, fitEvidence := platformFit(in)
	dims["platform_fit"] = Dimension{Score: ptr(fit), Weight: cfg.Weights["platform_fit"], Reliable: true, EvidenceStatus: fitEvidenceStatus, Evidence: fitEvidence}
	missing := []string{}
	for _, key := range []string{"demand", "competition"} {
		market, ok := in.Market[key]
		if !ok || market.ConfidenceBPS < minimumMarketConfidence || strings.EqualFold(strings.TrimSpace(market.Freshness), "expired") {
			dims[key] = Dimension{Score: nil, Weight: cfg.Weights[key], Reliable: false, Evidence: []string{"暂无可验证的有效市场信号"}}
			missing = append(missing, key)
			continue
		}
		score := clamp(market.Score)
		evidence := append([]string{}, market.Evidence...)
		evidence = append(evidence, "市场信号新鲜度："+market.Freshness)
		dims[key] = Dimension{Score: &score, Weight: cfg.Weights[key], Reliable: true, Evidence: evidence}
	}
	baseWeighted, baseWeight := int64(0), int64(0)
	for _, key := range []string{"profit", "data_quality", "supply", "risk", "platform_fit"} {
		d := dims[key]
		baseWeight += d.Weight
		if d.Score != nil && d.Reliable {
			baseWeighted += *d.Score * d.Weight
		}
	}
	baseQuality := int64(0)
	if baseWeight > 0 {
		baseQuality = clamp((baseWeighted + baseWeight/2) / baseWeight)
	}
	marketWeighted, marketWeight := int64(0), int64(0)
	for _, key := range []string{"demand", "competition"} {
		d := dims[key]
		if d.Score != nil && d.Reliable {
			marketWeighted += *d.Score * d.Weight
			marketWeight += d.Weight
		}
	}
	var marketOpportunity *int64
	if marketWeight > 0 {
		value := clamp((marketWeighted + marketWeight/2) / marketWeight)
		marketOpportunity = &value
	}
	// overallScore is retained as an API compatibility alias for base quality.
	// It no longer mixes missing market evidence into a single "worth selling" score.
	overall := baseQuality
	dataCoverage := int64(0)
	for _, ok := range checks {
		if ok {
			dataCoverage++
		}
	}
	productConfidenceBPS := dataCoverage * 10000 / int64(len(checks))
	costConfidenceBPS := int64(0)
	if in.Product.HasPurchaseCost {
		costConfidenceBPS += 2500
	}
	if in.Product.HasFreight {
		freightConfidence := in.Product.FreightConfidenceBPS
		if freightConfidence <= 0 {
			freightConfidence = 10000
		}
		costConfidenceBPS += freightConfidence / 4
	}
	if in.Pricing.EstimatedTotalCost >= 0 {
		costConfidenceBPS += 2500
	}
	if in.Pricing.SuggestedSalePrice > 0 {
		costConfidenceBPS += 2500
	}
	marketConfidenceBPS := int64(0)
	marketCovered := int64(0)
	for _, key := range []string{"demand", "competition"} {
		dimension := dims[key]
		if item, ok := in.Market[key]; ok && dimension.Reliable {
			marketConfidenceBPS += item.ConfidenceBPS / 2
			marketCovered++
		}
	}
	marketCoverageBPS := marketCovered * 5000
	marketSignalConfidenceBPS := int64(0)
	if marketCovered > 0 {
		marketSignalConfidenceBPS = marketConfidenceBPS * 2 / marketCovered
	}
	confidenceBPS := productConfidenceBPS*35/100 + costConfidenceBPS*45/100 + marketConfidenceBPS*20/100
	confidence := clamp(int64(math.Round(float64(confidenceBPS) / 100)))
	rec := RecommendationReject
	if len(blockers) == 0 {
		switch {
		case baseQuality < cfg.WatchThreshold:
			rec = RecommendationReject
		case marketOpportunity == nil || marketCoverageBPS == 0:
			rec = RecommendationWatch
		case marketCovered == 2 && baseQuality >= cfg.StrongRecommendThreshold && *marketOpportunity >= cfg.StrongRecommendThreshold && confidence >= 75 && marketCoverageBPS >= cfg.StrongMarketCoverageBPS:
			rec = RecommendationStrong
		case baseQuality >= cfg.RecommendThreshold && *marketOpportunity >= cfg.RecommendThreshold:
			rec = RecommendationRecommend
		default:
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
	if riskEvidenceStatus == RiskEvidenceNoKnown {
		reasons = append(reasons, "当前规则未发现已知高风险，但尚不等于风险已充分验证")
	}
	if marketOpportunity == nil {
		reasons = append(reasons, "基础经营条件较好，但缺少真实市场需求和竞争证据，仅建议小规模测试")
	}
	return Result{Dimensions: dims, OverallScore: overall, BaseQualityScore: baseQuality, MarketOpportunityScore: marketOpportunity, EvidenceCoverageBPS: marketCoverageBPS, ConfidenceScore: confidence, Recommendation: rec, Reasons: reasons, Warnings: warnings, Blockers: blockers, MissingDimensions: missing, ConfidenceBreakdown: ConfidenceBreakdown{ProductDataBPS: productConfidenceBPS, CostDataBPS: costConfidenceBPS, MarketCoverageBPS: marketCoverageBPS, MarketSignalConfidenceBPS: marketSignalConfidenceBPS, OverallAnalysisConfidenceBPS: confidenceBPS, BaseQualityScore: baseQuality, MarketOpportunityScore: marketOpportunity, EvidenceCoverageBPS: marketCoverageBPS, RiskEvidenceStatus: riskEvidenceStatus, PlatformFitEvidenceStatus: fitEvidenceStatus}}
}

func platformFit(in Input) (int64, string, []string) {
	fit := int64(0)
	evidence := []string{}
	price := in.Pricing.SuggestedSalePrice
	if price >= 1000 && price <= 50000 {
		fit += 20
		evidence = append(evidence, "售价处于当前平台基础适配区间")
	} else if price >= 500 && price <= 100000 {
		fit += 10
		evidence = append(evidence, "售价处于平台边缘适配区间")
	}
	switch {
	case in.Product.SKUCount > 0 && in.Product.SKUCount <= 5:
		fit += 20
	case in.Product.SKUCount <= 20:
		fit += 14
	case in.Product.SKUCount <= 50:
		fit += 8
	case in.Product.SKUCount > 50:
		fit += 2
	}
	if in.Product.Title != "" {
		fit += 5
	}
	if in.Product.Description != "" {
		fit += 5
	}
	if in.Product.ImageCount >= 3 {
		fit += 5
	}
	if in.Product.Category != "" {
		fit += 5
	}
	knownOperationalFactors := 0
	if strings.EqualFold(in.Product.AfterSaleComplexity, "low") {
		fit += 10
		knownOperationalFactors++
	} else if strings.TrimSpace(in.Product.AfterSaleComplexity) != "" {
		knownOperationalFactors++
	}
	if strings.TrimSpace(in.Product.DeliveryMode) != "" {
		knownOperationalFactors++
		if strings.EqualFold(in.Product.DeliveryMode, "standard_shipping") || strings.EqualFold(in.Product.DeliveryMode, "digital") {
			fit += 10
		}
	}
	if in.Product.SmallSellerSuitable != nil {
		knownOperationalFactors++
		if *in.Product.SmallSellerSuitable {
			fit += 10
		}
	}
	if in.Product.RequiresQualification != nil {
		knownOperationalFactors++
		if !*in.Product.RequiresQualification {
			fit += 10
		}
	}
	status := "partial_platform_evidence"
	if knownOperationalFactors == 4 {
		status = "verified_platform_evidence"
	}
	evidence = append(evidence, "平台适配同时考虑售价、SKU复杂度、内容、售后、交付、小卖家能力与资质；未知项不作为已适配证据")
	return clamp(fit), status, evidence
}
