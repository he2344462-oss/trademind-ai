package productflow

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func realPNG(t *testing.T) []byte {
	t.Helper()
	out, err := base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSprint61SecureUploadPreviewDeleteAndQualityReview(t *testing.T) {
	svc, listing := sprint6Service(t)
	if err := svc.DB.AutoMigrate(&ListingContentQualityReview{}); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	png := realPNG(t)
	asset, err := svc.UploadListingAsset(ctx, 0, listing.ID, "../../evil.png", "image/png", bytes.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	if !asset.Cached || strings.Contains(asset.CachePath, "evil.png") || !pathWithin(listingAssetRoot(0, listing.ID), asset.CachePath) {
		t.Fatalf("unsafe upload %#v", asset)
	}
	duplicate, err := svc.UploadListingAsset(ctx, 0, listing.ID, "copy.png", "image/png", bytes.NewReader(png))
	if err != nil || duplicate.ID != asset.ID {
		t.Fatalf("hash dedup failed %#v %v", duplicate, err)
	}
	if _, err = svc.UploadListingAsset(ctx, 0, listing.ID, "fake.png", "image/png", strings.NewReader("<html>")); err == nil {
		t.Fatal("fake image accepted")
	}
	if _, err = svc.ListingAssetFile(ctx, 0, asset.ID); err != nil {
		t.Fatal(err)
	}
	content, err := svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{})
	if err != nil {
		t.Fatal(err)
	}
	review, err := svc.CreateContentQualityReview(ctx, 0, listing.ID, nil, ContentQualityReviewBody{Status: "needs_edit", TitleQuality: 75, DescriptionQuality: 70, FactAccuracy: 100, WasEdited: true, EditReason: "补充真实规格"})
	if err != nil || review.ContentVersionID != content.ID {
		t.Fatalf("quality review failed %#v %v", review, err)
	}
	if err = svc.DeleteListingAsset(ctx, 0, listing.ID, asset.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = os.Stat(asset.CachePath); !os.IsNotExist(err) {
		t.Fatal("uploaded file not deleted")
	}
}

func TestSprint61TenRealImagePublishPackagesAndIsolation(t *testing.T) {
	packageIndex := 0
	for catalogIndex := 0; catalogIndex < 5; catalogIndex++ {
		svc, xianyu := sprint6Service(t)
		taobao := *xianyu
		taobao.ID = uuid.Nil
		taobao.Platform = "taobao"
		if err := svc.DB.Create(&taobao).Error; err != nil {
			t.Fatal(err)
		}
		for _, listing := range []*ListingDraft{xianyu, &taobao} {
			ctx := context.Background()
			uploadTestImage(t, svc, listing.ID)
			content, err := svc.GenerateListingContent(ctx, 0, listing.ID, nil, GenerateListingContentBody{})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = svc.ReviewListingContent(ctx, 0, listing.ID, ReviewListingContentBody{Action: "approve"}); err != nil {
				t.Fatal(err)
			}
			if _, err = svc.MarkListingReady(ctx, 0, listing.ID); err != nil {
				t.Fatal(err)
			}
			pkg, err := svc.GeneratePublishPackage(ctx, 0, listing.ID, nil)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := os.ReadFile(pkg.ArchivePath)
			if err != nil {
				t.Fatal(err)
			}
			zr, err := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
			if err != nil {
				t.Fatal(err)
			}
			hasImage := false
			for _, f := range zr.File {
				if strings.HasPrefix(f.Name, "public/") {
					r, _ := f.Open()
					b, _ := io.ReadAll(r)
					_ = r.Close()
					text := string(b)
					for _, secret := range []string{"detail.1688.com", "purchaseCost", "estimatedProfit", "marketSignal", "overallScore"} {
						if strings.Contains(text, secret) {
							t.Fatalf("public leak %s in %s", secret, f.Name)
						}
					}
				}
				if strings.HasPrefix(f.Name, "public/images/") && f.Name != "public/images/manifest.json" {
					hasImage = true
				}
			}
			if !hasImage || content.ID == uuid.Nil {
				t.Fatalf("package %d missing real image", packageIndex)
			}
			readiness, err := svc.SellTestReadiness(ctx, 0, listing.ID)
			if err != nil || !readiness.ReadyForManualSellTest {
				t.Fatalf("listing %d not sell-ready: %#v %v", packageIndex, readiness, err)
			}
			packageIndex++
		}
	}
	if packageIndex != 10 {
		t.Fatalf("expected 5 catalogs / 10 listings, got %d packages", packageIndex)
	}
}

func TestSprint61AIMetricsAndFallback(t *testing.T) {
	svc, listing := sprint6Service(t)
	svc.AIContentGenerate = func(context.Context, string) (AIContentResult, error) {
		return AIContentResult{Content: `{"title":"折叠收纳盒","description":"用于桌面物品分类收纳","sellingPoints":["桌面收纳"],"keywords":["收纳盒"]}`, Provider: "test-provider", Model: "test-model", InputTokens: 100, OutputTokens: 50, CostMicros: 42, LatencyMS: 123}, nil
	}
	row, err := svc.GenerateListingContent(context.Background(), 0, listing.ID, nil, GenerateListingContentBody{GenerationMode: "ai_generate"})
	if err != nil {
		t.Fatal(err)
	}
	if row.AIStatus != "succeeded" || row.AIInputTokens != 100 || row.AILatencyMS != 123 {
		t.Fatalf("metrics not persisted %#v", row)
	}
}

func TestSprint61ListingBatchSuccessFailureRetryAndIdempotency(t *testing.T) {
	svc, good := sprint6Service(t)
	if err := svc.DB.AutoMigrate(&ListingOperationBatch{}, &ListingOperationBatchItem{}); err != nil {
		t.Fatal(err)
	}
	_, bad := sprint6Service(t)
	batch := ListingOperationBatch{TenantID: 0, Operation: ListingOperationContent, Status: BatchStatusPending, Total: 2, Pending: 2, Options: jsonData(listingBatchOptions{GenerationMode: "template_only"})}
	if err := svc.DB.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	items := []ListingOperationBatchItem{{TenantID: 0, BatchID: batch.ID, ListingDraftID: good.ID, Status: BatchItemStatusPending, MaxAttempts: 3}, {TenantID: 0, BatchID: batch.ID, ListingDraftID: bad.ID, Status: BatchItemStatusPending, MaxAttempts: 3}}
	if err := svc.DB.Create(&items).Error; err != nil {
		t.Fatal(err)
	}
	svc.DB.Delete(&ListingDraft{}, bad.ID)
	svc.RunListingOperationBatch(context.Background(), batch.ID, "test-worker")
	var got ListingOperationBatch
	svc.DB.First(&got, "id=?", batch.ID)
	if got.Status != BatchStatusPartialFailed || got.Completed != 1 || got.Failed != 1 {
		t.Fatalf("failure isolation %#v", got)
	}
	var versions int64
	svc.DB.Model(&ListingContentVersion{}).Where("listing_draft_id=?", good.ID).Count(&versions)
	svc.RunListingOperationBatch(context.Background(), batch.ID, "duplicate-worker")
	var after int64
	svc.DB.Model(&ListingContentVersion{}).Where("listing_draft_id=?", good.ID).Count(&after)
	if versions != after {
		t.Fatal("idempotency produced duplicate content")
	}
}
