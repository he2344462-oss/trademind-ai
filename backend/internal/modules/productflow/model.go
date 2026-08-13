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

	BatchStatusPending       = "pending"
	BatchStatusRunning       = "running"
	BatchStatusCompleted     = "completed"
	BatchStatusPartialFailed = "partial_failed"
	BatchStatusFailed        = "failed"
	BatchStatusCancelled     = "cancelled"

	BatchItemStatusPending    = "pending"
	BatchItemStatusProcessing = "processing"
	BatchItemStatusCompleted  = "completed"
	BatchItemStatusFailed     = "failed"
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
	TenantID             int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID          uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_candidate_analysis_version,priority:1" json:"candidateId"`
	AnalysisVersion      int            `gorm:"not null;uniqueIndex:idx_candidate_analysis_version,priority:2" json:"analysisVersion"`
	AnalysisMode         string         `gorm:"size:32;not null;index" json:"analysisMode"`
	Platform             string         `gorm:"size:64;not null;index" json:"platform"`
	InputSnapshot        datatypes.JSON `gorm:"type:jsonb;not null" json:"inputSnapshot"`
	CostSnapshot         datatypes.JSON `gorm:"type:jsonb;not null" json:"costSnapshot"`
	SKUSnapshot          datatypes.JSON `gorm:"type:jsonb;not null" json:"skuSnapshot"`
	ScoreBreakdown       datatypes.JSON `gorm:"type:jsonb;not null" json:"scoreBreakdown"`
	OverallScore         int64          `gorm:"not null;index" json:"overallScore"`
	ConfidenceScore      int64          `gorm:"not null;index" json:"confidenceScore"`
	Recommendation       string         `gorm:"size:64;not null;index" json:"recommendation"`
	Reasons              datatypes.JSON `gorm:"type:jsonb;not null" json:"reasons"`
	Warnings             datatypes.JSON `gorm:"type:jsonb;not null" json:"warnings"`
	Blockers             datatypes.JSON `gorm:"type:jsonb;not null" json:"blockers"`
	Explanation          datatypes.JSON `gorm:"type:jsonb;not null" json:"explanation"`
	AIStatus             string         `gorm:"size:32;not null;index" json:"aiStatus"`
	PricingProfileID     *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	MarketSignalSnapshot datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"marketSignalSnapshot"`
	ConfidenceBreakdown  datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"confidenceBreakdown"`
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
	PricingProfileID  *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	PricingVersion    int            `gorm:"not null;default:1" json:"pricingVersion"`
	PlatformCategory  string         `gorm:"size:256;index" json:"platformCategory,omitempty"`
	PlatformSKUData   datatypes.JSON `gorm:"type:jsonb" json:"platformSkuData,omitempty"`
	PublishStatus     string         `gorm:"size:32;index;not null;default:draft" json:"publishStatus"`
	ExternalListingID string         `gorm:"size:256;index" json:"externalListingId,omitempty"`
	PublishedAt       *time.Time     `gorm:"index" json:"publishedAt,omitempty"`
	ErrorMessage      string         `gorm:"type:text" json:"errorMessage,omitempty"`
}

func (ListingDraft) TableName() string { return "listing_drafts" }

// PricingProfile stores a user-maintained estimate model. Monetary fixed fees are integer CNY fen; rates are basis points.
type PricingProfile struct {
	model.Base
	TenantID         int64  `gorm:"not null;default:0;uniqueIndex:idx_pricing_profile_name,priority:1;index" json:"tenantId"`
	Name             string `gorm:"size:128;not null;uniqueIndex:idx_pricing_profile_name,priority:2" json:"name"`
	Platform         string `gorm:"size:64;not null;index" json:"platform"`
	Currency         string `gorm:"size:8;not null;default:CNY" json:"currency"`
	PlatformFeeBPS   int64  `gorm:"not null;default:0" json:"platformFeeBps"`
	PlatformFeeFixed int64  `gorm:"not null;default:0" json:"platformFeeFixed"`
	PaymentFeeBPS    int64  `gorm:"not null;default:0" json:"paymentFeeBps"`
	PaymentFeeFixed  int64  `gorm:"not null;default:0" json:"paymentFeeFixed"`
	ReturnReserveBPS int64  `gorm:"not null;default:0" json:"returnReserveBps"`
	OtherBPS         int64  `gorm:"not null;default:0" json:"otherBps"`
	OtherFixed       int64  `gorm:"not null;default:0" json:"otherFixed"`
	IsDefault        bool   `gorm:"not null;default:false;index" json:"isDefault"`
	Enabled          bool   `gorm:"not null;default:true;index" json:"enabled"`
}

func (PricingProfile) TableName() string { return "pricing_profiles" }

// MarketSignalSnapshot is immutable evidence. Manual and fixture data are explicitly labelled and never presented as live platform truth.
type MarketSignalSnapshot struct {
	model.HardDeleteBase
	TenantID      int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID   uuid.UUID      `gorm:"type:char(36);not null;index" json:"candidateId"`
	Platform      string         `gorm:"size:64;not null;index" json:"platform"`
	SignalType    string         `gorm:"size:64;not null;index" json:"signalType"`
	Value         int64          `gorm:"not null" json:"value"`
	Unit          string         `gorm:"size:32;not null" json:"unit"`
	Source        string         `gorm:"size:256;not null;index" json:"source"`
	Origin        string         `gorm:"size:32;not null;index" json:"origin"`
	ConfidenceBPS int64          `gorm:"not null;default:0" json:"confidenceBps"`
	ObservedAt    time.Time      `gorm:"not null;index" json:"observedAt"`
	ExpiresAt     *time.Time     `gorm:"index" json:"expiresAt,omitempty"`
	RawData       datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"rawData"`
}

func (MarketSignalSnapshot) TableName() string { return "market_signal_snapshots" }

type CandidateAnalysisBatch struct {
	model.HardDeleteBase
	TenantID          int64          `gorm:"not null;default:0;index" json:"tenantId"`
	Status            string         `gorm:"size:32;not null;default:pending;index" json:"status"`
	Total             int            `gorm:"not null;default:0" json:"total"`
	Pending           int            `gorm:"not null;default:0" json:"pending"`
	Processing        int            `gorm:"not null;default:0" json:"processing"`
	Completed         int            `gorm:"not null;default:0" json:"completed"`
	Failed            int            `gorm:"not null;default:0" json:"failed"`
	Filters           datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"filters"`
	AnalysisMode      string         `gorm:"size:32;not null" json:"analysisMode"`
	Platform          string         `gorm:"size:64;not null;index" json:"platform"`
	PricingProfileID  *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	MinimumScore      int64          `gorm:"not null;default:0" json:"minimumScore"`
	MinimumMarginBPS  int64          `gorm:"not null;default:0" json:"minimumMarginBps"`
	MinimumProfit     int64          `gorm:"not null;default:0" json:"minimumProfit"`
	MinimumConfidence int64          `gorm:"not null;default:0" json:"minimumConfidence"`
	ExcludeBlocked    bool           `gorm:"not null;default:true" json:"excludeBlocked"`
	TopN              int            `gorm:"not null;default:20" json:"topN"`
	CreatedBy         *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ActorType         string         `gorm:"size:32;not null;default:user" json:"actorType"`
	AIExplanationDone bool           `gorm:"not null;default:false" json:"aiExplanationDone"`
	StartedAt         *time.Time     `gorm:"index" json:"startedAt,omitempty"`
	CompletedAt       *time.Time     `gorm:"index" json:"completedAt,omitempty"`
}

func (CandidateAnalysisBatch) TableName() string { return "candidate_analysis_batches" }

type CandidateAnalysisBatchItem struct {
	model.HardDeleteBase
	TenantID     int64      `gorm:"not null;default:0;index" json:"tenantId"`
	BatchID      uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:idx_analysis_batch_candidate,priority:1;index" json:"batchId"`
	CandidateID  uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:idx_analysis_batch_candidate,priority:2;index" json:"candidateId"`
	AnalysisID   *uuid.UUID `gorm:"type:char(36);index" json:"analysisId,omitempty"`
	Status       string     `gorm:"size:32;not null;default:pending;index" json:"status"`
	Attempts     int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts  int        `gorm:"not null;default:3" json:"maxAttempts"`
	ErrorMessage string     `gorm:"type:text" json:"errorMessage,omitempty"`
	RankingScore int64      `gorm:"not null;default:0;index" json:"rankingScore"`
	Rank         *int       `gorm:"index" json:"rank,omitempty"`
	WorkerID     string     `gorm:"size:128;index" json:"workerId,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

func (CandidateAnalysisBatchItem) TableName() string { return "candidate_analysis_batch_items" }
