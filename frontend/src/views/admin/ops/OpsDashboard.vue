<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import {
  Chart as ChartJS,
  Title,
  Tooltip,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  CategoryScale,
  BarElement,
  ArcElement,
  Filler
} from 'chart.js'
import zoomPlugin from 'chartjs-plugin-zoom'
import { useDebounceFn, useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ErrorDetailModal from '@/components/admin/ErrorDetailModal.vue'
import OpsDashboardHeader from './components/OpsDashboardHeader.vue'
import OpsMetricsCharts from './components/OpsMetricsCharts.vue'
import OpsGroupAvailabilityCard from './components/OpsGroupAvailabilityCard.vue'
import OpsRuntimeSettingsCard from './components/OpsRuntimeSettingsCard.vue'
import OpsEmailNotificationCard from './components/OpsEmailNotificationCard.vue'
import OpsAlertEventsCard from './components/OpsAlertEventsCard.vue'
import OpsErrorLogTable from './components/OpsErrorLogTable.vue'
import OpsDashboardSkeleton from './components/OpsDashboardSkeleton.vue'
import OpsRequestDetailsModal, { type OpsRequestDetailsPreset } from './components/OpsRequestDetailsModal.vue'
import { opsAPI, type OpsDashboardOverview, type ProviderHealthData, type LatencyHistogramResponse, type ErrorDistributionResponse, type OpsMetrics, type OpsErrorLog, type OpsWSStatus } from '@/api/admin/ops'
import { parseTimeRangeMinutes } from './utils/opsFormatters'
import type { ErrorFilters, ErrorLogsPagination } from './types'

ChartJS.register(
  Title,
  Tooltip,
  Legend,
  LineElement,
  LinearScale,
  PointElement,
  CategoryScale,
  BarElement,
  ArcElement,
  Filler,
  zoomPlugin
)

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
// Start in loading state so the first paint shows the skeleton immediately.
// Otherwise users may briefly see an "empty" dashboard before `onMounted()` kicks off the first fetch.
const loading = ref(true)
const errorMessage = ref('')
const hasLoadedOnce = ref(false)
const timeRange = ref('1h')
const lastUpdated = ref(new Date())

const overview = ref<OpsDashboardOverview | null>(null)
const providers = ref<ProviderHealthData[]>([])
const latencyData = ref<LatencyHistogramResponse | null>(null)
const errorDistribution = ref<ErrorDistributionResponse | null>(null)
const latestMetrics = ref<OpsMetrics | null>(null)
const metricsHistory = ref<OpsMetrics[]>([])

// Error logs section
// Same rationale as `loading`: avoid a first-paint flash of "empty table" before the first query starts.
const errorLogsLoading = ref(true)
const errorLogs = ref<OpsErrorLog[]>([])
const errorLogsTotal = ref(0)
const errorPagination = ref<ErrorLogsPagination>({
  page: 1,
  pageSize: 50
})

// Error detail modal
const showErrorDetail = ref(false)
const selectedErrorId = ref<number | null>(null)

function openErrorDetail(errorLog: OpsErrorLog) {
  selectedErrorId.value = errorLog.id
  showErrorDetail.value = true
}

function openErrorDetailById(id: number) {
  selectedErrorId.value = id
  showErrorDetail.value = true
}

function closeErrorDetail() {
  showErrorDetail.value = false
  // Delay clearing selectedErrorId to allow animation to complete
  setTimeout(() => {
    selectedErrorId.value = null
  }, 300)
}

// Request details modal (metric drill-down)
const showRequestDetails = ref(false)
const requestDetailsPreset = ref<OpsRequestDetailsPreset>({
  title: '',
  kind: 'all',
  sort: 'created_at_desc'
})

function openRequestDetails(preset: OpsRequestDetailsPreset) {
  requestDetailsPreset.value = preset
  showRequestDetails.value = true
}

const errorFilters = ref<ErrorFilters>({
  platforms: [],
  groupId: null,
  statusCodes: [],
  clientIp: '',
  severity: '',
  searchText: ''
})

// --- URL Query Persistence (Error log filters) ---
// Keeps Ops troubleshooting links shareable and restores state on refresh/back.
const QUERY_KEYS = {
  timeRange: 'tr',
  page: 'page',
  pageSize: 'pageSize',
  platforms: 'platforms',
  groupId: 'group_id',
  statusCodes: 'status_codes',
  severity: 'severity',
  clientIp: 'client_ip',
  search: 'q'
} as const

const allowedTimeRanges = new Set(['5m', '30m', '1h', '6h', '24h'])
const allowedSeverities = new Set(['P0', 'P1', 'P2', 'P3'])

const isApplyingRouteQuery = ref(false)
const isSyncingRouteQuery = ref(false)

const readQueryString = (key: string): string => {
  const value = route.query[key]
  if (typeof value === 'string') return value
  if (Array.isArray(value) && typeof value[0] === 'string') return value[0]
  return ''
}

const readQueryNumber = (key: string): number | null => {
  const raw = readQueryString(key)
  if (!raw) return null
  const n = Number.parseInt(raw, 10)
  return Number.isFinite(n) ? n : null
}

const splitCsv = (value: string): string[] => value.split(',').map(s => s.trim()).filter(Boolean)

const applyRouteQueryToState = () => {
  const nextTimeRange = readQueryString(QUERY_KEYS.timeRange)
  if (nextTimeRange && allowedTimeRanges.has(nextTimeRange)) {
    timeRange.value = nextTimeRange
  }

  const nextPage = readQueryNumber(QUERY_KEYS.page)
  if (nextPage && nextPage > 0) {
    errorPagination.value.page = nextPage
  }

  const nextPageSize = readQueryNumber(QUERY_KEYS.pageSize)
  if (nextPageSize && nextPageSize > 0) {
    errorPagination.value.pageSize = nextPageSize
  }

  const platformsCsv = readQueryString(QUERY_KEYS.platforms)
  const groupIdRaw = readQueryNumber(QUERY_KEYS.groupId)
  const statusCodesCsv = readQueryString(QUERY_KEYS.statusCodes)
  const severityRaw = readQueryString(QUERY_KEYS.severity)
  const clientIp = readQueryString(QUERY_KEYS.clientIp)
  const searchText = readQueryString(QUERY_KEYS.search)

  const platforms = platformsCsv ? splitCsv(platformsCsv) : []
  const statusCodes = statusCodesCsv
    ? splitCsv(statusCodesCsv).map(v => Number.parseInt(v, 10)).filter(n => Number.isFinite(n))
    : []
  const severity = allowedSeverities.has(severityRaw) ? (severityRaw as ErrorFilters['severity']) : ''

  const groupId = typeof groupIdRaw === 'number' && groupIdRaw > 0 ? groupIdRaw : null

  errorFilters.value = {
    platforms,
    groupId,
    statusCodes,
    clientIp,
    severity,
    searchText
  }
}

// Apply once during setup, before watchers register, so initial fetch uses query state.
applyRouteQueryToState()

const buildQueryFromState = () => {
  const next: Record<string, any> = { ...route.query }

  // Remove keys we own (so defaults don't linger).
  Object.values(QUERY_KEYS).forEach((k) => {
    delete next[k]
  })

  if (timeRange.value !== '1h') next[QUERY_KEYS.timeRange] = timeRange.value
  if (errorPagination.value.page !== 1) next[QUERY_KEYS.page] = String(errorPagination.value.page)
  if (errorPagination.value.pageSize !== 50) next[QUERY_KEYS.pageSize] = String(errorPagination.value.pageSize)

  if (errorFilters.value.platforms.length > 0) next[QUERY_KEYS.platforms] = errorFilters.value.platforms.join(',')
  if (typeof errorFilters.value.groupId === 'number' && errorFilters.value.groupId > 0) next[QUERY_KEYS.groupId] = String(errorFilters.value.groupId)
  if (errorFilters.value.statusCodes.length > 0) next[QUERY_KEYS.statusCodes] = errorFilters.value.statusCodes.join(',')
  if (errorFilters.value.severity) next[QUERY_KEYS.severity] = errorFilters.value.severity
  if (errorFilters.value.clientIp) next[QUERY_KEYS.clientIp] = errorFilters.value.clientIp
  if (errorFilters.value.searchText) next[QUERY_KEYS.search] = errorFilters.value.searchText

  return next
}

const syncQueryToRoute = useDebounceFn(async () => {
  if (isApplyingRouteQuery.value) return
  const nextQuery = buildQueryFromState()

  // Avoid spamming router updates.
  const curr = route.query
  const nextKeys = Object.keys(nextQuery)
  const currKeys = Object.keys(curr)
  const sameLength = nextKeys.length === currKeys.length
  const sameValues = sameLength && nextKeys.every(k => String((curr as any)[k] ?? '') === String(nextQuery[k] ?? ''))
  if (sameValues) return

  try {
    isSyncingRouteQuery.value = true
    await router.replace({ query: nextQuery })
  } finally {
    isSyncingRouteQuery.value = false
  }
}, 250)

// WebSocket for real-time QPS
const realTimeQPS = ref(0)
const realTimeTPS = ref(0)
const wsStatus = ref<OpsWSStatus>('connecting')
const wsReconnectInMs = ref<number | null>(null)
let unsubscribeQPS: (() => void) | null = null

const fetchData = async () => {
  loading.value = true
  errorMessage.value = ''
  try {
    const [ov, pr, lt, er] = await Promise.all([
      opsAPI.getDashboardOverview(timeRange.value),
      opsAPI.getProviderHealth(timeRange.value),
      opsAPI.getLatencyHistogram(timeRange.value),
      opsAPI.getErrorDistribution(timeRange.value)
    ])
    overview.value = ov
    providers.value = pr.providers
    latencyData.value = lt
    errorDistribution.value = er
    lastUpdated.value = new Date()

    try {
      const minutes = parseTimeRangeMinutes(timeRange.value)
      const historyLimit = Math.min(Math.max(minutes + 5, 10), 24 * 60 + 10)
      const [m, history] = await Promise.all([
        opsAPI.getMetrics(),
        opsAPI.listMetricsHistory({ window_minutes: 1, minutes, limit: historyLimit })
      ])
      latestMetrics.value = m
      metricsHistory.value = history.items
    } catch (e) {
      console.warn('[OpsDashboard] Failed to fetch system metrics', e)
    }
  } catch (err) {
    console.error('Failed to fetch ops data', err)
    errorMessage.value = t('admin.ops.failedToLoad')
  } finally {
    loading.value = false
    hasLoadedOnce.value = true
  }
}

// Fetch error logs
const fetchErrors = async () => {
  errorLogsLoading.value = true
  try {
    const params: any = {
      page: errorPagination.value.page,
      page_size: errorPagination.value.pageSize
    }

    // Apply time range filter
    const minutes = parseTimeRangeMinutes(timeRange.value)
    const endTime = new Date()
    const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)
    params.start_time = startTime.toISOString()
    params.end_time = endTime.toISOString()

    // Apply filters
    if (errorFilters.value.platforms.length > 0) {
      params.platforms = errorFilters.value.platforms.join(',')
    }
    if (typeof errorFilters.value.groupId === 'number' && errorFilters.value.groupId > 0) {
      params.group_id = errorFilters.value.groupId
    }
    if (errorFilters.value.severity) {
      params.severity = errorFilters.value.severity
    }
    if (errorFilters.value.statusCodes.length > 0) {
      params.status_codes = errorFilters.value.statusCodes.join(',')
    }
    if (errorFilters.value.clientIp) {
      params.client_ip = errorFilters.value.clientIp
    }
    if (errorFilters.value.searchText) {
      params.q = errorFilters.value.searchText
    }

    const response = await opsAPI.listErrorLogs(params)

    errorLogs.value = response.items
    errorLogsTotal.value = response.total ?? response.items.length
  } catch (err) {
    console.error('Failed to fetch error logs', err)
  } finally {
    errorLogsLoading.value = false
  }
}

const fetchErrorsDebounced = useDebounceFn(fetchErrors, 400)

// Refresh data every 30 seconds (fallback for L2/L3)
useIntervalFn(fetchData, 30000)

onMounted(() => {
  fetchData()
  fetchErrors()
  unsubscribeQPS = opsAPI.subscribeQPS(
    (payload) => {
      if (payload && typeof payload === 'object' && payload.type === 'qps_update' && payload.data) {
        realTimeQPS.value = payload.data.qps || 0
        realTimeTPS.value = payload.data.tps || 0
      }
    },
    {
      onStatusChange: (status) => {
        wsStatus.value = status
        if (status === 'connected') wsReconnectInMs.value = null
      },
      onReconnectScheduled: ({ delayMs }) => {
        wsReconnectInMs.value = delayMs
      },
      // QPS updates may be sparse in idle periods; keep the timeout conservative.
      staleTimeoutMs: 180_000
    }
  )
})

onUnmounted(() => {
  wsStatus.value = 'closed'
  if (unsubscribeQPS) unsubscribeQPS()
  unsubscribeQPS = null
})

watch(timeRange, () => {
  errorPagination.value.page = 1
  fetchData()
  fetchErrors()
})

watch(
  () => route.query,
  () => {
    if (isSyncingRouteQuery.value) return

    const prevTimeRange = timeRange.value
    const prevPage = errorPagination.value.page
    const prevPageSize = errorPagination.value.pageSize
    const prevFilters = errorFilters.value

    isApplyingRouteQuery.value = true
    applyRouteQueryToState()
    isApplyingRouteQuery.value = false

    const timeRangeChanged = prevTimeRange !== timeRange.value
    const pageChanged = prevPage !== errorPagination.value.page || prevPageSize !== errorPagination.value.pageSize
    const filtersChanged = JSON.stringify(prevFilters) !== JSON.stringify(errorFilters.value)

    if (timeRangeChanged) {
      fetchData()
      fetchErrors()
      return
    }

    if (pageChanged || filtersChanged) {
      fetchErrors()
    }
  }
)

function handleErrorFiltersUpdate(nextFilters: ErrorFilters) {
  const prev = errorFilters.value

  const sameNonSearch =
    prev.clientIp === nextFilters.clientIp &&
    prev.severity === nextFilters.severity &&
    prev.groupId === nextFilters.groupId &&
    prev.platforms.join(',') === nextFilters.platforms.join(',') &&
    prev.statusCodes.join(',') === nextFilters.statusCodes.join(',')

  const searchOnlyChanged = sameNonSearch && prev.searchText !== nextFilters.searchText

  errorFilters.value = nextFilters
  errorPagination.value.page = 1

  if (searchOnlyChanged) {
    fetchErrorsDebounced()
  } else {
    fetchErrors()
  }

  syncQueryToRoute()
}

function handleErrorPageChange(page: number) {
  errorPagination.value.page = page
  fetchErrors()
  syncQueryToRoute()
}

function handleErrorPageSizeChange(pageSize: number) {
  errorPagination.value.pageSize = pageSize
  errorPagination.value.page = 1
  fetchErrors()
  syncQueryToRoute()
}

watch(timeRange, () => {
  syncQueryToRoute()
})
</script>

<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- Error Message -->
      <div v-if="errorMessage" class="rounded-2xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
        {{ errorMessage }}
      </div>

      <!-- First-load skeleton -->
      <OpsDashboardSkeleton v-if="loading && !hasLoadedOnce" />

      <!-- L1: Header & Core Metrics -->
  <OpsDashboardHeader
        v-else
        :overview="overview"
        :wsStatus="wsStatus"
        :wsReconnectInMs="wsReconnectInMs"
        :realTimeQPS="realTimeQPS"
        :realTimeTPS="realTimeTPS"
        :platform="errorFilters.platforms.length === 1 ? errorFilters.platforms[0] : ''"
        :group-id="errorFilters.groupId"
        :timeRange="timeRange"
        :loading="loading"
        :lastUpdated="lastUpdated"
        @update:timeRange="timeRange = $event"
        @refresh="fetchData"
        @update:platform="handleErrorFiltersUpdate({ ...errorFilters, platforms: $event ? [$event] : [] })"
        @update:group="handleErrorFiltersUpdate({ ...errorFilters, groupId: $event })"
        @openRequestDetails="openRequestDetails"
      />

      <!-- L2: Visual Analysis -->
      <OpsMetricsCharts
        v-if="!(loading && !hasLoadedOnce)"
        :hasLoadedOnce="hasLoadedOnce"
        :loading="loading"
        :timeRange="timeRange"
        :providers="providers"
        :latencyData="latencyData"
        :errorDistribution="errorDistribution"
        :latestMetrics="latestMetrics"
        :metricsHistory="metricsHistory"
        :overview="overview"
      />

      <!-- Group Availability Monitoring -->
      <OpsGroupAvailabilityCard v-if="!(loading && !hasLoadedOnce)" />

      <!-- Ops Runtime Settings -->
      <OpsRuntimeSettingsCard v-if="!(loading && !hasLoadedOnce)" />

      <!-- Email Notification Configuration -->
      <OpsEmailNotificationCard v-if="!(loading && !hasLoadedOnce)" />

      <!-- Alert Events -->
      <OpsAlertEventsCard v-if="!(loading && !hasLoadedOnce)" />

      <!-- L3: Error Logs Query Section -->
      <OpsErrorLogTable
        v-if="!(loading && !hasLoadedOnce)"
        :errorLogs="errorLogs"
        :errorLogsTotal="errorLogsTotal"
        :errorLogsLoading="errorLogsLoading"
        :filters="errorFilters"
        :page="errorPagination.page"
        :page-size="errorPagination.pageSize"
        @update:filters="handleErrorFiltersUpdate"
        @update:page="handleErrorPageChange"
        @update:pageSize="handleErrorPageSizeChange"
        @openErrorDetail="openErrorDetail"
      />
    </div>

    <!-- Error Detail Modal -->
    <ErrorDetailModal
      v-if="selectedErrorId !== null"
      v-model="showErrorDetail"
      :error-id="selectedErrorId"
      @update:model-value="closeErrorDetail"
    />

    <!-- Request Details Modal -->
  <OpsRequestDetailsModal
      v-model="showRequestDetails"
      :time-range="timeRange"
      :preset="requestDetailsPreset"
      :platform="errorFilters.platforms.length === 1 ? errorFilters.platforms[0] : ''"
      :group-id="errorFilters.groupId"
      @openErrorDetail="openErrorDetailById"
    />
  </AppLayout>
</template>
