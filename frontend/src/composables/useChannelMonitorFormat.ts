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

/** Availability HSL hue multiplier: 0%=red(0) / 50%=yellow(60) / 100%=green(120). */
const HSL_HUE_PER_PERCENT = 1.2
const HSL_SATURATION = 72
const HSL_LIGHTNESS = 42

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
    switch (p) {
      case PROVIDER_OPENAI:
        return 'bg-emerald-500/15 text-emerald-600'
      case PROVIDER_ANTHROPIC:
        return 'bg-orange-500/15 text-orange-600'
      case PROVIDER_GEMINI:
        return 'bg-sky-500/15 text-sky-600'
      case PROVIDER_GROK:
        return 'bg-zinc-500/15 text-zinc-600'
      case PROVIDER_ANTIGRAVITY:
        return 'bg-purple-500/15 text-purple-600'
      case PROVIDER_KIMI:
        return 'bg-pink-500/15 text-pink-600'
      case PROVIDER_ZHIPU:
        return 'bg-indigo-500/15 text-indigo-600'
      case PROVIDER_DEEPSEEK:
        return 'bg-teal-500/15 text-teal-600'
      default:
        return NEUTRAL_BADGE
    }
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
   * Tailwind class for a provider radio-button-style picker (active/inactive state).
   * Reuses the same emerald/orange/sky palette as providerBadgeClass to keep
   * visual semantics consistent across badges and pickers.
   */
  function providerPickerClass(p: Provider | string, active: boolean): string {
    switch (p) {
      case PROVIDER_OPENAI:
        return active
          ? 'border-emerald-500 bg-emerald-500/15 text-emerald-700'
          : 'border-line bg-surface text-muted hover:border-emerald-300 hover:text-emerald-700'
      case PROVIDER_ANTHROPIC:
        return active
          ? 'border-orange-500 bg-orange-500/15 text-orange-700'
          : 'border-line bg-surface text-muted hover:border-orange-300 hover:text-orange-700'
      case PROVIDER_GEMINI:
        return active
          ? 'border-sky-500 bg-sky-500/15 text-sky-700'
          : 'border-line bg-surface text-muted hover:border-sky-300 hover:text-sky-700'
      case PROVIDER_GROK:
        return active
          ? 'border-zinc-500 bg-zinc-500/15 text-zinc-700'
          : 'border-line bg-surface text-muted hover:border-zinc-400 hover:text-zinc-700'
      case PROVIDER_ANTIGRAVITY:
        return active
          ? 'border-purple-500 bg-purple-500/15 text-purple-700'
          : 'border-line bg-surface text-muted hover:border-purple-300 hover:text-purple-700'
      case PROVIDER_KIMI:
        return active
          ? 'border-pink-500 bg-pink-500/15 text-pink-700'
          : 'border-line bg-surface text-muted hover:border-pink-300 hover:text-pink-700'
      case PROVIDER_ZHIPU:
        return active
          ? 'border-indigo-500 bg-indigo-500/15 text-indigo-700'
          : 'border-line bg-surface text-muted hover:border-indigo-300 hover:text-indigo-700'
      case PROVIDER_DEEPSEEK:
        return active
          ? 'border-teal-500 bg-teal-500/15 text-teal-700'
          : 'border-line bg-surface text-muted hover:border-teal-300 hover:text-teal-700'
      default:
        return active
          ? 'border-line bg-surface-2 text-foreground'
          : 'border-line bg-surface text-muted hover:border-line hover:text-foreground'
    }
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
 * Map availability percent to an HSL colour (red -> yellow -> green).
 * Returns undefined for null/NaN so callers can fall back to a neutral colour.
 */
export function hslForPct(pct: number | null | undefined): string | undefined {
  if (pct === null || pct === undefined || Number.isNaN(pct)) return undefined
  const clamped = Math.max(0, Math.min(100, pct))
  const hue = clamped * HSL_HUE_PER_PERCENT
  return `hsl(${hue} ${HSL_SATURATION}% ${HSL_LIGHTNESS}%)`
}

/**
 * Tailwind gradient class for the provider icon tile background.
 */
export function providerGradient(provider: string): string {
  switch (provider) {
    case PROVIDER_OPENAI:
      return 'bg-gradient-to-br from-emerald-500/10 to-emerald-500/20'
    case PROVIDER_ANTHROPIC:
      return 'bg-gradient-to-br from-orange-500/10 to-amber-500/20'
    case PROVIDER_GEMINI:
      return 'bg-gradient-to-br from-sky-500/10 to-indigo-500/20'
    case PROVIDER_GROK:
      return 'bg-gradient-to-br from-zinc-500/10 to-neutral-500/20'
    case PROVIDER_ANTIGRAVITY:
      return 'bg-gradient-to-br from-purple-500/10 to-purple-500/20'
    case PROVIDER_KIMI:
      return 'bg-gradient-to-br from-pink-500/10 to-pink-500/20'
    case PROVIDER_ZHIPU:
      return 'bg-gradient-to-br from-indigo-500/10 to-indigo-500/20'
    case PROVIDER_DEEPSEEK:
      return 'bg-gradient-to-br from-teal-500/10 to-teal-500/20'
    default:
      return 'bg-gradient-to-br from-[color-mix(in_oklch,var(--foreground)_6%,transparent)] to-[color-mix(in_oklch,var(--foreground)_10%,transparent)]'
  }
}
