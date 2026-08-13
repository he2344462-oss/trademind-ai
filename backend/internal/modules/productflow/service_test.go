package productflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SourceProduct{}, &Candidate{}, &CandidateAnalysis{}, &ListingDraft{}, &product.Product{}, &product.ProductImage{}, &product.ProductSKU{}))
	return &Service{DB: db}
}

func TestCandidateAnalysisHistoryAndApprovalSnapshot(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("analysis-1"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	body := AnalyzeCandidateBody{PackagingCost: "0.50", OtherCost: "0.50", ExpectedReturnLoss: "1.50", SalePrice: "49.90"}
	first, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, body)
	require.NoError(t, err)
	second, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, body)
	require.NoError(t, err)
	require.Equal(t, 1, first.Analysis.AnalysisVersion)
	require.Equal(t, 2, second.Analysis.AnalysisVersion)
	history, err := svc.ListAnalyses(ctx, 0, candidate.ID)
	require.NoError(t, err)
	require.Len(t, history, 2)
	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	catalog := approved.Catalog.(product.Product)
	require.NotNil(t, catalog.PackagingCost)
	require.InDelta(t, 0.5, *catalog.PackagingCost, 0.001)
	require.NotNil(t, catalog.EstimatedProfit)
	_, err = svc.AnalyzeCandidate(ctx, 0, candidate.ID, body)
	require.ErrorIs(t, err, ErrInvalidTransition)
}

func TestAIExplanationFallsBackUnlessStructured(t *testing.T) {
	svc := newTestService(t)
	svc.AIExplain = func(context.Context, string) (string, error) { return "not-json", nil }
	source, err := svc.CreateSource(context.Background(), 0, testSourceBody("ai-fallback"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(context.Background(), 0, source.ID)
	require.NoError(t, err)
	out, err := svc.AnalyzeCandidate(context.Background(), 0, candidate.ID, AnalyzeCandidateBody{AnalysisMode: "ai_explanation", SalePrice: "49.90"})
	require.NoError(t, err)
	require.Equal(t, "rules_template", out.Explanation.Source)
	require.Equal(t, "fallback", out.Analysis.AIStatus)
}

func TestListingProfilesProduceDifferentPricing(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("listing-pricing"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	_, err = svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{SalePrice: "49.90"})
	require.NoError(t, err)
	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	catalog := approved.Catalog.(product.Product)
	xianyu, _, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu"})
	require.NoError(t, err)
	profile := &PricingProfileBody{Code: "taobao-test", PlatformFeeBPS: 500, PaymentFeeBPS: 100}
	taobao, _, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "taobao", PricingProfile: profile})
	require.NoError(t, err)
	require.NotEqual(t, *xianyu.SalePrice, *taobao.SalePrice)
	require.NotEqual(t, *xianyu.EstimatedProfit, *taobao.EstimatedProfit)
	changed := 55.0
	updated, err := svc.UpdateListingDraft(ctx, 0, taobao.ID, UpdateListingDraftBody{SalePrice: &changed})
	require.NoError(t, err)
	require.Equal(t, changed, *updated.SalePrice)
	require.NotEqual(t, *taobao.EstimatedProfit, *updated.EstimatedProfit)
}

func testSourceBody(id string) CreateSourceProductBody {
	price, freight := 30.0, 5.0
	return CreateSourceProductBody{
		SourcePlatform: "1688", SourceProductID: id, SourceURL: "https://detail.1688.com/offer/" + id + ".html",
		SupplierName: "测试供应商", OriginalTitle: "测试折叠收纳箱", OriginalDescription: "原始货源描述",
		OriginalImages: json.RawMessage(`["https://cbu01.alicdn.com/img/test.jpg"]`), SourcePrice: &price, Freight: &freight,
		SKUData: json.RawMessage(`[{"skuCode":"BLUE-L","skuName":"蓝色/L","price":30,"stock":50}]`),
	}
}

func TestProductFlowCompleteLoopAndIdempotency(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()

	source, err := svc.CreateSource(ctx, 0, testSourceBody("10001"))
	require.NoError(t, err)
	require.NotEmpty(t, source.ID)

	candidate, created, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	require.True(t, created)
	require.Equal(t, CandidateStatusPending, candidate.Status)

	sameCandidate, created, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, candidate.ID, sameCandidate.ID)

	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	require.True(t, approved.Created)
	catalog := approved.Catalog.(product.Product)
	require.Equal(t, candidate.ID, *catalog.CandidateID)
	require.Len(t, catalog.Images, 1)
	require.Len(t, catalog.SKUs, 1)

	again, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	require.False(t, again.Created)
	secondCatalog := again.Catalog.(product.Product)
	require.Equal(t, catalog.ID, secondCatalog.ID)
	var catalogCount int64
	require.NoError(t, svc.DB.Model(&product.Product{}).Where("candidate_id = ?", candidate.ID).Count(&catalogCount).Error)
	require.EqualValues(t, 1, catalogCount)

	xianyu, created, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu"})
	require.NoError(t, err)
	require.True(t, created)
	taobao, created, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "taobao"})
	require.NoError(t, err)
	require.True(t, created)
	require.NotEqual(t, xianyu.ID, taobao.ID)

	sameXianyu, created, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu"})
	require.NoError(t, err)
	require.False(t, created)
	require.Equal(t, xianyu.ID, sameXianyu.ID)

	updatedTitle := "闲鱼专用标题"
	ready := ListingStatusReady
	updated, err := svc.UpdateListingDraft(ctx, 0, xianyu.ID, UpdateListingDraftBody{Title: &updatedTitle, PublishStatus: &ready})
	require.NoError(t, err)
	require.Equal(t, updatedTitle, updated.Title)
	require.Equal(t, ListingStatusReady, updated.PublishStatus)

	invalid := ListingStatusPublished
	_, err = svc.UpdateListingDraft(ctx, 0, xianyu.ID, UpdateListingDraftBody{PublishStatus: &invalid})
	require.ErrorIs(t, err, ErrInvalidTransition)

	list, err := svc.ListListingDrafts(ctx, 0, ListQuery{CatalogID: &catalog.ID})
	require.NoError(t, err)
	require.EqualValues(t, 2, list.Total)
}

func TestCandidateRejectAndInvalidApproval(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("10002"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	rejected, err := svc.RejectCandidate(ctx, 0, candidate.ID, "利润不足")
	require.NoError(t, err)
	require.Equal(t, CandidateStatusRejected, rejected.Status)
	require.Equal(t, "利润不足", rejected.RejectionReason)
	_, err = svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.ErrorIs(t, err, ErrInvalidTransition)
	_, err = svc.WatchCandidate(ctx, 0, candidate.ID)
	require.ErrorIs(t, err, ErrInvalidTransition)
}

func TestListingDraftDeleteOnlyBeforePublication(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("10003"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	catalog := approved.Catalog.(product.Product)
	draft, _, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "taobao"})
	require.NoError(t, err)
	require.NoError(t, svc.DeleteListingDraft(ctx, 0, draft.ID))
	_, err = svc.GetListingDraft(ctx, 0, draft.ID)
	require.ErrorIs(t, err, ErrNotFound)
}
