import { computed, ref, watch } from 'vue'
import { opsAPI, type OpsRealtimeTrafficSummary } from '@/api/admin/ops'
import { useAdminSettingsStore } from '@/stores'

export type RealtimeWindow = '1min' | '5min' | '30min' | '1h'

export interface UseOpsRealtimeTrafficOptions {
  timeRange: () => string
  platform: () => string
  groupId: () => number | null
  autoRefreshEnabled: () => boolean | undefined
  autoRefreshCountdown: () => number | undefined
  loading: () => boolean
}

export function useOpsRealtimeTraffic(options: UseOpsRealtimeTrafficOptions) {
  const adminSettingsStore = useAdminSettingsStore()

  const realtimeWindow = ref<RealtimeWindow>('1min')

  const REALTIME_WINDOW_MINUTES: Record<RealtimeWindow, number> = {
    '1min': 1,
    '5min': 5,
    '30min': 30,
    '1h': 60
  }

  const TOOLBAR_RANGE_MINUTES: Record<string, number> = {
    '5m': 5,
    '30m': 30,
    '1h': 60,
    '6h': 6 * 60,
    '24h': 24 * 60
  }

  const availableRealtimeWindows = computed(() => {
    const toolbarMinutes = TOOLBAR_RANGE_MINUTES[options.timeRange()] ?? 60
    return (['1min', '5min', '30min', '1h'] as const).filter((w) => REALTIME_WINDOW_MINUTES[w] <= toolbarMinutes)
  })

  watch(
    () => options.timeRange(),
    () => {
      // The realtime window must be inside the toolbar window; reset to keep UX predictable.
      realtimeWindow.value = '1min'
      // Keep realtime traffic consistent with toolbar changes even when the window is already 1min.
      loadRealtimeTrafficSummary()
    }
  )

  const realtimeTrafficSummary = ref<OpsRealtimeTrafficSummary | null>(null)
  const realtimeTrafficLoading = ref(false)

  function makeZeroRealtimeTrafficSummary(): OpsRealtimeTrafficSummary {
    const now = new Date().toISOString()
    return {
      window: realtimeWindow.value,
      start_time: now,
      end_time: now,
      platform: options.platform(),
      group_id: options.groupId(),
      qps: { current: 0, peak: 0, avg: 0 },
      tps: { current: 0, peak: 0, avg: 0 }
    }
  }

  async function loadRealtimeTrafficSummary() {
    if (realtimeTrafficLoading.value) return
    if (!adminSettingsStore.opsRealtimeMonitoringEnabled) {
      realtimeTrafficSummary.value = makeZeroRealtimeTrafficSummary()
      return
    }
    realtimeTrafficLoading.value = true
    try {
      const res = await opsAPI.getRealtimeTrafficSummary(realtimeWindow.value, options.platform(), options.groupId())
      if (res && res.enabled === false) {
        adminSettingsStore.setOpsRealtimeMonitoringEnabledLocal(false)
      }
      realtimeTrafficSummary.value = res?.summary ?? null
    } catch (err) {
      console.error('[OpsDashboardHeader] Failed to load realtime traffic summary', err)
      realtimeTrafficSummary.value = null
    } finally {
      realtimeTrafficLoading.value = false
    }
  }

  watch(
    () => [realtimeWindow.value, options.platform(), options.groupId()] as const,
    () => {
      loadRealtimeTrafficSummary()
    },
    { immediate: true }
  )

  watch(
    () => adminSettingsStore.opsRealtimeMonitoringEnabled,
    (enabled) => {
      if (!enabled) {
        // Keep UI stable when realtime monitoring is turned off.
        realtimeTrafficSummary.value = makeZeroRealtimeTrafficSummary()
      } else {
        loadRealtimeTrafficSummary()
      }
    },
    { immediate: true }
  )

  // Realtime traffic refresh follows the parent (OpsDashboard) refresh cadence.
  watch(
    () => [options.autoRefreshEnabled(), options.autoRefreshCountdown(), options.loading()] as const,
    ([enabled, countdown, loading]) => {
      if (!enabled) return
      if (loading) return
      // Treat countdown reset (or reaching 0) as a refresh boundary.
      if (countdown === 0) {
        loadRealtimeTrafficSummary()
      }
    }
  )

  // no-op: parent controls refresh cadence

  const displayRealTimeQps = computed(() => {
    const v = realtimeTrafficSummary.value?.qps?.current
    return typeof v === 'number' && Number.isFinite(v) ? v : 0
  })

  const displayRealTimeTps = computed(() => {
    const v = realtimeTrafficSummary.value?.tps?.current
    return typeof v === 'number' && Number.isFinite(v) ? v : 0
  })

  const realtimeQpsPeakLabel = computed(() => {
    const v = realtimeTrafficSummary.value?.qps?.peak
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(1) : '-'
  })
  const realtimeTpsPeakLabel = computed(() => {
    const v = realtimeTrafficSummary.value?.tps?.peak
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(1) : '-'
  })
  const realtimeQpsAvgLabel = computed(() => {
    const v = realtimeTrafficSummary.value?.qps?.avg
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(1) : '-'
  })
  const realtimeTpsAvgLabel = computed(() => {
    const v = realtimeTrafficSummary.value?.tps?.avg
    return typeof v === 'number' && Number.isFinite(v) ? v.toFixed(1) : '-'
  })

  return {
    realtimeWindow,
    availableRealtimeWindows,
    realtimeTrafficSummary,
    loadRealtimeTrafficSummary,
    displayRealTimeQps,
    displayRealTimeTps,
    realtimeQpsPeakLabel,
    realtimeTpsPeakLabel,
    realtimeQpsAvgLabel,
    realtimeTpsAvgLabel
  }
}
