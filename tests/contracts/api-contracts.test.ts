import { describe, expect, it } from 'vitest';
import contracts from './api-contracts.json';

const routeKey = (endpoint: { method: string; path: string }) => `${endpoint.method} ${endpoint.path}`;

describe('TradeMind API contract registry', () => {
  it('keeps the backend envelope explicit for frontend and E2E mocks', () => {
    expect(contracts.envelope.success).toEqual(['code', 'message', 'data']);
    expect(contracts.envelope.optional).toContain('traceId');
    expect(contracts.envelope.errorCodeRule).toContain('non-zero');
  });

  it('covers the core Admin product publishing and readiness endpoints', () => {
    const routes = new Set(contracts.endpoints.map(routeKey));

    expect(routes).toEqual(new Set([
        'POST /api/v1/candidates/:id/analyze',
        'GET /api/v1/candidates/:id/analysis',
        'GET /api/v1/candidates/:id/analyses',
        'GET /api/v1/cost-center',
        'GET /api/v1/candidate-analysis-batches',
        'GET /api/v1/candidate-analysis-batches/:id/items',
        'POST /api/v1/candidate-analysis-batches/:id/pause',
        'POST /api/v1/candidate-analysis-batches/:id/resume',
        'POST /api/v1/candidate-analysis-batches/:id/cancel',
        'POST /api/v1/candidate-analysis-batches/:id/retry-failed',
        'POST /api/v1/market-signals/import',
        'GET /api/v1/market-signal-providers/status',
        'POST /api/v1/performance/import',
        'GET /api/v1/performance/summary',
        'GET /api/v1/evaluation/selection-performance',
        'POST /api/v1/imports/market-signals/preview',
        'POST /api/v1/imports/market-signals/confirm',
        'POST /api/v1/imports/performance/preview',
        'POST /api/v1/imports/performance/confirm',
        'GET /api/v1/market-signal-provider-configs',
        'POST /api/v1/market-signal-provider-configs/:id/health-check',
        'GET /api/v1/calibration/report',
        'GET /api/v1/selection-configs',
        'POST /api/v1/selection-configs/:id/activate',
        'GET /api/v1/listing-drafts/:id/workspace',
        'POST /api/v1/listing-drafts/:id/content/restore',
        'PUT /api/v1/listing-drafts/:id/assets',
        'POST /api/v1/listing-drafts/:id/content/generate',
        'PUT /api/v1/listing-drafts/:id/content',
        'POST /api/v1/listing-drafts/:id/content/review',
        'POST /api/v1/listing-drafts/:id/ready',
        'POST /api/v1/listing-drafts/:id/publish-packages',
        'POST /api/v1/publish-packages/bulk-generate',
        'GET /api/v1/publish-packages/:id/download',
        'POST /api/v1/listing-drafts/:id/published-manual',
        'GET /api/v1/auth/profile',
        'GET /api/v1/image/providers',
        'GET /api/v1/products/:id',
        'GET /api/v1/products/:id/readiness',
        'GET /api/v1/products/:id/publications',
        'GET /api/v1/product-publications/:id/douyin/sku-bindings',
        'GET /api/v1/products/:id/publish-targets',
        'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft',
        'POST /api/v1/products/:id/publish',
      ]));
  });

  it('defines payload/query contracts for state-changing publish APIs', () => {
    const createDraft = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/platform-configs/douyin_shop/create-draft');
    const publish = contracts.endpoints.find((item) => routeKey(item) === 'POST /api/v1/products/:id/publish');
    const readiness = contracts.endpoints.find((item) => routeKey(item) === 'GET /api/v1/products/:id/readiness');

    expect(createDraft?.requestBody).toEqual(['shopId', 'publishMode', 'force']);
    expect(publish?.requestBody).toEqual(['shopId', 'options', 'force']);
    expect(readiness?.query).toEqual(['platform', 'shopId', 'mode']);
  });

  it('marks every protected Admin endpoint as authenticated', () => {
    expect(contracts.endpoints).toHaveLength(44);
    expect(contracts.endpoints.every((endpoint) => endpoint.auth === true)).toBe(true);
  });
});
