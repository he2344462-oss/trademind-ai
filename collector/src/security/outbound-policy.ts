import { isIP } from 'node:net';
import { lookup as dnsLookup } from 'node:dns/promises';

import type { BrowserContext, Page, Request, Route } from 'playwright';
import { getCollectorDnsTimeoutMs, getCollectorRedirectLimit } from '../config/env.js';

export type DnsAddress = { address: string; family: number };
export type DnsResolver = (hostname: string) => Promise<DnsAddress[]>;

export type OutboundPolicy = {
  provider: string;
  allowedDomains: readonly string[];
  dnsResolver?: DnsResolver;
  dnsTimeoutMs?: number;
  redirectLimit?: number;
};

export class OutboundSecurityError extends Error {
  constructor(readonly reason: string, readonly hostname = '') {
    super(`OUTBOUND_SECURITY_BLOCKED:${reason}${hostname ? `:${hostname}` : ''}`);
    this.name = 'OutboundSecurityError';
  }
}

function normalizedDomain(value: string): string {
  return value.trim().toLowerCase().replace(/^\.+|\.+$/g, '');
}

export function domainAllowed(hostname: string, allowedDomains: readonly string[]): boolean {
  const host = normalizedDomain(hostname);
  return allowedDomains.some((entry) => {
    const domain = normalizedDomain(entry);
    return Boolean(domain) && (host === domain || host.endsWith(`.${domain}`));
  });
}

function parseIpv4(address: string): number[] | null {
  if (isIP(address) !== 4) return null;
  const parts = address.split('.').map(Number);
  return parts.length === 4 && parts.every((part) => Number.isInteger(part) && part >= 0 && part <= 255)
    ? parts
    : null;
}

function expandIpv6(address: string): number[] | null {
  const zoneFree = address.split('%')[0]?.toLowerCase() ?? '';
  if (isIP(zoneFree) !== 6) return null;
  let source = zoneFree;
  const ipv4Tail = source.match(/(\d+\.\d+\.\d+\.\d+)$/)?.[1];
  if (ipv4Tail) {
    const bytes = parseIpv4(ipv4Tail);
    if (!bytes) return null;
    source = source.slice(0, -ipv4Tail.length) + `${((bytes[0] << 8) | bytes[1]).toString(16)}:${((bytes[2] << 8) | bytes[3]).toString(16)}`;
  }
  const halves = source.split('::');
  if (halves.length > 2) return null;
  const left = halves[0] ? halves[0].split(':').filter(Boolean) : [];
  const right = halves[1] ? halves[1].split(':').filter(Boolean) : [];
  const zeros = halves.length === 2 ? 8 - left.length - right.length : 0;
  const words = [...left, ...Array(Math.max(zeros, 0)).fill('0'), ...right].map((word) => Number.parseInt(word, 16));
  return words.length === 8 && words.every((word) => Number.isInteger(word) && word >= 0 && word <= 0xffff)
    ? words
    : null;
}

export function isPublicIp(address: string): boolean {
  const v4 = parseIpv4(address);
  if (v4) {
    const [a, b, c] = v4;
    if (a === 0 || a === 10 || a === 127) return false;
    if (a === 100 && b >= 64 && b <= 127) return false;
    if (a === 169 && b === 254) return false;
    if (a === 172 && b >= 16 && b <= 31) return false;
    if (a === 192 && b === 168) return false;
    if (a === 192 && b === 0 && c === 0) return false;
    if (a === 192 && b === 0 && c === 2) return false;
    if (a === 192 && b === 31 && c === 196) return false;
    if (a === 192 && b === 52 && c === 193) return false;
    if (a === 192 && b === 88 && c === 99) return false;
    if (a === 192 && b === 175 && c === 48) return false;
    if (a === 198 && (b === 18 || b === 19)) return false;
    if (a === 198 && b === 51 && c === 100) return false;
    if (a === 203 && b === 0 && c === 113) return false;
    if (a >= 224) return false;
    return true;
  }

  const v6 = expandIpv6(address);
  if (!v6) return false;
  const [first, second, third, fourth, fifth, sixth] = v6;
  if (v6.every((word) => word === 0) || v6.slice(0, 7).every((word) => word === 0) && v6[7] === 1) return false;
  if ((first & 0xfe00) === 0xfc00) return false;
  if ((first & 0xffc0) === 0xfe80) return false;
  if ((first & 0xff00) === 0xff00) return false;
  if (first === 0x0064 && second === 0xff9b) return false;
  if (first === 0x0100 && second === 0 && third === 0 && fourth === 0) return false;
  if (first === 0x2001 && second <= 0x01ff) return false;
  if (first === 0x2001 && second === 0x0db8) return false;
  if (first === 0x2001 && second === 0x0002) return false;
  if (first === 0x2002) return false;
  if (first === 0x3fff && (second & 0xf000) === 0) return false;
  if (first === 0x5f00) return false;
  if (first === 0 && second === 0 && third === 0 && fourth === 0 && fifth === 0 && sixth === 0xffff) {
    const mapped = `${v6[6] >> 8}.${v6[6] & 0xff}.${v6[7] >> 8}.${v6[7] & 0xff}`;
    return isPublicIp(mapped);
  }
  return (first & 0xe000) === 0x2000;
}

async function defaultDnsResolver(hostname: string): Promise<DnsAddress[]> {
  const records = await dnsLookup(hostname, { all: true, verbatim: true });
  return records.map((record) => ({ address: record.address, family: record.family }));
}

async function resolveWithTimeout(hostname: string, policy: OutboundPolicy): Promise<DnsAddress[]> {
  const timeoutMs = policy.dnsTimeoutMs ?? getCollectorDnsTimeoutMs();
  const resolver = policy.dnsResolver ?? defaultDnsResolver;
  let timer: NodeJS.Timeout | undefined;
  try {
    return await Promise.race([
      resolver(hostname),
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new OutboundSecurityError('dns_timeout', hostname)), timeoutMs);
      }),
    ]);
  } finally {
    if (timer) clearTimeout(timer);
  }
}

export async function assertOutboundUrl(rawUrl: string, policy: OutboundPolicy): Promise<URL> {
  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    throw new OutboundSecurityError('invalid_url');
  }
  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    throw new OutboundSecurityError('unsupported_protocol');
  }
  if (url.username || url.password) {
    throw new OutboundSecurityError('url_credentials_forbidden');
  }
  const hostname = url.hostname.toLowerCase().replace(/^\[|\]$/g, '');
  if (
    hostname === 'localhost' ||
    hostname.endsWith('.localhost') ||
    hostname === 'host.docker.internal' ||
    hostname === 'gateway.docker.internal' ||
    hostname.endsWith('.local') ||
    hostname.endsWith('.internal')
  ) {
    throw new OutboundSecurityError('reserved_hostname', hostname);
  }
  if (!domainAllowed(hostname, policy.allowedDomains)) {
    throw new OutboundSecurityError('domain_not_allowed', hostname);
  }

  const literalFamily = isIP(hostname);
  const addresses = literalFamily
    ? [{ address: hostname, family: literalFamily }]
    : await resolveWithTimeout(hostname, policy).catch((error: unknown) => {
        if (error instanceof OutboundSecurityError) throw error;
        throw new OutboundSecurityError('dns_resolution_failed', hostname);
      });
  if (addresses.length === 0) throw new OutboundSecurityError('dns_no_addresses', hostname);
  if (addresses.some(({ address }) => !isPublicIp(address))) {
    throw new OutboundSecurityError('non_public_address', hostname);
  }
  return url;
}

function redirectCount(request: Request): number {
  let count = 0;
  let previous = request.redirectedFrom();
  while (previous) {
    count += 1;
    previous = previous.redirectedFrom();
  }
  return count;
}

export async function assertRequestAllowed(request: Request, policy: OutboundPolicy): Promise<void> {
  if (request.isNavigationRequest()) {
    const limit = policy.redirectLimit ?? getCollectorRedirectLimit();
    if (redirectCount(request) > limit) throw new OutboundSecurityError('redirect_limit_exceeded');
  }
  await assertOutboundUrl(request.url(), policy);
}

async function guardRoute(route: Route, policy: OutboundPolicy): Promise<void> {
  const request = route.request();
  try {
    await assertRequestAllowed(request, policy);
    await route.continue();
  } catch (error) {
    const reason = error instanceof OutboundSecurityError ? error.reason : 'policy_error';
    console.warn(`[collector-security] navigation blocked provider=${policy.provider} reason=${reason}`);
    await route.abort('blockedbyclient');
  }
}

export async function installPageOutboundPolicy(page: Page, policy: OutboundPolicy): Promise<void> {
  await page.route('**/*', (route) => guardRoute(route, policy));
}

export async function installContextOutboundPolicy(context: BrowserContext, policy: OutboundPolicy): Promise<void> {
  await context.route('**/*', (route) => guardRoute(route, policy));
}

export async function secureGoto(
  page: Page,
  rawUrl: string,
  policy: OutboundPolicy,
  options: Parameters<Page['goto']>[1],
): ReturnType<Page['goto']> {
  await assertOutboundUrl(rawUrl, policy);
  const response = await page.goto(rawUrl, options);
  const finalUrl = page.url();
  if (finalUrl && finalUrl !== 'about:blank') await assertOutboundUrl(finalUrl, policy);
  return response;
}
