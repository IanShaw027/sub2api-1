/**
 * Shared formatting helpers for channel monitor views (admin + user).
 *
 * Centralises:
 *  - status / provider label + badge class lookups
 *  - latency / availability / percent number formatting
 *  - dashboard-style helpers (HSL for availability, provider gradient, relative time)
 *
 * i18n keys live under `monitorCommon.*` so admin and user views share the
 * same translation source.
 */

import { useI18n } from 'vue-i18n'
import type { CheckMode, MonitorStatus, Provider } from '@/api/admin/channelMonitor'
import {
  PROVIDER_OPENAI,
  PROVIDER_ANTHROPIC,
  PROVIDER_GEMINI,
  PROVIDER_GROK,
  PROVIDER_ANTIGRAVITY,
  PROVIDER_KIMI,
  PROVIDER_ZHIPU,
  PROVIDER_DEEPSEEK,
  PROVIDERS,
  STATUS_OPERATIONAL,
  STATUS_DEGRADED,
  STATUS_FAILED,
  STATUS_ERROR,
  CHECK_MODE_PROBE,
  CHECK_MODE_QUOTA,
  CHECK_MODE_QUOTA_PROBE,
} from '@/constants/channelMonitor'

const NEUTRAL_BADGE = 'badge-tone-muted'

/**
 * Per-provider identity colour, expressed purely as token / color-mix()
 * expressions (never literal hex) so provider badges stay theme- and
 * accent-reactive. Four providers map straight onto existing semantic
 * tokens; the rest are token-composed blends to keep 8 providers visually
 * distinct without introducing new literal brand colours.
 */
const PROVIDER_TOKEN: Partial<Record<Provider, string>> = {
  [PROVIDER_OPENAI]: 'var(--success)',
  [PROVIDER_ANTHROPIC]: 'var(--warning)',
  [PROVIDER_GEMINI]: 'var(--accent)',
  [PROVIDER_GROK]: 'var(--muted)',
  [PROVIDER_ANTIGRAVITY]: 'color-mix(in oklch, var(--accent) 55%, var(--danger) 45%)',
  [PROVIDER_KIMI]: 'color-mix(in oklch, var(--danger) 80%, var(--accent) 20%)',
  [PROVIDER_ZHIPU]: 'color-mix(in oklch, var(--accent) 70%, var(--danger) 30%)',
  [PROVIDER_DEEPSEEK]: 'color-mix(in oklch, var(--accent) 50%, var(--success) 50%)',
}

/** Tailwind arbitrary-value syntax needs `_` instead of spaces. */
function bracket(value: string): string {
  return value.replace(/\s+/g, '_')
}

export interface AvailabilityRow {
  primary_status: MonitorStatus | ''
  availability_7d: number | null | undefined
}

export function useChannelMonitorFormat() {
  const { t } = useI18n()

  function statusLabel(s: MonitorStatus | ''): string {
    if (!s) return t('monitorCommon.status.unknown')
    return t(`monitorCommon.status.${s}`)
  }

  function statusBadgeClass(s: MonitorStatus | ''): string {
    switch (s) {
      case STATUS_OPERATIONAL:
        return 'badge-tone-success'
      case STATUS_DEGRADED:
        return 'badge-tone-warning'
      case STATUS_FAILED:
        return 'badge-tone-danger'
      case STATUS_ERROR:
      default:
        return NEUTRAL_BADGE
    }
  }

  function providerLabel(p: Provider | string): string {
    if (PROVIDERS.includes(p as Provider)) {
      return t(`monitorCommon.providers.${p}`)
    }
    return p || '-'
  }

  function checkModeLabel(m: CheckMode | string): string {
    if (m === 'probe' || m === 'quota' || m === 'quota_probe') {
      return t(`monitorCommon.checkMode.${m}`)
    }
    return m || '-'
  }

  /**
   * Display label for a monitor's primary model. Pure-quota monitors carry the
   * literal placeholder "quota" (the probe target is an account, not a model),
   * which must not leak into the UI as a fake model name — render the
   * localized mode label instead. quota_probe keeps a real model name.
   */
  const QUOTA_MODEL_PLACEHOLDER = 'quota'

  function formatMonitorModel(model: string): string {
    if (model === QUOTA_MODEL_PLACEHOLDER) {
      return t('monitorCommon.checkMode.quota')
    }
    return model
  }

  function providerBadgeClass(p: Provider | string): string {
    const token = PROVIDER_TOKEN[p as Provider]
    if (!token) return NEUTRAL_BADGE
    const bg = bracket(`color-mix(in oklch, ${token} 15%, transparent)`)
    return `bg-[${bg}] text-[${bracket(token)}]`
  }

  /**
   * Tailwind class for the check-mode badge shown next to the provider badge
   * in the admin monitor list. Quota-bearing modes = accent (数据源是账号配额),
   * plain probe = neutral grey.
   */
  function checkModeBadgeClass(m: CheckMode | string): string {
    switch (m) {
      case CHECK_MODE_QUOTA:
      case CHECK_MODE_QUOTA_PROBE:
        return 'badge-tone-accent'
      case CHECK_MODE_PROBE:
      default:
        return NEUTRAL_BADGE
    }
  }

  /**
   * Class for a provider radio-button-style picker (active/inactive state).
   * Reuses the same token-composed palette as providerBadgeClass to keep
   * visual semantics consistent across badges and pickers.
   */
  function providerPickerClass(p: Provider | string, active: boolean): string {
    const token = PROVIDER_TOKEN[p as Provider]
    if (!token) {
      return active
        ? 'border-line bg-surface-2 text-foreground'
        : 'border-line bg-surface text-muted hover:border-line hover:text-foreground'
    }
    const tint = bracket(`color-mix(in oklch, ${token} 15%, transparent)`)
    const hoverTint = bracket(`color-mix(in oklch, ${token} 45%, var(--border) 55%)`)
    const tokenClass = bracket(token)
    return active
      ? `border-[${tokenClass}] bg-[${tint}] text-[${tokenClass}]`
      : `border-line bg-surface text-muted hover:border-[${hoverTint}] hover:text-[${tokenClass}]`
  }

  function formatLatency(ms: number | null | undefined): string {
    if (ms == null) return t('monitorCommon.latencyEmpty')
    return String(Math.round(ms))
  }

  function formatPercent(v: number | null | undefined): string {
    if (v == null || Number.isNaN(v)) return '-'
    return `${v.toFixed(2)}%`
  }

  function formatAvailability(row: AvailabilityRow): string {
    if (!row.primary_status) return '-'
    return formatPercent(row.availability_7d)
  }

  function formatRelativeTime(iso: string | null | undefined): string {
    if (!iso) return t('monitorCommon.latencyEmpty')
    const ts = Date.parse(iso)
    if (Number.isNaN(ts)) return t('monitorCommon.latencyEmpty')
    const diffSec = Math.max(0, Math.floor((Date.now() - ts) / 1000))
    if (diffSec < 60) return t('monitorCommon.relativeSecondsAgo', { n: diffSec })
    const diffMin = Math.floor(diffSec / 60)
    if (diffMin < 60) return t('monitorCommon.relativeMinutesAgo', { n: diffMin })
    const diffHour = Math.floor(diffMin / 60)
    if (diffHour < 24) return t('monitorCommon.relativeHoursAgo', { n: diffHour })
    const diffDay = Math.floor(diffHour / 24)
    return t('monitorCommon.relativeDaysAgo', { n: diffDay })
  }

  return {
    statusLabel,
    statusBadgeClass,
    providerLabel,
    checkModeLabel,
    formatMonitorModel,
    providerBadgeClass,
    checkModeBadgeClass,
    providerPickerClass,
    formatLatency,
    formatPercent,
    formatAvailability,
    formatRelativeTime,
  }
}

/**
 * Map availability percent to a token colour (danger -> warning -> success),
 * as a `color-mix()` expression so it stays theme/accent-reactive. Returns
 * undefined for null/NaN so callers can fall back to a neutral colour.
 */
export function tokenColorForPct(pct: number | null | undefined): string | undefined {
  if (pct === null || pct === undefined || Number.isNaN(pct)) return undefined
  const clamped = Math.max(0, Math.min(100, pct))
  if (clamped <= 50) {
    const warmth = clamped * 2
    return `color-mix(in oklch, var(--warning) ${warmth}%, var(--danger) ${100 - warmth}%)`
  }
  const cool = (clamped - 50) * 2
  return `color-mix(in oklch, var(--success) ${cool}%, var(--warning) ${100 - cool}%)`
}

/**
 * Gradient class for the provider icon tile background. Token-composed
 * (color-mix over the same PROVIDER_TOKEN palette as the badge/picker
 * helpers above) instead of a literal Tailwind palette gradient.
 */
export function providerGradient(provider: string): string {
  const token = PROVIDER_TOKEN[provider as Provider]
  if (!token) {
    return 'bg-gradient-to-br from-[color-mix(in_oklch,var(--foreground)_6%,transparent)] to-[color-mix(in_oklch,var(--foreground)_10%,transparent)]'
  }
  const from = bracket(`color-mix(in oklch, ${token} 10%, transparent)`)
  const to = bracket(`color-mix(in oklch, ${token} 20%, transparent)`)
  return `bg-gradient-to-br from-[${from}] to-[${to}]`
}
