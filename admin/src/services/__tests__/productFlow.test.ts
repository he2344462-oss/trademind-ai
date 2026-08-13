import { request } from '@umijs/max';
import { describe, expect, it, vi } from 'vitest';
import { addSourceToCandidates, analyzeCandidate, approveCandidate, createListingDraft, deleteListingDraft, fetchCandidateAnalysis, fetchCostCenter, fetchSourceProducts, updateListingDraft } from '../productFlow';

const requestMock = vi.mocked(request);
describe('product flow service contracts', () => {
  it('maps source product pagination and sorting to the backend contract', async () => {
    requestMock.mockResolvedValueOnce({ code: 0, message: 'ok', data: { list: [], pagination: { total: 0 } } });
    await fetchSourceProducts({ current: 2, pageSize: 10, keyword: '收纳箱' });
    expect(requestMock).toHaveBeenCalledWith('/api/v1/source-products', { method: 'GET', params: { page: 2, pageSize: 10, keyword: '收纳箱', status: undefined, platform: undefined, catalogProductId: undefined, sortBy: undefined, sortOrder: undefined } });
  });
  it('uses dedicated state-transition endpoints', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: {} });
    await addSourceToCandidates('source-1'); await approveCandidate('candidate-1');
    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/v1/source-products/source-1/candidate', { method: 'POST', data: {} });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/v1/candidates/candidate-1/approve', { method: 'POST', data: {} });
  });
  it('uses candidate analysis and cost-center contracts', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: {} });
    await analyzeCandidate('candidate-1', { analysisMode: 'rules_only', packagingCost: '0.50' }); await fetchCandidateAnalysis('candidate-1'); await fetchCostCenter();
    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/v1/candidates/candidate-1/analyze', { method: 'POST', data: { analysisMode: 'rules_only', packagingCost: '0.50' } });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/v1/candidates/candidate-1/analysis', { method: 'GET', params: undefined });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/api/v1/cost-center', { method: 'GET', params: undefined });
  });
  it('creates, updates and deletes listing drafts without publish APIs', async () => {
    requestMock.mockResolvedValue({ code: 0, message: 'ok', data: {} });
    await createListingDraft('catalog-1', 'xianyu'); await updateListingDraft('draft-1', { title: '闲鱼标题', salePrice: 59 }); await deleteListingDraft('draft-1');
    expect(requestMock).toHaveBeenNthCalledWith(1, '/api/v1/catalog-products/catalog-1/listing-drafts', { method: 'POST', data: { platform: 'xianyu', pricingProfile: undefined } });
    expect(requestMock).toHaveBeenNthCalledWith(2, '/api/v1/listing-drafts/draft-1', { method: 'PUT', data: { title: '闲鱼标题', salePrice: 59 } });
    expect(requestMock).toHaveBeenNthCalledWith(3, '/api/v1/listing-drafts/draft-1', { method: 'DELETE' });
  });
});
