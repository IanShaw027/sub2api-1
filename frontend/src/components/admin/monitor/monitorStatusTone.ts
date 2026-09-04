import type { MonitorStatus } from '@/api/admin/channelMonitor'

/**
 * Tone vocabulary for monitor status dots/sparklines. Narrower than
 * StatusBadgeTone (drops 'accent') because a monitor check result is never
 * "accent" — it's operational/degraded/failed/error(unknown).
 *
 * Mirrors useChannelMonitorFormat().statusBadgeClass's semantics, expressed
 * as a tone instead of a badge class string, so StatusCell/StatusBadge
 * (dot + pulse) can be used instead of a hand-rolled pill span.
 */
export type MonitorTone = 'success' | 'warning' | 'danger' | 'muted'

export function monitorStatusTone(status: MonitorStatus | ''): MonitorTone {
  switch (status) {
    case 'operational':
      return 'success'
    case 'degraded':
      return 'warning'
    case 'failed':
      return 'danger'
    case 'error':
    default:
      return 'muted'
  }
}
