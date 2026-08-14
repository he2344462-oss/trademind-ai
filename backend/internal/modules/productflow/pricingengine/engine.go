package pricingengine

import (
	"fmt"
	"strings"
)

type Profile struct {
	Code             string `json:"code"`
	Platform         string `json:"platform"`
	Currency         string `json:"currency"`
	Configured       bool   `json:"configured"`
	PlatformFeeBPS   int64  `json:"platformFeeBps"`
	PlatformFeeFixed Money  `json:"platformFeeFixed"`
	PaymentFeeBPS    int64  `json:"paymentFeeBps"`
	PaymentFeeFixed  Money  `json:"paymentFeeFixed"`
	ReturnReserveBPS int64  `json:"returnReserveBps"`
	OtherBPS         int64  `json:"otherBps"`
	OtherFixed       Money  `json:"otherFixed"`
}

func DefaultProfile(platform string) Profile {
	return Profile{Code: platform + "-default", Platform: platform, Currency: "CNY", Configured: false}
}

type CostInput struct {
	Currency             string  `json:"currency"`
	PurchaseCost         Money   `json:"purchaseCost"`
	FreightCost          Money   `json:"freightCost"`
	FreightStatus        string  `json:"freightStatus"`
	FreightConfidenceBPS int64   `json:"freightConfidenceBps"`
	PackagingCost        Money   `json:"packagingCost"`
	OtherCost            Money   `json:"otherCost"`
	ExpectedReturnLoss   Money   `json:"expectedReturnLoss"`
	AfterSaleReserve     Money   `json:"afterSaleReserve"`
	DiscountBuffer       Money   `json:"discountBuffer"`
	CouponBuffer         Money   `json:"couponBuffer"`
	TargetProfit         Money   `json:"targetProfit"`
	TargetMarginBPS      int64   `json:"targetMarginBps"`
	MinimumProfit        Money   `json:"minimumProfit"`
	MinimumMarginBPS     int64   `json:"minimumMarginBps"`
	SalePrice            *Money  `json:"salePrice,omitempty"`
	Profile              Profile `json:"profile"`
}

type PricingResult struct {
	Currency               string   `json:"currency"`
	BaseCost               Money    `json:"baseCost"`
	TotalFixedCost         Money    `json:"totalFixedCost"`
	EstimatedPlatformFee   Money    `json:"estimatedPlatformFee"`
	EstimatedPaymentFee    Money    `json:"estimatedPaymentFee"`
	EstimatedAfterSaleLoss Money    `json:"estimatedAfterSaleLoss"`
	EstimatedTotalCost     Money    `json:"estimatedTotalCost"`
	BreakEvenPrice         Money    `json:"breakEvenPrice"`
	MinimumSalePrice       Money    `json:"minimumSalePrice"`
	SuggestedSalePrice     Money    `json:"suggestedSalePrice"`
	EstimatedProfit        Money    `json:"estimatedProfit"`
	EstimatedMarginBPS     int64    `json:"estimatedMarginBps"`
	MarkupRateBPS          int64    `json:"markupRateBps"`
	Profile                Profile  `json:"profile"`
	FreightStatus          string   `json:"freightStatus"`
	FreightCost            Money    `json:"freightCost"`
	FreightIncluded        bool     `json:"freightIncluded"`
	ProfitBasis            string   `json:"profitBasis"`
	Warnings               []string `json:"warnings"`
}

func ValidateProfile(p Profile) error {
	for _, rate := range []int64{p.PlatformFeeBPS, p.PaymentFeeBPS, p.ReturnReserveBPS, p.OtherBPS} {
		if rate < 0 || rate >= 10000 {
			return fmt.Errorf("profile rate must be between 0 and 9999 bps")
		}
	}
	return nil
}

func Calculate(in CostInput) (PricingResult, error) {
	if in.Currency == "" {
		in.Currency = "CNY"
	}
	if err := ValidateProfile(in.Profile); err != nil {
		return PricingResult{}, err
	}
	freightStatus := strings.ToLower(strings.TrimSpace(in.FreightStatus))
	if freightStatus == "" {
		// Backward compatible for callers that predate freight evidence.
		freightStatus = "verified"
		if in.FreightConfidenceBPS == 0 {
			in.FreightConfidenceBPS = 10000
		}
	}
	if freightStatus != "verified" && freightStatus != "estimated" && freightStatus != "free_shipping" && freightStatus != "unknown" {
		return PricingResult{}, fmt.Errorf("invalid freight status")
	}
	freightIncluded := freightStatus != "unknown"
	profitBasis := "complete"
	warnings := []string{}
	if !freightIncluded {
		profitBasis = "excluding_freight"
		warnings = append(warnings, "采购运费待确认：当前利润未包含可靠采购运费，仅供初筛")
	}
	for _, value := range []Money{in.PurchaseCost, in.FreightCost, in.PackagingCost, in.OtherCost, in.ExpectedReturnLoss, in.AfterSaleReserve, in.DiscountBuffer, in.CouponBuffer, in.TargetProfit, in.MinimumProfit} {
		if value < 0 {
			return PricingResult{}, fmt.Errorf("cost values must be non-negative")
		}
	}
	rateBPS := in.Profile.PlatformFeeBPS + in.Profile.PaymentFeeBPS + in.Profile.ReturnReserveBPS + in.Profile.OtherBPS
	if rateBPS >= 10000 {
		return PricingResult{}, fmt.Errorf("combined variable rate must be below 100%%")
	}
	base := in.PurchaseCost
	fixedBusiness := in.PurchaseCost + in.FreightCost + in.PackagingCost + in.OtherCost
	fixedAll := fixedBusiness + in.Profile.PlatformFeeFixed + in.Profile.PaymentFeeFixed + in.Profile.OtherFixed + in.ExpectedReturnLoss + in.AfterSaleReserve + in.DiscountBuffer + in.CouponBuffer
	breakEven := Money(ceilDiv(int64(fixedAll)*10000, 10000-rateBPS))
	profitPrice := Money(ceilDiv(int64(fixedAll+in.MinimumProfit)*10000, 10000-rateBPS))
	marginDenominator := int64(10000 - rateBPS - in.MinimumMarginBPS)
	marginPrice := breakEven
	if in.MinimumMarginBPS > 0 && marginDenominator > 0 {
		marginPrice = Money(ceilDiv(int64(fixedAll)*10000, marginDenominator))
	}
	minimum := profitPrice
	if marginPrice > minimum {
		minimum = marginPrice
	}
	targetProfitPrice := Money(ceilDiv(int64(fixedAll+in.TargetProfit)*10000, 10000-rateBPS))
	targetMarginPrice := minimum
	targetDenominator := int64(10000 - rateBPS - in.TargetMarginBPS)
	if in.TargetMarginBPS > 0 && targetDenominator > 0 {
		targetMarginPrice = Money(ceilDiv(int64(fixedAll)*10000, targetDenominator))
	}
	suggested := targetProfitPrice
	if targetMarginPrice > suggested {
		suggested = targetMarginPrice
	}
	if minimum > suggested {
		suggested = minimum
	}
	if in.SalePrice != nil {
		suggested = *in.SalePrice
	}
	platformFee := rateAmount(suggested, in.Profile.PlatformFeeBPS) + in.Profile.PlatformFeeFixed
	paymentFee := rateAmount(suggested, in.Profile.PaymentFeeBPS) + in.Profile.PaymentFeeFixed
	afterSale := rateAmount(suggested, in.Profile.ReturnReserveBPS) + in.ExpectedReturnLoss + in.AfterSaleReserve
	otherVariable := rateAmount(suggested, in.Profile.OtherBPS) + in.Profile.OtherFixed
	total := fixedBusiness + platformFee + paymentFee + afterSale + otherVariable + in.DiscountBuffer + in.CouponBuffer
	profit := suggested - total
	margin, markup := int64(0), int64(0)
	if suggested != 0 {
		margin = int64(profit) * 10000 / int64(suggested)
	}
	if total != 0 {
		markup = int64(profit) * 10000 / int64(total)
	}
	return PricingResult{Currency: in.Currency, BaseCost: base, TotalFixedCost: fixedBusiness, EstimatedPlatformFee: platformFee, EstimatedPaymentFee: paymentFee, EstimatedAfterSaleLoss: afterSale, EstimatedTotalCost: total, BreakEvenPrice: breakEven, MinimumSalePrice: minimum, SuggestedSalePrice: suggested, EstimatedProfit: profit, EstimatedMarginBPS: margin, MarkupRateBPS: markup, Profile: in.Profile, FreightStatus: freightStatus, FreightCost: in.FreightCost, FreightIncluded: freightIncluded, ProfitBasis: profitBasis, Warnings: warnings}, nil
}
