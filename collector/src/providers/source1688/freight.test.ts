import { describe, expect, it } from 'vitest';
import { parse1688FreightEvidence } from './freight.js';

describe('1688 freight evidence', () => {
  it('accepts explicit destination freight', () => {
    const result = parse1688FreightEvidence(['发货地 广东 送至 天津 运费 8.00 元'], { destination: '天津', quantity: 1 });
    expect(result).toMatchObject({ status: 'verified', orderAmount: '8.00', unitAmount: '8.00' });
  });
  it('recognizes explicit free shipping', () => {
    const result = parse1688FreightEvidence(['送至 天津 包邮'], { destination: '天津', quantity: 1 });
    expect(result).toMatchObject({ status: 'free_shipping', unitAmount: '0.00' });
  });
  it('keeps lower-bound freight unknown', () => {
    const result = parse1688FreightEvidence(['送至 天津 运费 2 元起'], { destination: '天津', quantity: 1 });
    expect(result.status).toBe('unknown');
    expect(result.unitAmount).toBeUndefined();
  });
  it('does not mistake compensation copy for purchase freight', () => {
    const result = parse1688FreightEvidence(
      ['送至 天津', '晚到必赔运费 2 元', '退货包运费'],
      { destination: '天津', quantity: 1 },
    );
    expect(result.status).toBe('unknown');
    expect(result.unitAmount).toBeUndefined();
  });
  it('allocates an explicit order freight by quantity', () => {
    const result = parse1688FreightEvidence(['送至 天津 数量 10 件 运费 8 元'], { destination: '天津', quantity: 10 });
    expect(result).toMatchObject({ status: 'verified', orderAmount: '8.00', unitAmount: '0.80' });
  });
});
