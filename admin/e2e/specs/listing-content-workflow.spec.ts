import { test, expect } from '../fixtures/admin.fixture';
import { ok } from '../mocks/envelope';
import { expectNoRootOverflow } from '../utils/assertions';

test.describe('@listing-content semi-automatic listing workflow', () => {
  test('generates, reviews and packages a listing', async ({ admin, page }) => {
    const content = { id: 'content-2', listingDraftId: 'demo-listing', platform: 'xianyu', version: 2, generationMode: 'template_only', contentProfileVersion: 'xianyu-content-v1', promptVersion: 'xianyu-v1', title: '桌面收纳盒', description: '结构化商品说明', sellingPoints: ['真实 SKU 可选'], keywords: ['收纳盒'], faq: [], skuContent: [{ code: 'W-S' }], warnings: [], blockers: [], reviewStatus: 'needs_review', aiStatus: 'not_requested' };
    admin.writeGuard.allow({ operation: 'generate', method: 'POST', path: /\/api\/v1\/listing-drafts\/demo-listing\/content\/generate$/, response: ok(content) });
    admin.writeGuard.allow({ operation: 'review', method: 'POST', path: /\/api\/v1\/listing-drafts\/demo-listing\/content\/review$/, response: ok({ ...content, reviewStatus: 'approved' }) });
    admin.writeGuard.allow({ operation: 'ready', method: 'POST', path: /\/api\/v1\/listing-drafts\/demo-listing\/ready$/, response: ok({ publishStatus: 'ready_to_publish' }) });
    admin.writeGuard.allow({ operation: 'package', method: 'POST', path: /\/api\/v1\/listing-drafts\/demo-listing\/publish-packages$/, response: ok({ id: 'package-1', packageVersion: 1, sizeBytes: 2048 }) });
    await page.route('**/api/v1/publish-packages/package-1/download', (route) => route.fulfill({ status: 200, contentType: 'application/zip', body: 'demo' }));
    await admin.goto('/listing-drafts/demo-listing/review');
    await expect(page.getByText('内容审核').first()).toBeVisible();
    await page.getByRole('button', { name: '重新模板生成' }).click();
    await page.getByRole('button', { name: '人工批准内容' }).click();
    await page.getByRole('button', { name: '完成发布前检查' }).click();
    await page.getByRole('button', { name: '生成发布包' }).click();
    await admin.writeGuard.expectRequestCount('generate', 1);
    await admin.writeGuard.expectRequestCount('review', 1);
    await admin.writeGuard.expectRequestCount('ready', 1);
    await admin.writeGuard.expectRequestCount('package', 1);
    await expectNoRootOverflow(page);
  });

  test('records manual publish data without invoking a platform API', async ({ admin, page }) => {
    await page.route('**/api/v1/listing-drafts/demo-listing/workspace', (route) => route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(ok({ listing: { id: 'demo-listing', platform: 'xianyu', salePrice: 39.9, estimatedProfit: 18.2, estimatedMargin: 0.4561, publishStatus: 'ready_to_publish' }, catalog: { id: 'demo-catalog', title: '桌面收纳盒', skus: [] }, content: { id: 'content-1', version: 1, promptVersion: 'xianyu-v1', title: '桌面收纳盒', description: '结构化商品说明', sellingPoints: [], keywords: [], warnings: [], blockers: [], reviewStatus: 'approved' }, assets: [], packages: [], publishRecords: [] })) }));
    admin.writeGuard.allow({ operation: 'manual-publish', method: 'POST', path: /\/api\/v1\/listing-drafts\/demo-listing\/published-manual$/, response: ok({ id: 'record-1', platformListingId: 'XY-10001', publishMethod: 'manual' }) });
    await admin.goto('/listing-drafts/demo-listing/review');
    await page.getByLabel('平台商品 ID').fill('XY-10001');
    await page.getByLabel('商品链接').fill('https://www.goofish.com/item?id=XY-10001');
    await page.getByRole('button', { name: '标记已人工发布' }).click();
    await admin.writeGuard.expectRequestCount('manual-publish', 1);
  });
});
