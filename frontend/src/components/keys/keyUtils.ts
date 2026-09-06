import type { ApiKey } from '@/types'
import type { StatusBadgeTone } from '@/components/ui/types'

// Shared pure helpers for API key quota / rate-limit math, used by both
// KeysView.vue (table cells, row menu) and KeyFormModal.vue (edit form).

export const formatCost = (value: number | null | undefined, decimals = 2): string =>
  `$${(value ?? 0).toFixed(decimals)}`

export const quotaPercent = (key: ApiKey): number => {
  if (!key.quota || key.quota <= 0) return 0
  return Math.min(100, (key.quota_used / key.quota) * 100)
}

export const quotaBarClass = (key: ApiKey): string => {
  const pct = quotaPercent(key)
  if (pct >= 90) return 'progress-bar-danger'
  if (pct >= 70) return 'progress-bar-warning'
  return ''
}

export type RateLimitWindowKey = 'rate_limit_5h' | 'rate_limit_1d' | 'rate_limit_7d'

export interface RateLimitWindowDef {
  key: RateLimitWindowKey
  shortLabel: string
  limitField: RateLimitWindowKey
  usageField: 'usage_5h' | 'usage_1d' | 'usage_7d'
  resetField: 'reset_5h_at' | 'reset_1d_at' | 'reset_7d_at'
}

export const RATE_LIMIT_WINDOW_DEFS: RateLimitWindowDef[] = [
  { key: 'rate_limit_5h', shortLabel: '5h', limitField: 'rate_limit_5h', usageField: 'usage_5h', resetField: 'reset_5h_at' },
  { key: 'rate_limit_1d', shortLabel: '1d', limitField: 'rate_limit_1d', usageField: 'usage_1d', resetField: 'reset_1d_at' },
  { key: 'rate_limit_7d', shortLabel: '7d', limitField: 'rate_limit_7d', usageField: 'usage_7d', resetField: 'reset_7d_at' }
]

export const windowPercent = (key: ApiKey, window: RateLimitWindowDef): number => {
  const limit = key[window.limitField]
  if (!limit || limit <= 0) return 0
  return Math.min(100, (key[window.usageField] / limit) * 100)
}

export const windowToneClass = (key: ApiKey, window: RateLimitWindowDef): string => {
  const pct = windowPercent(key, window)
  if (pct >= 90) return 'keys-tone-danger'
  if (pct >= 70) return 'keys-tone-warning'
  return ''
}

export const hasRateLimit = (key: ApiKey): boolean =>
  key.usage_5h > 0 || key.usage_1d > 0 || key.usage_7d > 0

export const activeRateLimitWindows = (key: ApiKey): RateLimitWindowDef[] =>
  RATE_LIMIT_WINDOW_DEFS.filter((window) => (key[window.limitField] ?? 0) > 0)

export const mostConstrainedRateLimitWindow = (key: ApiKey): RateLimitWindowDef | null => {
  const windows = activeRateLimitWindows(key)
  if (windows.length === 0) return null
  return windows.reduce((worst, window) =>
    windowPercent(key, window) > windowPercent(key, worst) ? window : worst
  , windows[0])
}

export const statusTone = (status: ApiKey['status']): StatusBadgeTone => {
  if (status === 'active') return 'success'
  if (status === 'quota_exhausted') return 'warning'
  if (status === 'expired' || status === 'disabled' || status === 'inactive') return 'danger'
  return 'muted'
}

export const expiryToneClass = (value: string | null, now: Date): string =>
  value && new Date(value) < now ? 'keys-tone-danger' : ''

export const hasIpRestriction = (key: ApiKey): boolean =>
  (key.ip_whitelist?.length > 0) || (key.ip_blacklist?.length > 0)

export const rateLimitSummary = (key: ApiKey): string => {
  const window = mostConstrainedRateLimitWindow(key)
  if (!window) return ''
  return `${window.shortLabel} ${Math.round(windowPercent(key, window))}%`
}

export const rateLimitToneClass = (key: ApiKey): string => {
  const window = mostConstrainedRateLimitWindow(key)
  return window ? windowToneClass(key, window) : ''
}

// `formatReset` resolves a window's reset timestamp to a localized countdown string
// (kept as a callback since it depends on the caller's `now`/i18n `t`).
export const rateLimitDetail = (key: ApiKey, formatReset: (resetAt: string) => string): string => {
  const windows = activeRateLimitWindows(key)
  if (windows.length === 0) return ''
  return windows
    .map((window) => {
      const base = `${window.shortLabel}: ${formatCost(key[window.usageField])} / ${formatCost(key[window.limitField])}`
      const resetAt = key[window.resetField]
      const resetText = resetAt ? formatReset(resetAt) : ''
      return resetText ? `${base} · ${resetText}` : base
    })
    .join('\n')
}

// Formats a Date/ISO string for a `datetime-local` input's value attribute.
export const formatDateTimeLocal = (isoDate: string): string => {
  const date = new Date(isoDate)
  const pad = (n: number) => n.toString().padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}
