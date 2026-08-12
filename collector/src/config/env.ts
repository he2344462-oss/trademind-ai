/**
 * 环境变量（由 docker / systemd / .env 注入，不写入代码默认值中的密钥）。
 */
import { getBrowserProfileRoot, get1688UserDataDir, getStorageStateRoot } from '../browser/browser-paths.js';

export { getBrowserProfileRoot, get1688UserDataDir, getStorageStateRoot };

export function getHttpPort(): number {
  const raw = process.env.COLLECTOR_HTTP_ADDR ?? ':3100';
  const n = Number(String(raw).replace(/^\:/, ''));
  return Number.isFinite(n) && n > 0 ? n : 3100;
}

export function getHttpHost(): string {
  return process.env.COLLECTOR_HTTP_HOST?.trim() || '127.0.0.1';
}

export function getInternalToken(): string {
  const token = process.env.COLLECTOR_INTERNAL_TOKEN?.trim() ?? '';
  if (!token) {
    throw new Error('COLLECTOR_INTERNAL_TOKEN is required');
  }
  if (token.length < 32) {
    throw new Error('COLLECTOR_INTERNAL_TOKEN must be at least 32 characters');
  }
  return token;
}

export function getMaxBodyBytes(): number {
  const n = Number(process.env.COLLECTOR_MAX_BODY_BYTES ?? '1048576');
  return Number.isSafeInteger(n) && n > 0 ? n : 1048576;
}

export function getDefaultNavigationTimeoutMs(): number {
  const n = Number(process.env.COLLECTOR_GOTO_TIMEOUT_MS ?? '45000');
  return Number.isFinite(n) && n > 0 ? n : 45000;
}

export function getCollectorDnsTimeoutMs(): number {
  const n = Number(process.env.COLLECTOR_DNS_TIMEOUT_MS ?? '3000');
  return Number.isFinite(n) && n >= 100 && n <= 30_000 ? n : 3000;
}

export function getCollectorRedirectLimit(): number {
  const n = Number(process.env.COLLECTOR_REDIRECT_LIMIT ?? '5');
  return Number.isSafeInteger(n) && n >= 0 && n <= 20 ? n : 5;
}

export function getCustomAllowedDomains(): string[] {
  return [...new Set(
    String(process.env.COLLECTOR_CUSTOM_ALLOWED_DOMAINS ?? '')
      .split(',')
      .map((value) => value.trim().toLowerCase().replace(/^\.+|\.+$/g, ''))
      .filter(Boolean),
  )];
}

export function getBrowserHeadless(): boolean {
  const v = process.env.COLLECTOR_HEADLESS;
  if (v === '0' || v === 'false') return false;
  return true;
}

export function getBrowserExecutablePath(): string | undefined {
  return process.env.COLLECTOR_BROWSER_EXECUTABLE_PATH?.trim() || undefined;
}

/** @deprecated 使用 getBrowserProfileRoot() */
export function getBrowserProfileBaseDir(): string {
  return getBrowserProfileRoot();
}

/** @deprecated 使用 getStorageStateRoot() */
export function getStorageStateBaseDir(): string {
  return getStorageStateRoot();
}
