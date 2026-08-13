package productflow

import (
	"encoding/json"
	"time"

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
	PricingProfileID   *uuid.UUID         `json:"pricingProfileId"`
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
	Platform         string              `json:"platform" binding:"required"`
	ShopID           *uuid.UUID          `json:"shopId"`
	PricingProfile   *PricingProfileBody `json:"pricingProfile"`
	PricingProfileID *uuid.UUID          `json:"pricingProfileId"`
}

type PricingProfileInput struct {
	Name             string `json:"name" binding:"required"`
	Platform         string `json:"platform" binding:"required"`
	Currency         string `json:"currency"`
	PlatformFeeBPS   int64  `json:"platformFeeBps"`
	PlatformFeeFixed string `json:"platformFeeFixed"`
	PaymentFeeBPS    int64  `json:"paymentFeeBps"`
	PaymentFeeFixed  string `json:"paymentFeeFixed"`
	ReturnReserveBPS int64  `json:"returnReserveBps"`
	OtherBPS         int64  `json:"otherBps"`
	OtherFixed       string `json:"otherFixed"`
	IsDefault        bool   `json:"isDefault"`
	Enabled          *bool  `json:"enabled"`
}

type BatchFilters struct {
	Status         string `json:"status"`
	SourcePlatform string `json:"sourcePlatform"`
	UnanalyzedOnly bool   `json:"unanalyzedOnly"`
	Limit          int    `json:"limit"`
}

type AnalyzeBatchBody struct {
	CandidateIDs      []uuid.UUID  `json:"candidateIds"`
	Filters           BatchFilters `json:"filters"`
	Platform          string       `json:"platform"`
	AnalysisMode      string       `json:"analysisMode"`
	PricingProfileID  *uuid.UUID   `json:"pricingProfileId"`
	MinimumScore      int64        `json:"minimumScore"`
	MinimumMarginBPS  int64        `json:"minimumMarginBps"`
	MinimumProfit     string       `json:"minimumProfit"`
	MinimumConfidence int64        `json:"minimumConfidence"`
	ExcludeBlocked    *bool        `json:"excludeBlocked"`
	TopN              int          `json:"topN"`
}

type BulkCandidateActionBody struct {
	CandidateIDs []uuid.UUID `json:"candidateIds" binding:"required"`
	Reason       string      `json:"reason"`
	Source       string      `json:"source"`
	AllowBlocked bool        `json:"allowBlocked"`
}

type BulkSourceImportBody struct {
	Items []CreateSourceProductBody `json:"items" binding:"required"`
}

type RecalculateListingBody struct {
	PricingProfileID *uuid.UUID `json:"pricingProfileId"`
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

type SelectionDashboard struct {
	TodaySources               int64                    `json:"todaySources"`
	PendingCandidates          int64                    `json:"pendingCandidates"`
	TodayAnalyzed              int64                    `json:"todayAnalyzed"`
	StrongRecommend            int64                    `json:"strongRecommend"`
	Recommend                  int64                    `json:"recommend"`
	Watch                      int64                    `json:"watch"`
	Reject                     int64                    `json:"reject"`
	AverageMarginBPS           int64                    `json:"averageMarginBps"`
	RunningBatches             []CandidateAnalysisBatch `json:"runningBatches"`
	PausedBatches              int64                    `json:"pausedBatches"`
	FailedBatches              int64                    `json:"failedBatches"`
	MarketCoverageBPS          int64                    `json:"marketCoverageBps"`
	RealMarketCoverageBPS      int64                    `json:"realMarketCoverageBps"`
	RealPerformanceCoverageBPS int64                    `json:"realPerformanceCoverageBps"`
	CalibrationSampleCount     int64                    `json:"calibrationSampleCount"`
	CalibrationReadiness       string                   `json:"calibrationReadiness"`
	SelectionConfigVersion     string                   `json:"selectionConfigVersion"`
	PerformanceUpdatedAt       *time.Time               `json:"performanceUpdatedAt,omitempty"`
	OperationalSummary         string                   `json:"operationalSummary"`
}
