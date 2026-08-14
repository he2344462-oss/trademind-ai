import { describe, expect, it } from 'vitest';
import type { Request } from 'playwright';

import {
  assertOutboundUrl,
  assertRequestAllowed,
  domainAllowed,
  isPublicIp,
  OutboundSecurityError,
  type DnsResolver,
  type OutboundPolicy,
} from '../outbound-policy.js';
import { allowedDomainsForProvider } from '../provider-policies.js';

const publicDns: DnsResolver = async () => [{ address: '93.184.216.34', family: 4 }];

function policy(allowedDomains: string[], dnsResolver: DnsResolver = publicDns): OutboundPolicy {
  return { provider: 'test', allowedDomains, dnsResolver, dnsTimeoutMs: 100, redirectLimit: 2 };
}

async function blocked(promise: Promise<unknown>, reason: string): Promise<void> {
  await expect(promise).rejects.toMatchObject({ reason });
}

describe('collector outbound security policy', () => {
  it('allows an explicitly permitted public domain and its subdomains', async () => {
    await expect(assertOutboundUrl('https://shop.example.com/item/1', policy(['example.com']))).resolves.toBeInstanceOf(URL);
    expect(domainAllowed('shop.example.com', ['example.com'])).toBe(true);
  });

  it('rejects an unauthorized public domain', async () => {
    await blocked(assertOutboundUrl('https://other.example/item/1', policy(['example.com'])), 'domain_not_allowed');
  });

  it.each([
    ['loopback IPv4', 'http://127.0.0.1/admin'],
    ['special integer IPv4', 'http://2130706433/admin'],
    ['private IPv4 10/8', 'http://10.0.0.1/admin'],
    ['private IPv4 172.16/12', 'http://172.20.0.1/admin'],
    ['private IPv4 192.168/16', 'http://192.168.1.1/admin'],
    ['IPv6 loopback', 'http://[::1]/admin'],
    ['private IPv6', 'http://[fd00::1]/admin'],
    ['IPv4 link-local', 'http://169.254.1.1/admin'],
    ['IPv6 link-local', 'http://[fe80::1]/admin'],
    ['metadata service', 'http://169.254.169.254/latest/meta-data'],
    ['unspecified IPv4', 'http://0.0.0.0/'],
    ['multicast IPv4', 'http://224.0.0.1/'],
  ])('rejects %s', async (_name, url) => {
    const host = new URL(url).hostname.replace(/^\[|\]$/g, '');
    await blocked(assertOutboundUrl(url, policy([host])), 'non_public_address');
  });

  it.each(['http://localhost/admin', 'http://host.docker.internal/admin', 'http://service.local/admin'])('rejects reserved hostname %s', async (url) => {
    const host = new URL(url).hostname;
    await blocked(assertOutboundUrl(url, policy([host])), 'reserved_hostname');
  });

  it('rejects a hostname whose DNS answer contains a private address', async () => {
    const resolver: DnsResolver = async () => [
      { address: '93.184.216.34', family: 4 },
      { address: '192.168.1.50', family: 4 },
    ];
    await blocked(assertOutboundUrl('https://shop.example.com/item', policy(['example.com'], resolver)), 'non_public_address');
  });

  it('rejects a redirect navigation from a public URL to a private target', async () => {
    const first = { redirectedFrom: () => null } as unknown as Request;
    const redirected = {
      isNavigationRequest: () => true,
      url: () => 'http://127.0.0.1/private',
      redirectedFrom: () => first,
    } as unknown as Request;
    await blocked(assertRequestAllowed(redirected, policy(['127.0.0.1'])), 'non_public_address');
  });

  it.each(['file:///etc/passwd', 'ftp://example.com/file', 'data:text/plain,hello'])('rejects unsupported protocol %s', async (url) => {
    await blocked(assertOutboundUrl(url, policy(['example.com'])), 'unsupported_protocol');
  });

  it('rejects custom provider navigation when no configured domain exists', async () => {
    const original = process.env.COLLECTOR_CUSTOM_ALLOWED_DOMAINS;
    delete process.env.COLLECTOR_CUSTOM_ALLOWED_DOMAINS;
    try {
      expect(allowedDomainsForProvider('custom')).toEqual([]);
      await blocked(assertOutboundUrl('https://example.com/item', { provider: 'custom', allowedDomains: [] }), 'domain_not_allowed');
    } finally {
      if (original === undefined) delete process.env.COLLECTOR_CUSTOM_ALLOWED_DOMAINS;
      else process.env.COLLECTOR_CUSTOM_ALLOWED_DOMAINS = original;
    }
  });

  it('allows only the exact additional login hosts required by the 1688 provider', () => {
    const domains = allowedDomainsForProvider('1688');
    expect(domainAllowed('login.taobao.com', domains)).toBe(true);
    expect(domainAllowed('main.m.tmall.com', domains)).toBe(true);
    expect(domainAllowed('passport.taobao.com', domains)).toBe(true);
    expect(domainAllowed('www.taobao.com', domains)).toBe(false);
    expect(domainAllowed('www.tmall.com', domains)).toBe(false);
  });

  it('rejects DNS resolution timeout', async () => {
    const resolver: DnsResolver = () => new Promise(() => undefined);
    await blocked(assertOutboundUrl('https://example.com/item', policy(['example.com'], resolver)), 'dns_timeout');
  });

  it('classifies only globally routable IP ranges as public', () => {
    expect(isPublicIp('8.8.8.8')).toBe(true);
    expect(isPublicIp('2606:4700:4700::1111')).toBe(true);
    expect(isPublicIp('100.64.0.1')).toBe(false);
    expect(isPublicIp('::ffff:127.0.0.1')).toBe(false);
    expect(isPublicIp('64:ff9b:1::1')).toBe(false);
    expect(isPublicIp('2001::1')).toBe(false);
    expect(isPublicIp('2002:7f00:1::1')).toBe(false);
  });
});
