import { describe, expect, it } from 'vitest';
import { hasValidCollectorToken } from '../internal-auth.js';

describe('collector internal authentication', () => {
  it('accepts only an exact token match', () => {
    expect(hasValidCollectorToken('shared-secret', 'shared-secret')).toBe(true);
    expect(hasValidCollectorToken('shared-secreu', 'shared-secret')).toBe(false);
    expect(hasValidCollectorToken('short', 'shared-secret')).toBe(false);
    expect(hasValidCollectorToken(undefined, 'shared-secret')).toBe(false);
    expect(hasValidCollectorToken(['shared-secret'], 'shared-secret')).toBe(false);
  });
});
