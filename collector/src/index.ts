import { existsSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { BrowserManager } from './browser/manager.js';
import { listenCollectorHttp } from './http/server.js';

const rootEnvPath = fileURLToPath(new URL('../../.env', import.meta.url));
if (existsSync(rootEnvPath)) {
  process.loadEnvFile(rootEnvPath);
}

const browser = new BrowserManager();
const server = listenCollectorHttp(browser);

function shutdown() {
  console.info('[collector] shutting down...');
  server.close(() => {
    browser.close().finally(() => process.exit(0));
  });
}

process.on('SIGINT', shutdown);
process.on('SIGTERM', shutdown);
