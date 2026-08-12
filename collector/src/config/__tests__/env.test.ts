import { afterEach, describe, expect, it } from 'vitest';
import {
  getBrowserHeadless,
  getDefaultNavigationTimeoutMs,
  getHttpHost,
  getHttpPort,
  getInternalToken,
  getMaxBodyBytes,
} from '../env.js';

const KEYS = ['COLLECTOR_HTTP_ADDR', 'COLLECTOR_HTTP_HOST', 'COLLECTOR_INTERNAL_TOKEN', 'COLLECTOR_MAX_BODY_BYTES', 'COLLECTOR_GOTO_TIMEOUT_MS', 'COLLECTOR_HEADLESS'] as const;
const original = Object.fromEntries(KEYS.map((key) => [key, process.env[key]]));

afterEach(() => {
  for (const key of KEYS) {
    const value = original[key];
    if (value === undefined) delete process.env[key];
    else process.env[key] = value;
  }
});

describe('collector env helpers', () => {
  it('parses colon-prefixed HTTP ports and falls back safely', () => {
    process.env.COLLECTOR_HTTP_ADDR = ':3201';
    expect(getHttpPort()).toBe(3201);

    process.env.COLLECTOR_HTTP_ADDR = 'not-a-port';
    expect(getHttpPort()).toBe(3100);
  });

  it('uses a positive navigation timeout', () => {
    process.env.COLLECTOR_GOTO_TIMEOUT_MS = '60000';
    expect(getDefaultNavigationTimeoutMs()).toBe(60000);

    process.env.COLLECTOR_GOTO_TIMEOUT_MS = '-1';
    expect(getDefaultNavigationTimeoutMs()).toBe(45000);
  });

  it('binds loopback by default and requires an internal token', () => {
    delete process.env.COLLECTOR_HTTP_HOST;
    expect(getHttpHost()).toBe('127.0.0.1');
    process.env.COLLECTOR_HTTP_HOST = '0.0.0.0';
    expect(getHttpHost()).toBe('0.0.0.0');

    delete process.env.COLLECTOR_INTERNAL_TOKEN;
    expect(() => getInternalToken()).toThrow('COLLECTOR_INTERNAL_TOKEN is required');
    process.env.COLLECTOR_INTERNAL_TOKEN = 'short';
    expect(() => getInternalToken()).toThrow('must be at least 32 characters');
    process.env.COLLECTOR_INTERNAL_TOKEN = ` ${'t'.repeat(32)} `;
    expect(getInternalToken()).toBe('t'.repeat(32));
  });

  it('caps request bodies with a safe default', () => {
    delete process.env.COLLECTOR_MAX_BODY_BYTES;
    expect(getMaxBodyBytes()).toBe(1048576);
    process.env.COLLECTOR_MAX_BODY_BYTES = '2048';
    expect(getMaxBodyBytes()).toBe(2048);
    process.env.COLLECTOR_MAX_BODY_BYTES = '-1';
    expect(getMaxBodyBytes()).toBe(1048576);
  });

  it('keeps headless mode on unless explicitly disabled', () => {
    delete process.env.COLLECTOR_HEADLESS;
    expect(getBrowserHeadless()).toBe(true);

    process.env.COLLECTOR_HEADLESS = '0';
    expect(getBrowserHeadless()).toBe(false);

    process.env.COLLECTOR_HEADLESS = 'false';
    expect(getBrowserHeadless()).toBe(false);
  });
});
