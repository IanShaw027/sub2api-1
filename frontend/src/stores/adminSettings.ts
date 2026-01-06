import { defineStore } from 'pinia'
import { ref } from 'vue'
import { adminAPI } from '@/api'

export const useAdminSettingsStore = defineStore('adminSettings', () => {
  const loaded = ref(false)
  const loading = ref(false)

  const readOpsMonitoringEnabledCache = (): boolean => {
    try {
      const raw = localStorage.getItem('ops_monitoring_enabled_cached')
      if (raw === 'true') return true
      if (raw === 'false') return false
    } catch {
      // ignore localStorage failures
    }
    // Default open (per product requirement).
    return true
  }

  const writeOpsMonitoringEnabledCache = (value: boolean) => {
    try {
      localStorage.setItem('ops_monitoring_enabled_cached', value ? 'true' : 'false')
    } catch {
      // ignore localStorage failures
    }
  }

  // Default open, but honor cached value to reduce UI flicker on first paint.
  const opsMonitoringEnabled = ref(readOpsMonitoringEnabledCache())

  const readOpsRealtimeEnabledCache = (): boolean => {
    try {
      const raw = localStorage.getItem('ops_realtime_monitoring_enabled_cached')
      if (raw === 'true') return true
      if (raw === 'false') return false
    } catch {
      // ignore localStorage failures
    }
    return true
  }

  const writeOpsRealtimeEnabledCache = (value: boolean) => {
    try {
      localStorage.setItem('ops_realtime_monitoring_enabled_cached', value ? 'true' : 'false')
    } catch {
      // ignore localStorage failures
    }
  }

  const opsRealtimeMonitoringEnabled = ref(readOpsRealtimeEnabledCache())

  async function fetch(force = false): Promise<void> {
    if (loaded.value && !force) return
    if (loading.value) return

    loading.value = true
    try {
      const settings = await adminAPI.settings.getSettings()
      opsMonitoringEnabled.value = settings.ops_monitoring_enabled ?? true
      writeOpsMonitoringEnabledCache(opsMonitoringEnabled.value)
      opsRealtimeMonitoringEnabled.value = settings.ops_realtime_monitoring_enabled ?? true
      writeOpsRealtimeEnabledCache(opsRealtimeMonitoringEnabled.value)
      loaded.value = true
    } catch (err) {
      // Keep cached/default value: do not "flip" the UI based on a transient fetch failure.
      loaded.value = true
      console.error('[adminSettings] Failed to fetch settings:', err)
    } finally {
      loading.value = false
    }
  }

  function setOpsMonitoringEnabledLocal(value: boolean) {
    opsMonitoringEnabled.value = value
    writeOpsMonitoringEnabledCache(value)
    loaded.value = true
  }

  function setOpsRealtimeMonitoringEnabledLocal(value: boolean) {
    opsRealtimeMonitoringEnabled.value = value
    writeOpsRealtimeEnabledCache(value)
    loaded.value = true
  }

  // Keep UI consistent if we learn that ops is disabled via feature-gated 404s.
  // (event is dispatched from the axios interceptor)
  try {
    window.addEventListener('ops-monitoring-disabled', () => {
      setOpsMonitoringEnabledLocal(false)
    })
  } catch {
    // ignore window access failures
  }

  return {
    loaded,
    loading,
    opsMonitoringEnabled,
    opsRealtimeMonitoringEnabled,
    fetch,
    setOpsMonitoringEnabledLocal,
    setOpsRealtimeMonitoringEnabledLocal
  }
})
