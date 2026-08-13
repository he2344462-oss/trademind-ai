import { deleteJSON, getWithParams, postJSON, putJSON } from './request';

export type Pagination = { page: number; pageSize: number; total: number; totalPages: number };
export type Paged<T> = { list: T[]; pagination: Pagination };

export type SourceProduct = {
  id: string; sourcePlatform: string; sourceProductId: string; sourceUrl: string;
  supplierName?: string; originalTitle: string; originalDescription?: string;
  originalImages?: string[]; sourcePrice?: number; freight?: number; skuData?: unknown[];
  collectedAt: string; status: string;
};
export type Candidate = {
  id: string; sourceProductId: string; status: string; potentialScore?: number;
  estimatedCost?: number; estimatedSalePrice?: number; estimatedProfit?: number;
  estimatedMargin?: number; confidenceScore?: number; recommendation?: string; analyzedAt?: string;
  analysisVersion?: number; sourceProduct?: SourceProduct; rejectionReason?: string;
};
export type PricingResult = { currency: string; baseCost: string; totalFixedCost: string; estimatedPlatformFee: string; estimatedPaymentFee: string; estimatedAfterSaleLoss: string; estimatedTotalCost: string; breakEvenPrice: string; minimumSalePrice: string; suggestedSalePrice: string; estimatedProfit: string; estimatedMarginBps: number; markupRateBps: number; profile: { code: string; platform: string; configured: boolean } };
export type ScoreDimension = { score?: number; weight: number; reliable: boolean; evidence: string[] };
export type ConfidenceBreakdown = { productDataBps: number; costDataBps: number; marketCoverageBps: number };
export type AnalysisResult = { analysis: { id: string; analysisVersion: number; overallScore: number; confidenceScore: number; recommendation: string; createdAt: string }; cost: PricingResult; skuProfit: { minimumSkuMarginBps?: number; maximumSkuMarginBps?: number; averageSkuMarginBps?: number; warnings: string[] }; score: { dimensions: Record<string, ScoreDimension>; overallScore: number; confidenceScore: number; recommendation: string; reasons: string[]; warnings: string[]; blockers: string[]; missingDimensions: string[] }; explanation: { conclusion: string; reasons: string[]; risks: string[]; pricingExplanation: string; nextStep: string; source: string } };
export type CandidateAnalysis = AnalysisResult['analysis'] & { costSnapshot: PricingResult; confidenceBreakdown?: ConfidenceBreakdown; scoreBreakdown: Record<string, ScoreDimension>; reasons: string[]; warnings: string[]; blockers: string[]; explanation: AnalysisResult['explanation'] };
export type AnalyzeCandidateInput = { analysisMode?: 'rules_only' | 'ai_explanation'; platform?: 'xianyu' | 'taobao'; packagingCost?: string; otherCost?: string; expectedReturnLoss?: string; afterSaleReserve?: string; discountBuffer?: string; couponBuffer?: string; targetProfit?: string; targetMarginBps?: number; minimumProfit?: string; minimumMarginBps?: number; salePrice?: string; pricingProfile?: { code?: string; platformFeeBps?: number; platformFeeFixed?: string; paymentFeeBps?: number; paymentFeeFixed?: string; returnReserveBps?: number; otherBps?: number; otherFixed?: string } };
export type CatalogProduct = {
  id: string; title: string; supplier?: string; purchaseCost?: number; freightCost?: number;
  suggestedSalePrice?: number; salePrice?: number; estimatedProfit?: number; estimatedMargin?: number;
  catalogStatus: string; images?: Array<{ publicUrl?: string; originUrl?: string }>;
  skus?: Array<{ id: string; skuCode: string; skuName: string }>;
};
export type ListingDraft = {
  id: string; catalogProductId: string; platform: 'xianyu' | 'taobao'; title: string;
  description?: string; images?: string[]; salePrice?: number; platformCategory?: string;
  platformSkuData?: unknown[]; publishStatus: string; estimatedProfit?: number; estimatedMargin?: number; pricingSnapshot?: PricingResult; createdAt: string;
};

const value = (input: unknown): string | number | undefined =>
  typeof input === 'string' || typeof input === 'number' ? input : undefined;
const listParams = (params: Record<string, unknown>): Record<string, string | number | undefined> => ({
  page: value(params.current), pageSize: value(params.pageSize), keyword: value(params.keyword), status: value(params.status),
  platform: value(params.platform), catalogProductId: value(params.catalogProductId), sortBy: value(params.sortBy), sortOrder: value(params.sortOrder),
});
export const fetchSourceProducts = (params: Record<string, unknown>) => getWithParams<Paged<SourceProduct>>('/api/v1/source-products', listParams(params));
export const createSourceProduct = (body: Record<string, unknown>) => postJSON<SourceProduct>('/api/v1/source-products', body);
export const importSourceProducts = (items: Record<string, unknown>[]) => postJSON<{ list: SourceProduct[]; count: number }>('/api/v1/source-products/import', { items });
export const addSourceToCandidates = (id: string) => postJSON<{ candidate: Candidate; created: boolean }>(`/api/v1/source-products/${id}/candidate`, {});
export const fetchCandidates = (params: Record<string, unknown>) => getWithParams<Paged<Candidate>>('/api/v1/candidates', listParams(params));
export const approveCandidate = (id: string) => postJSON<{ catalogProduct: CatalogProduct; created: boolean }>(`/api/v1/candidates/${id}/approve`, {});
export const watchCandidate = (id: string) => postJSON<Candidate>(`/api/v1/candidates/${id}/watch`, {});
export const rejectCandidate = (id: string, rejectionReason = '') => postJSON<Candidate>(`/api/v1/candidates/${id}/reject`, { rejectionReason });
export const analyzeCandidate = (id: string, body: AnalyzeCandidateInput = {}) => postJSON<AnalysisResult>(`/api/v1/candidates/${id}/analyze`, body);
export const fetchCandidateAnalysis = (id: string) => getWithParams<CandidateAnalysis>(`/api/v1/candidates/${id}/analysis`);
export const fetchCandidateAnalyses = (id: string) => getWithParams<{ list: CandidateAnalysis[] }>(`/api/v1/candidates/${id}/analyses`);
export type CandidateAnalysisBatch = { id: string; status: string; total: number; pending: number; processing: number; completed: number; failed: number; platform: string; analysisMode: string; topN: number; createdAt: string; completedAt?: string };
export const createCandidateAnalysisBatch = (body: Record<string, unknown>) => postJSON<{ batchJobId: string; batch: CandidateAnalysisBatch }>('/api/v1/candidates/analyze-batch', body);
export const fetchCandidateAnalysisBatch = (id: string) => getWithParams<CandidateAnalysisBatch>(`/api/v1/candidate-analysis-batches/${id}`);
export type RecommendationRow = { id: string; rank: number; rankingScore: number; candidate: Candidate; analysis: CandidateAnalysis };
export const fetchTopRecommendations = (id: string, limit = 20) => getWithParams<{ list: RecommendationRow[] }>(`/api/v1/candidate-analysis-batches/${id}/top`, { limit });
export const bulkCandidateAction = (action: 'watch' | 'reject' | 'approve', candidateIds: string[], source = 'manual') => postJSON<{ completed: string[]; failed: Record<string, string> }>(`/api/v1/candidates/bulk/${action}`, { candidateIds, source, reason: action === 'reject' ? '批量淘汰' : '' });
export type PricingProfile = { id: string; name: string; platform: string; currency: string; platformFeeBps: number; platformFeeFixed: number; paymentFeeBps: number; paymentFeeFixed: number; returnReserveBps: number; otherBps: number; otherFixed: number; isDefault: boolean; enabled: boolean };
export const fetchPricingProfiles = (platform?: string) => getWithParams<{ list: PricingProfile[] }>('/api/v1/pricing-profiles', { platform });
export const createPricingProfile = (body: Record<string, unknown>) => postJSON<PricingProfile>('/api/v1/pricing-profiles', body);
export const updatePricingProfile = (id: string, body: Record<string, unknown>) => putJSON<PricingProfile, Record<string, unknown>>(`/api/v1/pricing-profiles/${id}`, body);
export const deletePricingProfile = (id: string) => deleteJSON<{ deleted: boolean }>(`/api/v1/pricing-profiles/${id}`);
export const fetchCatalogProducts = (params: Record<string, unknown>) => getWithParams<Paged<CatalogProduct>>('/api/v1/catalog-products', listParams(params));
export const createListingDraft = (catalogId: string, platform: 'xianyu' | 'taobao', pricingProfile?: AnalyzeCandidateInput['pricingProfile']) => postJSON<{ listingDraft: ListingDraft; created: boolean }>(`/api/v1/catalog-products/${catalogId}/listing-drafts`, { platform, pricingProfile });
export const fetchListingDrafts = (params: Record<string, unknown>) => getWithParams<Paged<ListingDraft>>('/api/v1/listing-drafts', listParams(params));
export const updateListingDraft = (id: string, body: Partial<ListingDraft>) => putJSON<ListingDraft, Partial<ListingDraft>>(`/api/v1/listing-drafts/${id}`, body);
export const deleteListingDraft = (id: string) => deleteJSON<{ deleted: boolean }>(`/api/v1/listing-drafts/${id}`);
export const recalculateListingDraft = (id: string, pricingProfileId?: string) => postJSON<ListingDraft>(`/api/v1/listing-drafts/${id}/recalculate`, { pricingProfileId });
export type CostCenterSummary = { productCount: number; averageMarginBps: number; lowProfitCount: number; highProfitCount: number; costAnomalyCount: number; products: Array<CatalogProduct & { coverUrl?: string }> };
export const fetchCostCenter = () => getWithParams<CostCenterSummary>('/api/v1/cost-center');
export type SelectionDashboard = { todaySources: number; pendingCandidates: number; todayAnalyzed: number; strongRecommend: number; recommend: number; watch: number; reject: number; averageMarginBps: number; runningBatches: CandidateAnalysisBatch[] };
export const fetchSelectionDashboard = () => getWithParams<SelectionDashboard>('/api/v1/selection-dashboard');
