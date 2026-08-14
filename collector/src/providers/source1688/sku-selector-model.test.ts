import { describe, expect, it } from 'vitest';

import { parse1688SkuSelectorModel, skuPriceRange } from './sku-selector-model.js';

describe('1688 network sku selector model', () => {
  it('preserves a real two-dimensional SKU matrix with price, stock, image, and source IDs', () => {
    const model = {
      originalSkuInfoMap: {
        '红色&gt;24': { skuId: 101, specId: 'spec-101', specAttrs: '红色&gt;24', price: '5.50', canBookCount: 12 },
        '红色&gt;25': { skuId: 102, specId: 'spec-102', specAttrs: '红色&gt;25', price: '5.50', canBookCount: 0 },
        '黑色&gt;24': { skuId: 103, specId: 'spec-103', specAttrs: '黑色&gt;24', price: '6.50', canBookCount: 8 },
        '黑色&gt;25': { skuId: 104, specId: 'spec-104', specAttrs: '黑色&gt;25', price: '6.50', canBookCount: 9 },
      },
      skuSelectorModel: {
        skuPropsList: [
          { prop: '颜色', value: [{ name: '红色', imageUrl: 'https://cbu01.alicdn.com/red.jpg' }, { name: '黑色', imageUrl: 'https://cbu01.alicdn.com/black.jpg' }] },
          { prop: '尺码', value: [{ name: '24' }, { name: '25' }] },
        ],
      },
    };

    const skus = parse1688SkuSelectorModel(model);

    expect(skus).toHaveLength(4);
    expect(skus.map((sku) => sku.properties)).toEqual([
      { 颜色: '红色', 尺码: '24' },
      { 颜色: '红色', 尺码: '25' },
      { 颜色: '黑色', 尺码: '24' },
      { 颜色: '黑色', 尺码: '25' },
    ]);
    expect(skus[0]).toMatchObject({ skuCode: '101', price: 5.5, stock: 12, image: 'https://cbu01.alicdn.com/red.jpg' });
    expect(skus[1]?.stock).toBe(0);
    expect(skus[0]?.raw).toMatchObject({ sourceSkuId: '101', specId: 'spec-101' });
    expect(skuPriceRange(skus)).toEqual({ min: 5.5, max: 6.5 });
  });

  it('does not manufacture missing combinations', () => {
    const skus = parse1688SkuSelectorModel({
      originalSkuInfoMap: {
        '红色&gt;24': { skuId: 101, specAttrs: '红色&gt;24', price: '5.50', canBookCount: 1 },
        '黑色&gt;25': { skuId: 104, specAttrs: '黑色&gt;25', price: '6.50', canBookCount: 2 },
      },
      skuSelectorModel: {
        skuPropsList: [
          { prop: '颜色', value: [{ name: '红色' }, { name: '黑色' }] },
          { prop: '尺码', value: [{ name: '24' }, { name: '25' }] },
        ],
      },
    });

    expect(skus).toHaveLength(2);
    expect(skus.map((sku) => sku.skuCode)).toEqual(['101', '104']);
  });
});
