import type { ProductSku } from '../../types/product.js';
import type { DimRow } from './sku-helpers.js';
import {
  extractSkuBucketPrice,
  extractSkuBucketStock,
  inferTwoDimNames,
  normalizeSkuPropertyKeys,
  parseComboKey,
  skuNameFromProps,
} from './sku-helpers.js';
import { normalizeImageUrl, trimStr } from './utils.js';

const MAX_NETWORK_SKUS = 500;
const BASE_URL = 'https://detail.1688.com/offer/';

function recordOf(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null;
}

function dimensionRows(model: Record<string, unknown>): DimRow[] {
  const selector = recordOf(model.skuSelectorModel);
  const list = selector?.skuPropsList;
  if (!Array.isArray(list)) return [];
  const rows: DimRow[] = [];
  for (const item of list) {
    const row = recordOf(item);
    const name = trimStr(String(row?.prop ?? row?.name ?? ''));
    if (!name || !Array.isArray(row?.value)) continue;
    const values = row.value
      .map((value) => recordOf(value))
      .map((value) => trimStr(String(value?.name ?? value?.value ?? '')))
      .filter(Boolean);
    if (values.length) rows.push({ name, values: [...new Set(values)] });
  }
  return rows;
}

function dimensionImages(model: Record<string, unknown>): Map<string, string> {
  const selector = recordOf(model.skuSelectorModel);
  const list = selector?.skuPropsList;
  const images = new Map<string, string>();
  if (!Array.isArray(list)) return images;
  for (const item of list) {
    const row = recordOf(item);
    if (!Array.isArray(row?.value)) continue;
    for (const rawValue of row.value) {
      const value = recordOf(rawValue);
      const name = trimStr(String(value?.name ?? value?.value ?? ''));
      const rawImage = trimStr(String(value?.imageUrl ?? value?.imageURL ?? value?.picUrl ?? ''));
      const image = rawImage ? normalizeImageUrl(rawImage, BASE_URL) : null;
      if (name && image) images.set(name, image);
    }
  }
  return images;
}

export function parse1688SkuSelectorModel(value: unknown): ProductSku[] {
  const model = recordOf(value);
  if (!model) return [];
  const skuMap = recordOf(model.originalSkuInfoMap) ?? recordOf(model.skuInfoMap);
  if (!skuMap) return [];

  const dimensions = dimensionRows(model);
  const dimensionNames = inferTwoDimNames(dimensions, '', '');
  const images = dimensionImages(model);
  const result: ProductSku[] = [];

  for (const [mapKey, rawBucket] of Object.entries(skuMap).slice(0, MAX_NETWORK_SKUS)) {
    const bucket = recordOf(rawBucket);
    if (!bucket) continue;
    const specAttrs = trimStr(String(bucket.specAttrs ?? mapKey));
    const properties = normalizeSkuPropertyKeys(parseComboKey(specAttrs, dimensions), dimensionNames);
    if (!Object.keys(properties).length) continue;

    const sourceSkuId = trimStr(String(bucket.skuId ?? ''));
    const specId = trimStr(String(bucket.specId ?? ''));
    const stock = extractSkuBucketStock(bucket);
    const propertyImage = Object.values(properties).map((name) => images.get(name)).find(Boolean);

    result.push({
      skuCode: sourceSkuId || specId,
      properties,
      price: extractSkuBucketPrice(bucket),
      stock: stock !== undefined && stock >= 0 ? stock : undefined,
      image: propertyImage,
      raw: {
        source: '1688-network-sku-selector',
        sourceSkuId,
        specId,
        specAttrs,
        skuNameHint: skuNameFromProps(properties),
        price: bucket.price,
        discountPrice: bucket.discountPrice,
        multiPrice: bucket.multiPrice,
        canBookCount: bucket.canBookCount,
      },
    });
  }
  return result;
}

export function skuPriceRange(skus: ProductSku[]): { min?: number; max?: number } {
  const prices = skus
    .map((sku) => sku.price)
    .filter((price): price is number => typeof price === 'number' && price > 0);
  if (!prices.length) return {};
  return { min: Math.min(...prices), max: Math.max(...prices) };
}
