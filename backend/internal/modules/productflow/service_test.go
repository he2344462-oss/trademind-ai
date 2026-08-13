package productflow

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/trademind-ai/trademind/backend/internal/modules/product"
	"gorm.io/gorm"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&SourceProduct{}, &Candidate{}, &CandidateAnalysis{}, &ListingDraft{}, &PricingProfile{}, &PricingProfileRevision{}, &MarketSignalSnapshot{}, &MarketSignalProviderConfig{}, &CandidateAnalysisBatch{}, &CandidateAnalysisBatchItem{}, &ListingPerformanceSnapshot{}, &SelectionOutcomeEvaluation{}, &SelectionConfig{}, &product.Product{}, &product.ProductImage{}, &product.ProductSKU{}))
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

func TestPricingProfileSnapshotAndManualRecalculation(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	enabled := true
	profile, err := svc.CreatePricingProfile(ctx, 0, PricingProfileInput{Name: "闲鱼测试费用", Platform: "xianyu", Currency: "CNY", PlatformFeeBPS: 200, PlatformFeeFixed: "0.50", Enabled: &enabled, IsDefault: true})
	require.NoError(t, err)
	source, err := svc.CreateSource(ctx, 0, testSourceBody("profile-history"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	_, err = svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{PricingProfileID: &profile.ID})
	require.NoError(t, err)
	approved, err := svc.ApproveCandidate(ctx, 0, candidate.ID)
	require.NoError(t, err)
	catalog := approved.Catalog.(product.Product)
	draft, _, err := svc.CreateListingDraft(ctx, 0, catalog.ID, CreateListingDraftBody{Platform: "xianyu", PricingProfileID: &profile.ID})
	require.NoError(t, err)
	original := append([]byte(nil), draft.PricingSnapshot...)
	profile.PlatformFeeBPS = 900
	require.NoError(t, svc.DB.Save(profile).Error)
	unchanged, err := svc.GetListingDraft(ctx, 0, draft.ID)
	require.NoError(t, err)
	require.JSONEq(t, string(original), string(unchanged.PricingSnapshot))
	recalculated, err := svc.RecalculateListingDraft(ctx, 0, draft.ID, RecalculateListingBody{PricingProfileID: &profile.ID})
	require.NoError(t, err)
	require.Equal(t, 2, recalculated.PricingVersion)
	require.NotEqual(t, string(original), string(recalculated.PricingSnapshot))
}

func TestMarketSignalFreshnessFeedsDemandWithoutPretendingLiveData(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	source, err := svc.CreateSource(ctx, 0, testSourceBody("market-signal"))
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	_, err = svc.CreateMarketSignal(ctx, 0, candidate.ID, CreateMarketSignalBody{Platform: "xianyu", SignalType: SignalDemandScore, Value: 80, Source: "人工调研表", Origin: SignalOriginManual, ConfidenceBPS: 7000})
	require.NoError(t, err)
	out, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{})
	require.NoError(t, err)
	require.NotNil(t, out.Score.Dimensions["demand"].Score)
	require.Contains(t, out.Score.MissingDimensions, "competition")
	require.NotEmpty(t, out.Analysis.MarketSignalSnapshot)
}

func TestRankingEngineAndBlockerBulkApproval(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	sourceBody := testSourceBody("blocked-approval")
	sourceBody.SourcePrice = nil
	source, err := svc.CreateSource(ctx, 0, sourceBody)
	require.NoError(t, err)
	candidate, _, err := svc.CreateCandidate(ctx, 0, source.ID)
	require.NoError(t, err)
	out, err := svc.AnalyzeCandidate(ctx, 0, candidate.ID, AnalyzeCandidateBody{})
	require.NoError(t, err)
	require.NotEmpty(t, out.Score.Blockers)
	result, err := svc.BulkCandidateAction(ctx, 0, nil, "approve", BulkCandidateActionBody{CandidateIDs: []uuid.UUID{candidate.ID}, Source: "batch_recommendation"})
	require.NoError(t, err)
	require.Empty(t, result.Completed)
	require.Contains(t, result.Failed, candidate.ID.String())
}

func TestBatchNonRetryableFailureIsIsolatedWithoutPointlessRetry(t *testing.T) {
	svc := newTestService(t)
	ctx := context.Background()
	batch := CandidateAnalysisBatch{TenantID: 0, Status: BatchStatusPending, Total: 1, Pending: 1, AnalysisMode: "rules_only", Platform: "xianyu", TopN: 20, ExcludeBlocked: true}
	require.NoError(t, svc.DB.Create(&batch).Error)
	missing := uuid.New()
	item := CandidateAnalysisBatchItem{TenantID: 0, BatchID: batch.ID, CandidateID: missing, Status: BatchItemStatusPending, MaxAttempts: 2}
	require.NoError(t, svc.DB.Create(&item).Error)
	svc.RunAnalysisBatch(ctx, batch.ID, "test-worker")
	detail, err := svc.GetAnalysisBatch(ctx, 0, batch.ID, true)
	require.NoError(t, err)
	require.Equal(t, BatchStatusFailed, detail.Status)
	require.Equal(t, 1, detail.Failed)
	require.Equal(t, 1, detail.Items[0].Attempts)
	require.Equal(t, BatchItemStatusFailed, detail.Items[0].Status)
	require.Equal(t, BatchErrorNonRetryable, detail.Items[0].ErrorType)
}

func TestRecoverStaleAnalysisItem(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC().Add(-20 * time.Minute)
	batch := CandidateAnalysisBatch{TenantID: 0, Status: BatchStatusRunning, Total: 1, Processing: 1, AnalysisMode: "rules_only", Platform: "xianyu", TopN: 20}
	require.NoError(t, svc.DB.Create(&batch).Error)
	item := CandidateAnalysisBatchItem{TenantID: 0, BatchID: batch.ID, CandidateID: uuid.New(), Status: BatchItemStatusProcessing, Attempts: 1, MaxAttempts: 3, StartedAt: &now}
	require.NoError(t, svc.DB.Create(&item).Error)
	require.NoError(t, svc.RecoverStaleAnalysisItems(context.Background(), time.Now().UTC().Add(-10*time.Minute)))
	detail, err := svc.GetAnalysisBatch(context.Background(), 0, batch.ID, true)
	require.NoError(t, err)
	require.Equal(t, 1, detail.Pending)
	require.Zero(t, detail.Processing)
	require.Equal(t, BatchItemStatusPending, detail.Items[0].Status)
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
