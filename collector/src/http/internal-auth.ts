import { timingSafeEqual } from 'node:crypto';

export const COLLECTOR_TOKEN_HEADER = 'x-trademind-collector-token';

export function hasValidCollectorToken(received: string | string[] | undefined, expected: string): boolean {
  if (typeof received !== 'string' || !expected) return false;
  const actualBuffer = Buffer.from(received, 'utf8');
  const expectedBuffer = Buffer.from(expected, 'utf8');
  if (actualBuffer.length !== expectedBuffer.length) return false;
  return timingSafeEqual(actualBuffer, expectedBuffer);
}
