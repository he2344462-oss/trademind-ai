package productflow

import (
	"encoding/json"

	"github.com/google/uuid"
)

type CreateSourceProductBody struct {
	SourcePlatform      string          `json:"sourcePlatform" binding:"required"`
	SourceProductID     string          `json:"sourceProductId" binding:"required"`
	SourceURL           string          `json:"sourceUrl" binding:"required"`
	SupplierID          string          `json:"supplierId"`
	SupplierName        string          `json:"supplierName"`
	OriginalTitle       string          `json:"originalTitle" binding:"required"`
	OriginalDescription string          `json:"originalDescription"`
	OriginalImages      json.RawMessage `json:"originalImages"`
	OriginalCategory    string          `json:"originalCategory"`
	SourcePrice         *float64        `json:"sourcePrice"`
	Freight             *float64        `json:"freight"`
	MinOrderQuantity    int             `json:"minOrderQuantity"`
	SKUData             json.RawMessage `json:"skuData"`
	RawData             json.RawMessage `json:"rawData"`
}

type TransitionCandidateBody struct {
	RejectionReason string `json:"rejectionReason"`
}

type AnalyzeCandidateBody struct {
	AnalysisMode       string             `json:"analysisMode"`
	Platform           string             `json:"platform"`
	PackagingCost      string             `json:"packagingCost"`
	OtherCost          string             `json:"otherCost"`
	ExpectedReturnLoss string             `json:"expectedReturnLoss"`
	AfterSaleReserve   string             `json:"afterSaleReserve"`
	DiscountBuffer     string             `json:"discountBuffer"`
	CouponBuffer       string             `json:"couponBuffer"`
	TargetProfit       string             `json:"targetProfit"`
	TargetMarginBPS    int64              `json:"targetMarginBps"`
	MinimumProfit      string             `json:"minimumProfit"`
	MinimumMarginBPS   int64              `json:"minimumMarginBps"`
	SalePrice          string             `json:"salePrice"`
	PricingProfile     PricingProfileBody `json:"pricingProfile"`
}

type PricingProfileBody struct {
	Code             string `json:"code"`
	PlatformFeeBPS   int64  `json:"platformFeeBps"`
	PlatformFeeFixed string `json:"platformFeeFixed"`
	PaymentFeeBPS    int64  `json:"paymentFeeBps"`
	PaymentFeeFixed  string `json:"paymentFeeFixed"`
	ReturnReserveBPS int64  `json:"returnReserveBps"`
	OtherBPS         int64  `json:"otherBps"`
	OtherFixed       string `json:"otherFixed"`
}

type CreateListingDraftBody struct {
	Platform       string              `json:"platform" binding:"required"`
	ShopID         *uuid.UUID          `json:"shopId"`
	PricingProfile *PricingProfileBody `json:"pricingProfile"`
}

type UpdateListingDraftBody struct {
	Title            *string         `json:"title"`
	Description      *string         `json:"description"`
	Images           json.RawMessage `json:"images"`
	SalePrice        *float64        `json:"salePrice"`
	PlatformCategory *string         `json:"platformCategory"`
	PlatformSKUData  json.RawMessage `json:"platformSkuData"`
	PublishStatus    *string         `json:"publishStatus"`
}

type ListQuery struct {
	Page      int
	PageSize  int
	Status    string
	Platform  string
	Keyword   string
	CatalogID *uuid.UUID
	SortBy    string
	SortOrder string
}

type PageResult[T any] struct {
	List       []T   `json:"list"`
	Page       int   `json:"page"`
	PageSize   int   `json:"pageSize"`
	Total      int64 `json:"total"`
	TotalPages int   `json:"totalPages"`
}

type ApproveResult struct {
	Candidate Candidate `json:"candidate"`
	Catalog   any       `json:"catalogProduct"`
	Created   bool      `json:"created"`
}

type CostCenterSummary struct {
	ProductCount     int64 `json:"productCount"`
	AverageMarginBPS int64 `json:"averageMarginBps"`
	LowProfitCount   int64 `json:"lowProfitCount"`
	HighProfitCount  int64 `json:"highProfitCount"`
	CostAnomalyCount int64 `json:"costAnomalyCount"`
	Products         []any `json:"products"`
}
