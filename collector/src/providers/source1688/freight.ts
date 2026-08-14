import type { ProductFreight } from '../../types/product.js';

export type FreightScenario = {
  destination?: string;
  quantity?: number;
};

function decimalToCents(value: string): number | undefined {
  const normalized = value.trim().replace(',', '');
  if (!/^\d+(?:\.\d{1,2})?$/.test(normalized)) return undefined;
  const [whole, fraction = ''] = normalized.split('.');
  const cents = Number(whole) * 100 + Number(fraction.padEnd(2, '0'));
  return Number.isSafeInteger(cents) ? cents : undefined;
}

function centsToDecimal(cents: number): string {
  return `${Math.floor(cents / 100)}.${String(cents % 100).padStart(2, '0')}`;
}

function baseUnknown(texts: string[], scenario: FreightScenario, observedAt: string): ProductFreight {
  return {
    status: 'unknown',
    currency: 'CNY',
    destination: scenario.destination?.trim() || undefined,
    quantity: Math.max(1, Math.trunc(scenario.quantity ?? 1)),
    source: 'official_page_dom',
    confidenceBps: 0,
    observedAt,
    rawSnapshot: { texts },
    calculationMethod: texts.length ? 'page_evidence_not_deterministic' : 'no_freight_evidence',
  };
}

/** Parses only explicit 1688 page evidence. "X 元起" is never deterministic. */
export function parse1688FreightEvidence(
  rawTexts: string[],
  scenario: FreightScenario = {},
  observedAt = new Date().toISOString(),
): ProductFreight {
  const texts = [...new Set(rawTexts.map((item) => item.replace(/\s+/g, ' ').trim()).filter(Boolean))].slice(0, 20);
  const result = baseUnknown(texts, scenario, observedAt);
  const combined = texts.join(' | ');
  const destination = scenario.destination?.trim() ?? '';
  const quantity = result.quantity;
  const destinationMatched = destination === '' || combined.includes(destination);

  if (/(?:包邮|免运费|运费\s*[:：]?\s*[¥￥]?\s*0(?:\.00)?(?:\s*元)?(?:\D|$))/i.test(combined) && destinationMatched) {
    return {
      ...result,
      status: 'free_shipping',
      orderAmount: '0.00',
      unitAmount: '0.00',
      confidenceBps: destination ? 10000 : 8500,
      calculationMethod: 'explicit_free_shipping',
    };
  }

  const amountPattern = /(?:运费|快递费|物流费|配送费)\s*[:：]?\s*[¥￥]?\s*(\d+(?:\.\d{1,2})?)\s*(?:元)?/i;
  const amountText = texts.find(
    (text) =>
      amountPattern.test(text) &&
      !/(?:退货包运费|运费险|晚到必赔|迟到赔|赔付|补偿)/i.test(text),
  );
  const amountMatch = amountText ? amountPattern.exec(amountText) : null;
  const lowerBound = amountText
    ? /(?:运费|快递费|物流费|配送费).{0,30}(?:起|起步|以上)/i.test(amountText)
    : false;
  if (!amountMatch?.[1] || lowerBound || !destinationMatched) return result;

  const orderCents = decimalToCents(amountMatch[1]);
  if (orderCents === undefined) return result;
  if (quantity > 1 && !new RegExp(`(?:数量|采购量|购买量)\\s*[:：]?\\s*${quantity}(?:件|个|双)?`).test(combined)) {
    return result;
  }

  const unitCents = Math.ceil(orderCents / quantity);
  return {
    ...result,
    status: destination ? 'verified' : 'estimated',
    orderAmount: centsToDecimal(orderCents),
    unitAmount: centsToDecimal(unitCents),
    confidenceBps: destination ? 9000 : 6000,
    calculationMethod: quantity > 1 ? 'official_order_freight_divided_by_quantity' : 'explicit_page_freight',
  };
}
