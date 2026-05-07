import { normalizePaymentMethodForDisplay } from '@/views/user/paymentUx'

const ACCOUNT_STATUS_KEY_MAP: Record<string, string> = {
  active: 'active',
  inactive: 'inactive',
  error: 'error',
  cooldown: 'cooldown',
  paused: 'paused',
  limited: 'limited',
  rate_limited: 'rateLimited',
  ratelimited: 'rateLimited',
  rateLimited: 'rateLimited',
  overloaded: 'overloaded',
  temp_unschedulable: 'tempUnschedulable',
  tempunschedulable: 'tempUnschedulable',
  tempUnschedulable: 'tempUnschedulable',
  quota_exceeded: 'quotaExceeded',
  quotaexceeded: 'quotaExceeded',
  quotaExceeded: 'quotaExceeded',
  unschedulable: 'unschedulable',
}

const GROUP_PLATFORM_KEY_MAP: Record<string, string> = {
  anthropic: 'anthropic',
  claude: 'anthropic',
  openai: 'openai',
  gemini: 'gemini',
  antigravity: 'antigravity',
  kiro: 'kiro',
  sora: 'sora',
}

const REDEEM_STATUS_KEY_MAP: Record<string, string> = {
  active: 'unused',
  unused: 'unused',
  used: 'used',
  expired: 'expired',
  disabled: 'disabled',
}

export function accountStatusI18nKey(status: string): string {
  const normalized = ACCOUNT_STATUS_KEY_MAP[status] ?? ACCOUNT_STATUS_KEY_MAP[status.toLowerCase()] ?? status
  return `admin.accounts.status.${normalized}`
}

export function groupPlatformI18nKey(platform: string): string {
  const normalized = GROUP_PLATFORM_KEY_MAP[platform] ?? GROUP_PLATFORM_KEY_MAP[platform.toLowerCase()] ?? platform
  return `admin.groups.platforms.${normalized}`
}

export function redeemStatusI18nKey(status: string): string {
  const normalized = REDEEM_STATUS_KEY_MAP[status] ?? REDEEM_STATUS_KEY_MAP[status.toLowerCase()] ?? status
  return `admin.redeem.status.${normalized}`
}

export function paymentMethodDisplayKey(paymentType: string): string {
  return `payment.methods.${normalizePaymentMethodForDisplay(paymentType)}`
}

export function paymentStatusI18nKey(status: string): string {
  return `payment.status.${status.toLowerCase()}`
}
