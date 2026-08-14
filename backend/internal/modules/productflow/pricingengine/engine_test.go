package pricingengine

import (
	"github.com/stretchr/testify/require"
	"testing"
)

func mustMoney(t *testing.T, v string) Money {
	t.Helper()
	m, e := ParseMoney(v)
	require.NoError(t, e)
	return m
}

func TestMoneyDecimalPrecision(t *testing.T) {
	a := mustMoney(t, "0.10")
	b := mustMoney(t, "0.20")
	require.Equal(t, "0.30", (a + b).String())
	_, err := ParseMoney("1.001")
	require.Error(t, err)
}

func TestCalculateFullCostAndFees(t *testing.T) {
	in := CostInput{Currency: "CNY", PurchaseCost: mustMoney(t, "12.80"), FreightCost: mustMoney(t, "4.00"), PackagingCost: mustMoney(t, "0.50"), OtherCost: mustMoney(t, "0.50"), ExpectedReturnLoss: mustMoney(t, "1.50"), TargetProfit: mustMoney(t, "10.00"), TargetMarginBPS: 4000, MinimumProfit: mustMoney(t, "5.00"), MinimumMarginBPS: 2000, SalePrice: func() *Money { v := mustMoney(t, "39.90"); return &v }(), Profile: Profile{Code: "custom", Platform: "taobao", Currency: "CNY", Configured: true, PlatformFeeBPS: 300, PlatformFeeFixed: mustMoney(t, "0.10"), PaymentFeeBPS: 75, PaymentFeeFixed: mustMoney(t, "0.05")}}
	r, err := Calculate(in)
	require.NoError(t, err)
	require.Equal(t, "17.80", r.TotalFixedCost.String())
	require.Equal(t, "1.30", r.EstimatedPlatformFee.String())
	require.Equal(t, "0.35", r.EstimatedPaymentFee.String())
	require.Equal(t, "1.50", r.EstimatedAfterSaleLoss.String())
	require.Equal(t, "20.95", r.EstimatedTotalCost.String())
	require.Equal(t, "18.95", r.EstimatedProfit.String())
	require.EqualValues(t, 4749, r.EstimatedMarginBPS)
	require.Greater(t, r.MarkupRateBPS, r.EstimatedMarginBPS)
	require.LessOrEqual(t, r.BreakEvenPrice, r.MinimumSalePrice)
}

func TestCalculateZeroFreightNegativeProfitAndBreakEven(t *testing.T) {
	sale := mustMoney(t, "8.00")
	r, err := Calculate(CostInput{PurchaseCost: mustMoney(t, "10.00"), SalePrice: &sale, Profile: DefaultProfile("xianyu")})
	require.NoError(t, err)
	require.Equal(t, "-2.00", r.EstimatedProfit.String())
	require.Equal(t, "10.00", r.BreakEvenPrice.String())
}

func TestPlatformProfilesProduceDifferentResults(t *testing.T) {
	base := CostInput{PurchaseCost: mustMoney(t, "20"), TargetProfit: mustMoney(t, "10"), TargetMarginBPS: 3000, MinimumProfit: mustMoney(t, "5"), MinimumMarginBPS: 2000}
	x := base
	x.Profile = DefaultProfile("xianyu")
	xr, e := Calculate(x)
	require.NoError(t, e)
	tao := base
	tao.Profile = Profile{Code: "taobao-custom", Platform: "taobao", Configured: true, PlatformFeeBPS: 500}
	tr, e := Calculate(tao)
	require.NoError(t, e)
	require.Greater(t, tr.SuggestedSalePrice, xr.SuggestedSalePrice)
}

func TestUnknownFreightIsExplicitlyExcludedFromProfit(t *testing.T) {
	r, err := Calculate(CostInput{PurchaseCost: mustMoney(t, "6.50"), FreightStatus: "unknown", PackagingCost: mustMoney(t, "0.50"), OtherCost: mustMoney(t, "0.50"), ExpectedReturnLoss: mustMoney(t, "1.50"), TargetProfit: mustMoney(t, "10.00"), Profile: DefaultProfile("xianyu")})
	require.NoError(t, err)
	require.False(t, r.FreightIncluded)
	require.Equal(t, "excluding_freight", r.ProfitBasis)
	require.Equal(t, "9.00", r.EstimatedTotalCost.String())
	require.Contains(t, r.Warnings, "采购运费待确认：当前利润未包含可靠采购运费，仅供初筛")
}

func TestHighestSKUPurchaseCostPlusFreight(t *testing.T) {
	r, err := Calculate(CostInput{PurchaseCost: mustMoney(t, "6.50"), FreightCost: mustMoney(t, "2.00"), FreightStatus: "verified", FreightConfidenceBPS: 10000, PackagingCost: mustMoney(t, "0.50"), OtherCost: mustMoney(t, "0.50"), ExpectedReturnLoss: mustMoney(t, "1.50"), TargetProfit: mustMoney(t, "10.00"), Profile: DefaultProfile("xianyu")})
	require.NoError(t, err)
	require.True(t, r.FreightIncluded)
	require.Equal(t, "11.00", r.EstimatedTotalCost.String())
	require.Equal(t, "21.00", r.SuggestedSalePrice.String())
}
