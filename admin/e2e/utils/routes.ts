import type { Page } from '@playwright/test';
import { ok } from '../mocks/envelope';
import { e2eUser, E2E_TOKEN } from '../mocks/auth';
import { productsResponse } from '../mocks/products';
import { readinessResponse } from '../mocks/readiness';
import { publishResponse, skuBindingsResponse } from '../mocks/publish';
import { inventoryResponse } from '../mocks/inventory';
import { inventorySyncP9Response } from '../mocks/inventory-sync-p9';
import { imageProviderCapabilities } from '../mocks/image-providers';

export async function seedAdminAuth(page: Page) {
  await page.addInitScript(([key, token]) => {
    window.localStorage.setItem(key, token);
  }, ['trademind_admin_token', E2E_TOKEN]);
}

export async function routeStaticAssets(page: Page) {
  await page.route('**/*.{png,jpg,jpeg,webp,gif,svg}', async (route) => {
    await route.fulfill({ status: 200, contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" />' });
  });
}

export async function routeAdminApi(page: Page) {
  await routeStaticAssets(page);
  await page.route('**/api/v1/**', async (route) => {
    const request = route.request();
    if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method().toUpperCase())) {
      await route.fallback();
      return;
    }

    const url = new URL(request.url());
    const path = url.pathname;
    const response =
      (path === '/api/v1/auth/profile' ? ok(e2eUser) : null) ??
      (path === '/api/v1/settings' ? ok({ items: [] }) : null) ??
      (path === '/api/v1/image/providers' ? ok(imageProviderCapabilities) : null) ??
      (path === '/api/v1/selection-dashboard' ? ok({ todaySources: 12, pendingCandidates: 8, todayAnalyzed: 20, strongRecommend: 2, recommend: 5, watch: 7, reject: 6, averageMarginBps: 3250, runningBatches: [], pausedBatches: 1, failedBatches: 0, marketCoverageBps: 2500, realMarketCoverageBps: 2000, realPerformanceCoverageBps: 1500, calibrationSampleCount: 25, calibrationReadiness: 'low', selectionConfigVersion: 'selection-v3-default', operationalSummary: '测试运营摘要', pendingContent: 3, pendingReview: 2, readyToPublish: 1, publishedListings: 4, todayGeneratedContent: 5, estimatedListingProfit: '123.45' }) : null) ??
      (path === '/api/v1/listing-drafts/demo-listing/workspace' ? ok({ listing: { id: 'demo-listing', catalogProductId: 'demo-catalog', platform: 'xianyu', title: '桌面收纳盒', description: '结构化商品说明', images: ['https://example.com/a.jpg'], salePrice: 39.9, estimatedProfit: 18.2, estimatedMargin: 0.4561, platformSkuData: [{ code: 'W-S', displayName: '白色 / 小号' }], publishStatus: 'needs_review', currentContentVersionId: 'content-1' }, catalog: { id: 'demo-catalog', title: '桌面收纳盒', supplier: '内部测试供应商', purchaseCost: 12.8, skus: [{ id: 'sku-1', skuCode: 'W-S', skuName: '白色 / 小号' }] }, content: { id: 'content-1', listingDraftId: 'demo-listing', platform: 'xianyu', version: 1, generationMode: 'template_only', contentProfileVersion: 'xianyu-content-v1', promptVersion: 'xianyu-v1', title: '桌面收纳盒', description: '结构化商品说明', sellingPoints: ['真实 SKU 可选'], keywords: ['收纳盒'], faq: [], skuContent: [{ code: 'W-S' }], warnings: [], blockers: [], reviewStatus: 'needs_review', aiStatus: 'not_requested', createdAt: '2026-08-13T00:00:00Z' }, assets: [{ id: 'asset-1', sourceUrl: 'https://example.com/a.jpg', sourceType: 'catalog', sortOrder: 0, isPrimary: true, excluded: false, sizeBytes: 0 }], packages: [], publishRecords: [] }) : null) ??
      (path === '/api/v1/listing-drafts/demo-listing/content-versions' ? ok({ list: [] }) : null) ??
      (path === '/api/v1/performance/summary' ? ok({ snapshotCount: 20, listingCount: 20, views: 1200, inquiries: 80, orders: 12, refunds: 1, grossRevenue: 47880, realizedProfit: 16800, actualMarginBps: 3508 }) : null) ??
      (path === '/api/v1/evaluation/selection-performance' ? ok({ generatedAt: '2026-08-13T00:00:00Z', groups: [] }) : null) ??
      (path === '/api/v1/calibration/report' ? ok({ generatedAt: '2026-08-13T00:00:00Z', includeTestData: false, sampleCount: 25, recommendationGroups: [], scoreBuckets: [], dimensionReports: [], ruleEffectiveness: [], suggestions: [{ message: '建议继续积累样本', suggestionOnly: true, sampleSize: 25 }], readiness: { level: 'low', realSamples: 25, timeSpanDays: 14, message: '样本有限，建议人工复核。' } }) : null) ??
      (path === '/api/v1/market-signal-provider-configs' ? ok({ list: [] }) : null) ??
      (path === '/api/v1/selection-configs' ? ok({ list: [{ id: 'selection-config-1', version: 'selection-v3-default', weights: { profit: 30, data_quality: 20, supply: 15, risk: 15, platform_fit: 10, demand: 5, competition: 5 }, thresholds: { strongRecommend: 85, recommend: 70, watch: 50, minimumMarginBps: 2000, minimumProfit: 500, staleAfterDays: 90 }, blockers: { sensitiveKeywords: [], blockerKeywords: [] }, status: 'active', createdAt: '2026-08-13T00:00:00Z' }] }) : null) ??
      (path === '/api/v1/pricing-profiles' ? ok({ list: [] }) : null) ??
      inventorySyncP9Response(path) ??
      productsResponse(path) ??
      readinessResponse(path) ??
      publishResponse(path) ??
      inventoryResponse(path) ??
      (path.includes('/product-publications/') && path.endsWith('/douyin/sku-bindings') ? skuBindingsResponse(path.split('/').at(-3) || undefined) : null) ??
      ok({ list: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } });

    await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(response) });
  });
}
