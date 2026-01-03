<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Bar, Doughnut, Line } from 'vue-chartjs'
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
import AppLayout from '@/components/layout/AppLayout.vue'
import ErrorDetailModal from '@/components/admin/ErrorDetailModal.vue'
import { opsAPI, type OpsDashboardOverview, type ProviderHealthData, type LatencyHistogramResponse, type ErrorDistributionResponse, type OpsMetrics, type OpsErrorLog, type OpsPlatform, type OpsSeverity } from '@/api/admin/ops'
import { formatBytes, formatNumber } from '@/utils/format'

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
const errorLogsPagination = ref({
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
const errorFilters = ref({
  platforms: [] as OpsPlatform[],
  statusCodes: [] as number[],
  clientIp: '',
  severity: '' as OpsSeverity | '',
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
    errorMessage.value = '数据加载失败，请稍后重试'
  } finally {
    loading.value = false
    hasLoadedOnce.value = true
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
  fetchData()
})

// Platform and status code options
const platformOptions: OpsPlatform[] = ['openai', 'anthropic', 'gemini', 'antigravity']
const statusCodeOptions = [400, 401, 403, 404, 429, 500, 502, 503, 504]
const severityOptions: OpsSeverity[] = ['P0', 'P1', 'P2', 'P3']

// Fetch error logs
const fetchErrors = async () => {
  errorLogsLoading.value = true
  try {
    const params: any = {
      limit: errorLogsPagination.value.pageSize
    }

    // Apply time range filter
    const minutes = parseTimeRangeMinutes(timeRange.value)
    const endTime = new Date()
    const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)
    params.start_time = startTime.toISOString()
    params.end_time = endTime.toISOString()

    // Apply filters
    if (errorFilters.value.platforms.length > 0) {
      params.platform = errorFilters.value.platforms[0] // API只支持单个platform
    }
    if (errorFilters.value.severity) {
      params.severity = errorFilters.value.severity
    }
    if (errorFilters.value.searchText) {
      params.q = errorFilters.value.searchText
    }

    const response = await opsAPI.listErrors(params)

    // 客户端过滤状态码和IP(如果API不支持)
    let filtered = response.items
    if (errorFilters.value.statusCodes.length > 0) {
      filtered = filtered.filter(log => errorFilters.value.statusCodes.includes(log.status_code))
    }
    if (errorFilters.value.clientIp) {
      filtered = filtered.filter(log => log.client_ip?.includes(errorFilters.value.clientIp))
    }
    if (errorFilters.value.platforms.length > 1) {
      filtered = filtered.filter(log => errorFilters.value.platforms.includes(log.platform))
    }

    errorLogs.value = filtered
    errorLogsTotal.value = response.total || filtered.length
  } catch (err) {
    console.error('Failed to fetch error logs', err)
  } finally {
    errorLogsLoading.value = false
  }
}

// Watch for filter changes
watch([errorFilters, timeRange], () => {
  errorLogsPagination.value.page = 1
  fetchErrors()
}, { deep: true })

// Severity badge class
const getSeverityClass = (severity: OpsSeverity) => {
  const classes = {
    P0: 'bg-red-100 text-red-800 dark:bg-red-900/30 dark:text-red-400',
    P1: 'bg-orange-100 text-orange-800 dark:bg-orange-900/30 dark:text-orange-400',
    P2: 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900/30 dark:text-yellow-400',
    P3: 'bg-blue-100 text-blue-800 dark:bg-blue-900/30 dark:text-blue-400'
  }
  return classes[severity] || classes.P3
}

// Truncate message
const truncateMessage = (msg: string, maxLength = 80) => {
  if (!msg) return ''
  return msg.length > maxLength ? msg.substring(0, maxLength) + '...' : msg
}

// Format date time
const formatDateTime = (dateStr: string) => {
  const d = new Date(dateStr)
  if (Number.isNaN(d.getTime())) return ''
  return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`
}

function sumNumbers(values: Array<number | null | undefined>): number {
  return values.reduce<number>((acc, v) => {
    const n = typeof v === 'number' && Number.isFinite(v) ? v : 0
    return acc + n
  }, 0)
}

const emptyRequestHintText = computed(() => '当前时间段内无请求记录')
const emptyErrorHintText = computed(() => '当前时间段内无错误记录')

// Chart Data: Latency Distribution
const latencyHasData = computed(() => (latencyData.value?.total_requests ?? 0) > 0)
const latencyChartState = computed<'loading' | 'empty' | 'ready'>(() => {
  if (!hasLoadedOnce.value) return 'loading'
  if (latencyHasData.value) return 'ready'
  if (loading.value) return 'loading'
  return 'empty'
})
const latencyChartData = computed(() => {
  if (!latencyData.value || !latencyHasData.value) return null
  return {
    labels: latencyData.value.buckets.map(b => b.range),
    datasets: [
      {
        label: t('admin.ops.charts.requestCount'),
        data: latencyData.value.buckets.map(b => b.count),
        backgroundColor: '#3b82f6',
        borderRadius: 4
      }
    ]
  }
})

// Chart Data: Error Distribution
const errorTotalCount = computed(() => sumNumbers(errorDistribution.value?.items?.map(i => i.count) ?? []))
const errorChartState = computed<'loading' | 'empty' | 'ready'>(() => {
  if (!hasLoadedOnce.value) return 'loading'
  if (errorTotalCount.value > 0) return 'ready'
  if (loading.value) return 'loading'
  return 'empty'
})
const errorChartData = computed(() => {
  if (!errorDistribution.value || errorTotalCount.value <= 0) return null
  return {
    labels: errorDistribution.value.items.map(i => i.code),
    datasets: [
      {
        data: errorDistribution.value.items.map(i => i.count),
        backgroundColor: [
          '#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899'
        ]
      }
    ]
  }
})

// Chart Data: Provider SLA
const providerTotalRequests = computed(() => sumNumbers(providers.value.map(p => p.request_count)))
const providerChartState = computed<'loading' | 'empty' | 'ready'>(() => {
  if (!hasLoadedOnce.value) return 'loading'
  if (providers.value.length > 0 && providerTotalRequests.value > 0) return 'ready'
  if (loading.value) return 'loading'
  return 'empty'
})
const providerChartData = computed(() => {
  if (!providers.value.length || providerTotalRequests.value <= 0) return null
  return {
    labels: providers.value.map(p => p.name),
    datasets: [
      {
        label: 'SLA (%)',
        data: providers.value.map(p => p.success_rate),
        backgroundColor: providers.value.map(p => p.success_rate > 99.5 ? '#10b981' : p.success_rate > 98 ? '#f59e0b' : '#ef4444'),
        borderRadius: 4
      }
    ]
  }
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: {
      display: false
    }
  },
  scales: {
    y: {
      beginAtZero: true,
      grid: {
        display: false
      }
    },
    x: {
      grid: {
        display: false
      }
    }
  }
}

const healthScoreClass = computed(() => {
  const score = overview.value?.health_score || 0
  if (score >= 90) return 'text-green-500 border-green-500'
  if (score >= 70) return 'text-yellow-500 border-yellow-500'
  return 'text-red-500 border-red-500'
})

const displayRealTimeQPS = computed(() => {
  if (wsConnected.value && realTimeQPS.value > 0) return realTimeQPS.value
  return overview.value?.qps.current ?? realTimeQPS.value
})

const displayRealTimeTPS = computed(() => {
  if (wsConnected.value && realTimeTPS.value > 0) return realTimeTPS.value
  return overview.value?.tps.current ?? realTimeTPS.value
})

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const lineColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

function parseTimeRangeMinutes(range: string): number {
  const trimmed = (range || '').trim()
  if (!trimmed) return 60
  if (trimmed.endsWith('m')) {
    const v = Number.parseInt(trimmed.slice(0, -1), 10)
    return Number.isFinite(v) && v > 0 ? v : 60
  }
  if (trimmed.endsWith('h')) {
    const v = Number.parseInt(trimmed.slice(0, -1), 10)
    return Number.isFinite(v) && v > 0 ? v * 60 : 60
  }
  return 60
}

function formatHistoryLabel(date: string | undefined): string {
  if (!date) return ''
  const d = new Date(date)
  if (Number.isNaN(d.getTime())) return ''
  const minutes = parseTimeRangeMinutes(timeRange.value)
  if (minutes >= 24 * 60) {
    return `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
  }
  return `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`
}

function formatByteRate(bytes: number, windowMinutes: number): string {
  const seconds = Math.max(1, (windowMinutes || 1) * 60)
  return `${formatBytes(bytes / seconds, 1)}/s`
}

const networkIOText = computed(() => {
  const m = latestMetrics.value
  if (!m) return null
  const rx = formatByteRate(m.network_in_bytes ?? 0, m.window_minutes || 1)
  const tx = formatByteRate(m.network_out_bytes ?? 0, m.window_minutes || 1)
  return `RX ${rx} · TX ${tx}`
})

const diskIOText = computed(() => {
  const m = latestMetrics.value
  if (!m) return null
  const hasReadWrite = (m.disk_read_bytes ?? 0) > 0 || (m.disk_write_bytes ?? 0) > 0
  if (hasReadWrite) {
    const read = formatByteRate(m.disk_read_bytes ?? 0, m.window_minutes || 1)
    const write = formatByteRate(m.disk_write_bytes ?? 0, m.window_minutes || 1)
    return `Read ${read} · Write ${write}`
  }
  return `${formatNumber(m.disk_iops ?? 0)} IOPS`
})

const throughputChartData = computed(() => {
  const totalRequests = sumNumbers(metricsHistory.value.map(m => m.request_count))
  if (!metricsHistory.value.length || totalRequests <= 0) return null
  return {
    labels: metricsHistory.value.map(m => formatHistoryLabel(m.updated_at)),
    datasets: [
      {
        label: 'QPS',
        data: metricsHistory.value.map(m => m.qps ?? 0),
        borderColor: '#3b82f6',
        backgroundColor: '#3b82f620',
        fill: true,
        tension: 0.3,
        pointRadius: 0
      },
      {
        label: 'TPS (K)',
        data: metricsHistory.value.map(m => (m.tps ?? 0) / 1000),
        borderColor: '#10b981',
        backgroundColor: '#10b98120',
        fill: true,
        tension: 0.3,
        pointRadius: 0
      }
    ]
  }
})

const throughputChartState = computed<'loading' | 'empty' | 'ready'>(() => {
  if (!hasLoadedOnce.value) return 'loading'
  if (throughputChartData.value) return 'ready'
  if (loading.value) return 'loading'
  return 'empty'
})

const throughputChartOptions = computed(() => ({
  responsive: true,
  maintainAspectRatio: false,
  interaction: {
    intersect: false,
    mode: 'index' as const
  },
  plugins: {
    legend: {
      position: 'top' as const,
      labels: {
        color: lineColors.value.text,
        usePointStyle: true,
        pointStyle: 'circle',
        padding: 12,
        font: {
          size: 11
        }
      }
    },
    tooltip: {
      callbacks: {
        label: (context: any) => {
          const label = context.dataset.label as string
          if (label === 'TPS (K)') return `${label}: ${Number(context.raw).toFixed(1)}K`
          return `${label}: ${Number(context.raw).toFixed(1)}`
        }
      }
    }
  },
  scales: {
    x: {
      grid: {
        color: lineColors.value.grid
      },
      ticks: {
        color: lineColors.value.text,
        font: {
          size: 10
        },
        maxTicksLimit: 8
      }
    },
    y: {
      beginAtZero: true,
      grid: {
        color: lineColors.value.grid
      },
      ticks: {
        color: lineColors.value.text,
        font: {
          size: 10
        }
      }
    }
  }
}))

</script>

<template>
  <AppLayout>
    <div class="space-y-6 pb-12">
      <!-- Error Message -->
      <div v-if="errorMessage" class="rounded-2xl bg-red-50 p-4 text-sm text-red-600 dark:bg-red-900/20 dark:text-red-400">
        {{ errorMessage }}
      </div>

      <!-- L1: Header & Realtime Stats -->
      <div class="flex flex-wrap items-center justify-between gap-4 rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex items-center gap-6">
          <!-- Health Score Gauge -->
          <div class="flex h-20 w-20 flex-col items-center justify-center rounded-full border-4 bg-gray-50 dark:bg-dark-900" :class="healthScoreClass">
            <span class="text-2xl font-black">{{ overview?.health_score || '--' }}</span>
            <span class="text-[10px] font-bold opacity-60">HEALTH</span>
          </div>
          
          <div>
            <h1 class="text-xl font-black text-gray-900 dark:text-white">运维监控中心 2.0</h1>
            <div class="mt-1 flex items-center gap-3">
              <span class="flex items-center gap-1.5">
                <span class="h-2 w-2 rounded-full bg-green-500 animate-pulse" v-if="wsConnected"></span>
                <span class="h-2 w-2 rounded-full bg-red-500" v-else></span>
                <span class="text-xs font-medium text-gray-500">{{ wsConnected ? '实时连接中' : '连接已断开' }}</span>
              </span>
              <span class="text-xs text-gray-400">最后更新: {{ lastUpdated.toLocaleTimeString() }}</span>
            </div>
          </div>
        </div>

        <div class="flex items-center gap-4">
          <div class="hidden items-center gap-6 border-r border-gray-100 pr-6 dark:border-dark-700 lg:flex">
            <div class="text-center">
              <div class="text-sm font-black text-gray-900 dark:text-white">{{ displayRealTimeQPS.toFixed(1) }}</div>
              <div class="text-[10px] font-bold text-gray-400 uppercase">实时 QPS</div>
            </div>
            <div class="text-center">
              <div class="text-sm font-black text-gray-900 dark:text-white">{{ formatNumber(displayRealTimeTPS) }}</div>
              <div class="text-[10px] font-bold text-gray-400 uppercase">实时 TPS</div>
            </div>
          </div>
          
          <select v-model="timeRange" class="rounded-lg border-gray-200 bg-gray-50 py-1.5 pl-3 pr-8 text-sm font-medium text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300">
            <option value="5m">5 分钟</option>
            <option value="30m">30 分钟</option>
            <option value="1h">1 小时</option>
            <option value="24h">24 小时</option>
          </select>
          
          <button @click="fetchData" :disabled="loading" class="flex h-9 w-9 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400">
            <svg class="h-5 w-5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
          </button>
        </div>
      </div>

      <!-- L1: Core Metrics Grid -->
      <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-gray-400 uppercase tracking-wider">服务可用率 (SLA)</span>
            <span class="rounded-full bg-green-50 px-2 py-0.5 text-[10px] font-bold text-green-600 dark:bg-green-900/30">{{ overview?.sla.status }}</span>
          </div>
          <div class="mt-2 flex items-baseline gap-2">
            <span class="text-2xl font-black text-gray-900 dark:text-white">{{ overview?.sla.current.toFixed(2) }}%</span>
            <span class="text-xs font-bold" :class="overview?.sla.change_24h && overview.sla.change_24h >= 0 ? 'text-green-500' : 'text-red-500'">
              {{ overview?.sla.change_24h && overview.sla.change_24h >= 0 ? '+' : '' }}{{ overview?.sla.change_24h }}%
            </span>
          </div>
          <div class="mt-3 h-1 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
            <div class="h-full bg-green-500" :style="{ width: `${overview?.sla.current}%` }"></div>
          </div>
        </div>

        <div class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-gray-400 uppercase tracking-wider">P99 响应延迟</span>
            <span class="rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-bold text-blue-600 dark:bg-blue-900/30">Target 1s</span>
          </div>
          <div class="mt-2 flex items-baseline gap-2">
            <span class="text-2xl font-black text-gray-900 dark:text-white">{{ overview?.latency.p99 }}ms</span>
            <span class="text-xs font-bold text-gray-400">Avg: {{ overview?.latency.avg }}ms</span>
          </div>
          <div class="mt-3 flex gap-1">
            <div v-for="i in 10" :key="i" class="h-1 flex-1 rounded-full" :class="i <= (overview?.latency.p99 || 0) / 200 ? 'bg-blue-500' : 'bg-gray-100 dark:bg-dark-700'"></div>
          </div>
        </div>

        <div class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-gray-400 uppercase tracking-wider">周期请求总数</span>
            <svg class="h-4 w-4 text-gray-300" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" /></svg>
          </div>
          <div class="mt-2 flex items-baseline gap-2">
            <span class="text-2xl font-black text-gray-900 dark:text-white">{{ overview?.qps.avg_1h.toFixed(1) }}</span>
            <span class="text-xs font-bold text-gray-400">req/s</span>
          </div>
          <div class="mt-1 text-[10px] font-bold text-gray-400 uppercase">对比昨日: {{ overview?.qps.change_vs_yesterday }}%</div>
        </div>

        <div class="rounded-2xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="flex items-center justify-between">
            <span class="text-xs font-bold text-gray-400 uppercase tracking-wider">周期错误数</span>
            <span class="rounded-full bg-red-50 px-2 py-0.5 text-[10px] font-bold text-red-600 dark:bg-red-900/30">{{ overview?.errors.error_rate.toFixed(2) }}%</span>
          </div>
          <div class="mt-2 flex items-baseline gap-2">
            <span class="text-2xl font-black text-gray-900 dark:text-white">{{ overview?.errors.total_count }}</span>
            <span class="text-xs font-bold text-red-500">5xx: {{ overview?.errors['5xx_count'] }}</span>
          </div>
          <div class="mt-1 text-[10px] font-bold text-gray-400 uppercase">主要错误码: {{ overview?.errors.top_error?.code || 'N/A' }}</div>
        </div>
      </div>

      <!-- L2: Visual Analysis -->
      <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
        <!-- Latency Distribution -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">请求延迟分布</h3>
          </div>
          <div class="h-64">
            <Bar v-if="latencyChartState === 'ready' && latencyChartData" :data="latencyChartData" :options="chartOptions" />
            <div v-else class="flex h-full flex-col items-center justify-center gap-1 text-center text-gray-400">
              <div v-if="latencyChartState === 'loading'" class="text-sm font-medium">加载中...</div>
              <div v-else class="text-sm font-medium">{{ emptyRequestHintText }}</div>
            </div>
          </div>
        </div>

        <!-- Provider Health -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">上游供应商健康度 (SLA)</h3>
          </div>
          <div class="h-64">
            <Bar v-if="providerChartState === 'ready' && providerChartData" :data="providerChartData" :options="chartOptions" />
            <div v-else class="flex h-full flex-col items-center justify-center gap-1 text-center text-gray-400">
              <div v-if="providerChartState === 'loading'" class="text-sm font-medium">加载中...</div>
              <div v-else class="text-sm font-medium">{{ emptyRequestHintText }}</div>
            </div>
          </div>
        </div>

        <!-- Throughput Trend -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">吞吐趋势 (QPS/TPS)</h3>
          </div>
          <div class="h-64">
            <Line v-if="throughputChartState === 'ready' && throughputChartData" :data="throughputChartData" :options="throughputChartOptions" />
            <div v-else class="flex h-full flex-col items-center justify-center gap-1 text-center text-gray-400">
              <div v-if="throughputChartState === 'loading'" class="text-sm font-medium">加载中...</div>
              <div v-else class="text-sm font-medium">{{ emptyRequestHintText }}</div>
            </div>
          </div>
        </div>

        <!-- Error Distribution -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">错误类型分布</h3>
          </div>
          <div class="h-64">
            <div v-if="errorChartState === 'ready' && errorChartData" class="flex h-full gap-6">
              <div class="relative w-1/2">
                <Doughnut :data="errorChartData" :options="{ ...chartOptions, cutout: '70%' }" />
              </div>
              <div class="flex flex-1 flex-col justify-center space-y-3">
                <div v-for="(item, idx) in errorDistribution?.items.slice(0, 5)" :key="item.code" class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <div class="h-2 w-2 rounded-full" :style="{ backgroundColor: ['#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6'][idx] }"></div>
                    <span class="text-xs font-bold text-gray-700 dark:text-gray-300">{{ item.code }}</span>
                  </div>
                  <span class="text-xs font-black text-gray-900 dark:text-white">{{ item.percentage }}%</span>
                </div>
              </div>
            </div>
            <div v-else class="flex h-full flex-col items-center justify-center gap-1 text-center text-gray-400">
              <div v-if="errorChartState === 'loading'" class="text-sm font-medium">加载中...</div>
              <div v-else class="text-sm font-medium">{{ emptyErrorHintText }}</div>
            </div>
          </div>
        </div>

        <!-- System Resources -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">系统运行状态</h3>
          </div>
          <div class="grid grid-cols-2 gap-6">
            <div class="space-y-4">
              <div>
                <div class="mb-1 flex justify-between text-[10px] font-bold text-gray-400 uppercase">CPU 使用率</div>
                <div class="h-2 w-full rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-purple-500" :style="{ width: `${overview?.resources.cpu_usage}%` }"></div>
                </div>
                <div class="mt-1 text-right text-xs font-bold text-gray-900 dark:text-white">{{ overview?.resources.cpu_usage }}%</div>
              </div>
              <div>
                <div class="mb-1 flex justify-between text-[10px] font-bold text-gray-400 uppercase">内存使用率</div>
                <div class="h-2 w-full rounded-full bg-gray-100 dark:bg-dark-700">
                  <div class="h-full rounded-full bg-indigo-500" :style="{ width: `${overview?.resources.memory_usage}%` }"></div>
                </div>
                <div class="mt-1 text-right text-xs font-bold text-gray-900 dark:text-white">{{ overview?.resources.memory_usage }}%</div>
              </div>
            </div>
            <div class="flex flex-col justify-center space-y-4 rounded-xl bg-gray-50 p-4 dark:bg-dark-900">
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-bold text-gray-400 uppercase">Redis 状态</span>
                <span class="text-xs font-bold text-green-500 uppercase">{{ overview?.system_status.redis }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-bold text-gray-400 uppercase">DB 连接</span>
                <span class="text-xs font-bold text-gray-900 dark:text-white">{{ overview?.resources.db_connections.active }} / {{ overview?.resources.db_connections.max }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-bold text-gray-400 uppercase">Goroutines</span>
                <span class="text-xs font-bold text-gray-900 dark:text-white">{{ latestMetrics?.goroutine_count ?? overview?.resources.goroutines }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-bold text-gray-400 uppercase">Network I/O</span>
                <span class="text-xs font-bold text-gray-900 dark:text-white">{{ networkIOText || '--' }}</span>
              </div>
              <div class="flex items-center justify-between">
                <span class="text-[10px] font-bold text-gray-400 uppercase">Disk I/O</span>
                <span class="text-xs font-bold text-gray-900 dark:text-white">{{ diskIOText || '--' }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- L3: Error Logs Query Section -->
      <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="mb-6 flex items-center justify-between">
          <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">错误日志查询</h3>
          <span class="text-xs font-medium text-gray-500">共 {{ errorLogsTotal }} 条记录</span>
        </div>

        <!-- Filters Bar -->
        <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
          <!-- Platform Multi-Select -->
          <div>
            <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">平台</label>
            <select
              v-model="errorFilters.platforms"
              multiple
              class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
              style="height: 38px; overflow: auto;"
            >
              <option v-for="platform in platformOptions" :key="platform" :value="platform">
                {{ platform }}
              </option>
            </select>
          </div>

          <!-- Status Code Multi-Select -->
          <div>
            <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">错误码</label>
            <select
              v-model="errorFilters.statusCodes"
              multiple
              class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
              style="height: 38px; overflow: auto;"
            >
              <option v-for="code in statusCodeOptions" :key="code" :value="code">
                {{ code }}
              </option>
            </select>
          </div>

          <!-- Severity Select -->
          <div>
            <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">严重级别</label>
            <select
              v-model="errorFilters.severity"
              class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
            >
              <option value="">全部</option>
              <option v-for="sev in severityOptions" :key="sev" :value="sev">
                {{ sev }}
              </option>
            </select>
          </div>

          <!-- IP Address Input -->
          <div>
            <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">IP地址</label>
            <input
              v-model="errorFilters.clientIp"
              type="text"
              placeholder="搜索IP"
              class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 placeholder-gray-400 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
            />
          </div>

          <!-- Search Input -->
          <div>
            <label class="mb-1.5 block text-xs font-bold text-gray-400 uppercase">搜索</label>
            <input
              v-model="errorFilters.searchText"
              type="text"
              placeholder="request_id / 错误信息"
              class="w-full rounded-lg border-gray-200 bg-gray-50 py-2 px-3 text-sm text-gray-700 placeholder-gray-400 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
            />
          </div>
        </div>

        <!-- Error Logs Table -->
        <div class="overflow-x-auto">
          <div v-if="errorLogsLoading" class="flex items-center justify-center py-12">
            <div class="text-sm font-medium text-gray-400">加载中...</div>
          </div>
          <div v-else-if="errorLogs.length === 0" class="flex items-center justify-center py-12">
            <div class="text-sm font-medium text-gray-400">当前筛选条件下无错误记录</div>
          </div>
          <table v-else class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900">
              <tr>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">时间</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">Request ID</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">平台</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">错误码</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">严重级别</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">延迟(ms)</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">错误信息</th>
                <th scope="col" class="px-3 py-3 text-left text-xs font-bold uppercase tracking-wider text-gray-500">客户端IP</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr
                v-for="log in errorLogs"
                :key="log.id"
                class="cursor-pointer transition-colors hover:bg-gray-50 dark:hover:bg-dark-700/50"
                @click="openErrorDetail(log)"
              >
                <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-900 dark:text-gray-300">
                  {{ formatDateTime(log.created_at) }}
                </td>
                <td class="px-3 py-3 text-xs font-mono text-gray-700 dark:text-gray-400">
                  <div class="max-w-[120px] truncate" :title="log.request_id">{{ log.request_id }}</div>
                </td>
                <td class="whitespace-nowrap px-3 py-3">
                  <span class="rounded-full bg-blue-100 px-2 py-1 text-xs font-bold text-blue-800 dark:bg-blue-900/30 dark:text-blue-400">
                    {{ log.platform }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-3 py-3">
                  <span class="rounded-full bg-gray-100 px-2 py-1 text-xs font-bold text-gray-800 dark:bg-gray-900/30 dark:text-gray-400">
                    {{ log.status_code }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-3 py-3">
                  <span class="rounded-full px-2 py-1 text-xs font-bold" :class="getSeverityClass(log.severity)">
                    {{ log.severity }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-3 py-3 text-xs text-gray-700 dark:text-gray-400">
                  {{ log.latency_ms !== null ? log.latency_ms.toFixed(0) : '--' }}
                </td>
                <td class="px-3 py-3 text-xs text-gray-700 dark:text-gray-400">
                  <div class="max-w-[300px]" :title="log.message">{{ truncateMessage(log.message) }}</div>
                </td>
                <td class="whitespace-nowrap px-3 py-3 text-xs font-mono text-gray-600 dark:text-gray-500">
                  {{ log.client_ip || '--' }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <!-- Pagination Info -->
        <div v-if="errorLogs.length > 0" class="mt-4 flex items-center justify-between border-t border-gray-200 pt-4 dark:border-dark-700">
          <div class="text-xs text-gray-500">
            显示 {{ errorLogs.length }} 条记录 (共 {{ errorLogsTotal }} 条)
          </div>
          <div class="text-xs text-gray-400">
            分页功能后续可扩展
          </div>
        </div>
      </div>
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

<style scoped>
/* Custom select styling */
select {
  appearance: none;
  background-image: url("data:image/svg+xml,%3csvg xmlns='http://www.w3.org/2000/svg' fill='none' viewBox='0 0 20 20'%3e%3cpath stroke='%236b7280' stroke-linecap='round' stroke-linejoin='round' stroke-width='1.5' d='M6 8l4 4 4-4'/%3e%3c/svg%3e");
  background-repeat: no-repeat;
  background-position: right 0.5rem center;
  background-size: 1.5em 1.5em;
}
</style>
