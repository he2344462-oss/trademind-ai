package productflow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/rankingengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/selectionengine"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SKUProfit struct {
	SKUCode            string              `json:"skuCode"`
	PurchaseCost       pricingengine.Money `json:"purchaseCost"`
	SuggestedSalePrice pricingengine.Money `json:"suggestedSalePrice"`
	EstimatedProfit    pricingengine.Money `json:"estimatedProfit"`
	EstimatedMarginBPS int64               `json:"estimatedMarginBps"`
}
type SKUSummary struct {
	Items               []SKUProfit `json:"items"`
	MinimumSKUMarginBPS *int64      `json:"minimumSkuMarginBps"`
	MaximumSKUMarginBPS *int64      `json:"maximumSkuMarginBps"`
	AverageSKUMarginBPS *int64      `json:"averageSkuMarginBps"`
	Warnings            []string    `json:"warnings"`
}
type Explanation struct {
	Conclusion         string   `json:"conclusion"`
	Reasons            []string `json:"reasons"`
	Risks              []string `json:"risks"`
	PricingExplanation string   `json:"pricingExplanation"`
	NextStep           string   `json:"nextStep"`
	Source             string   `json:"source"`
}
type AnalysisOutput struct {
	Analysis    CandidateAnalysis           `json:"analysis"`
	Cost        pricingengine.PricingResult `json:"cost"`
	SKUProfit   SKUSummary                  `json:"skuProfit"`
	Score       selectionengine.Result      `json:"score"`
	Explanation Explanation                 `json:"explanation"`
}

func parseOptionalMoney(value string) (pricingengine.Money, error) {
	return pricingengine.ParseMoney(value)
}
func moneyFloat(m pricingengine.Money) *float64 { v := float64(m) / 100; return &v }
func bpsFloat(bps int64) *float64               { v := float64(bps) / 10000; return &v }

func profileFromBody(platform string, body PricingProfileBody) (pricingengine.Profile, error) {
	p := pricingengine.DefaultProfile(platform)
	if strings.TrimSpace(body.Code) != "" {
		p.Code = strings.TrimSpace(body.Code)
		p.Configured = true
	}
	var err error
	p.PlatformFeeBPS = body.PlatformFeeBPS
	p.PaymentFeeBPS = body.PaymentFeeBPS
	p.ReturnReserveBPS = body.ReturnReserveBPS
	p.OtherBPS = body.OtherBPS
	if p.PlatformFeeFixed, err = parseOptionalMoney(body.PlatformFeeFixed); err != nil {
		return p, err
	}
	if p.PaymentFeeFixed, err = parseOptionalMoney(body.PaymentFeeFixed); err != nil {
		return p, err
	}
	if p.OtherFixed, err = parseOptionalMoney(body.OtherFixed); err != nil {
		return p, err
	}
	if body.PlatformFeeBPS != 0 || body.PaymentFeeBPS != 0 || body.ReturnReserveBPS != 0 || body.OtherBPS != 0 || p.PlatformFeeFixed != 0 || p.PaymentFeeFixed != 0 || p.OtherFixed != 0 {
		p.Configured = true
	}
	return p, pricingengine.ValidateProfile(p)
}

func pricingInput(source SourceProduct, body AnalyzeCandidateBody, profile pricingengine.Profile) (pricingengine.CostInput, error) {
	purchase, err := sourcePricingPurchaseCost(source)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	freight := pricingengine.Money(0)
	if source.FreightAmount != nil {
		freight = pricingengine.Money(*source.FreightAmount)
	} else {
		var err error
		freight, _, err = pricingengine.MoneyFromLegacyFloat(source.Freight)
		if err != nil {
			return pricingengine.CostInput{}, err
		}
	}
	freightStatus := source.FreightStatus
	if strings.TrimSpace(freightStatus) == "" {
		if source.Freight != nil {
			freightStatus = FreightStatusVerified
		} else {
			freightStatus = FreightStatusUnknown
		}
	}
	packaging, err := parseOptionalMoney(body.PackagingCost)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	other, err := parseOptionalMoney(body.OtherCost)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	returnLoss, err := parseOptionalMoney(body.ExpectedReturnLoss)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	afterSale, err := parseOptionalMoney(body.AfterSaleReserve)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	discount, err := parseOptionalMoney(body.DiscountBuffer)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	coupon, err := parseOptionalMoney(body.CouponBuffer)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	targetProfit, err := parseOptionalMoney(body.TargetProfit)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	minimumProfit, err := parseOptionalMoney(body.MinimumProfit)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	if targetProfit == 0 {
		targetProfit = 1000
	}
	if minimumProfit == 0 {
		minimumProfit = 500
	}
	if body.TargetMarginBPS == 0 {
		body.TargetMarginBPS = 4000
	}
	if body.MinimumMarginBPS == 0 {
		body.MinimumMarginBPS = 2000
	}
	var sale *pricingengine.Money
	if strings.TrimSpace(body.SalePrice) != "" {
		v, e := pricingengine.ParseMoney(body.SalePrice)
		if e != nil {
			return pricingengine.CostInput{}, e
		}
		sale = &v
	}
	return pricingengine.CostInput{Currency: "CNY", PurchaseCost: purchase, FreightCost: freight, FreightStatus: freightStatus, FreightConfidenceBPS: source.FreightConfidenceBPS, PackagingCost: packaging, OtherCost: other, ExpectedReturnLoss: returnLoss, AfterSaleReserve: afterSale, DiscountBuffer: discount, CouponBuffer: coupon, TargetProfit: targetProfit, TargetMarginBPS: body.TargetMarginBPS, MinimumProfit: minimumProfit, MinimumMarginBPS: body.MinimumMarginBPS, SalePrice: sale, Profile: profile}, nil
}

func analyzeSKUs(source SourceProduct, in pricingengine.CostInput) (SKUSummary, error) {
	var skus []map[string]any
	if len(source.SKUData) > 0 {
		_ = json.Unmarshal(source.SKUData, &skus)
	}
	out := SKUSummary{Items: []SKUProfit{}, Warnings: []string{}}
	var sum int64
	for i, sku := range skus {
		code := stringValue(sku, "skuCode", "SKUCode", "id")
		if code == "" {
			code = fmt.Sprintf("SKU-%d", i+1)
		}
		price := numberValue(sku, "costPrice", "price", "CostPrice", "Price")
		skuIn := in
		if price != nil {
			m, _, e := pricingengine.MoneyFromLegacyFloat(price)
			if e != nil {
				return out, e
			}
			skuIn.PurchaseCost = m
		}
		if extra := numberValue(sku, "extraCost", "ExtraCost"); extra != nil {
			m, _, e := pricingengine.MoneyFromLegacyFloat(extra)
			if e != nil {
				return out, e
			}
			skuIn.OtherCost += m
		}
		result, e := pricingengine.Calculate(skuIn)
		if e != nil {
			return out, e
		}
		out.Items = append(out.Items, SKUProfit{SKUCode: code, PurchaseCost: skuIn.PurchaseCost, SuggestedSalePrice: result.SuggestedSalePrice, EstimatedProfit: result.EstimatedProfit, EstimatedMarginBPS: result.EstimatedMarginBPS})
		v := result.EstimatedMarginBPS
		sum += v
		if out.MinimumSKUMarginBPS == nil || v < *out.MinimumSKUMarginBPS {
			x := v
			out.MinimumSKUMarginBPS = &x
		}
		if out.MaximumSKUMarginBPS == nil || v > *out.MaximumSKUMarginBPS {
			x := v
			out.MaximumSKUMarginBPS = &x
		}
		if v < in.MinimumMarginBPS {
			out.Warnings = append(out.Warnings, "SKU "+code+" 利润率低于最低目标")
		}
	}
	if len(out.Items) > 0 {
		x := sum / int64(len(out.Items))
		out.AverageSKUMarginBPS = &x
	}
	return out, nil
}

func templateExplanation(score selectionengine.Result, cost pricingengine.PricingResult) Explanation {
	conclusion := map[string]string{selectionengine.RecommendationStrong: "强烈建议测试", selectionengine.RecommendationRecommend: "建议测试", selectionengine.RecommendationWatch: "建议观察", selectionengine.RecommendationReject: "不建议进入经营"}[score.Recommendation]
	if score.Recommendation == selectionengine.RecommendationWatch && score.EvidenceCoverageBPS == 0 {
		conclusion = "经营条件合格，待市场验证"
	}
	return Explanation{Conclusion: conclusion, Reasons: score.Reasons, Risks: append(score.Warnings, score.Blockers...), PricingExplanation: fmt.Sprintf("建议售价 ¥%s，预计利润 ¥%s，预计利润率 %.2f%%。", cost.SuggestedSalePrice.String(), cost.EstimatedProfit.String(), float64(cost.EstimatedMarginBPS)/100), NextStep: "先核验供应和类目资料，再由人工决定是否批准进入商品库。", Source: "rules_template"}
}

func (s *Service) AnalyzeCandidate(ctx context.Context, tenantID int64, id uuid.UUID, body AnalyzeCandidateBody) (*AnalysisOutput, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("product flow unavailable")
	}
	mode := strings.ToLower(strings.TrimSpace(body.AnalysisMode))
	if mode == "" {
		mode = "rules_only"
	}
	if mode != "rules_only" && mode != "ai_explanation" {
		return nil, fmt.Errorf("%w: unsupported analysis mode", ErrValidation)
	}
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	if platform == "" {
		platform = "xianyu"
	}
	if !SupportedDraftPlatform(platform) {
		return nil, fmt.Errorf("%w: unsupported platform", ErrValidation)
	}
	candidate, err := s.GetCandidate(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	body, err = s.effectiveAnalyzeCandidateBody(ctx, tenantID, id, body)
	if err != nil {
		return nil, err
	}
	preserveApprovedStatus := candidate.Status == CandidateStatusApproved
	profile, profileID, err := s.resolvePricingProfile(ctx, tenantID, platform, body.PricingProfileID, body.PricingProfile)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	profileVersion := 1
	if profileID != nil {
		if profileRow, profileErr := s.GetPricingProfile(ctx, tenantID, *profileID); profileErr == nil {
			profileVersion = profileRow.Version
		}
	}
	costIn, err := pricingInput(candidate.SourceProduct, body, profile)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	cost, err := pricingengine.Calculate(costIn)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	skuInput := costIn
	skuInput.SalePrice = &cost.SuggestedSalePrice
	sku, err := analyzeSKUs(candidate.SourceProduct, skuInput)
	if err != nil {
		return nil, err
	}
	market, marketViews, err := s.marketDimensions(ctx, tenantID, id, platform)
	if err != nil {
		return nil, err
	}
	var images []string
	_ = json.Unmarshal(candidate.SourceProduct.OriginalImages, &images)
	var rawSKUs []map[string]any
	_ = json.Unmarshal(candidate.SourceProduct.SKUData, &rawSKUs)
	complete := 0
	hasStock := false
	for _, v := range rawSKUs {
		if stringValue(v, "skuCode", "SKUCode", "id") != "" && numberValue(v, "price", "costPrice", "Price", "CostPrice") != nil {
			complete++
		}
		if intValue(v, "stock", "Stock") != nil {
			hasStock = true
		}
	}
	selectionConfig, scoreCfg, err := s.activeSelectionConfig(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	freightKnown := candidate.SourceProduct.FreightStatus == FreightStatusVerified || candidate.SourceProduct.FreightStatus == FreightStatusEstimated || candidate.SourceProduct.FreightStatus == FreightStatusFreeShipping
	score := selectionengine.Score(selectionengine.Input{Pricing: cost, MinimumSKUMarginBPS: sku.MinimumSKUMarginBPS, Market: market, Product: selectionengine.ProductData{Title: candidate.SourceProduct.OriginalTitle, Description: candidate.SourceProduct.OriginalDescription, ImageCount: len(images), SKUCount: len(rawSKUs), CompleteSKUCount: complete, HasPurchaseCost: candidate.SourceProduct.SourcePrice != nil, HasFreight: freightKnown, FreightConfidenceBPS: candidate.SourceProduct.FreightConfidenceBPS, Supplier: candidate.SourceProduct.SupplierName, SourceURL: candidate.SourceProduct.SourceURL, Category: candidate.SourceProduct.OriginalCategory, MOQ: candidate.SourceProduct.MinOrderQuantity, HasStockData: hasStock, CollectedAt: candidate.SourceProduct.CollectedAt, Platform: platform}}, scoreCfg)
	explanation := templateExplanation(score, cost)
	aiStatus := "not_requested"
	if mode == "ai_explanation" && s.AIExplain != nil {
		promptBytes, _ := json.Marshal(map[string]any{"cost": cost, "score": score, "instruction": "仅解释结构化结果，不修改任何分数、利润、利润率或 blocker"})
		if content, e := s.AIExplain(ctx, string(promptBytes)); e == nil && strings.TrimSpace(content) != "" {
			var ai Explanation
			if json.Unmarshal([]byte(strings.TrimSpace(content)), &ai) == nil && strings.TrimSpace(ai.Conclusion) != "" {
				ai.Source = "ai"
				explanation = ai
				aiStatus = "success"
			} else {
				aiStatus = "fallback"
			}
		} else {
			aiStatus = "fallback"
		}
	}
	inputJSON, _ := json.Marshal(map[string]any{"sourceProduct": candidate.SourceProduct, "request": body, "scoreConfig": scoreCfg, "scoringAlgorithmVersion": selectionengine.ConfigVersion})
	costJSON, _ := json.Marshal(cost)
	skuJSON, _ := json.Marshal(sku)
	scoreJSON, _ := json.Marshal(score.Dimensions)
	marketJSON, _ := json.Marshal(marketViews)
	confidenceJSON, _ := json.Marshal(score.ConfidenceBreakdown)
	reasons, _ := json.Marshal(score.Reasons)
	warnings, _ := json.Marshal(score.Warnings)
	blockers, _ := json.Marshal(score.Blockers)
	explainJSON, _ := json.Marshal(explanation)
	now := time.Now().UTC()
	var saved CandidateAnalysis
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var locked Candidate
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&locked).Error; e != nil {
			return e
		}
		version := locked.AnalysisVersion + 1
		saved = CandidateAnalysis{TenantID: tenantID, CandidateID: id, AnalysisVersion: version, AnalysisMode: mode, Platform: platform, InputSnapshot: inputJSON, CostSnapshot: costJSON, SKUSnapshot: skuJSON, ScoreBreakdown: scoreJSON, OverallScore: score.OverallScore, ConfidenceScore: score.ConfidenceScore, Recommendation: score.Recommendation, Reasons: reasons, Warnings: warnings, Blockers: blockers, Explanation: explainJSON, AIStatus: aiStatus, PricingProfileID: profileID, MarketSignalSnapshot: marketJSON, ConfidenceBreakdown: confidenceJSON, ScoringConfigVersion: selectionengine.ConfigVersion, SelectionConfigID: &selectionConfig.ID, SelectionConfigVersion: selectionConfig.Version, PricingProfileVersion: profileVersion, RankingConfigVersion: rankingengine.ConfigVersion}
		if e := tx.Create(&saved).Error; e != nil {
			return e
		}
		status := CandidateStatusRecommended
		if score.Recommendation == selectionengine.RecommendationWatch {
			status = CandidateStatusWatch
		} else if score.Recommendation == selectionengine.RecommendationReject {
			status = CandidateStatusRejected
		}
		if preserveApprovedStatus {
			status = CandidateStatusApproved
		}
		updates := map[string]any{"status": status, "potential_score": score.OverallScore, "profit_score": score.Dimensions["profit"].Score, "data_quality_score": score.Dimensions["data_quality"].Score, "supply_score": score.Dimensions["supply"].Score, "risk_score": score.Dimensions["risk"].Score, "demand_score": score.Dimensions["demand"].Score, "competition_score": score.Dimensions["competition"].Score, "ai_score": nil, "confidence_score": float64(score.ConfidenceScore) / 100, "analysis_version": version, "analyzed_at": now, "recommendation": score.Recommendation, "analysis_summary": explanation.Conclusion, "estimated_cost": moneyFloat(cost.EstimatedTotalCost), "estimated_sale_price": moneyFloat(cost.SuggestedSalePrice), "estimated_profit": moneyFloat(cost.EstimatedProfit), "estimated_margin": bpsFloat(cost.EstimatedMarginBPS)}
		return tx.Model(&locked).Updates(updates).Error
	})
	if err != nil {
		return nil, err
	}
	return &AnalysisOutput{Analysis: saved, Cost: cost, SKUProfit: sku, Score: score, Explanation: explanation}, nil
}

func (s *Service) LatestAnalysis(ctx context.Context, tenantID int64, candidateID uuid.UUID) (*CandidateAnalysis, error) {
	var row CandidateAnalysis
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, candidateID).Order("analysis_version DESC").First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}
func (s *Service) ListAnalyses(ctx context.Context, tenantID int64, candidateID uuid.UUID) ([]CandidateAnalysis, error) {
	var rows []CandidateAnalysis
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id = ?", tenantID, candidateID).Order("analysis_version DESC").Find(&rows).Error
	return rows, err
}
