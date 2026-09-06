import { computed, ref } from 'vue'
import type { OpsDashboardOverview } from '@/api/admin/ops'

export interface UseOpsSystemHealthOptions {
  overview: () => OpsDashboardOverview | null
  t: (key: string) => string
}

export function useOpsSystemHealth(options: UseOpsSystemHealthOptions) {
  const { t } = options

  const systemMetrics = computed(() => options.overview()?.system_metrics ?? null)

  function formatTimeShort(ts?: string | null): string {
    if (!ts) return '-'
    const d = new Date(ts)
    if (Number.isNaN(d.getTime())) return '-'
    return d.toLocaleTimeString()
  }

  const cpuPercentValue = computed<number | null>(() => {
    const v = systemMetrics.value?.cpu_usage_percent
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const cpuPercentClass = computed(() => {
    const v = cpuPercentValue.value
    if (v == null) return 'text-foreground '
    if (v >= 95) return 'text-danger-text '
    if (v >= 80) return 'text-warning-text '
    return 'text-success-text '
  })

  const memPercentValue = computed<number | null>(() => {
    const v = systemMetrics.value?.memory_usage_percent
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const memPercentClass = computed(() => {
    const v = memPercentValue.value
    if (v == null) return 'text-foreground '
    if (v >= 95) return 'text-danger-text '
    if (v >= 85) return 'text-warning-text '
    return 'text-success-text '
  })

  const dbConnActiveValue = computed<number | null>(() => {
    const v = systemMetrics.value?.db_conn_active
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const dbConnIdleValue = computed<number | null>(() => {
    const v = systemMetrics.value?.db_conn_idle
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const dbConnWaitingValue = computed<number | null>(() => {
    const v = systemMetrics.value?.db_conn_waiting
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const dbConnOpenValue = computed<number | null>(() => {
    if (dbConnActiveValue.value == null || dbConnIdleValue.value == null) return null
    return dbConnActiveValue.value + dbConnIdleValue.value
  })

  const dbMaxOpenConnsValue = computed<number | null>(() => {
    const v = systemMetrics.value?.db_max_open_conns
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const dbUsagePercent = computed<number | null>(() => {
    if (dbConnOpenValue.value == null || dbMaxOpenConnsValue.value == null || dbMaxOpenConnsValue.value <= 0) return null
    return Math.min(100, Math.max(0, (dbConnOpenValue.value / dbMaxOpenConnsValue.value) * 100))
  })

  const dbMiddleLabel = computed(() => {
    if (systemMetrics.value?.db_ok === false) return 'FAIL'
    if (dbUsagePercent.value != null) return `${dbUsagePercent.value.toFixed(0)}%`
    if (systemMetrics.value?.db_ok === true) return t('admin.ops.ok')
    return t('admin.ops.noData')
  })

  const dbMiddleClass = computed(() => {
    if (systemMetrics.value?.db_ok === false) return 'text-danger-text '
    if (dbUsagePercent.value != null) {
      if (dbUsagePercent.value >= 90) return 'text-danger-text '
      if (dbUsagePercent.value >= 70) return 'text-warning-text '
      return 'text-success-text '
    }
    if (systemMetrics.value?.db_ok === true) return 'text-success-text '
    return 'text-foreground '
  })

  const redisConnTotalValue = computed<number | null>(() => {
    const v = systemMetrics.value?.redis_conn_total
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const redisConnIdleValue = computed<number | null>(() => {
    const v = systemMetrics.value?.redis_conn_idle
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const redisConnActiveValue = computed<number | null>(() => {
    if (redisConnTotalValue.value == null || redisConnIdleValue.value == null) return null
    return Math.max(redisConnTotalValue.value - redisConnIdleValue.value, 0)
  })

  const redisPoolSizeValue = computed<number | null>(() => {
    const v = systemMetrics.value?.redis_pool_size
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const redisUsagePercent = computed<number | null>(() => {
    if (redisConnTotalValue.value == null || redisPoolSizeValue.value == null || redisPoolSizeValue.value <= 0) return null
    return Math.min(100, Math.max(0, (redisConnTotalValue.value / redisPoolSizeValue.value) * 100))
  })

  const redisMiddleLabel = computed(() => {
    if (systemMetrics.value?.redis_ok === false) return 'FAIL'
    if (redisUsagePercent.value != null) return `${redisUsagePercent.value.toFixed(0)}%`
    if (systemMetrics.value?.redis_ok === true) return t('admin.ops.ok')
    return t('admin.ops.noData')
  })

  const redisMiddleClass = computed(() => {
    if (systemMetrics.value?.redis_ok === false) return 'text-danger-text '
    if (redisUsagePercent.value != null) {
      if (redisUsagePercent.value >= 90) return 'text-danger-text '
      if (redisUsagePercent.value >= 70) return 'text-warning-text '
      return 'text-success-text '
    }
    if (systemMetrics.value?.redis_ok === true) return 'text-success-text '
    return 'text-foreground '
  })

  const goroutineCountValue = computed<number | null>(() => {
    const v = systemMetrics.value?.goroutine_count
    return typeof v === 'number' && Number.isFinite(v) ? v : null
  })

  const goroutinesWarnThreshold = 8_000
  const goroutinesCriticalThreshold = 15_000

  const goroutineStatus = computed<'ok' | 'warning' | 'critical' | 'unknown'>(() => {
    const n = goroutineCountValue.value
    if (n == null) return 'unknown'
    if (n >= goroutinesCriticalThreshold) return 'critical'
    if (n >= goroutinesWarnThreshold) return 'warning'
    return 'ok'
  })

  const goroutineStatusLabel = computed(() => {
    switch (goroutineStatus.value) {
      case 'ok':
        return t('admin.ops.ok')
      case 'warning':
        return t('common.warning')
      case 'critical':
        return t('common.critical')
      default:
        return t('admin.ops.noData')
    }
  })

  const goroutineStatusClass = computed(() => {
    switch (goroutineStatus.value) {
      case 'ok':
        return 'text-success-text '
      case 'warning':
        return 'text-warning-text '
      case 'critical':
        return 'text-danger-text '
      default:
        return 'text-foreground '
    }
  })

  const jobHeartbeats = computed(() => options.overview()?.job_heartbeats ?? [])

  const jobsStatus = computed<'ok' | 'warn' | 'unknown'>(() => {
    const list = jobHeartbeats.value
    if (!list.length) return 'unknown'
    for (const hb of list) {
      if (!hb) continue
      if (hb.last_error_at && (!hb.last_success_at || hb.last_error_at > hb.last_success_at)) return 'warn'
    }
    return 'ok'
  })

  const jobsWarnCount = computed(() => {
    let warn = 0
    for (const hb of jobHeartbeats.value) {
      if (!hb) continue
      if (hb.last_error_at && (!hb.last_success_at || hb.last_error_at > hb.last_success_at)) warn++
    }
    return warn
  })

  const jobsStatusLabel = computed(() => {
    switch (jobsStatus.value) {
      case 'ok':
        return t('admin.ops.ok')
      case 'warn':
        return t('common.warning')
      default:
        return t('admin.ops.noData')
    }
  })

  const jobsStatusClass = computed(() => {
    switch (jobsStatus.value) {
      case 'ok':
        return 'text-success-text '
      case 'warn':
        return 'text-warning-text '
      default:
        return 'text-foreground '
    }
  })

  const showJobsDetails = ref(false)

  function openJobsDetails() {
    showJobsDetails.value = true
  }

  return {
    systemMetrics,
    formatTimeShort,
    cpuPercentValue,
    cpuPercentClass,
    memPercentValue,
    memPercentClass,
    dbConnActiveValue,
    dbConnIdleValue,
    dbConnWaitingValue,
    dbConnOpenValue,
    dbMaxOpenConnsValue,
    dbMiddleLabel,
    dbMiddleClass,
    redisConnTotalValue,
    redisConnIdleValue,
    redisConnActiveValue,
    redisPoolSizeValue,
    redisMiddleLabel,
    redisMiddleClass,
    goroutineCountValue,
    goroutinesWarnThreshold,
    goroutinesCriticalThreshold,
    goroutineStatusLabel,
    goroutineStatusClass,
    jobHeartbeats,
    jobsWarnCount,
    jobsStatusLabel,
    jobsStatusClass,
    showJobsDetails,
    openJobsDetails
  }
}
