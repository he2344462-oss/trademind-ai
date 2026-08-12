import type { AddressInfo } from 'node:net';
import { afterEach, describe, expect, it } from 'vitest';
import type { BrowserManager } from '../../browser/manager.js';
import { createCollectorServer } from '../server.js';

const originalToken = process.env.COLLECTOR_INTERNAL_TOKEN;

afterEach(() => {
  if (originalToken === undefined) delete process.env.COLLECTOR_INTERNAL_TOKEN;
  else process.env.COLLECTOR_INTERNAL_TOKEN = originalToken;
});

describe('collector HTTP authentication', () => {
  it('keeps health public and protects every v1 route', async () => {
    const token = 'collector-test-token-that-is-32-chars';
    process.env.COLLECTOR_INTERNAL_TOKEN = token;
    const server = createCollectorServer({} as BrowserManager);
    await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));

    try {
      const port = (server.address() as AddressInfo).port;
      const base = `http://127.0.0.1:${port}`;
      expect((await fetch(`${base}/health`)).status).toBe(200);
      expect((await fetch(`${base}/v1/providers`)).status).toBe(401);
      expect((await fetch(`${base}/v1/providers`, {
        headers: { 'X-TradeMind-Collector-Token': token },
      })).status).toBe(200);
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    }
  });
});
