package productflow

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/contentengine"
	"github.com/trademind-ai/trademind/backend/internal/modules/productflow/pricingengine"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	maxBulkContentItems = 20
	maxPackageImages    = 30
)

type ListingWorkspace struct {
	Listing        *ListingDraft                 `json:"listing"`
	Catalog        *product.Product              `json:"catalog"`
	Content        *ListingContentVersion        `json:"content,omitempty"`
	Assets         []ListingAsset                `json:"assets"`
	Packages       []ListingPublishPackage       `json:"packages"`
	PublishRecords []ManualPublishRecord         `json:"publishRecords"`
	QualityReviews []ListingContentQualityReview `json:"qualityReviews"`
}

type SellTestReadiness struct {
	ListingDraftID          uuid.UUID `json:"listingDraftId"`
	HasRealImages           bool      `json:"hasRealImages"`
	ContentApproved         bool      `json:"contentApproved"`
	SKUValid                bool      `json:"skuValid"`
	SalePriceValid          bool      `json:"salePriceValid"`
	PositiveEstimatedProfit bool      `json:"positiveEstimatedProfit"`
	NoBlockers              bool      `json:"noBlockers"`
	PublishPackageComplete  bool      `json:"publishPackageComplete"`
	PublicInternalIsolated  bool      `json:"publicInternalIsolated"`
	CanMarkManualPublished  bool      `json:"canMarkManualPublished"`
	ReadyForManualSellTest  bool      `json:"readyForManualSellTest"`
}

func (s *Service) SellTestReadiness(ctx context.Context, tenantID int64, id uuid.UUID) (*SellTestReadiness, error) {
	w, err := s.ListingWorkspace(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	var blockers []string
	if w.Content != nil {
		_ = json.Unmarshal(w.Content.Blockers, &blockers)
	}
	var skus []any
	_ = json.Unmarshal(w.Listing.PlatformSKUData, &skus)
	hasImages := false
	for _, a := range w.Assets {
		if !a.Excluded && a.Cached {
			hasImages = true
			break
		}
	}
	completePackage, isolated := false, false
	for _, p := range w.Packages {
		if file, e := s.PublishPackageFile(ctx, tenantID, p.ID); e == nil && p.SizeBytes > 0 {
			completePackage = true
			isolated = verifyPackageIsolation(file.ArchivePath)
			break
		}
	}
	r := &SellTestReadiness{ListingDraftID: id, HasRealImages: hasImages, ContentApproved: w.Content != nil && w.Content.ReviewStatus == "approved", SKUValid: len(skus) > 0, SalePriceValid: w.Listing.SalePrice != nil && *w.Listing.SalePrice > 0, PositiveEstimatedProfit: w.Listing.EstimatedProfit != nil && *w.Listing.EstimatedProfit > 0, NoBlockers: len(blockers) == 0, PublishPackageComplete: completePackage, PublicInternalIsolated: isolated, CanMarkManualPublished: w.Listing.PublishStatus == ListingStatusReadyToPublish}
	r.ReadyForManualSellTest = r.HasRealImages && r.ContentApproved && r.SKUValid && r.SalePriceValid && r.PositiveEstimatedProfit && r.NoBlockers && r.PublishPackageComplete && r.PublicInternalIsolated && r.CanMarkManualPublished
	return r, nil
}

func verifyPackageIsolation(path string) bool {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false
	}
	defer zr.Close()
	for _, f := range zr.File {
		if !strings.HasPrefix(f.Name, "public/") {
			continue
		}
		r, err := f.Open()
		if err != nil {
			return false
		}
		data, err := io.ReadAll(io.LimitReader(r, 1024*1024))
		_ = r.Close()
		if err != nil {
			return false
		}
		text := strings.ToLower(string(data))
		for _, forbidden := range []string{"sourceurl", "supplier", "purchasecost", "freightcost", "estimatedprofit", "estimatedmargin", "marketsignal", "overallscore", "1688.com"} {
			if strings.Contains(text, forbidden) {
				return false
			}
		}
	}
	return true
}

func (s *Service) contentInput(ctx context.Context, tenantID int64, listing *ListingDraft, userFacts map[string]string) (contentengine.Input, error) {
	catalog, err := s.GetCatalog(ctx, tenantID, listing.CatalogProductID)
	if err != nil {
		return contentengine.Input{}, err
	}
	facts := map[string]string{}
	for k, v := range userFacts {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k != "" && v != "" {
			facts[k] = v
		}
	}
	images := []string{}
	for _, img := range catalog.Images {
		u := img.PublicURL
		if u == "" {
			u = img.OriginURL
		}
		if u != "" {
			images = append(images, u)
		}
	}
	skus := make([]contentengine.SKU, 0, len(catalog.SKUs))
	for _, sku := range catalog.SKUs {
		attrs := map[string]any{}
		_ = json.Unmarshal(sku.Attrs, &attrs)
		skus = append(skus, contentengine.SKU{Code: sku.SKUCode, Name: sku.SKUName, Attributes: attrs})
	}
	return contentengine.Input{CatalogTitle: catalog.Title, SourceTitle: catalog.OriginalTitle, Description: catalog.Description, Category: catalog.Category, Supplier: catalog.Supplier, SourceURL: catalog.SourceURL, Images: images, SKUs: skus, Facts: facts}, nil
}

func jsonData(v any) datatypes.JSON { b, _ := json.Marshal(v); return datatypes.JSON(b) }

func (s *Service) GenerateListingContent(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, body GenerateListingContentBody) (*ListingContentVersion, error) {
	return s.generateListingContent(ctx, tenantID, id, actor, body, nil)
}

func (s *Service) generateListingContent(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, body GenerateListingContentBody, batchItemID *uuid.UUID) (*ListingContentVersion, error) {
	if batchItemID != nil {
		var existing ListingContentVersion
		if err := s.DB.WithContext(ctx).Where("operation_batch_item_id=?", *batchItemID).First(&existing).Error; err == nil {
			return &existing, nil
		}
	}
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.PublishStatus == ListingStatusPublishedManual || listing.PublishStatus == ListingStatusPublishedAPI || listing.PublishStatus == ListingStatusOffline {
		return nil, ErrInvalidTransition
	}
	profile, err := contentengine.ProfileFor(listing.Platform)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrValidation, err)
	}
	in, err := s.contentInput(ctx, tenantID, listing, body.UserFacts)
	if err != nil {
		return nil, err
	}
	mode := strings.ToLower(strings.TrimSpace(body.GenerationMode))
	if mode == "" {
		mode = contentengine.ModeTemplate
	}
	if mode != contentengine.ModeTemplate && mode != contentengine.ModeAI {
		return nil, fmt.Errorf("%w: invalid generation mode", ErrValidation)
	}
	out := contentengine.GenerateTemplate(in, profile)
	aiStatus := "not_requested"
	provider := ""
	modelName := ""
	inputTokens, outputTokens := 0, 0
	var costMicros, latencyMS int64
	if mode == contentengine.ModeAI {
		aiStatus = "fallback_template"
		if s.AIContentGenerate != nil {
			aiResult, e := s.AIContentGenerate(ctx, contentengine.BuildPrompt(in, profile))
			if e == nil {
				if parsed, parseErr := contentengine.ParseAI(aiResult.Content, in, profile); parseErr == nil {
					out = parsed
					aiStatus = "succeeded"
					provider, modelName = aiResult.Provider, aiResult.Model
					inputTokens, outputTokens = aiResult.InputTokens, aiResult.OutputTokens
					costMicros, latencyMS = aiResult.CostMicros, aiResult.LatencyMS
				} else {
					out.Warnings = append(out.Warnings, "AI返回格式无效，已降级为模板内容")
				}
			} else {
				out.Warnings = append(out.Warnings, "AI不可用或超时，已降级为模板内容")
			}
		}
	}
	var row ListingContentVersion
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxVersion int
		if e := tx.Model(&ListingContentVersion{}).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Select("COALESCE(MAX(version),0)").Scan(&maxVersion).Error; e != nil {
			return e
		}
		status := "needs_review"
		row = ListingContentVersion{TenantID: tenantID, ListingDraftID: id, CatalogProductID: listing.CatalogProductID, Platform: listing.Platform, Version: maxVersion + 1, GenerationMode: mode, ContentProfileVersion: profile.Version, PromptVersion: profile.PromptVersion, InputSnapshot: jsonData(in), Title: out.Title, Description: out.Description, SellingPoints: jsonData(out.SellingPoints), Keywords: jsonData(out.Keywords), FAQ: jsonData(out.FAQ), SKUContent: jsonData(out.SKUContent), Warnings: jsonData(out.Warnings), Blockers: jsonData(out.Blockers), ReviewStatus: status, AIStatus: aiStatus, ModelProvider: provider, ModelName: modelName, AIInputTokens: inputTokens, AIOutputTokens: outputTokens, AICostMicros: costMicros, AILatencyMS: latencyMS, CreatedBy: actor, OperationBatchItemID: batchItemID}
		if e := tx.Create(&row).Error; e != nil {
			return e
		}
		if e := syncListingAssets(tx, tenantID, listing.CatalogProductID, id, in.Images); e != nil {
			return e
		}
		return tx.Model(&ListingDraft{}).Where("tenant_id=? AND id=?", tenantID, id).Updates(map[string]any{"current_content_version_id": row.ID, "title": out.Title, "description": out.Description, "platform_sku_data": row.SKUContent, "publish_status": ListingStatusNeedsReview}).Error
	})
	return &row, err
}

func syncListingAssets(tx *gorm.DB, tenantID int64, catalogID, listingID uuid.UUID, urls []string) error {
	seen := map[string]bool{}
	order := 0
	for _, raw := range urls {
		u := strings.TrimSpace(raw)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		var count int64
		if err := tx.Model(&ListingAsset{}).Where("tenant_id=? AND listing_draft_id=? AND source_url=?", tenantID, listingID, u).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			row := ListingAsset{TenantID: tenantID, CatalogProductID: catalogID, ListingDraftID: &listingID, SourceType: "catalog", SourceURL: u, SourceReference: "catalog_image", SortOrder: order, IsPrimary: order == 0}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		order++
	}
	return nil
}

func (s *Service) UpdateListingContent(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, body UpdateListingContentBody) (*ListingContentVersion, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.CurrentContentVersionID == nil {
		return nil, ErrNotFound
	}
	var current ListingContentVersion
	if err = s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, *listing.CurrentContentVersionID).First(&current).Error; err != nil {
		return nil, err
	}
	in := contentengine.Input{}
	if err = json.Unmarshal(current.InputSnapshot, &in); err != nil {
		return nil, fmt.Errorf("%w: invalid content snapshot", ErrValidation)
	}
	profile, _ := contentengine.ProfileFor(listing.Platform)
	out := contentengine.Guard(in, profile, contentengine.Output{Title: body.Title, Description: body.Description, SellingPoints: body.SellingPoints, Keywords: body.Keywords, FAQ: body.FAQ})
	if strings.TrimSpace(out.Title) == "" || strings.TrimSpace(out.Description) == "" {
		return nil, fmt.Errorf("%w: title and description required", ErrValidation)
	}
	var row ListingContentVersion
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var max int
		tx.Model(&ListingContentVersion{}).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Select("COALESCE(MAX(version),0)").Scan(&max)
		row = current
		row.ID = uuid.Nil
		row.CreatedAt = time.Time{}
		row.Version = max + 1
		row.GenerationMode = "manual_edit"
		row.Title = out.Title
		row.Description = out.Description
		row.SellingPoints = jsonData(out.SellingPoints)
		row.Keywords = jsonData(out.Keywords)
		row.FAQ = jsonData(out.FAQ)
		row.Warnings = jsonData(out.Warnings)
		row.Blockers = jsonData(out.Blockers)
		row.ReviewStatus = "needs_review"
		row.CreatedBy = actor
		if e := tx.Create(&row).Error; e != nil {
			return e
		}
		return tx.Model(&ListingDraft{}).Where("tenant_id=? AND id=?", tenantID, id).Updates(map[string]any{"current_content_version_id": row.ID, "title": row.Title, "description": row.Description, "publish_status": ListingStatusNeedsReview}).Error
	})
	return &row, err
}

func (s *Service) ReviewListingContent(ctx context.Context, tenantID int64, id uuid.UUID, body ReviewListingContentBody) (*ListingContentVersion, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.CurrentContentVersionID == nil {
		return nil, ErrNotFound
	}
	action := strings.ToLower(strings.TrimSpace(body.Action))
	if action != "approve" && action != "reject" {
		return nil, fmt.Errorf("%w: review action", ErrValidation)
	}
	var row ListingContentVersion
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("tenant_id=? AND id=?", tenantID, *listing.CurrentContentVersionID).First(&row).Error; e != nil {
			return e
		}
		status := "approved"
		listingStatus := ListingStatusApproved
		if action == "reject" {
			status = "rejected"
			listingStatus = ListingStatusNeedsReview
		}
		if action == "approve" {
			var blockers []string
			_ = json.Unmarshal(row.Blockers, &blockers)
			if len(blockers) > 0 {
				return fmt.Errorf("%w: content blockers must be resolved", ErrValidation)
			}
		}
		if e := tx.Model(&row).Update("review_status", status).Error; e != nil {
			return e
		}
		return tx.Model(&ListingDraft{}).Where("tenant_id=? AND id=?", tenantID, id).Update("publish_status", listingStatus).Error
	})
	return &row, err
}

func (s *Service) MarkListingReady(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingDraft, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.CurrentContentVersionID == nil {
		return nil, fmt.Errorf("%w: no approved content", ErrValidation)
	}
	var cv ListingContentVersion
	if err = s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, *listing.CurrentContentVersionID).First(&cv).Error; err != nil {
		return nil, err
	}
	var blockers []string
	_ = json.Unmarshal(cv.Blockers, &blockers)
	var cachedImages int64
	_ = s.DB.WithContext(ctx).Model(&ListingAsset{}).Where("tenant_id=? AND listing_draft_id=? AND excluded=false AND cache_path<>''", tenantID, id).Count(&cachedImages).Error
	var skus []any
	_ = json.Unmarshal(listing.PlatformSKUData, &skus)
	if listing.SalePrice == nil || *listing.SalePrice <= 0 {
		return nil, fmt.Errorf("%w: publish checklist failed (approved content, title, description, image, price, sku/pricing and positive profit required)", ErrValidation)
	}
	catalog, err := s.GetCatalog(ctx, tenantID, listing.CatalogProductID)
	if err != nil {
		return nil, err
	}
	var previousPricing pricingengine.PricingResult
	if err = json.Unmarshal(listing.PricingSnapshot, &previousPricing); err != nil {
		return nil, fmt.Errorf("%w: invalid pricing snapshot", ErrValidation)
	}
	salePrice, _, err := pricingengine.MoneyFromLegacyFloat(listing.SalePrice)
	if err != nil {
		return nil, err
	}
	pricingInput, err := s.catalogPricingInput(ctx, tenantID, catalog, listing.Platform, previousPricing.Profile, &salePrice)
	if err != nil {
		return nil, err
	}
	currentPricing, err := pricingengine.Calculate(pricingInput)
	if err != nil {
		return nil, err
	}
	if cv.ReviewStatus != "approved" || strings.TrimSpace(cv.Title) == "" || strings.TrimSpace(cv.Description) == "" || cachedImages == 0 || listing.EstimatedProfit == nil || *listing.EstimatedProfit <= 0 || currentPricing.EstimatedProfit <= 0 || len(blockers) > 0 {
		return nil, fmt.Errorf("%w: publish checklist failed (approved content, title, description, image, price, sku/pricing and positive profit required)", ErrValidation)
	}
	if len(skus) == 0 {
		return nil, fmt.Errorf("%w: listing sku required", ErrValidation)
	}
	pricingSnapshot, _ := json.Marshal(currentPricing)
	if err = s.DB.WithContext(ctx).Model(listing).Updates(map[string]any{
		"publish_status":   ListingStatusReadyToPublish,
		"estimated_profit": moneyFloat(currentPricing.EstimatedProfit),
		"estimated_margin": bpsFloat(currentPricing.EstimatedMarginBPS),
		"pricing_snapshot": pricingSnapshot,
	}).Error; err != nil {
		return nil, err
	}
	return s.GetListingDraft(ctx, tenantID, id)
}

func safeFileText(s string, max int) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if len([]rune(s)) > max {
		s = string([]rune(s)[:max])
	}
	return s
}
func addZip(z *zip.Writer, name string, data []byte) error {
	clean := filepath.ToSlash(filepath.Clean(name))
	if strings.HasPrefix(clean, "../") || strings.HasPrefix(clean, "/") || strings.Contains(clean, ":") {
		return errors.New("unsafe archive path")
	}
	w, err := z.Create(clean)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

func (s *Service) GeneratePublishPackage(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID) (*ListingPublishPackage, error) {
	return s.generatePublishPackage(ctx, tenantID, id, actor, nil)
}

func (s *Service) generatePublishPackage(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, batchItemID *uuid.UUID) (*ListingPublishPackage, error) {
	if batchItemID != nil {
		var existing ListingPublishPackage
		if err := s.DB.WithContext(ctx).Where("operation_batch_item_id=?", *batchItemID).First(&existing).Error; err == nil {
			return &existing, nil
		}
	}
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.PublishStatus != ListingStatusReadyToPublish {
		return nil, ErrInvalidTransition
	}
	var cv ListingContentVersion
	if err = s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, *listing.CurrentContentVersionID).First(&cv).Error; err != nil {
		return nil, err
	}
	catalog, err := s.GetCatalog(ctx, tenantID, listing.CatalogProductID)
	if err != nil {
		return nil, err
	}
	var points, keywords []string
	_ = json.Unmarshal(cv.SellingPoints, &points)
	_ = json.Unmarshal(cv.Keywords, &keywords)
	public := map[string]any{"title": cv.Title, "description": cv.Description, "sellingPoints": points, "keywords": keywords, "sku": json.RawMessage(cv.SKUContent), "salePrice": listing.SalePrice, "platform": listing.Platform}
	internal := map[string]any{"supplier": catalog.Supplier, "sourceUrl": catalog.SourceURL, "purchaseCost": catalog.PurchaseCost, "freightCost": catalog.FreightCost, "estimatedProfit": listing.EstimatedProfit, "estimatedMargin": listing.EstimatedMargin, "pricing": json.RawMessage(listing.PricingSnapshot)}
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	pubJSON, _ := json.MarshalIndent(public, "", "  ")
	skuJSON, _ := json.MarshalIndent(json.RawMessage(cv.SKUContent), "", "  ")
	pricingJSON, _ := json.MarshalIndent(map[string]any{"currency": "CNY", "salePrice": listing.SalePrice}, "", "  ")
	sourceJSON, _ := json.MarshalIndent(internal, "", "  ")
	files := map[string][]byte{"public/metadata.json": pubJSON, "public/title.txt": []byte(safeFileText(cv.Title, 512)), "public/description.txt": []byte(safeFileText(cv.Description, 10000)), "public/selling-points.txt": []byte(strings.Join(points, "\n")), "public/keywords.txt": []byte(strings.Join(keywords, "\n")), "public/sku.json": skuJSON, "public/pricing.json": pricingJSON, "internal/source.json": sourceJSON, "internal/pricing.json": []byte(listing.PricingSnapshot), "README.txt": []byte("public/ 为消费者上架资料；internal/ 仅供运营人员查看，禁止复制到消费者详情。\n图片使用前请确认版权和来源。")}
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		if err = addZip(z, n, files[n]); err != nil {
			return nil, err
		}
	}
	var assets []ListingAsset
	_ = s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=? AND excluded=false", tenantID, id).Order("sort_order").Find(&assets).Error
	publicAssets := make([]map[string]any, 0, len(assets))
	for i, asset := range assets {
		if i >= maxPackageImages {
			break
		}
		publicAssets = append(publicAssets, map[string]any{"position": i + 1, "isPrimary": asset.IsPrimary, "included": !asset.Excluded})
	}
	publicAssetManifest, _ := json.MarshalIndent(publicAssets, "", "  ")
	internalAssetManifest, _ := json.MarshalIndent(assets, "", "  ")
	_ = addZip(z, "public/images/manifest.json", publicAssetManifest)
	_ = addZip(z, "internal/image-sources.json", internalAssetManifest)
	includedImages := 0
	for _, a := range assets {
		if a.CachePath == "" {
			continue
		}
		data, e := trustedAssetBytes(tenantID, id, a)
		if e != nil {
			return nil, fmt.Errorf("%w: cached image validation failed", ErrValidation)
		}
		includedImages++
		if e = addZip(z, fmt.Sprintf("public/images/%02d%s", includedImages, safeImageExt(a.MimeType)), data); e != nil {
			return nil, e
		}
	}
	if includedImages == 0 {
		return nil, fmt.Errorf("%w: at least one cached image is required", ErrValidation)
	}
	if err = z.Close(); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(buf.Bytes())
	root := filepath.Join(os.TempDir(), "trademind-publish-packages", fmt.Sprintf("tenant-%d", tenantID), id.String())
	if err = os.MkdirAll(root, 0700); err != nil {
		return nil, err
	}
	var version int
	s.DB.WithContext(ctx).Model(&ListingPublishPackage{}).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Select("COALESCE(MAX(package_version),0)").Scan(&version)
	version++
	path := filepath.Join(root, fmt.Sprintf("package-v%d.zip", version))
	if err = os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		return nil, err
	}
	manifest := map[string]any{"publicFiles": []string{"metadata.json", "title.txt", "description.txt", "selling-points.txt", "keywords.txt", "sku.json", "pricing.json", "images/"}, "internalFiles": []string{"source.json", "pricing.json", "image-sources.json"}, "contentVersion": cv.Version, "pricingVersion": listing.PricingVersion}
	row := ListingPublishPackage{TenantID: tenantID, ListingDraftID: id, ContentVersionID: cv.ID, PackageVersion: version, PricingVersion: listing.PricingVersion, Manifest: jsonData(manifest), ArchivePath: path, ArchiveHash: hex.EncodeToString(sum[:]), SizeBytes: int64(buf.Len()), GeneratedAt: time.Now(), CreatedBy: actor, OperationBatchItemID: batchItemID}
	if err = s.DB.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return &row, nil
}
func safeImageExt(m string) string {
	switch strings.ToLower(m) {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "image/gif":
		return ".gif"
	}
	return ".img"
}

func (s *Service) PublishPackageFile(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingPublishPackage, error) {
	var row ListingPublishPackage
	err := s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	root := filepath.Clean(filepath.Join(os.TempDir(), "trademind-publish-packages", fmt.Sprintf("tenant-%d", tenantID)))
	path := filepath.Clean(row.ArchivePath)
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) {
		return nil, fmt.Errorf("%w: unsafe package path", ErrValidation)
	}
	return &row, nil
}

func (s *Service) MarkPublishedManual(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, body ManualPublishBody) (*ManualPublishRecord, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	if listing.PublishStatus != ListingStatusReadyToPublish {
		return nil, ErrInvalidTransition
	}
	when := time.Now()
	if body.PublishedAt != nil {
		when = *body.PublishedAt
	}
	var pkg ListingPublishPackage
	pkgID := (*uuid.UUID)(nil)
	if e := s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("package_version DESC").First(&pkg).Error; e == nil {
		pkgID = &pkg.ID
	}
	row := ManualPublishRecord{TenantID: tenantID, ListingDraftID: id, ContentVersionID: *listing.CurrentContentVersionID, PublishPackageID: pkgID, Platform: listing.Platform, PlatformListingID: strings.TrimSpace(body.PlatformListingID), ListingURL: strings.TrimSpace(body.ListingURL), PublishMethod: "manual", PublishedAt: when, Notes: safeFileText(body.Notes, 2000), CreatedBy: actor}
	if row.PlatformListingID == "" {
		return nil, fmt.Errorf("%w: platform listing id required", ErrValidation)
	}
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&row).Error; e != nil {
			return e
		}
		return tx.Model(listing).Updates(map[string]any{"publish_status": ListingStatusPublishedManual, "publish_method": "manual", "external_listing_id": row.PlatformListingID, "listing_url": row.ListingURL, "published_at": when}).Error
	})
	return &row, err
}

func (s *Service) ListingWorkspace(ctx context.Context, tenantID int64, id uuid.UUID) (*ListingWorkspace, error) {
	listing, err := s.GetListingDraft(ctx, tenantID, id)
	if err != nil {
		return nil, err
	}
	catalog, err := s.GetCatalog(ctx, tenantID, listing.CatalogProductID)
	if err != nil {
		return nil, err
	}
	out := &ListingWorkspace{Listing: listing, Catalog: catalog}
	if listing.CurrentContentVersionID != nil {
		var c ListingContentVersion
		if s.DB.WithContext(ctx).Where("tenant_id=? AND id=?", tenantID, *listing.CurrentContentVersionID).First(&c).Error == nil {
			out.Content = &c
		}
	}
	s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("sort_order").Find(&out.Assets)
	for i := range out.Assets {
		out.Assets[i].Cached = out.Assets[i].CachePath != "" && pathWithin(listingAssetRoot(tenantID, id), out.Assets[i].CachePath)
	}
	s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("package_version DESC").Find(&out.Packages)
	s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("published_at DESC").Find(&out.PublishRecords)
	s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("created_at DESC").Find(&out.QualityReviews)
	return out, nil
}
func (s *Service) ListContentVersions(ctx context.Context, tenantID int64, id uuid.UUID) ([]ListingContentVersion, error) {
	var out []ListingContentVersion
	err := s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("version DESC").Find(&out).Error
	return out, err
}

func (s *Service) RestoreListingContent(ctx context.Context, tenantID int64, id uuid.UUID, actor *uuid.UUID, contentID uuid.UUID) (*ListingContentVersion, error) {
	var source ListingContentVersion
	if err := s.DB.WithContext(ctx).Where("tenant_id=? AND listing_draft_id=? AND id=?", tenantID, id, contentID).First(&source).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return s.UpdateListingContent(ctx, tenantID, id, actor, UpdateListingContentBody{Title: source.Title, Description: source.Description, SellingPoints: stringList(source.SellingPoints), Keywords: stringList(source.Keywords), FAQ: objectList(source.FAQ)})
}

func stringList(raw datatypes.JSON) []string {
	var out []string
	_ = json.Unmarshal(raw, &out)
	return out
}
func objectList(raw datatypes.JSON) []map[string]string {
	var out []map[string]string
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Service) UpdateListingAssets(ctx context.Context, tenantID int64, id uuid.UUID, body UpdateListingAssetsBody) ([]ListingAsset, error) {
	if len(body.Assets) == 0 || len(body.Assets) > maxPackageImages {
		return nil, fmt.Errorf("%w: asset selection must contain 1-%d items", ErrValidation, maxPackageImages)
	}
	returnAssets := []ListingAsset{}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		primarySeen := false
		for _, item := range body.Assets {
			if item.IsPrimary && !item.Excluded {
				if primarySeen {
					return fmt.Errorf("%w: only one primary image is allowed", ErrValidation)
				}
				primarySeen = true
			}
			res := tx.Model(&ListingAsset{}).Where("tenant_id=? AND listing_draft_id=? AND id=?", tenantID, id, item.ID).Updates(map[string]any{"sort_order": item.SortOrder, "is_primary": item.IsPrimary && !item.Excluded, "excluded": item.Excluded})
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected != 1 {
				return ErrNotFound
			}
		}
		return tx.Where("tenant_id=? AND listing_draft_id=?", tenantID, id).Order("sort_order").Find(&returnAssets).Error
	})
	for i := range returnAssets {
		returnAssets[i].Cached = returnAssets[i].CachePath != "" && pathWithin(listingAssetRoot(tenantID, id), returnAssets[i].CachePath)
	}
	return returnAssets, err
}

func (s *Service) BulkGeneratePublishPackages(ctx context.Context, tenantID int64, actor *uuid.UUID, body BulkPublishPackageBody) map[string]any {
	completed := []uuid.UUID{}
	failed := map[string]string{}
	if len(body.ListingDraftIDs) > maxBulkContentItems {
		return map[string]any{"completed": completed, "failed": map[string]string{"request": "maximum 20 items"}}
	}
	for _, id := range body.ListingDraftIDs {
		if _, err := s.GeneratePublishPackage(ctx, tenantID, id, actor); err != nil {
			failed[id.String()] = err.Error()
		} else {
			completed = append(completed, id)
		}
	}
	return map[string]any{"completed": completed, "failed": failed}
}

func (s *Service) BulkGenerateListingContent(ctx context.Context, tenantID int64, actor *uuid.UUID, body BulkListingContentBody) map[string]any {
	completed := []uuid.UUID{}
	failed := map[string]string{}
	if len(body.CatalogProductIDs) > maxBulkContentItems {
		return map[string]any{"completed": completed, "failed": map[string]string{"request": "maximum 20 items"}}
	}
	for _, catalogID := range body.CatalogProductIDs {
		listing, _, err := s.CreateListingDraft(ctx, tenantID, catalogID, CreateListingDraftBody{Platform: body.Platform, PricingProfileID: body.PricingProfileID})
		if err == nil {
			_, err = s.GenerateListingContent(ctx, tenantID, listing.ID, actor, GenerateListingContentBody{GenerationMode: body.GenerationMode})
		}
		if err != nil {
			failed[catalogID.String()] = err.Error()
		} else {
			completed = append(completed, catalogID)
		}
	}
	return map[string]any{"completed": completed, "failed": failed}
}

// ValidateTrustedImageBytes applies cache limits after bytes were fetched through
// the Collector outbound policy. It intentionally does not fetch arbitrary URLs.
func ValidateTrustedImageBytes(contentType string, data []byte) error {
	_, _, err := detectImage(data, contentType)
	return err
}

var _ = pricingengine.Money(0)
