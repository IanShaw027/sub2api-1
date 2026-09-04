import { ref, type ComponentPublicInstance } from 'vue'
import type { ApiKey } from '@/types'
import { formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

// Extracted out of KeysView.vue to keep the view under the
// frontend-health-cleanup 6.4 file-size warning threshold.
//
// Group cell (resting state): a single accent pill with the effective rate
// multiplier as a muted suffix, matching the prototype. Peak-rate/subscription
// detail that used to live in the stacked GroupBadge is preserved via the
// pill's tooltip so no information is lost — the full picker (opened on
// click) still shows the richer group list.
export function useGroupCellDisplay(
  userGroupRates: { value: Record<number, number> },
  getServerUtcOffset: () => string | undefined,
  t: (key: string) => string
) {
  const groupButtonRefs = ref<Map<number, HTMLElement>>(new Map())

  const setGroupButtonRef = (keyId: number, el: Element | ComponentPublicInstance | null) => {
    if (el instanceof HTMLElement) {
      groupButtonRefs.value.set(keyId, el)
    } else {
      groupButtonRefs.value.delete(keyId)
    }
  }

  const groupCellSuffix = (row: ApiKey): string => {
    const group = row.group
    if (!group) return ''
    if (group.subscription_type === 'subscription') {
      return t('groups.subscription')
    }
    const effectiveRate = userGroupRates.value[group.id] ?? group.rate_multiplier
    if (effectiveRate !== undefined && effectiveRate !== null && effectiveRate !== 1) {
      return `${effectiveRate}x`
    }
    return ''
  }

  const groupCellTooltip = (row: ApiKey): string => {
    const group = row.group
    if (!group) return t('keys.clickToChangeGroup')
    const parts = [group.name]
    const effectiveRate = userGroupRates.value[group.id] ?? group.rate_multiplier
    if (effectiveRate !== undefined && effectiveRate !== null) {
      parts.push(`${effectiveRate}x`)
    }
    if (group.peak_rate_enabled && group.peak_start && group.peak_end) {
      parts.push(
        formatPeakRateWindow(
          {
            peak_rate_enabled: group.peak_rate_enabled,
            peak_start: group.peak_start,
            peak_end: group.peak_end,
            peak_rate_multiplier: group.peak_rate_multiplier
          },
          serverTimezoneLabel(getServerUtcOffset())
        )
      )
    }
    parts.push(t('keys.clickToChangeGroup'))
    return parts.join(' · ')
  }

  return { groupButtonRefs, setGroupButtonRef, groupCellSuffix, groupCellTooltip }
}
