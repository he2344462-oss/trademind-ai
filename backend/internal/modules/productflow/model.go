package productflow

import (
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/model"
	"gorm.io/datatypes"
)

const (
	SourceStatusCollected = "collected"
	SourceStatusCandidate = "candidate"
	SourceStatusApproved  = "approved"
	SourceStatusArchived  = "archived"

	CandidateStatusPending     = "pending"
	CandidateStatusAnalyzing   = "analyzing"
	CandidateStatusRecommended = "recommended"
	CandidateStatusWatch       = "watch"
	CandidateStatusRejected    = "rejected"
	CandidateStatusApproved    = "approved"

	CatalogStatusDraft    = "draft"
	CatalogStatusActive   = "active"
	CatalogStatusPaused   = "paused"
	CatalogStatusArchived = "archived"

	ListingStatusDraft      = "draft"
	ListingStatusReady      = "ready"
	ListingStatusPublishing = "publishing"
	ListingStatusPublished  = "published"
	ListingStatusFailed     = "failed"
	ListingStatusOffline    = "offline"
)

// SourceProduct preserves the original supplier-platform payload before any operating edits.
type SourceProduct struct {
	model.Base
	TenantID            int64          `gorm:"not null;default:0;uniqueIndex:idx_source_product_identity,priority:1;index" json:"tenantId"`
	SourcePlatform      string         `gorm:"size:64;not null;uniqueIndex:idx_source_product_identity,priority:2;index" json:"sourcePlatform"`
	SourceProductID     string         `gorm:"size:256;not null;uniqueIndex:idx_source_product_identity,priority:3" json:"sourceProductId"`
	SourceURL           string         `gorm:"size:2048;not null" json:"sourceUrl"`
	SupplierID          string         `gorm:"size:256;index" json:"supplierId,omitempty"`
	SupplierName        string         `gorm:"size:256;index" json:"supplierName,omitempty"`
	OriginalTitle       string         `gorm:"size:512;not null;index" json:"originalTitle"`
	OriginalDescription string         `gorm:"type:text" json:"originalDescription,omitempty"`
	OriginalImages      datatypes.JSON `gorm:"type:jsonb" json:"originalImages,omitempty"`
	OriginalCategory    string         `gorm:"size:256;index" json:"originalCategory,omitempty"`
	SourcePrice         *float64       `gorm:"type:numeric(18,2)" json:"sourcePrice,omitempty"`
	Freight             *float64       `gorm:"type:numeric(18,2)" json:"freight,omitempty"`
	MinOrderQuantity    int            `gorm:"not null;default:1" json:"minOrderQuantity"`
	SKUData             datatypes.JSON `gorm:"type:jsonb" json:"skuData,omitempty"`
	RawData             datatypes.JSON `gorm:"type:jsonb" json:"rawData,omitempty"`
	CollectedAt         time.Time      `gorm:"index;not null" json:"collectedAt"`
	Status              string         `gorm:"size:32;index;not null;default:collected" json:"status"`
}

func (SourceProduct) TableName() string { return "source_products" }

// Candidate records the human selection workflow and reserved scoring dimensions.
type Candidate struct {
	model.Base
	TenantID           int64         `gorm:"not null;default:0;uniqueIndex:idx_candidate_source,priority:1;index" json:"tenantId"`
	SourceProductID    uuid.UUID     `gorm:"type:char(36);not null;uniqueIndex:idx_candidate_source,priority:2;index" json:"sourceProductId"`
	Status             string        `gorm:"size:32;index;not null;default:pending" json:"status"`
	PotentialScore     *float64      `gorm:"type:numeric(8,4)" json:"potentialScore,omitempty"`
	ProfitScore        *float64      `gorm:"type:numeric(8,4)" json:"profitScore,omitempty"`
	DemandScore        *float64      `gorm:"type:numeric(8,4)" json:"demandScore,omitempty"`
	CompetitionScore   *float64      `gorm:"type:numeric(8,4)" json:"competitionScore,omitempty"`
	SupplyScore        *float64      `gorm:"type:numeric(8,4)" json:"supplyScore,omitempty"`
	RiskScore          *float64      `gorm:"type:numeric(8,4)" json:"riskScore,omitempty"`
	DataQualityScore   *float64      `gorm:"type:numeric(8,4)" json:"dataQualityScore,omitempty"`
	AIScore            *float64      `gorm:"type:numeric(8,4)" json:"aiScore,omitempty"`
	ConfidenceScore    *float64      `gorm:"type:numeric(8,4)" json:"confidenceScore,omitempty"`
	AnalysisVersion    int           `gorm:"not null;default:0" json:"analysisVersion"`
	AnalyzedAt         *time.Time    `gorm:"index" json:"analyzedAt,omitempty"`
	Recommendation     string        `gorm:"size:64" json:"recommendation,omitempty"`
	AnalysisSummary    string        `gorm:"type:text" json:"analysisSummary,omitempty"`
	RejectionReason    string        `gorm:"type:text" json:"rejectionReason,omitempty"`
	EstimatedCost      *float64      `gorm:"type:numeric(18,2)" json:"estimatedCost,omitempty"`
	EstimatedSalePrice *float64      `gorm:"type:numeric(18,2)" json:"estimatedSalePrice,omitempty"`
	EstimatedProfit    *float64      `gorm:"type:numeric(18,2)" json:"estimatedProfit,omitempty"`
	EstimatedMargin    *float64      `gorm:"type:numeric(8,4)" json:"estimatedMargin,omitempty"`
	SourceProduct      SourceProduct `gorm:"-" json:"sourceProduct,omitempty"`
}

func (Candidate) TableName() string { return "candidates" }

// CandidateAnalysis is an immutable, replayable decision snapshot.
type CandidateAnalysis struct {
	model.HardDeleteBase
	TenantID        int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID     uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_candidate_analysis_version,priority:1" json:"candidateId"`
	AnalysisVersion int            `gorm:"not null;uniqueIndex:idx_candidate_analysis_version,priority:2" json:"analysisVersion"`
	AnalysisMode    string         `gorm:"size:32;not null;index" json:"analysisMode"`
	Platform        string         `gorm:"size:64;not null;index" json:"platform"`
	InputSnapshot   datatypes.JSON `gorm:"type:jsonb;not null" json:"inputSnapshot"`
	CostSnapshot    datatypes.JSON `gorm:"type:jsonb;not null" json:"costSnapshot"`
	SKUSnapshot     datatypes.JSON `gorm:"type:jsonb;not null" json:"skuSnapshot"`
	ScoreBreakdown  datatypes.JSON `gorm:"type:jsonb;not null" json:"scoreBreakdown"`
	OverallScore    int64          `gorm:"not null;index" json:"overallScore"`
	ConfidenceScore int64          `gorm:"not null;index" json:"confidenceScore"`
	Recommendation  string         `gorm:"size:64;not null;index" json:"recommendation"`
	Reasons         datatypes.JSON `gorm:"type:jsonb;not null" json:"reasons"`
	Warnings        datatypes.JSON `gorm:"type:jsonb;not null" json:"warnings"`
	Blockers        datatypes.JSON `gorm:"type:jsonb;not null" json:"blockers"`
	Explanation     datatypes.JSON `gorm:"type:jsonb;not null" json:"explanation"`
	AIStatus        string         `gorm:"size:32;not null;index" json:"aiStatus"`
}

func (CandidateAnalysis) TableName() string { return "candidate_analyses" }

// ListingDraft is a platform-specific, non-published sales version of a catalog product.
type ListingDraft struct {
	model.Base
	TenantID          int64          `gorm:"not null;default:0;uniqueIndex:idx_listing_draft_platform,priority:1;index" json:"tenantId"`
	CatalogProductID  uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_listing_draft_platform,priority:2;index" json:"catalogProductId"`
	Platform          string         `gorm:"size:64;not null;uniqueIndex:idx_listing_draft_platform,priority:3;index" json:"platform"`
	ShopID            *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Title             string         `gorm:"size:512;not null" json:"title"`
	Description       string         `gorm:"type:text" json:"description,omitempty"`
	Images            datatypes.JSON `gorm:"type:jsonb" json:"images,omitempty"`
	SalePrice         *float64       `gorm:"type:numeric(18,2)" json:"salePrice,omitempty"`
	EstimatedProfit   *float64       `gorm:"type:numeric(18,2)" json:"estimatedProfit,omitempty"`
	EstimatedMargin   *float64       `gorm:"type:numeric(8,4)" json:"estimatedMargin,omitempty"`
	PricingSnapshot   datatypes.JSON `gorm:"type:jsonb" json:"pricingSnapshot,omitempty"`
	PlatformCategory  string         `gorm:"size:256;index" json:"platformCategory,omitempty"`
	PlatformSKUData   datatypes.JSON `gorm:"type:jsonb" json:"platformSkuData,omitempty"`
	PublishStatus     string         `gorm:"size:32;index;not null;default:draft" json:"publishStatus"`
	ExternalListingID string         `gorm:"size:256;index" json:"externalListingId,omitempty"`
	PublishedAt       *time.Time     `gorm:"index" json:"publishedAt,omitempty"`
	ErrorMessage      string         `gorm:"type:text" json:"errorMessage,omitempty"`
}

func (ListingDraft) TableName() string { return "listing_drafts" }
