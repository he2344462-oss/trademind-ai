package productflow

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
)

func sprint6Service(t *testing.T) (*Service, *ListingDraft) {
	t.Helper()
	svc := newTestService(t)
	db := svc.DB
	if err := db.AutoMigrate(&ListingContentVersion{}, &ListingAsset{}, &ListingPublishPackage{}, &ManualPublishRecord{}); err != nil {
		t.Fatal(err)
	}
	price := 39.9
	cost := 12.8
	profit := 20.0
	margin := 0.5
	candidateID := uuid.New()
	p := product.Product{TenantID: 0, CandidateID: &candidateID, Source: "1688", SourceURL: "https://detail.1688.com/offer/1.html", Title: "折叠收纳盒", Description: "用于桌面物品分类收纳", Category: "家居", Supplier: "测试供应商", PurchaseCost: &cost, SalePrice: &price, Images: []product.ProductImage{{OriginURL: "https://cbu01.alicdn.com/img/a.jpg", SortOrder: 0}}, SKUs: []product.ProductSKU{{SKUCode: "W-S", SKUName: "白色 / 小号"}}}
	if err := db.Create(&p).Error; err != nil {
		t.Fatal(err)
	}
	pricing := jsonData(map[string]any{"estimatedProfit": "20.00"})
	images := jsonData([]string{"https://cbu01.alicdn.com/img/a.jpg"})
	skus := jsonData([]map[string]string{{"code": "W-S", "displayName": "白色 / 小号"}})
	l := ListingDraft{TenantID: 0, CatalogProductID: p.ID, Platform: "xianyu", Title: p.Title, Description: p.Description, Images: images, SalePrice: &price, EstimatedProfit: &profit, EstimatedMargin: &margin, PricingSnapshot: pricing, PricingVersion: 1, PlatformSKUData: skus, PublishStatus: ListingStatusDraft}
	if err := db.Create(&l).Error; err != nil {
		t.Fatal(err)
	}
	return svc, &l
}

func uploadTestImage(t *testing.T, svc *Service, listingID uuid.UUID) {
	t.Helper()
	pngBytes, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if _, err := svc.UploadListingAsset(context.Background(), 0, listingID, "main.png", "image/png", bytes.NewReader(pngBytes)); err != nil {
		t.Fatal(err)
	}
}

func TestListingContentVersionsReviewReadyPackageAndManualPublish(t *testing.T) {
	svc, l := sprint6Service(t)
	ctx := context.Background()
	v1, err := svc.GenerateListingContent(ctx, 0, l.ID, nil, GenerateListingContentBody{GenerationMode: "template_only"})
	if err != nil {
		t.Fatal(err)
	}
	v2, err := svc.GenerateListingContent(ctx, 0, l.ID, nil, GenerateListingContentBody{GenerationMode: "ai_generate"})
	if err != nil {
		t.Fatal(err)
	}
	if v1.Version != 1 || v2.Version != 2 || v2.AIStatus != "fallback_template" {
		t.Fatalf("unexpected versions/fallback %#v %#v", v1, v2)
	}
	if _, err = svc.ReviewListingContent(ctx, 0, l.ID, ReviewListingContentBody{Action: "approve"}); err != nil {
		t.Fatal(err)
	}
	uploadTestImage(t, svc, l.ID)
	if _, err = svc.MarkListingReady(ctx, 0, l.ID); err != nil {
		t.Fatal(err)
	}
	pkg, err := svc.GeneratePublishPackage(ctx, 0, l.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(pkg.ArchivePath)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, f := range zr.File {
		names[f.Name] = true
		if strings.HasPrefix(f.Name, "public/") {
			r, _ := f.Open()
			b, _ := io.ReadAll(r)
			_ = r.Close()
			if strings.Contains(string(b), "测试供应商") || strings.Contains(string(b), "detail.1688.com") {
				t.Fatalf("internal data leaked in %s", f.Name)
			}
		}
	}
	if !names["public/title.txt"] || !names["internal/source.json"] || !names["public/images/01.png"] {
		t.Fatalf("bad package files %#v", names)
	}
	record, err := svc.MarkPublishedManual(ctx, 0, l.ID, nil, ManualPublishBody{PlatformListingID: "XY-DEMO-1", ListingURL: "https://www.goofish.com/item?id=demo", PublishedAt: ptrTime(time.Now())})
	if err != nil {
		t.Fatal(err)
	}
	if record.PublishMethod != "manual" {
		t.Fatal("wrong publish method")
	}
}

func ptrTime(v time.Time) *time.Time { return &v }
func TestReadyBlockedByNegativeProfitAndMissingDescription(t *testing.T) {
	svc, l := sprint6Service(t)
	v, err := svc.GenerateListingContent(context.Background(), 0, l.ID, nil, GenerateListingContentBody{})
	if err != nil {
		t.Fatal(err)
	}
	svc.DB.Model(v).Updates(map[string]any{"review_status": "approved", "description": ""})
	svc.DB.Model(l).Updates(map[string]any{"current_content_version_id": v.ID, "estimated_profit": -1, "publish_status": ListingStatusApproved})
	if _, err = svc.MarkListingReady(context.Background(), 0, l.ID); err == nil {
		t.Fatal("negative profit/missing description must block")
	}
}
func TestTrustedImageValidation(t *testing.T) {
	pngBytes, _ := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if ValidateTrustedImageBytes("image/png", pngBytes) != nil {
		t.Fatal("valid image rejected")
	}
	if ValidateTrustedImageBytes("text/html", []byte("x")) == nil {
		t.Fatal("non-image accepted")
	}
	if ValidateTrustedImageBytes("image/jpeg", make([]byte, 10*1024*1024+1)) == nil {
		t.Fatal("oversize accepted")
	}
}

func TestPackagePathFilenameAndDuplicateImageSecurity(t *testing.T) {
	svc, listing := sprint6Service(t)
	ctx := context.Background()
	content, err := svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{})
	if err != nil {
		t.Fatal(err)
	}
	longHostileTitle := strings.Repeat("../CON:<bad>|", 100)
	if _, err = svc.UpdateListingContent(ctx, 0, listing.ID, nil, UpdateListingContentBody{Title: longHostileTitle, Description: content.Description}); err != nil {
		t.Fatal(err)
	}
	if _, err = svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{}); err != nil {
		t.Fatal(err)
	}
	var assets int64
	svc.DB.Model(&ListingAsset{}).Where("listing_draft_id=?", listing.ID).Count(&assets)
	if assets != 1 {
		t.Fatalf("duplicate source URL created %d assets", assets)
	}
	var row ListingPublishPackage
	row = ListingPublishPackage{TenantID: 0, ListingDraftID: listing.ID, ContentVersionID: content.ID, PackageVersion: 99, Manifest: jsonData(map[string]any{}), ArchivePath: "C:\\Windows\\outside.zip", GeneratedAt: time.Now()}
	if err = svc.DB.Create(&row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = svc.PublishPackageFile(ctx, 0, row.ID); err == nil {
		t.Fatal("path traversal package was accepted")
	}
}

func TestAIUnavailableTimeoutInvalidAndFabricatedContent(t *testing.T) {
	cases := []struct {
		name        string
		fn          func(context.Context, string) (AIContentResult, error)
		wantStatus  string
		wantBlocker bool
	}{
		{"unavailable", func(context.Context, string) (AIContentResult, error) {
			return AIContentResult{}, errors.New("not configured")
		}, "fallback_template", false},
		{"timeout", func(context.Context, string) (AIContentResult, error) {
			return AIContentResult{}, context.DeadlineExceeded
		}, "fallback_template", false},
		{"invalid json", func(context.Context, string) (AIContentResult, error) {
			return AIContentResult{Content: "not-json", Provider: "fake", Model: "fake"}, nil
		}, "fallback_template", false},
		{"fabricated fact", func(context.Context, string) (AIContentResult, error) {
			return AIContentResult{Content: `{"title":"官方正品收纳盒","description":"航空铝合金，国家认证，当天发货","sellingPoints":["绝对保证"],"keywords":[]}`, Provider: "fake", Model: "fake"}, nil
		}, "succeeded", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, l := sprint6Service(t)
			svc.AIContentGenerate = tc.fn
			row, err := svc.GenerateListingContent(context.Background(), 0, l.ID, nil, GenerateListingContentBody{GenerationMode: "ai_generate"})
			if err != nil {
				t.Fatal(err)
			}
			if row.AIStatus != tc.wantStatus {
				t.Fatalf("status=%s", row.AIStatus)
			}
			var blockers []string
			_ = json.Unmarshal(row.Blockers, &blockers)
			if tc.wantBlocker && len(blockers) == 0 {
				t.Fatal("fabricated facts were not blocked")
			}
		})
	}
}

func TestFiveCatalogProductContentDemo(t *testing.T) {
	cases := []struct {
		name          string
		skus          int
		description   string
		title         string
		profit        float64
		expectWarning bool
		canReady      bool
	}{
		{"A高利润普通商品", 1, "完整收纳说明", "桌面收纳盒", 20, false, true},
		{"B多SKU商品", 3, "多规格收纳说明", "多规格收纳篮", 16, false, true},
		{"C资料不完整", 1, "", "待补充参数商品", 12, false, true},
		{"D品牌风险", 1, "真实标题包含 Apple 兼容词", "Apple兼容支架", 15, true, true},
		{"E负利润", 1, "完整说明", "低利润商品", -1, false, false},
	}
	packageCount := 0
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, l := sprint6Service(t)
			var p product.Product
			if err := svc.DB.Preload("SKUs").First(&p, l.CatalogProductID).Error; err != nil {
				t.Fatal(err)
			}
			p.Title = tc.title
			p.Description = tc.description
			svc.DB.Model(&p).Updates(map[string]any{"title": p.Title, "description": p.Description})
			for i := 1; i < tc.skus; i++ {
				svc.DB.Create(&product.ProductSKU{ProductID: p.ID, SKUCode: fmt.Sprintf("SKU-%d", i), SKUName: fmt.Sprintf("规格 %d", i)})
			}
			svc.DB.Model(l).Update("estimated_profit", tc.profit)
			v, err := svc.GenerateListingContent(context.Background(), 0, l.ID, nil, GenerateListingContentBody{GenerationMode: "template_only"})
			if err != nil {
				t.Fatal(err)
			}
			if tc.expectWarning && len(v.Warnings) == 0 {
				t.Fatal("expected brand warning")
			}
			_, err = svc.ReviewListingContent(context.Background(), 0, l.ID, ReviewListingContentBody{Action: "approve"})
			if err != nil {
				t.Fatal(err)
			}
			uploadTestImage(t, svc, l.ID)
			_, err = svc.MarkListingReady(context.Background(), 0, l.ID)
			if !tc.canReady {
				if err == nil {
					t.Fatal("negative profit entered ready")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.GeneratePublishPackage(context.Background(), 0, l.ID, nil); err != nil {
				t.Fatal(err)
			}
			packageCount++
		})
	}
	if packageCount != 4 {
		t.Fatalf("expected four eligible packages, got %d", packageCount)
	}
}

func TestSameCatalogHasIndependentXianyuAndTaobaoPackages(t *testing.T) {
	svc, xianyu := sprint6Service(t)
	taobao := *xianyu
	taobao.ID = uuid.Nil
	taobao.Platform = "taobao"
	taobao.PublishStatus = ListingStatusDraft
	if err := svc.DB.Create(&taobao).Error; err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, listing := range []*ListingDraft{xianyu, &taobao} {
		content, err := svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{GenerationMode: "template_only"})
		if err != nil {
			t.Fatal(err)
		}
		if content.Platform != listing.Platform || content.PromptVersion != listing.Platform+"-v1" {
			t.Fatalf("wrong profile %#v", content)
		}
		if _, err = svc.ReviewListingContent(ctx, 0, listing.ID, ReviewListingContentBody{Action: "approve"}); err != nil {
			t.Fatal(err)
		}
		uploadTestImage(t, svc, listing.ID)
		if _, err = svc.MarkListingReady(ctx, 0, listing.ID); err != nil {
			t.Fatal(err)
		}
		if _, err = svc.GeneratePublishPackage(ctx, 0, listing.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	var packages int64
	svc.DB.Model(&ListingPublishPackage{}).Count(&packages)
	if packages != 2 {
		t.Fatalf("expected independent platform packages, got %d", packages)
	}
}

func TestContentRestoreAndAssetSelectionCreateAuditableVersions(t *testing.T) {
	svc, listing := sprint6Service(t)
	ctx := context.Background()
	v1, err := svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = svc.UpdateListingContent(ctx, 0, listing.ID, nil, UpdateListingContentBody{Title: "人工版本", Description: "人工编辑描述"}); err != nil {
		t.Fatal(err)
	}
	restored, err := svc.RestoreListingContent(ctx, 0, listing.ID, nil, v1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if restored.Version != 3 || restored.Title != v1.Title {
		t.Fatalf("restore must create v3, got %#v", restored)
	}
	var assets []ListingAsset
	svc.DB.Where("listing_draft_id=?", listing.ID).Find(&assets)
	if len(assets) != 1 {
		t.Fatalf("expected one source asset, got %d", len(assets))
	}
	out, err := svc.UpdateListingAssets(ctx, 0, listing.ID, UpdateListingAssetsBody{Assets: []ListingAssetUpdate{{ID: assets[0].ID, SortOrder: 2, IsPrimary: true}}})
	if err != nil || len(out) != 1 || !out[0].IsPrimary || out[0].SortOrder != 2 {
		t.Fatalf("asset update failed %#v %v", out, err)
	}
}
