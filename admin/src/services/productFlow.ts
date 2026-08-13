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
export type ConfidenceBreakdown = { productDataBps: number; costDataBps: number; marketCoverageBps: number; marketSignalConfidenceBps?: number; overallAnalysisConfidenceBps?: number };
export type AnalysisResult = { analysis: { id: string; analysisVersion: number; overallScore: number; confidenceScore: number; recommendation: string; createdAt: string; confidenceBreakdown?: ConfidenceBreakdown; marketSignalSnapshot?: MarketSignal[] }; cost: PricingResult; skuProfit: { minimumSkuMarginBps?: number; maximumSkuMarginBps?: number; averageSkuMarginBps?: number; warnings: string[] }; score: { dimensions: Record<string, ScoreDimension>; overallScore: number; confidenceScore: number; recommendation: string; reasons: string[]; warnings: string[]; blockers: string[]; missingDimensions: string[] }; explanation: { conclusion: string; reasons: string[]; risks: string[]; pricingExplanation: string; nextStep: string; source: string } };
export type CandidateAnalysis = AnalysisResult['analysis'] & { costSnapshot: PricingResult; confidenceBreakdown?: ConfidenceBreakdown; marketSignalSnapshot?: MarketSignal[]; scoreBreakdown: Record<string, ScoreDimension>; reasons: string[]; warnings: string[]; blockers: string[]; explanation: AnalysisResult['explanation'] };
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
export type CandidateAnalysisBatch = { id: string; status: string; total: number; pending: number; processing: number; completed: number; failed: number; platform: string; analysisMode: string; topN: number; createdAt: string; startedAt?: string; completedAt?: string; controlVersion: number };
export const createCandidateAnalysisBatch = (body: Record<string, unknown>) => postJSON<{ batchJobId: string; batch: CandidateAnalysisBatch }>('/api/v1/candidates/analyze-batch', body);
export const fetchCandidateAnalysisBatch = (id: string) => getWithParams<CandidateAnalysisBatch>(`/api/v1/candidate-analysis-batches/${id}`);
export type BatchItem = { id: string; candidateId: string; status: string; attempts: number; maxAttempts: number; errorType?: string; errorMessage?: string; rankingScore: number; rank?: number; candidate?: Candidate; analysis?: CandidateAnalysis };
export const fetchCandidateAnalysisBatches = (params: Record<string, unknown>) => getWithParams<Paged<CandidateAnalysisBatch>>('/api/v1/candidate-analysis-batches', listParams(params));
export const fetchCandidateAnalysisBatchItems = (id: string, params: Record<string, unknown>) => getWithParams<Paged<BatchItem>>(`/api/v1/candidate-analysis-batches/${id}/items`, listParams(params));
export const controlCandidateAnalysisBatch = (id: string, action: 'pause' | 'resume' | 'cancel' | 'retry-failed') => postJSON<{ batch: CandidateAnalysisBatch; affectedItems: number }>(`/api/v1/candidate-analysis-batches/${id}/${action}`, {});
export type RecommendationRow = { id: string; rank: number; rankingScore: number; candidate: Candidate; analysis: CandidateAnalysis };
export const fetchTopRecommendations = (id: string, limit = 20) => getWithParams<{ list: RecommendationRow[] }>(`/api/v1/candidate-analysis-batches/${id}/top`, { limit });
export const bulkCandidateAction = (action: 'watch' | 'reject' | 'approve', candidateIds: string[], source = 'manual') => postJSON<{ completed: string[]; failed: Record<string, string> }>(`/api/v1/candidates/bulk/${action}`, { candidateIds, source, reason: action === 'reject' ? '批量淘汰' : '' });
export type PricingProfile = { id: string; name: string; platform: string; currency: string; platformFeeBps: number; platformFeeFixed: number; paymentFeeBps: number; paymentFeeFixed: number; returnReserveBps: number; otherBps: number; otherFixed: number; isDefault: boolean; enabled: boolean; version: number };
export const fetchPricingProfiles = (platform?: string) => getWithParams<{ list: PricingProfile[] }>('/api/v1/pricing-profiles', { platform });
export const createPricingProfile = (body: Record<string, unknown>) => postJSON<PricingProfile>('/api/v1/pricing-profiles', body);
export const updatePricingProfile = (id: string, body: Record<string, unknown>) => putJSON<PricingProfile, Record<string, unknown>>(`/api/v1/pricing-profiles/${id}`, body);
export const deletePricingProfile = (id: string) => deleteJSON<{ deleted: boolean }>(`/api/v1/pricing-profiles/${id}`);
export const copyPricingProfile = (id: string) => postJSON<PricingProfile>(`/api/v1/pricing-profiles/${id}/copy`, {});
export const setDefaultPricingProfile = (id: string) => postJSON<PricingProfile>(`/api/v1/pricing-profiles/${id}/default`, {});
export type MarketSignal = { id: string; candidateId: string; platform: string; signalType: string; value: number; unit: string; source: string; origin: string; confidenceBps: number; effectiveConfidenceBps: number; freshness: 'fresh' | 'stale' | 'expired'; participates: boolean; observedAt: string };
export type MarketSignalProviderStatus = { id: string; name: string; sourceType: string; supportedPlatforms: string[]; supportedSignals: string[]; configured: boolean; health: { status: string; message: string } };
export const fetchMarketSignals = (candidateId: string, platform?: string) => getWithParams<{ list: MarketSignal[] }>(`/api/v1/candidates/${candidateId}/market-signals`, { platform });
export const importMarketSignals = (body: Record<string, unknown>) => postJSON<{ imported: number; duplicates: number; failed: Record<string, string> }>('/api/v1/market-signals/import', body);
export const fetchMarketSignalProviders = () => getWithParams<{ list: MarketSignalProviderStatus[] }>('/api/v1/market-signal-providers/status');
export type MarketProviderConfig = { id: string; providerId: string; providerType: string; name: string; platform: string; sourceLevel: string; enabled: boolean; status: string; config: Record<string, unknown>; credentialConfigured: boolean; credentialHint?: string; lastHealthCheckAt?: string; lastSuccessAt?: string; lastError?: string; createdAt: string; updatedAt: string };
export const fetchMarketProviderConfigs = () => getWithParams<{ list: MarketProviderConfig[] }>('/api/v1/market-signal-provider-configs');
export const createMarketProviderConfig = (body: Record<string, unknown>) => postJSON<MarketProviderConfig>('/api/v1/market-signal-provider-configs', body);
export const updateMarketProviderConfig = (id: string, body: Record<string, unknown>) => putJSON<MarketProviderConfig, Record<string, unknown>>(`/api/v1/market-signal-provider-configs/${id}`, body);
export const controlMarketProvider = (id: string, action: 'enable' | 'disable') => postJSON<MarketProviderConfig>(`/api/v1/market-signal-provider-configs/${id}/${action}`, {});
export const healthCheckMarketProvider = (id: string) => postJSON<MarketProviderConfig>(`/api/v1/market-signal-provider-configs/${id}/health-check`, {});
export type CSVImportPreview = { headers: string[]; rows: Array<Record<string, string>>; totalRows: number; validRows: number; invalidRows: number; errors: Array<{ row: number; field?: string; message: string }> };
export type CSVImportBody = { csv: string; mapping: Record<string, string> };
export const previewMarketSignalCSV = (body: CSVImportBody) => postJSON<CSVImportPreview>('/api/v1/imports/market-signals/preview', body);
export const confirmMarketSignalCSV = (body: CSVImportBody) => postJSON<{ imported: number; duplicates: number; failed: Record<string, string> }>('/api/v1/imports/market-signals/confirm', body);
export const previewPerformanceCSV = (body: CSVImportBody) => postJSON<CSVImportPreview>('/api/v1/imports/performance/preview', body);
export const confirmPerformanceCSV = (body: CSVImportBody) => postJSON<{ imported: number; duplicates: number; failed: Record<string, string> }>('/api/v1/imports/performance/confirm', body);
export type PerformanceSummary = { snapshotCount: number; listingCount: number; views: number; inquiries: number; orders: number; refunds: number; grossRevenue: number; realizedProfit: number; actualMarginBps: number; latestObservedAt?: string };
export const fetchPerformanceSummary = () => getWithParams<PerformanceSummary>('/api/v1/performance/summary');
export const importPerformance = (body: Record<string, unknown>) => postJSON<{ imported: number; duplicates: number; failed: Record<string, string> }>('/api/v1/performance/import', body);
export type SelectionPerformanceReport = { generatedAt: string; groups: Array<{ recommendation: string; tested: number; withOrders: number; positiveProfit: number; totalProfit: number }> };
export const fetchSelectionPerformance = () => getWithParams<SelectionPerformanceReport>('/api/v1/evaluation/selection-performance');
export type CalibrationMetrics = { samples: number; withExposure: number; withInquiries: number; withOrders: number; positiveProfit: number; withRefunds: number; averageProfit: number; averageMarginBps: number; orderRateBps: number; positiveProfitRateBps: number; insufficientSample: boolean };
export type CalibrationReport = { generatedAt: string; includeTestData: boolean; sampleCount: number; recommendationGroups: Array<CalibrationMetrics & { recommendation: string }>; scoreBuckets: Array<CalibrationMetrics & { range: string; minimum: number; maximum: number }>; dimensionReports: Array<CalibrationMetrics & { dimension: string; highScoreThreshold: number; note: string }>; ruleEffectiveness: Array<CalibrationMetrics & { ruleType: string; rule: string; note: string }>; suggestions: Array<{ dimension?: string; message: string; suggestionOnly: boolean; sampleSize: number }>; readiness: { level: string; realSamples: number; timeSpanDays: number; message: string } };
export const fetchCalibrationReport = (includeTestData = false) => getWithParams<CalibrationReport>('/api/v1/calibration/report', { includeTestData: includeTestData ? 'true' : undefined });
export type SelectionConfig = { id: string; version: string; weights: Record<string, number>; thresholds: { strongRecommend: number; recommend: number; watch: number; minimumMarginBps: number; minimumProfit: number; staleAfterDays: number }; blockers: { sensitiveKeywords: string[]; blockerKeywords: string[] }; status: string; createdAt: string; activatedAt?: string };
export const fetchSelectionConfigs = () => getWithParams<{ list: SelectionConfig[] }>('/api/v1/selection-configs');
export const createSelectionConfigDraft = (body: Record<string, unknown>) => postJSON<SelectionConfig>('/api/v1/selection-configs', body);
export const transitionSelectionConfig = (id: string, action: 'review' | 'activate') => postJSON<SelectionConfig>(`/api/v1/selection-configs/${id}/${action}`, {});
export const fetchCatalogProducts = (params: Record<string, unknown>) => getWithParams<Paged<CatalogProduct>>('/api/v1/catalog-products', listParams(params));
export const createListingDraft = (catalogId: string, platform: 'xianyu' | 'taobao', pricingProfile?: AnalyzeCandidateInput['pricingProfile']) => postJSON<{ listingDraft: ListingDraft; created: boolean }>(`/api/v1/catalog-products/${catalogId}/listing-drafts`, { platform, pricingProfile });
export const fetchListingDrafts = (params: Record<string, unknown>) => getWithParams<Paged<ListingDraft>>('/api/v1/listing-drafts', listParams(params));
export const updateListingDraft = (id: string, body: Partial<ListingDraft>) => putJSON<ListingDraft, Partial<ListingDraft>>(`/api/v1/listing-drafts/${id}`, body);
export const deleteListingDraft = (id: string) => deleteJSON<{ deleted: boolean }>(`/api/v1/listing-drafts/${id}`);
export const recalculateListingDraft = (id: string, pricingProfileId?: string) => postJSON<ListingDraft>(`/api/v1/listing-drafts/${id}/recalculate`, { pricingProfileId });
export type CostCenterSummary = { productCount: number; averageMarginBps: number; lowProfitCount: number; highProfitCount: number; costAnomalyCount: number; products: Array<CatalogProduct & { coverUrl?: string }> };
export const fetchCostCenter = () => getWithParams<CostCenterSummary>('/api/v1/cost-center');
export type SelectionDashboard = { todaySources: number; pendingCandidates: number; todayAnalyzed: number; strongRecommend: number; recommend: number; watch: number; reject: number; averageMarginBps: number; runningBatches: CandidateAnalysisBatch[]; pausedBatches: number; failedBatches: number; marketCoverageBps: number; realMarketCoverageBps: number; realPerformanceCoverageBps: number; calibrationSampleCount: number; calibrationReadiness: string; selectionConfigVersion: string; performanceUpdatedAt?: string; operationalSummary: string };
export const fetchSelectionDashboard = () => getWithParams<SelectionDashboard>('/api/v1/selection-dashboard');
