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

type CreateListingDraftBody struct {
	Platform string     `json:"platform" binding:"required"`
	ShopID   *uuid.UUID `json:"shopId"`
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
