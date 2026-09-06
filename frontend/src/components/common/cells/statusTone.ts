import type { StatusBadgeTone } from '@/components/ui/types'

/**
 * Default status -> tone mapping shared by every list page. Pass an explicit
 * `tone` prop on `StatusCell` to override this guess for a status vocabulary
 * that doesn't fit the generic buckets below.
 */
const SUCCESS_STATUSES = new Set([
  'active', 'enabled', 'online', 'connected', 'success', 'completed', 'paid',
  'normal', 'healthy', 'valid', 'available', 'running'
])
const WARNING_STATUSES = new Set([
  'pending', 'warning', 'quota_exhausted', 'degraded', 'rate_limited',
  'expiring', 'processing', 'partial'
])
const DANGER_STATUSES = new Set([
  'disabled', 'inactive', 'error', 'errored', 'failed', 'failure', 'banned',
  'suspended', 'danger', 'rejected', 'expired', 'blocked', 'cancelled', 'canceled'
])

export function statusTone(status: string | null | undefined): StatusBadgeTone {
  const key = String(status ?? '').toLowerCase()
  if (SUCCESS_STATUSES.has(key)) return 'success'
  if (WARNING_STATUSES.has(key)) return 'warning'
  if (DANGER_STATUSES.has(key)) return 'danger'
  return 'muted'
}
