<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
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
import { useIntervalFn } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ErrorDetailModal from '@/components/admin/ErrorDetailModal.vue'
import OpsDashboardHeader from './components/OpsDashboardHeader.vue'
import OpsMetricsCharts from './components/OpsMetricsCharts.vue'
import OpsGroupAvailabilityCard from './components/OpsGroupAvailabilityCard.vue'
import OpsErrorLogTable from './components/OpsErrorLogTable.vue'
import { opsAPI, type OpsDashboardOverview, type ProviderHealthData, type LatencyHistogramResponse, type ErrorDistributionResponse, type OpsMetrics, type OpsErrorLog } from '@/api/admin/ops'
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
  Filler
)

const { t } = useI18n()
const loading = ref(false)
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
const errorLogsLoading = ref(false)
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

function closeErrorDetail() {
  showErrorDetail.value = false
  // Delay clearing selectedErrorId to allow animation to complete
  setTimeout(() => {
    selectedErrorId.value = null
  }, 300)
}

const errorFilters = ref<ErrorFilters>({
  platforms: [],
  statusCodes: [],
  clientIp: '',
  severity: '',
  searchText: ''
})

// WebSocket for real-time QPS
const realTimeQPS = ref(0)
const realTimeTPS = ref(0)
const wsConnected = ref(false)
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
      onOpen: () => {
        wsConnected.value = true
      },
      onClose: () => {
        wsConnected.value = false
      }
    }
  )
})

onUnmounted(() => {
  wsConnected.value = false
  if (unsubscribeQPS) unsubscribeQPS()
  unsubscribeQPS = null
})

watch(timeRange, () => {
  errorPagination.value.page = 1
  fetchData()
  fetchErrors()
})

function handleErrorFiltersUpdate(nextFilters: ErrorFilters) {
  errorFilters.value = nextFilters
  errorPagination.value.page = 1
  fetchErrors()
}

function handleErrorPageChange(page: number) {
  errorPagination.value.page = page
  fetchErrors()
}

function handleErrorPageSizeChange(pageSize: number) {
  errorPagination.value.pageSize = pageSize
  errorPagination.value.page = 1
  fetchErrors()
}
</script>

<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- Error Message -->
      <div v-if="errorMessage" class="rounded-2xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
        {{ errorMessage }}
      </div>

      <!-- L1: Header & Core Metrics -->
      <OpsDashboardHeader
        :overview="overview"
        :wsConnected="wsConnected"
        :realTimeQPS="realTimeQPS"
        :realTimeTPS="realTimeTPS"
        :timeRange="timeRange"
        :loading="loading"
        :lastUpdated="lastUpdated"
        @update:timeRange="timeRange = $event"
        @refresh="fetchData"
      />

      <!-- L2: Visual Analysis -->
      <OpsMetricsCharts
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
      <OpsGroupAvailabilityCard />

      <!-- L3: Error Logs Query Section -->
      <OpsErrorLogTable
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
  </AppLayout>
</template>
