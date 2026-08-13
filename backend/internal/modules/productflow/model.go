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

	ListingStatusDraft            = "draft"
	ListingStatusContentGenerated = "content_generated"
	ListingStatusNeedsReview      = "needs_review"
	ListingStatusApproved         = "approved"
	ListingStatusReadyToPublish   = "ready_to_publish"
	ListingStatusPublishedManual  = "published_manual"
	ListingStatusPublishedAPI     = "published_api"
	ListingStatusReady            = "ready"
	ListingStatusPublishing       = "publishing"
	ListingStatusPublished        = "published"
	ListingStatusFailed           = "failed"
	ListingStatusOffline          = "offline"

	BatchStatusPending       = "pending"
	BatchStatusRunning       = "running"
	BatchStatusPausing       = "pausing"
	BatchStatusPaused        = "paused"
	BatchStatusCancelling    = "cancelling"
	BatchStatusCompleted     = "completed"
	BatchStatusPartialFailed = "partial_failed"
	BatchStatusFailed        = "failed"
	BatchStatusCancelled     = "cancelled"

	BatchItemStatusPending    = "pending"
	BatchItemStatusProcessing = "processing"
	BatchItemStatusCompleted  = "completed"
	BatchItemStatusFailed     = "failed"
	BatchItemStatusCancelled  = "cancelled"

	BatchErrorRetryable    = "retryable"
	BatchErrorNonRetryable = "non_retryable"
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
	TenantID               int64          `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID            uuid.UUID      `gorm:"type:char(36);not null;index;uniqueIndex:idx_candidate_analysis_version,priority:1" json:"candidateId"`
	AnalysisVersion        int            `gorm:"not null;uniqueIndex:idx_candidate_analysis_version,priority:2" json:"analysisVersion"`
	AnalysisMode           string         `gorm:"size:32;not null;index" json:"analysisMode"`
	Platform               string         `gorm:"size:64;not null;index" json:"platform"`
	InputSnapshot          datatypes.JSON `gorm:"type:jsonb;not null" json:"inputSnapshot"`
	CostSnapshot           datatypes.JSON `gorm:"type:jsonb;not null" json:"costSnapshot"`
	SKUSnapshot            datatypes.JSON `gorm:"type:jsonb;not null" json:"skuSnapshot"`
	ScoreBreakdown         datatypes.JSON `gorm:"type:jsonb;not null" json:"scoreBreakdown"`
	OverallScore           int64          `gorm:"not null;index" json:"overallScore"`
	ConfidenceScore        int64          `gorm:"not null;index" json:"confidenceScore"`
	Recommendation         string         `gorm:"size:64;not null;index" json:"recommendation"`
	Reasons                datatypes.JSON `gorm:"type:jsonb;not null" json:"reasons"`
	Warnings               datatypes.JSON `gorm:"type:jsonb;not null" json:"warnings"`
	Blockers               datatypes.JSON `gorm:"type:jsonb;not null" json:"blockers"`
	Explanation            datatypes.JSON `gorm:"type:jsonb;not null" json:"explanation"`
	AIStatus               string         `gorm:"size:32;not null;index" json:"aiStatus"`
	PricingProfileID       *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	MarketSignalSnapshot   datatypes.JSON `gorm:"type:jsonb;not null;default:'[]'" json:"marketSignalSnapshot"`
	ConfidenceBreakdown    datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"confidenceBreakdown"`
	ScoringConfigVersion   string         `gorm:"size:64;not null;default:selection-v1;index" json:"scoringConfigVersion"`
	SelectionConfigID      *uuid.UUID     `gorm:"type:char(36);index" json:"selectionConfigId,omitempty"`
	SelectionConfigVersion string         `gorm:"size:64;not null;default:selection-v1;index" json:"selectionConfigVersion"`
	PricingProfileVersion  int            `gorm:"not null;default:1" json:"pricingProfileVersion"`
	RankingConfigVersion   string         `gorm:"size:64;not null;default:ranking-v1" json:"rankingConfigVersion"`
}

func (CandidateAnalysis) TableName() string { return "candidate_analyses" }

// ListingDraft is a platform-specific, non-published sales version of a catalog product.
type ListingDraft struct {
	model.Base
	TenantID                int64          `gorm:"not null;default:0;uniqueIndex:idx_listing_draft_platform,priority:1;index" json:"tenantId"`
	CatalogProductID        uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_listing_draft_platform,priority:2;index" json:"catalogProductId"`
	Platform                string         `gorm:"size:64;not null;uniqueIndex:idx_listing_draft_platform,priority:3;index" json:"platform"`
	ShopID                  *uuid.UUID     `gorm:"type:char(36);index" json:"shopId,omitempty"`
	Title                   string         `gorm:"size:512;not null" json:"title"`
	Description             string         `gorm:"type:text" json:"description,omitempty"`
	Images                  datatypes.JSON `gorm:"type:jsonb" json:"images,omitempty"`
	SalePrice               *float64       `gorm:"type:numeric(18,2)" json:"salePrice,omitempty"`
	EstimatedProfit         *float64       `gorm:"type:numeric(18,2)" json:"estimatedProfit,omitempty"`
	EstimatedMargin         *float64       `gorm:"type:numeric(8,4)" json:"estimatedMargin,omitempty"`
	PricingSnapshot         datatypes.JSON `gorm:"type:jsonb" json:"pricingSnapshot,omitempty"`
	PricingProfileID        *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	PricingVersion          int            `gorm:"not null;default:1" json:"pricingVersion"`
	PlatformCategory        string         `gorm:"size:256;index" json:"platformCategory,omitempty"`
	PlatformSKUData         datatypes.JSON `gorm:"type:jsonb" json:"platformSkuData,omitempty"`
	PublishStatus           string         `gorm:"size:32;index;not null;default:draft" json:"publishStatus"`
	ExternalListingID       string         `gorm:"size:256;index" json:"externalListingId,omitempty"`
	PublishedAt             *time.Time     `gorm:"index" json:"publishedAt,omitempty"`
	ErrorMessage            string         `gorm:"type:text" json:"errorMessage,omitempty"`
	CurrentContentVersionID *uuid.UUID     `gorm:"type:char(36);index" json:"currentContentVersionId,omitempty"`
	PublishMethod           string         `gorm:"size:32;index" json:"publishMethod,omitempty"`
	ListingURL              string         `gorm:"size:2048" json:"listingUrl,omitempty"`
}

func (ListingDraft) TableName() string { return "listing_drafts" }

// ListingContentVersion is an immutable, reviewable content snapshot. AI text is
// never treated as the source of product facts.
type ListingContentVersion struct {
	model.HardDeleteBase
	TenantID              int64          `gorm:"not null;default:0;index" json:"tenantId"`
	ListingDraftID        uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_listing_content_version,priority:1;index" json:"listingDraftId"`
	CatalogProductID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"catalogProductId"`
	Platform              string         `gorm:"size:64;not null;index" json:"platform"`
	Version               int            `gorm:"not null;uniqueIndex:idx_listing_content_version,priority:2" json:"version"`
	GenerationMode        string         `gorm:"size:32;not null" json:"generationMode"`
	ContentProfileVersion string         `gorm:"size:64;not null" json:"contentProfileVersion"`
	PromptVersion         string         `gorm:"size:64;not null" json:"promptVersion"`
	InputSnapshot         datatypes.JSON `gorm:"type:jsonb;not null" json:"inputSnapshot"`
	Title                 string         `gorm:"size:512;not null" json:"title"`
	Description           string         `gorm:"type:text" json:"description"`
	SellingPoints         datatypes.JSON `gorm:"type:jsonb;not null" json:"sellingPoints"`
	Keywords              datatypes.JSON `gorm:"type:jsonb;not null" json:"keywords"`
	FAQ                   datatypes.JSON `gorm:"type:jsonb;not null" json:"faq"`
	SKUContent            datatypes.JSON `gorm:"type:jsonb;not null" json:"skuContent"`
	Warnings              datatypes.JSON `gorm:"type:jsonb;not null" json:"warnings"`
	Blockers              datatypes.JSON `gorm:"type:jsonb;not null" json:"blockers"`
	ReviewStatus          string         `gorm:"size:32;not null;index" json:"reviewStatus"`
	AIStatus              string         `gorm:"size:32;not null" json:"aiStatus"`
	ModelProvider         string         `gorm:"size:64" json:"modelProvider,omitempty"`
	ModelName             string         `gorm:"size:128" json:"modelName,omitempty"`
	AIInputTokens         int            `gorm:"not null;default:0" json:"aiInputTokens"`
	AIOutputTokens        int            `gorm:"not null;default:0" json:"aiOutputTokens"`
	AICostMicros          int64          `gorm:"not null;default:0" json:"aiCostMicros"`
	AILatencyMS           int64          `gorm:"not null;default:0" json:"aiLatencyMs"`
	CreatedBy             *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	OperationBatchItemID  *uuid.UUID     `gorm:"type:char(36);uniqueIndex" json:"operationBatchItemId,omitempty"`
}

func (ListingContentVersion) TableName() string { return "listing_content_versions" }

type ListingAsset struct {
	model.HardDeleteBase
	TenantID         int64      `gorm:"not null;default:0;index;uniqueIndex:idx_listing_asset_url,priority:1" json:"tenantId"`
	CatalogProductID uuid.UUID  `gorm:"type:char(36);not null;index" json:"catalogProductId"`
	ListingDraftID   *uuid.UUID `gorm:"type:char(36);index;uniqueIndex:idx_listing_asset_url,priority:2" json:"listingDraftId,omitempty"`
	SourceType       string     `gorm:"size:32;not null" json:"sourceType"`
	SourceURL        string     `gorm:"size:2048;not null;uniqueIndex:idx_listing_asset_url,priority:3" json:"sourceUrl"`
	SourceReference  string     `gorm:"size:512" json:"sourceReference,omitempty"`
	ContentHash      string     `gorm:"size:64;index" json:"contentHash,omitempty"`
	MimeType         string     `gorm:"size:64" json:"mimeType,omitempty"`
	SizeBytes        int64      `gorm:"not null;default:0" json:"sizeBytes"`
	SortOrder        int        `gorm:"not null;default:0" json:"sortOrder"`
	IsPrimary        bool       `gorm:"not null;default:false" json:"isPrimary"`
	Excluded         bool       `gorm:"not null;default:false" json:"excluded"`
	CachePath        string     `gorm:"size:1024" json:"-"`
	Cached           bool       `gorm:"-" json:"cached"`
}

func (ListingAsset) TableName() string { return "listing_assets" }

type ListingPublishPackage struct {
	model.HardDeleteBase
	TenantID             int64          `gorm:"not null;default:0;index" json:"tenantId"`
	ListingDraftID       uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_listing_package_version,priority:1;index" json:"listingDraftId"`
	ContentVersionID     uuid.UUID      `gorm:"type:char(36);not null;index" json:"contentVersionId"`
	PackageVersion       int            `gorm:"not null;uniqueIndex:idx_listing_package_version,priority:2" json:"packageVersion"`
	PricingVersion       int            `gorm:"not null" json:"pricingVersion"`
	Manifest             datatypes.JSON `gorm:"type:jsonb;not null" json:"manifest"`
	ArchivePath          string         `gorm:"size:1024" json:"-"`
	ArchiveHash          string         `gorm:"size:64" json:"archiveHash"`
	SizeBytes            int64          `gorm:"not null" json:"sizeBytes"`
	GeneratedAt          time.Time      `gorm:"not null;index" json:"generatedAt"`
	CreatedBy            *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	OperationBatchItemID *uuid.UUID     `gorm:"type:char(36);uniqueIndex" json:"operationBatchItemId,omitempty"`
}

func (ListingPublishPackage) TableName() string { return "listing_publish_packages" }

type ManualPublishRecord struct {
	model.HardDeleteBase
	TenantID          int64      `gorm:"not null;default:0;index" json:"tenantId"`
	ListingDraftID    uuid.UUID  `gorm:"type:char(36);not null;index" json:"listingDraftId"`
	ContentVersionID  uuid.UUID  `gorm:"type:char(36);not null;index" json:"contentVersionId"`
	PublishPackageID  *uuid.UUID `gorm:"type:char(36);index" json:"publishPackageId,omitempty"`
	Platform          string     `gorm:"size:64;not null;index" json:"platform"`
	PlatformListingID string     `gorm:"size:256;not null;index" json:"platformListingId"`
	ListingURL        string     `gorm:"size:2048" json:"listingUrl,omitempty"`
	PublishMethod     string     `gorm:"size:32;not null;index" json:"publishMethod"`
	PublishedAt       time.Time  `gorm:"not null;index" json:"publishedAt"`
	Notes             string     `gorm:"type:text" json:"notes,omitempty"`
	CreatedBy         *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (ManualPublishRecord) TableName() string { return "manual_publish_records" }

type ListingContentQualityReview struct {
	model.HardDeleteBase
	TenantID           int64      `gorm:"not null;default:0;index" json:"tenantId"`
	ListingDraftID     uuid.UUID  `gorm:"type:char(36);not null;index" json:"listingDraftId"`
	ContentVersionID   uuid.UUID  `gorm:"type:char(36);not null;index" json:"contentVersionId"`
	Status             string     `gorm:"size:32;not null;index" json:"status"`
	TitleQuality       int        `gorm:"not null;default:0" json:"titleQuality"`
	DescriptionQuality int        `gorm:"not null;default:0" json:"descriptionQuality"`
	FactAccuracy       int        `gorm:"not null;default:0" json:"factAccuracy"`
	WasEdited          bool       `gorm:"not null;default:false" json:"wasEdited"`
	EditReason         string     `gorm:"type:text" json:"editReason,omitempty"`
	CreatedBy          *uuid.UUID `gorm:"type:char(36);index" json:"createdBy,omitempty"`
}

func (ListingContentQualityReview) TableName() string { return "listing_content_quality_reviews" }

type ListingOperationBatch struct {
	model.HardDeleteBase
	TenantID    int64          `gorm:"not null;default:0;index" json:"tenantId"`
	Operation   string         `gorm:"size:32;not null;index" json:"operation"`
	Status      string         `gorm:"size:32;not null;index" json:"status"`
	Total       int            `gorm:"not null" json:"total"`
	Pending     int            `gorm:"not null" json:"pending"`
	Processing  int            `gorm:"not null" json:"processing"`
	Completed   int            `gorm:"not null" json:"completed"`
	Failed      int            `gorm:"not null" json:"failed"`
	Options     datatypes.JSON `gorm:"type:jsonb;not null" json:"options"`
	CreatedBy   *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	StartedAt   *time.Time     `json:"startedAt,omitempty"`
	CompletedAt *time.Time     `json:"completedAt,omitempty"`
}

func (ListingOperationBatch) TableName() string { return "listing_operation_batches" }

type ListingOperationBatchItem struct {
	model.HardDeleteBase
	TenantID       int64      `gorm:"not null;default:0;index" json:"tenantId"`
	BatchID        uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:idx_listing_batch_item,priority:1;index" json:"batchId"`
	ListingDraftID uuid.UUID  `gorm:"type:char(36);not null;uniqueIndex:idx_listing_batch_item,priority:2;index" json:"listingDraftId"`
	Status         string     `gorm:"size:32;not null;index" json:"status"`
	Attempts       int        `gorm:"not null;default:0" json:"attempts"`
	MaxAttempts    int        `gorm:"not null;default:3" json:"maxAttempts"`
	ResultID       *uuid.UUID `gorm:"type:char(36);index" json:"resultId,omitempty"`
	ErrorMessage   string     `gorm:"type:text" json:"errorMessage,omitempty"`
	WorkerID       string     `gorm:"size:128;index" json:"workerId,omitempty"`
	StartedAt      *time.Time `json:"startedAt,omitempty"`
	CompletedAt    *time.Time `json:"completedAt,omitempty"`
}

func (ListingOperationBatchItem) TableName() string { return "listing_operation_batch_items" }

// PricingProfile stores a user-maintained estimate model. Monetary fixed fees are integer CNY fen; rates are basis points.
type PricingProfile struct {
	model.Base
	TenantID         int64      `gorm:"not null;default:0;uniqueIndex:idx_pricing_profile_name,priority:1;index" json:"tenantId"`
	Name             string     `gorm:"size:128;not null;uniqueIndex:idx_pricing_profile_name,priority:2" json:"name"`
	Platform         string     `gorm:"size:64;not null;index" json:"platform"`
	Currency         string     `gorm:"size:8;not null;default:CNY" json:"currency"`
	PlatformFeeBPS   int64      `gorm:"not null;default:0" json:"platformFeeBps"`
	PlatformFeeFixed int64      `gorm:"not null;default:0" json:"platformFeeFixed"`
	PaymentFeeBPS    int64      `gorm:"not null;default:0" json:"paymentFeeBps"`
	PaymentFeeFixed  int64      `gorm:"not null;default:0" json:"paymentFeeFixed"`
	ReturnReserveBPS int64      `gorm:"not null;default:0" json:"returnReserveBps"`
	OtherBPS         int64      `gorm:"not null;default:0" json:"otherBps"`
	OtherFixed       int64      `gorm:"not null;default:0" json:"otherFixed"`
	IsDefault        bool       `gorm:"not null;default:false;index" json:"isDefault"`
	Enabled          bool       `gorm:"not null;default:true;index" json:"enabled"`
	Version          int        `gorm:"not null;default:1" json:"version"`
	UpdatedBy        *uuid.UUID `gorm:"type:char(36);index" json:"updatedBy,omitempty"`
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
	Fingerprint   string         `gorm:"size:64;uniqueIndex" json:"fingerprint"`
}

func (MarketSignalSnapshot) TableName() string { return "market_signal_snapshots" }

type CandidateAnalysisBatch struct {
	model.HardDeleteBase
	TenantID             int64          `gorm:"not null;default:0;index" json:"tenantId"`
	Status               string         `gorm:"size:32;not null;default:pending;index" json:"status"`
	Total                int            `gorm:"not null;default:0" json:"total"`
	Pending              int            `gorm:"not null;default:0" json:"pending"`
	Processing           int            `gorm:"not null;default:0" json:"processing"`
	Completed            int            `gorm:"not null;default:0" json:"completed"`
	Failed               int            `gorm:"not null;default:0" json:"failed"`
	Filters              datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"filters"`
	AnalysisMode         string         `gorm:"size:32;not null" json:"analysisMode"`
	Platform             string         `gorm:"size:64;not null;index" json:"platform"`
	PricingProfileID     *uuid.UUID     `gorm:"type:char(36);index" json:"pricingProfileId,omitempty"`
	MinimumScore         int64          `gorm:"not null;default:0" json:"minimumScore"`
	MinimumMarginBPS     int64          `gorm:"not null;default:0" json:"minimumMarginBps"`
	MinimumProfit        int64          `gorm:"not null;default:0" json:"minimumProfit"`
	MinimumConfidence    int64          `gorm:"not null;default:0" json:"minimumConfidence"`
	ExcludeBlocked       bool           `gorm:"not null;default:true" json:"excludeBlocked"`
	TopN                 int            `gorm:"not null;default:20" json:"topN"`
	CreatedBy            *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ActorType            string         `gorm:"size:32;not null;default:user" json:"actorType"`
	AIExplanationDone    bool           `gorm:"not null;default:false" json:"aiExplanationDone"`
	StartedAt            *time.Time     `gorm:"index" json:"startedAt,omitempty"`
	CompletedAt          *time.Time     `gorm:"index" json:"completedAt,omitempty"`
	ControlVersion       int64          `gorm:"not null;default:1" json:"controlVersion"`
	RankingConfigVersion string         `gorm:"size:64;not null;default:ranking-v1" json:"rankingConfigVersion"`
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
	ErrorType    string     `gorm:"size:32;index" json:"errorType,omitempty"`
	RankingScore int64      `gorm:"not null;default:0;index" json:"rankingScore"`
	Rank         *int       `gorm:"index" json:"rank,omitempty"`
	WorkerID     string     `gorm:"size:128;index" json:"workerId,omitempty"`
	StartedAt    *time.Time `json:"startedAt,omitempty"`
	CompletedAt  *time.Time `json:"completedAt,omitempty"`
}

func (CandidateAnalysisBatchItem) TableName() string { return "candidate_analysis_batch_items" }

// PricingProfileRevision is an immutable snapshot used by historical analyses and listings.
type PricingProfileRevision struct {
	model.HardDeleteBase
	TenantID  int64          `gorm:"not null;default:0;index" json:"tenantId"`
	ProfileID uuid.UUID      `gorm:"type:char(36);not null;uniqueIndex:idx_pricing_profile_revision,priority:1;index" json:"profileId"`
	Version   int            `gorm:"not null;uniqueIndex:idx_pricing_profile_revision,priority:2" json:"version"`
	Snapshot  datatypes.JSON `gorm:"type:jsonb;not null" json:"snapshot"`
	ActorType string         `gorm:"size:32;not null;default:manual" json:"actorType"`
	ActorID   *uuid.UUID     `gorm:"type:char(36);index" json:"actorId,omitempty"`
}

func (PricingProfileRevision) TableName() string { return "pricing_profile_revisions" }

// MarketSignalProviderConfig records provider availability without storing plaintext credentials.
type MarketSignalProviderConfig struct {
	model.Base
	TenantID            int64          `gorm:"not null;default:0;uniqueIndex:idx_market_provider_platform,priority:1;index" json:"tenantId"`
	ProviderID          string         `gorm:"size:128;not null;uniqueIndex:idx_market_provider_platform,priority:2" json:"providerId"`
	ProviderType        string         `gorm:"size:128;not null;index" json:"providerType"`
	Name                string         `gorm:"size:128;not null" json:"name"`
	Platform            string         `gorm:"size:64;not null;index;uniqueIndex:idx_market_provider_platform,priority:3" json:"platform"`
	SourceType          string         `gorm:"size:32;not null;index" json:"sourceType"`
	Enabled             bool           `gorm:"not null;default:false;index" json:"enabled"`
	Config              datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"config"`
	CredentialReference string         `gorm:"size:256;not null;default:''" json:"-"`
	HealthStatus        string         `gorm:"size:32;not null;default:not_configured" json:"healthStatus"`
	LastCheckedAt       *time.Time     `json:"lastCheckedAt,omitempty"`
	LastSuccessAt       *time.Time     `json:"lastSuccessAt,omitempty"`
	LastError           string         `gorm:"type:text" json:"lastError,omitempty"`
}

func (MarketSignalProviderConfig) TableName() string { return "market_signal_provider_configs" }

// SelectionConfig is an immutable versioned rule configuration. Activation is
// explicit and historical CandidateAnalysis rows keep their original version.
type SelectionConfig struct {
	model.HardDeleteBase
	TenantID    int64          `gorm:"not null;default:0;uniqueIndex:idx_selection_config_version,priority:1;index" json:"tenantId"`
	Version     string         `gorm:"size:64;not null;uniqueIndex:idx_selection_config_version,priority:2" json:"version"`
	Weights     datatypes.JSON `gorm:"type:jsonb;not null" json:"weights"`
	Thresholds  datatypes.JSON `gorm:"type:jsonb;not null" json:"thresholds"`
	Blockers    datatypes.JSON `gorm:"type:jsonb;not null" json:"blockers"`
	Status      string         `gorm:"size:32;not null;index" json:"status"`
	CreatedBy   *uuid.UUID     `gorm:"type:char(36);index" json:"createdBy,omitempty"`
	ActivatedAt *time.Time     `gorm:"index" json:"activatedAt,omitempty"`
}

func (SelectionConfig) TableName() string { return "selection_configs" }

// ListingPerformanceSnapshot stores user-owned sales outcomes. Money is integer fen.
type ListingPerformanceSnapshot struct {
	model.HardDeleteBase
	TenantID         int64          `gorm:"not null;default:0;index" json:"tenantId"`
	ListingDraftID   *uuid.UUID     `gorm:"type:char(36);index" json:"listingDraftId,omitempty"`
	ContentVersionID *uuid.UUID     `gorm:"type:char(36);index" json:"contentVersionId,omitempty"`
	PricingVersion   int            `gorm:"not null;default:0" json:"pricingVersion"`
	CatalogProductID uuid.UUID      `gorm:"type:char(36);not null;index" json:"catalogProductId"`
	CandidateID      uuid.UUID      `gorm:"type:char(36);not null;index" json:"candidateId"`
	Platform         string         `gorm:"size:64;not null;index" json:"platform"`
	Source           string         `gorm:"size:32;not null;index" json:"source"`
	ObservedAt       time.Time      `gorm:"not null;index" json:"observedAt"`
	PeriodStart      time.Time      `gorm:"not null;index" json:"periodStart"`
	PeriodEnd        time.Time      `gorm:"not null;index" json:"periodEnd"`
	Impressions      *int64         `json:"impressions,omitempty"`
	Views            *int64         `json:"views,omitempty"`
	Clicks           *int64         `json:"clicks,omitempty"`
	Favorites        *int64         `json:"favorites,omitempty"`
	Inquiries        *int64         `json:"inquiries,omitempty"`
	Messages         *int64         `json:"messages,omitempty"`
	Orders           *int64         `json:"orders,omitempty"`
	UnitsSold        *int64         `json:"unitsSold,omitempty"`
	GrossRevenue     *int64         `json:"grossRevenue,omitempty"`
	RefundAmount     *int64         `json:"refundAmount,omitempty"`
	PlatformCost     *int64         `json:"platformCost,omitempty"`
	ActualCost       *int64         `json:"actualCost,omitempty"`
	RealizedProfit   *int64         `json:"realizedProfit,omitempty"`
	RefundCount      *int64         `json:"refundCount,omitempty"`
	ReturnCount      *int64         `json:"returnCount,omitempty"`
	AfterSaleCount   *int64         `json:"afterSaleCount,omitempty"`
	RawData          datatypes.JSON `gorm:"type:jsonb;not null;default:'{}'" json:"rawData"`
	Fingerprint      string         `gorm:"size:64;not null;uniqueIndex" json:"fingerprint"`
}

func (ListingPerformanceSnapshot) TableName() string { return "listing_performance_snapshots" }

// SelectionOutcomeEvaluation compares the immutable prediction with accumulated outcomes.
type SelectionOutcomeEvaluation struct {
	model.HardDeleteBase
	TenantID           int64     `gorm:"not null;default:0;index" json:"tenantId"`
	CandidateID        uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:idx_selection_evaluation,priority:1" json:"candidateId"`
	AnalysisID         uuid.UUID `gorm:"type:char(36);not null;index;uniqueIndex:idx_selection_evaluation,priority:2" json:"analysisId"`
	Recommendation     string    `gorm:"size:64;not null;index" json:"recommendation"`
	OverallScore       int64     `gorm:"not null" json:"overallScore"`
	PredictedMarginBPS int64     `gorm:"not null" json:"predictedMarginBps"`
	Views              int64     `gorm:"not null;default:0" json:"views"`
	Inquiries          int64     `gorm:"not null;default:0" json:"inquiries"`
	Orders             int64     `gorm:"not null;default:0" json:"orders"`
	Refunds            int64     `gorm:"not null;default:0" json:"refunds"`
	RealizedProfit     int64     `gorm:"not null;default:0" json:"realizedProfit"`
	ActualMarginBPS    int64     `gorm:"not null;default:0" json:"actualMarginBps"`
	EvaluatedAt        time.Time `gorm:"not null;index" json:"evaluatedAt"`
}

func (SelectionOutcomeEvaluation) TableName() string { return "selection_outcome_evaluations" }
