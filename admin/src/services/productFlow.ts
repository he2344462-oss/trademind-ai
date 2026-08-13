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
  estimatedMargin?: number; sourceProduct?: SourceProduct; rejectionReason?: string;
};
export type CatalogProduct = {
  id: string; title: string; supplier?: string; purchaseCost?: number; freightCost?: number;
  suggestedSalePrice?: number; salePrice?: number; estimatedProfit?: number; estimatedMargin?: number;
  catalogStatus: string; images?: Array<{ publicUrl?: string; originUrl?: string }>;
  skus?: Array<{ id: string; skuCode: string; skuName: string }>;
};
export type ListingDraft = {
  id: string; catalogProductId: string; platform: 'xianyu' | 'taobao'; title: string;
  description?: string; images?: string[]; salePrice?: number; platformCategory?: string;
  platformSkuData?: unknown[]; publishStatus: string; createdAt: string;
};

const value = (input: unknown): string | number | undefined =>
  typeof input === 'string' || typeof input === 'number' ? input : undefined;
const listParams = (params: Record<string, unknown>): Record<string, string | number | undefined> => ({
  page: value(params.current), pageSize: value(params.pageSize), keyword: value(params.keyword), status: value(params.status),
  platform: value(params.platform), catalogProductId: value(params.catalogProductId),
});
export const fetchSourceProducts = (params: Record<string, unknown>) => getWithParams<Paged<SourceProduct>>('/api/v1/source-products', listParams(params));
export const createSourceProduct = (body: Record<string, unknown>) => postJSON<SourceProduct>('/api/v1/source-products', body);
export const addSourceToCandidates = (id: string) => postJSON<{ candidate: Candidate; created: boolean }>(`/api/v1/source-products/${id}/candidate`, {});
export const fetchCandidates = (params: Record<string, unknown>) => getWithParams<Paged<Candidate>>('/api/v1/candidates', listParams(params));
export const approveCandidate = (id: string) => postJSON<{ catalogProduct: CatalogProduct; created: boolean }>(`/api/v1/candidates/${id}/approve`, {});
export const watchCandidate = (id: string) => postJSON<Candidate>(`/api/v1/candidates/${id}/watch`, {});
export const rejectCandidate = (id: string, rejectionReason = '') => postJSON<Candidate>(`/api/v1/candidates/${id}/reject`, { rejectionReason });
export const fetchCatalogProducts = (params: Record<string, unknown>) => getWithParams<Paged<CatalogProduct>>('/api/v1/catalog-products', listParams(params));
export const createListingDraft = (catalogId: string, platform: 'xianyu' | 'taobao') => postJSON<{ listingDraft: ListingDraft; created: boolean }>(`/api/v1/catalog-products/${catalogId}/listing-drafts`, { platform });
export const fetchListingDrafts = (params: Record<string, unknown>) => getWithParams<Paged<ListingDraft>>('/api/v1/listing-drafts', listParams(params));
export const updateListingDraft = (id: string, body: Partial<ListingDraft>) => putJSON<ListingDraft, Partial<ListingDraft>>(`/api/v1/listing-drafts/${id}`, body);
export const deleteListingDraft = (id: string) => deleteJSON<{ deleted: boolean }>(`/api/v1/listing-drafts/${id}`);
