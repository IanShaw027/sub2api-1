import { paymentMethodI18nKey } from '@/views/user/paymentUx'

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

const GROUP_PLATFORM_FALLBACK_LABEL_MAP: Record<string, string> = {
  anthropic: 'Anthropic',
  openai: 'OpenAI',
  gemini: 'Gemini',
  antigravity: 'Antigravity',
  kiro: 'Kiro',
  sora: 'Sora',
}

const REDEEM_STATUS_KEY_MAP: Record<string, string> = {
  active: 'unused',
  unused: 'unused',
  used: 'used',
  expired: 'expired',
  disabled: 'disabled',
}

const PAYMENT_STATUS_KEY_MAP: Record<string, string> = {
  pending: 'pending',
  paid: 'paid',
  recharging: 'recharging',
  completed: 'completed',
  expired: 'expired',
  cancelled: 'cancelled',
  failed: 'failed',
  refunded: 'refunded',
  refund_requested: 'refund_requested',
  refundrequested: 'refund_requested',
  refunding: 'refunding',
  partially_refunded: 'partially_refunded',
  partiallyrefunded: 'partially_refunded',
  refund_failed: 'refund_failed',
  refundfailed: 'refund_failed',
}

const PAYMENT_ORDER_TYPE_KEY_MAP: Record<string, string> = {
  balance: 'balance',
  subscription: 'subscription',
}

export function accountStatusI18nKey(status: string): string {
  const normalized = ACCOUNT_STATUS_KEY_MAP[status] ?? ACCOUNT_STATUS_KEY_MAP[status.toLowerCase()] ?? status
  return `admin.accounts.status.${normalized}`
}

export function groupPlatformI18nKey(platform: string): string {
  const normalized = GROUP_PLATFORM_KEY_MAP[platform] ?? GROUP_PLATFORM_KEY_MAP[platform.toLowerCase()] ?? platform
  return `admin.groups.platforms.${normalized}`
}

export function groupPlatformFallbackLabel(platform: string): string {
  const normalized = GROUP_PLATFORM_KEY_MAP[platform] ?? GROUP_PLATFORM_KEY_MAP[platform.toLowerCase()] ?? platform
  return GROUP_PLATFORM_FALLBACK_LABEL_MAP[normalized] ?? platform
}

export function redeemStatusI18nKey(status: string): string {
  const normalized = REDEEM_STATUS_KEY_MAP[status] ?? REDEEM_STATUS_KEY_MAP[status.toLowerCase()] ?? status
  return `admin.redeem.status.${normalized}`
}

export function paymentMethodDisplayKey(paymentType: string): string {
  return paymentMethodI18nKey(paymentType)
}

export function paymentStatusI18nKey(status: string): string {
  const normalized = PAYMENT_STATUS_KEY_MAP[status] ?? PAYMENT_STATUS_KEY_MAP[status.toLowerCase()] ?? status.toLowerCase()
  return `payment.status.${normalized}`
}

export function paymentOrderTypeI18nKey(orderType: string): string {
  const normalized =
    PAYMENT_ORDER_TYPE_KEY_MAP[orderType] ??
    PAYMENT_ORDER_TYPE_KEY_MAP[orderType.toLowerCase()] ??
    orderType.toLowerCase()
  return `payment.admin.${normalized}Order`
}
