import { getCustomAllowedDomains } from '../config/env.js';
import type { OutboundPolicy } from './outbound-policy.js';

export const PROVIDER_ALLOWED_DOMAINS: Readonly<Record<string, readonly string[]>> = {
  '1688': [
    '1688.com',
    'alibaba.com',
    'alicdn.com',
    'mmstat.com',
    'login.taobao.com',
    'main.m.tmall.com',
    'passport.taobao.com',
  ],
  aliexpress: ['aliexpress.com', 'aliexpress.us', 'alibaba.com', 'alicdn.com'],
  pinduoduo: ['yangkeduo.com', 'pinduoduo.com', 'qq.com', 'qpic.cn'],
  taobao_tmall: ['taobao.com', 'tmall.com', 'alibaba.com', 'alicdn.com', 'mmstat.com'],
  shein_temu: ['shein.com', 'temu.com'],
};

export function allowedDomainsForProvider(provider: string): readonly string[] {
  const key = provider.trim().toLowerCase();
  return key === 'custom' ? getCustomAllowedDomains() : (PROVIDER_ALLOWED_DOMAINS[key] ?? []);
}

export function outboundPolicyForProvider(provider: string): OutboundPolicy {
  return { provider, allowedDomains: allowedDomainsForProvider(provider) };
}
