package productflow

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrNotFound          = errors.New("resource not found")
	ErrConflict          = errors.New("resource conflict")
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrValidation        = errors.New("validation failed")
)

type Service struct {
	DB        *gorm.DB
	AIExplain func(context.Context, string) (string, error)
}

type CollectedSourceInput struct {
	TenantID        int64
	SourcePlatform  string
	SourceProductID string
	SourceURL       string
	Title           string
	Description     string
	Images          []string
	SKUs            any
	RawData         json.RawMessage
	SourcePrice     *float64
}

func StableSourceProductID(platform, sourceURL string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(platform)) + "\x00" + strings.TrimSpace(sourceURL)))
	return "url:" + hex.EncodeToString(sum[:12])
}

func normalizePage(q ListQuery) ListQuery {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize < 1 {
		q.PageSize = 20
	}
	if q.PageSize > 100 {
		q.PageSize = 100
	}
	return q
}

func totalPages(total int64, size int) int {
	if total == 0 {
		return 0
	}
	return int((total + int64(size) - 1) / int64(size))
}

func jsonValue(raw json.RawMessage, fallback any) (datatypes.JSON, error) {
	if len(raw) == 0 {
		b, err := json.Marshal(fallback)
		return datatypes.JSON(b), err
	}
	if !json.Valid(raw) {
		return nil, fmt.Errorf("%w: invalid json field", ErrValidation)
	}
	return datatypes.JSON(append([]byte(nil), raw...)), nil
}

func validateMoney(values ...*float64) error {
	for _, v := range values {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0) {
			return fmt.Errorf("%w: money values must be finite and non-negative", ErrValidation)
		}
	}
	return nil
}

func (s *Service) UpsertCollectedSource(ctx context.Context, in CollectedSourceInput) (*SourceProduct, error) {
	if s == nil || s.DB == nil {
		return nil, errors.New("product flow unavailable")
	}
	platform := strings.ToLower(strings.TrimSpace(in.SourcePlatform))
	sourceURL := strings.TrimSpace(in.SourceURL)
	sourceID := strings.TrimSpace(in.SourceProductID)
	if sourceID == "" {
		sourceID = StableSourceProductID(platform, sourceURL)
	}
	if platform == "" || sourceURL == "" || strings.TrimSpace(in.Title) == "" {
		return nil, fmt.Errorf("%w: platform, source URL and title are required", ErrValidation)
	}
	images, err := json.Marshal(in.Images)
	if err != nil {
		return nil, err
	}
	skus, err := json.Marshal(in.SKUs)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	row := SourceProduct{TenantID: in.TenantID, SourcePlatform: platform, SourceProductID: sourceID,
		SourceURL: sourceURL, OriginalTitle: strings.TrimSpace(in.Title), OriginalDescription: in.Description,
		OriginalImages: images, SourcePrice: in.SourcePrice, SKUData: skus, RawData: datatypes.JSON(in.RawData),
		CollectedAt: now, Status: SourceStatusCollected, MinOrderQuantity: 1}
	err = s.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}, {Name: "source_platform"}, {Name: "source_product_id"}},
		DoUpdates: clause.Assignments(map[string]any{"source_url": row.SourceURL, "original_title": row.OriginalTitle,
			"original_description": row.OriginalDescription, "original_images": row.OriginalImages, "source_price": row.SourcePrice,
			"sku_data": row.SKUData, "raw_data": row.RawData, "collected_at": now, "updated_at": now}),
	}).Create(&row).Error
	if err != nil {
		return nil, err
	}
	if row.ID == uuid.Nil {
		err = s.DB.WithContext(ctx).Where("tenant_id = ? AND source_platform = ? AND source_product_id = ?", in.TenantID, platform, sourceID).First(&row).Error
	}
	return &row, err
}

func (s *Service) CreateSource(ctx context.Context, tenantID int64, body CreateSourceProductBody) (*SourceProduct, error) {
	if err := validateMoney(body.SourcePrice, body.Freight); err != nil {
		return nil, err
	}
	images, err := jsonValue(body.OriginalImages, []string{})
	if err != nil {
		return nil, err
	}
	skus, err := jsonValue(body.SKUData, []any{})
	if err != nil {
		return nil, err
	}
	raw, err := jsonValue(body.RawData, map[string]any{})
	if err != nil {
		return nil, err
	}
	if body.MinOrderQuantity < 0 {
		return nil, fmt.Errorf("%w: min order quantity cannot be negative", ErrValidation)
	}
	if body.MinOrderQuantity == 0 {
		body.MinOrderQuantity = 1
	}
	row := SourceProduct{TenantID: tenantID, SourcePlatform: strings.ToLower(strings.TrimSpace(body.SourcePlatform)),
		SourceProductID: strings.TrimSpace(body.SourceProductID), SourceURL: strings.TrimSpace(body.SourceURL),
		SupplierID: strings.TrimSpace(body.SupplierID), SupplierName: strings.TrimSpace(body.SupplierName),
		OriginalTitle: strings.TrimSpace(body.OriginalTitle), OriginalDescription: body.OriginalDescription,
		OriginalImages: images, OriginalCategory: strings.TrimSpace(body.OriginalCategory), SourcePrice: body.SourcePrice,
		Freight: body.Freight, MinOrderQuantity: body.MinOrderQuantity, SKUData: skus, RawData: raw,
		CollectedAt: time.Now().UTC(), Status: SourceStatusCollected}
	if row.SourcePlatform == "" || row.SourceProductID == "" || row.SourceURL == "" || row.OriginalTitle == "" {
		return nil, fmt.Errorf("%w: required source product fields are missing", ErrValidation)
	}
	if err := s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		if isUniqueError(err) {
			return nil, fmt.Errorf("%w: source product already exists", ErrConflict)
		}
		return nil, err
	}
	return &row, nil
}

func (s *Service) ListSources(ctx context.Context, tenantID int64, q ListQuery) (PageResult[SourceProduct], error) {
	q = normalizePage(q)
	var rows []SourceProduct
	var total int64
	db := s.DB.WithContext(ctx).Model(&SourceProduct{}).Where("tenant_id = ?", tenantID)
	if q.Status != "" {
		db = db.Where("status = ?", q.Status)
	}
	if k := strings.TrimSpace(q.Keyword); k != "" {
		db = db.Where("original_title ILIKE ? OR supplier_name ILIKE ?", "%"+k+"%", "%"+k+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return PageResult[SourceProduct]{}, err
	}
	err := db.Order("collected_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return PageResult[SourceProduct]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, err
}

func (s *Service) GetSource(ctx context.Context, tenantID int64, id uuid.UUID) (*SourceProduct, error) {
	var row SourceProduct
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}

func (s *Service) CreateCandidate(ctx context.Context, tenantID int64, sourceID uuid.UUID) (*Candidate, bool, error) {
	var source SourceProduct
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, sourceID).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, ErrNotFound
		}
		return nil, false, err
	}
	estCost := sumMoney(source.SourcePrice, source.Freight)
	row := Candidate{TenantID: tenantID, SourceProductID: source.ID, Status: CandidateStatusPending, EstimatedCost: estCost}
	err := s.DB.WithContext(ctx).Create(&row).Error
	created := err == nil
	if err != nil && isUniqueError(err) {
		var existing Candidate
		err = s.DB.WithContext(ctx).Where("tenant_id = ? AND source_product_id = ?", tenantID, sourceID).First(&existing).Error
		row = existing
		created = false
	}
	if err != nil {
		return nil, false, err
	}
	_ = s.DB.WithContext(ctx).Model(&SourceProduct{}).Where("id = ?", sourceID).Update("status", SourceStatusCandidate).Error
	row.SourceProduct = source
	return &row, created, nil
}

func sumMoney(values ...*float64) *float64 {
	total := 0.0
	has := false
	for _, v := range values {
		if v != nil {
			total += *v
			has = true
		}
	}
	if !has {
		return nil
	}
	total = math.Round(total*100) / 100
	return &total
}

func (s *Service) ListCandidates(ctx context.Context, tenantID int64, q ListQuery) (PageResult[Candidate], error) {
	q = normalizePage(q)
	var rows []Candidate
	var total int64
	db := s.DB.WithContext(ctx).Model(&Candidate{}).Where("candidates.tenant_id = ?", tenantID)
	if q.Status != "" {
		db = db.Where("candidates.status = ?", q.Status)
	}
	if k := strings.TrimSpace(q.Keyword); k != "" {
		db = db.Joins("JOIN source_products sp ON sp.id = candidates.source_product_id").Where("sp.original_title ILIKE ?", "%"+k+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return PageResult[Candidate]{}, err
	}
	order := "candidates.created_at DESC"
	column := map[string]string{"overall_score": "potential_score", "estimated_margin": "estimated_margin", "estimated_profit": "estimated_profit", "analyzed_at": "analyzed_at"}[q.SortBy]
	if column != "" {
		direction := "DESC"
		if strings.EqualFold(q.SortOrder, "asc") {
			direction = "ASC"
		}
		order = "candidates." + column + " " + direction + " NULLS LAST"
	}
	err := db.Order(order).Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	if err == nil {
		err = s.attachCandidateSources(ctx, tenantID, rows)
	}
	return PageResult[Candidate]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, err
}

func (s *Service) CostCenter(ctx context.Context, tenantID int64) (*CostCenterSummary, error) {
	var products []product.Product
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND candidate_id IS NOT NULL", tenantID).Order("created_at DESC").Preload("Images").Find(&products).Error; err != nil {
		return nil, err
	}
	out := &CostCenterSummary{ProductCount: int64(len(products)), Products: make([]any, 0, len(products))}
	var marginSum int64
	var analyzedCount int64
	for _, p := range products {
		margin := int64(0)
		if p.EstimatedMargin != nil {
			margin = int64(math.Round(*p.EstimatedMargin * 10000))
			marginSum += margin
			analyzedCount++
		}
		profit := 0.0
		if p.EstimatedProfit != nil {
			profit = *p.EstimatedProfit
		}
		if p.EstimatedMargin != nil && (profit <= 0 || margin < 2000) {
			out.LowProfitCount++
		}
		if profit >= 20 && margin >= 4000 {
			out.HighProfitCount++
		}
		if p.PurchaseCost == nil || p.SuggestedSalePrice == nil || profit < 0 {
			out.CostAnomalyCount++
		}
		cover := ""
		if len(p.Images) > 0 {
			cover = p.Images[0].PublicURL
			if cover == "" {
				cover = p.Images[0].OriginURL
			}
		}
		out.Products = append(out.Products, map[string]any{"id": p.ID, "title": p.Title, "coverUrl": cover, "purchaseCost": p.PurchaseCost, "freightCost": p.FreightCost, "packagingCost": p.PackagingCost, "otherCost": p.OtherCost, "suggestedSalePrice": p.SuggestedSalePrice, "estimatedProfit": p.EstimatedProfit, "estimatedMargin": p.EstimatedMargin, "catalogStatus": p.CatalogStatus})
	}
	if analyzedCount > 0 {
		out.AverageMarginBPS = marginSum / analyzedCount
	}
	return out, nil
}

func (s *Service) GetCandidate(ctx context.Context, tenantID int64, id uuid.UUID) (*Candidate, error) {
	var row Candidate
	err := s.DB.WithContext(ctx).Where("candidates.tenant_id = ? AND candidates.id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err == nil {
		err = s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, row.SourceProductID).First(&row.SourceProduct).Error
	}
	return &row, err
}

func (s *Service) attachCandidateSources(ctx context.Context, tenantID int64, rows []Candidate) error {
	if len(rows) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.SourceProductID)
	}
	var sources []SourceProduct
	if err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id IN ?", tenantID, ids).Find(&sources).Error; err != nil {
		return err
	}
	byID := make(map[uuid.UUID]SourceProduct, len(sources))
	for _, source := range sources {
		byID[source.ID] = source
	}
	for i := range rows {
		rows[i].SourceProduct = byID[rows[i].SourceProductID]
	}
	return nil
}

func (s *Service) transitionCandidate(ctx context.Context, tenantID int64, id uuid.UUID, target, reason string) (*Candidate, error) {
	var row Candidate
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
			return err
		}
		if row.Status == target {
			return nil
		}
		allowed := false
		switch target {
		case CandidateStatusWatch:
			allowed = row.Status == CandidateStatusPending || row.Status == CandidateStatusAnalyzing || row.Status == CandidateStatusRecommended
		case CandidateStatusRejected:
			allowed = row.Status == CandidateStatusPending || row.Status == CandidateStatusAnalyzing || row.Status == CandidateStatusRecommended || row.Status == CandidateStatusWatch
		}
		if !allowed {
			return ErrInvalidTransition
		}
		updates := map[string]any{"status": target}
		if target == CandidateStatusRejected {
			updates["rejection_reason"] = strings.TrimSpace(reason)
		}
		return tx.Model(&row).Updates(updates).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetCandidate(ctx, tenantID, id)
}

func (s *Service) WatchCandidate(ctx context.Context, tenantID int64, id uuid.UUID) (*Candidate, error) {
	return s.transitionCandidate(ctx, tenantID, id, CandidateStatusWatch, "")
}
func (s *Service) RejectCandidate(ctx context.Context, tenantID int64, id uuid.UUID, reason string) (*Candidate, error) {
	return s.transitionCandidate(ctx, tenantID, id, CandidateStatusRejected, reason)
}

func (s *Service) ApproveCandidate(ctx context.Context, tenantID int64, id uuid.UUID) (*ApproveResult, error) {
	var candidate Candidate
	var catalog product.Product
	created := false
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("candidates.tenant_id = ? AND candidates.id = ?", tenantID, id).First(&candidate).Error; err != nil {
			return err
		}
		if candidate.Status == CandidateStatusApproved {
			return tx.Preload("Images").Preload("SKUs").Where("tenant_id = ? AND candidate_id = ?", tenantID, id).First(&catalog).Error
		}
		if candidate.Status == CandidateStatusRejected {
			return ErrInvalidTransition
		}
		var src SourceProduct
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, candidate.SourceProductID).First(&src).Error; err != nil {
			return err
		}
		candidate.SourceProduct = src
		packagingCost, otherCost := (*float64)(nil), (*float64)(nil)
		var latest CandidateAnalysis
		if err := tx.Where("tenant_id = ? AND candidate_id = ?", tenantID, candidate.ID).Order("analysis_version DESC").First(&latest).Error; err == nil {
			var input struct {
				Request AnalyzeCandidateBody `json:"request"`
			}
			_ = json.Unmarshal(latest.InputSnapshot, &input)
			if value, parseErr := pricingengine.ParseMoney(input.Request.PackagingCost); parseErr == nil {
				packagingCost = moneyFloat(value)
			}
			if value, parseErr := pricingengine.ParseMoney(input.Request.OtherCost); parseErr == nil {
				otherCost = moneyFloat(value)
			}
		}
		catalog = product.Product{TenantID: tenantID, SourceProductID: &src.ID, CandidateID: &candidate.ID, Source: src.SourcePlatform, SourceURL: src.SourceURL,
			OriginalTitle: src.OriginalTitle, Title: src.OriginalTitle, Description: src.OriginalDescription, Currency: "CNY", Status: product.StatusDraft,
			CatalogStatus: CatalogStatusDraft, Category: src.OriginalCategory, Supplier: src.SupplierName, PurchaseCost: src.SourcePrice, FreightCost: src.Freight, PackagingCost: packagingCost, OtherCost: otherCost,
			SuggestedSalePrice: candidate.EstimatedSalePrice, SalePrice: candidate.EstimatedSalePrice, EstimatedProfit: candidate.EstimatedProfit,
			EstimatedMargin: candidate.EstimatedMargin, RawData: src.RawData}
		if err := tx.Create(&catalog).Error; err != nil {
			return err
		}
		if err := copySourceAssets(tx, &catalog, src); err != nil {
			return err
		}
		if err := tx.Model(&candidate).Updates(map[string]any{"status": CandidateStatusApproved}).Error; err != nil {
			return err
		}
		if err := tx.Model(&src).Update("status", SourceStatusApproved).Error; err != nil {
			return err
		}
		created = true
		return nil
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	_ = s.DB.WithContext(ctx).Preload("Images").Preload("SKUs").First(&catalog, "id = ?", catalog.ID).Error
	candidate.Status = CandidateStatusApproved
	return &ApproveResult{Candidate: candidate, Catalog: catalog, Created: created}, nil
}

func copySourceAssets(tx *gorm.DB, catalog *product.Product, src SourceProduct) error {
	var images []string
	if len(src.OriginalImages) > 0 {
		_ = json.Unmarshal(src.OriginalImages, &images)
	}
	for i, u := range images {
		u = strings.TrimSpace(u)
		if u == "" {
			continue
		}
		row := product.ProductImage{ProductID: catalog.ID, ImageType: product.ImageTypeMain, Source: product.ImageSourceCollect, OriginURL: u, PublicURL: u, SortOrder: i}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	var skus []map[string]any
	if len(src.SKUData) > 0 {
		_ = json.Unmarshal(src.SKUData, &skus)
	}
	for i, v := range skus {
		raw, _ := json.Marshal(v)
		code := stringValue(v, "skuCode", "skuId", "id", "SKUCode")
		if code == "" {
			code = fmt.Sprintf("SKU-%d", i+1)
		}
		name := stringValue(v, "skuName", "name", "title", "SKUName")
		price := numberValue(v, "price", "costPrice", "Price", "CostPrice")
		stock := intValue(v, "stock", "Stock")
		row := product.ProductSKU{ProductID: catalog.ID, SKUCode: code, SKUName: name, Price: price, CostPrice: price, Stock: stock, RawData: raw}
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
	}
	return nil
}

func stringValue(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
func numberValue(m map[string]any, keys ...string) *float64 {
	for _, k := range keys {
		if v, ok := m[k].(float64); ok {
			x := v
			return &x
		}
	}
	return nil
}
func intValue(m map[string]any, keys ...string) *int {
	for _, key := range keys {
		if v, ok := m[key].(float64); ok {
			x := int(v)
			return &x
		}
	}
	return nil
}

func (s *Service) ListCatalog(ctx context.Context, tenantID int64, q ListQuery) (PageResult[product.Product], error) {
	q = normalizePage(q)
	var rows []product.Product
	var total int64
	db := s.DB.WithContext(ctx).Model(&product.Product{}).Where("tenant_id = ? AND candidate_id IS NOT NULL", tenantID)
	if q.Status != "" {
		db = db.Where("catalog_status = ?", q.Status)
	}
	if k := strings.TrimSpace(q.Keyword); k != "" {
		db = db.Where("title ILIKE ? OR supplier ILIKE ?", "%"+k+"%", "%"+k+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return PageResult[product.Product]{}, err
	}
	err := db.Preload("Images").Preload("SKUs").Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return PageResult[product.Product]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, err
}

func (s *Service) GetCatalog(ctx context.Context, tenantID int64, id uuid.UUID) (*product.Product, error) {
	var row product.Product
	err := s.DB.WithContext(ctx).Preload("Images").Preload("SKUs").Where("tenant_id = ? AND id = ? AND candidate_id IS NOT NULL", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}

var supportedDraftPlatforms = map[string]struct{}{"xianyu": {}, "taobao": {}}

func SupportedDraftPlatform(v string) bool {
	_, ok := supportedDraftPlatforms[strings.ToLower(strings.TrimSpace(v))]
	return ok
}

func (s *Service) CreateListingDraft(ctx context.Context, tenantID int64, catalogID uuid.UUID, body CreateListingDraftBody) (*ListingDraft, bool, error) {
	platform := strings.ToLower(strings.TrimSpace(body.Platform))
	if !SupportedDraftPlatform(platform) {
		return nil, false, fmt.Errorf("%w: unsupported listing platform", ErrValidation)
	}
	catalog, err := s.GetCatalog(ctx, tenantID, catalogID)
	if err != nil {
		return nil, false, err
	}
	images := make([]string, 0, len(catalog.Images))
	for _, img := range catalog.Images {
		u := img.PublicURL
		if u == "" {
			u = img.OriginURL
		}
		if u != "" {
			images = append(images, u)
		}
	}
	imageJSON, _ := json.Marshal(images)
	skuJSON, _ := json.Marshal(catalog.SKUs)
	profileBody := PricingProfileBody{}
	if body.PricingProfile != nil {
		profileBody = *body.PricingProfile
	}
	profile, profileErr := profileFromBody(platform, profileBody)
	if profileErr != nil {
		return nil, false, fmt.Errorf("%w: %v", ErrValidation, profileErr)
	}
	purchase, _, _ := pricingengine.MoneyFromLegacyFloat(catalog.PurchaseCost)
	freight, _, _ := pricingengine.MoneyFromLegacyFloat(catalog.FreightCost)
	packaging, _, _ := pricingengine.MoneyFromLegacyFloat(catalog.PackagingCost)
	other, _, _ := pricingengine.MoneyFromLegacyFloat(catalog.OtherCost)
	pricing, pricingErr := pricingengine.Calculate(pricingengine.CostInput{Currency: "CNY", PurchaseCost: purchase, FreightCost: freight, PackagingCost: packaging, OtherCost: other, TargetProfit: 1000, TargetMarginBPS: 4000, MinimumProfit: 500, MinimumMarginBPS: 2000, Profile: profile})
	if pricingErr != nil {
		return nil, false, pricingErr
	}
	pricingJSON, _ := json.Marshal(pricing)
	row := ListingDraft{TenantID: tenantID, CatalogProductID: catalog.ID, Platform: platform, ShopID: body.ShopID, Title: catalog.Title, Description: catalog.Description, Images: imageJSON, SalePrice: moneyFloat(pricing.SuggestedSalePrice), EstimatedProfit: moneyFloat(pricing.EstimatedProfit), EstimatedMargin: bpsFloat(pricing.EstimatedMarginBPS), PricingSnapshot: pricingJSON, PlatformCategory: catalog.Category, PlatformSKUData: skuJSON, PublishStatus: ListingStatusDraft}
	err = s.DB.WithContext(ctx).Create(&row).Error
	created := err == nil
	if err != nil && isUniqueError(err) {
		var existing ListingDraft
		err = s.DB.WithContext(ctx).Where("tenant_id = ? AND catalog_product_id = ? AND platform = ?", tenantID, catalogID, platform).First(&existing).Error
		row = existing
		created = false
	}
	return &row, created, err
}

func (s *Service) ListListingDrafts(ctx context.Context, tenantID int64, q ListQuery) (PageResult[ListingDraft], error) {
	q = normalizePage(q)
	var rows []ListingDraft
	var total int64
	db := s.DB.WithContext(ctx).Model(&ListingDraft{}).Where("tenant_id = ?", tenantID)
	if q.Status != "" {
		db = db.Where("publish_status = ?", q.Status)
	}
	if q.Platform != "" {
		db = db.Where("platform = ?", strings.ToLower(q.Platform))
	}
	if q.CatalogID != nil {
		db = db.Where("catalog_product_id = ?", *q.CatalogID)
	}
	if k := strings.TrimSpace(q.Keyword); k != "" {
		db = db.Where("title ILIKE ?", "%"+k+"%")
	}
	if err := db.Count(&total).Error; err != nil {
		return PageResult[ListingDraft]{}, err
	}
	err := db.Order("created_at DESC").Offset((q.Page - 1) * q.PageSize).Limit(q.PageSize).Find(&rows).Error
	return PageResult[ListingDraft]{List: rows, Page: q.Page, PageSize: q.PageSize, Total: total, TotalPages: totalPages(total, q.PageSize)}, err
}

func (s *Service) GetListingDraft(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingDraft, error) {
	var row ListingDraft
	err := s.DB.WithContext(ctx).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	return &row, err
}

func validListingTransition(from, to string) bool {
	if from == to {
		return true
	}
	switch from {
	case ListingStatusDraft:
		return to == ListingStatusReady
	case ListingStatusReady:
		return to == ListingStatusDraft
	case ListingStatusFailed:
		return to == ListingStatusDraft || to == ListingStatusReady
	}
	return false
}

func (s *Service) UpdateListingDraft(ctx context.Context, tenantID int64, id uuid.UUID, body UpdateListingDraftBody) (*ListingDraft, error) {
	var row ListingDraft
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id = ? AND id = ?", tenantID, id).First(&row).Error; err != nil {
			return err
		}
		if row.PublishStatus == ListingStatusPublishing || row.PublishStatus == ListingStatusPublished || row.PublishStatus == ListingStatusOffline {
			return ErrInvalidTransition
		}
		updates := map[string]any{}
		if body.Title != nil {
			v := strings.TrimSpace(*body.Title)
			if v == "" {
				return fmt.Errorf("%w: title is required", ErrValidation)
			}
			updates["title"] = v
		}
		if body.Description != nil {
			updates["description"] = *body.Description
		}
		if len(body.Images) > 0 {
			v, e := jsonValue(body.Images, []string{})
			if e != nil {
				return e
			}
			updates["images"] = v
		}
		if body.SalePrice != nil {
			if e := validateMoney(body.SalePrice); e != nil {
				return e
			}
			updates["sale_price"] = body.SalePrice
			catalog, e := s.GetCatalog(ctx, tenantID, row.CatalogProductID)
			if e != nil {
				return e
			}
			var previous pricingengine.PricingResult
			if e = json.Unmarshal(row.PricingSnapshot, &previous); e != nil {
				return fmt.Errorf("%w: invalid pricing snapshot", ErrValidation)
			}
			purchase, _, e := pricingengine.MoneyFromLegacyFloat(catalog.PurchaseCost)
			if e != nil {
				return e
			}
			freight, _, e := pricingengine.MoneyFromLegacyFloat(catalog.FreightCost)
			if e != nil {
				return e
			}
			packaging, _, e := pricingengine.MoneyFromLegacyFloat(catalog.PackagingCost)
			if e != nil {
				return e
			}
			other, _, e := pricingengine.MoneyFromLegacyFloat(catalog.OtherCost)
			if e != nil {
				return e
			}
			sale, _, e := pricingengine.MoneyFromLegacyFloat(body.SalePrice)
			if e != nil {
				return e
			}
			recalculated, e := pricingengine.Calculate(pricingengine.CostInput{Currency: "CNY", PurchaseCost: purchase, FreightCost: freight, PackagingCost: packaging, OtherCost: other, TargetProfit: 1000, TargetMarginBPS: 4000, MinimumProfit: 500, MinimumMarginBPS: 2000, SalePrice: &sale, Profile: previous.Profile})
			if e != nil {
				return e
			}
			snapshot, _ := json.Marshal(recalculated)
			updates["estimated_profit"] = moneyFloat(recalculated.EstimatedProfit)
			updates["estimated_margin"] = bpsFloat(recalculated.EstimatedMarginBPS)
			updates["pricing_snapshot"] = snapshot
		}
		if body.PlatformCategory != nil {
			updates["platform_category"] = strings.TrimSpace(*body.PlatformCategory)
		}
		if len(body.PlatformSKUData) > 0 {
			v, e := jsonValue(body.PlatformSKUData, []any{})
			if e != nil {
				return e
			}
			updates["platform_sku_data"] = v
		}
		if body.PublishStatus != nil {
			target := strings.ToLower(strings.TrimSpace(*body.PublishStatus))
			if !validListingTransition(row.PublishStatus, target) {
				return ErrInvalidTransition
			}
			updates["publish_status"] = target
		}
		if len(updates) == 0 {
			return nil
		}
		return tx.Model(&row).Updates(updates).Error
	})
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return s.GetListingDraft(ctx, tenantID, id)
}

func (s *Service) DeleteListingDraft(ctx context.Context, tenantID int64, id uuid.UUID) error {
	row, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return err
	}
	if row.PublishStatus == ListingStatusPublishing || row.PublishStatus == ListingStatusPublished || row.PublishStatus == ListingStatusOffline || row.ExternalListingID != "" || row.PublishedAt != nil {
		return ErrInvalidTransition
	}
	return s.DB.WithContext(ctx).Delete(row).Error
}

func isUniqueError(err error) bool {
	if err == nil {
		return false
	}
	v := strings.ToLower(err.Error())
	return strings.Contains(v, "unique constraint") || strings.Contains(v, "duplicate key") || strings.Contains(v, "unique failed")
}
