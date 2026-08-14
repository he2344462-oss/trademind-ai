package productflow

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
)

// sourcePricingPurchaseCost returns the conservative purchase-cost basis used
// for a product-level analysis. SourcePrice remains the supplier's floor price;
// a shared product price must cover the most expensive real SKU.
func sourcePricingPurchaseCost(source SourceProduct) (pricingengine.Money, error) {
	fallback, ok, err := pricingengine.MoneyFromLegacyFloat(source.SourcePrice)
	if err != nil {
		return 0, err
	}
	if !ok {
		fallback = 0
	}

	var skus []map[string]any
	if len(source.SKUData) == 0 || json.Unmarshal(source.SKUData, &skus) != nil {
		return fallback, nil
	}
	maximum := fallback
	for _, sku := range skus {
		price := numberValue(sku, "costPrice", "price", "CostPrice", "Price")
		if price == nil {
			continue
		}
		money, present, convertErr := pricingengine.MoneyFromLegacyFloat(price)
		if convertErr != nil {
			return 0, convertErr
		}
		if present && money > maximum {
			maximum = money
		}
	}
	return maximum, nil
}

func catalogPricingPurchaseCost(catalog *product.Product) (pricingengine.Money, error) {
	fallback, ok, err := pricingengine.MoneyFromLegacyFloat(catalog.PurchaseCost)
	if err != nil {
		return 0, err
	}
	if !ok {
		fallback = 0
	}
	maximum := fallback
	for _, sku := range catalog.SKUs {
		money, present, convertErr := pricingengine.MoneyFromLegacyFloat(sku.CostPrice)
		if convertErr != nil {
			return 0, convertErr
		}
		if present && money > maximum {
			maximum = money
		}
	}
	return maximum, nil
}

func (s *Service) catalogPricingInput(ctx context.Context, tenantID int64, catalog *product.Product, platform string, profile pricingengine.Profile, salePrice *pricingengine.Money) (pricingengine.CostInput, error) {
	body := AnalyzeCandidateBody{Platform: platform}
	if catalog.CandidateID != nil {
		var err error
		body, err = s.effectiveAnalyzeCandidateBody(ctx, tenantID, *catalog.CandidateID, body)
		if err != nil {
			return pricingengine.CostInput{}, err
		}
	}
	purchase, err := catalogPricingPurchaseCost(catalog)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	purchaseFloat := float64(purchase) / 100
	source := SourceProduct{SourcePrice: &purchaseFloat, Freight: catalog.FreightCost,
		FreightStatus: catalog.FreightStatus, FreightAmount: catalog.FreightAmount,
		FreightConfidenceBPS: catalog.FreightConfidenceBPS}
	in, err := pricingInput(source, body, profile)
	if err != nil {
		return pricingengine.CostInput{}, err
	}
	// Catalog values are operator-controlled and take precedence over the
	// historical candidate snapshot after approval.
	if catalog.PackagingCost != nil {
		in.PackagingCost, _, err = pricingengine.MoneyFromLegacyFloat(catalog.PackagingCost)
		if err != nil {
			return pricingengine.CostInput{}, err
		}
	}
	if catalog.OtherCost != nil {
		in.OtherCost, _, err = pricingengine.MoneyFromLegacyFloat(catalog.OtherCost)
		if err != nil {
			return pricingengine.CostInput{}, err
		}
	}
	in.SalePrice = salePrice
	return in, nil
}

func pricingProfileBodyEmpty(body PricingProfileBody) bool {
	return strings.TrimSpace(body.Code) == "" && body.PlatformFeeBPS == 0 &&
		strings.TrimSpace(body.PlatformFeeFixed) == "" && body.PaymentFeeBPS == 0 &&
		strings.TrimSpace(body.PaymentFeeFixed) == "" && body.ReturnReserveBPS == 0 &&
		body.OtherBPS == 0 && strings.TrimSpace(body.OtherFixed) == ""
}

func fillMissingPricingInputs(current, previous AnalyzeCandidateBody) AnalyzeCandidateBody {
	if strings.TrimSpace(current.PackagingCost) == "" {
		current.PackagingCost = previous.PackagingCost
	}
	if strings.TrimSpace(current.OtherCost) == "" {
		current.OtherCost = previous.OtherCost
	}
	if strings.TrimSpace(current.ExpectedReturnLoss) == "" {
		current.ExpectedReturnLoss = previous.ExpectedReturnLoss
	}
	if strings.TrimSpace(current.AfterSaleReserve) == "" {
		current.AfterSaleReserve = previous.AfterSaleReserve
	}
	if strings.TrimSpace(current.DiscountBuffer) == "" {
		current.DiscountBuffer = previous.DiscountBuffer
	}
	if strings.TrimSpace(current.CouponBuffer) == "" {
		current.CouponBuffer = previous.CouponBuffer
	}
	if strings.TrimSpace(current.TargetProfit) == "" {
		current.TargetProfit = previous.TargetProfit
	}
	if current.TargetMarginBPS == 0 {
		current.TargetMarginBPS = previous.TargetMarginBPS
	}
	if strings.TrimSpace(current.MinimumProfit) == "" {
		current.MinimumProfit = previous.MinimumProfit
	}
	if current.MinimumMarginBPS == 0 {
		current.MinimumMarginBPS = previous.MinimumMarginBPS
	}
	if strings.TrimSpace(current.SalePrice) == "" {
		current.SalePrice = previous.SalePrice
	}
	return current
}

// effectiveAnalyzeCandidateBody carries forward the last explicitly supplied
// operating inputs. It walks history so a prior incomplete refresh cannot erase
// an older, valid operator decision. A newly supplied profile always wins.
func (s *Service) effectiveAnalyzeCandidateBody(ctx context.Context, tenantID int64, candidateID uuid.UUID, current AnalyzeCandidateBody) (AnalyzeCandidateBody, error) {
	var history []CandidateAnalysis
	err := s.DB.WithContext(ctx).
		Select("input_snapshot", "pricing_profile_id", "platform", "analysis_version").
		Where("tenant_id = ? AND candidate_id = ?", tenantID, candidateID).
		Order("analysis_version DESC").
		Find(&history).Error
	if err != nil {
		return current, err
	}

	requestedPlatform := strings.ToLower(strings.TrimSpace(current.Platform))
	if requestedPlatform == "" {
		requestedPlatform = "xianyu"
	}
	explicitProfile := current.PricingProfileID != nil || !pricingProfileBodyEmpty(current.PricingProfile)
	for _, analysis := range history {
		var snapshot struct {
			Request AnalyzeCandidateBody `json:"request"`
		}
		if json.Unmarshal(analysis.InputSnapshot, &snapshot) == nil {
			current = fillMissingPricingInputs(current, snapshot.Request)
			if !explicitProfile && strings.EqualFold(analysis.Platform, requestedPlatform) && !pricingProfileBodyEmpty(snapshot.Request.PricingProfile) {
				current.PricingProfile = snapshot.Request.PricingProfile
				explicitProfile = true
			}
		}
		if !explicitProfile && strings.EqualFold(analysis.Platform, requestedPlatform) && analysis.PricingProfileID != nil {
			profileID := *analysis.PricingProfileID
			current.PricingProfileID = &profileID
			explicitProfile = true
		}
	}
	return current, nil
}
