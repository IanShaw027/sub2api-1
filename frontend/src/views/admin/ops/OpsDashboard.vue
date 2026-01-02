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
import { opsAPI, type OpsDashboardOverview, type ProviderHealthData, type LatencyHistogramResponse, type ErrorDistributionResponse, type OpsMetrics } from '@/api/admin/ops'
import { useAuthStore } from '@/stores/auth'
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
const authStore = useAuthStore()
const loading = ref(false)
const errorMessage = ref('')
const timeRange = ref('1h')
const lastUpdated = ref(new Date())

const overview = ref<OpsDashboardOverview | null>(null)
const providers = ref<ProviderHealthData[]>([])
const latencyData = ref<LatencyHistogramResponse | null>(null)
const errorDistribution = ref<ErrorDistributionResponse | null>(null)
const latestMetrics = ref<OpsMetrics | null>(null)
const metricsHistory = ref<OpsMetrics[]>([])

// WebSocket for real-time QPS
const realTimeQPS = ref(0)
const realTimeTPS = ref(0)
const wsConnected = ref(false)
let ws: WebSocket | null = null
let reconnectTimer: ReturnType<typeof setTimeout> | null = null

const connectWS = () => {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const wsBaseUrl = import.meta.env.VITE_WS_BASE_URL || window.location.host
  const wsURL = new URL(`${protocol}//${wsBaseUrl}/api/v1/admin/ops/ws/qps`)
  const token = authStore.token || localStorage.getItem('auth_token')
  if (token) {
    wsURL.searchParams.set('token', token)
  }
  ws = new WebSocket(wsURL.toString())

  ws.onopen = () => {
    wsConnected.value = true
  }

  ws.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data)
      if (payload && typeof payload === 'object' && payload.type === 'qps_update' && payload.data) {
        realTimeQPS.value = payload.data.qps || 0
        realTimeTPS.value = payload.data.tps || 0
      }
    } catch (e) {
      console.error('WS parse error', e)
    }
  }

  ws.onclose = () => {
    wsConnected.value = false
    if (reconnectTimer) clearTimeout(reconnectTimer)
    reconnectTimer = setTimeout(connectWS, 5000)
  }
}

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
  }
}

// Refresh data every 30 seconds (fallback for L2/L3)
useIntervalFn(fetchData, 30000)

onMounted(() => {
  fetchData()
  connectWS()
})

onUnmounted(() => {
  if (ws) ws.close()
  if (reconnectTimer) clearTimeout(reconnectTimer)
})

watch(timeRange, () => {
  fetchData()
})

// Chart Data: Latency Distribution
const latencyChartData = computed(() => {
  if (!latencyData.value) return null
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
const errorChartData = computed(() => {
  if (!errorDistribution.value) return null
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
const providerChartData = computed(() => {
  if (!providers.value.length) return null
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
  if (!metricsHistory.value.length) return null
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
            <Bar v-if="latencyChartData" :data="latencyChartData" :options="chartOptions" />
            <div v-else class="flex h-full items-center justify-center text-gray-400">加载中...</div>
          </div>
        </div>

        <!-- Provider Health -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">上游供应商健康度 (SLA)</h3>
          </div>
          <div class="h-64">
            <Bar v-if="providerChartData" :data="providerChartData" :options="chartOptions" />
            <div v-else class="flex h-full items-center justify-center text-gray-400">加载中...</div>
          </div>
        </div>

        <!-- Throughput Trend -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">吞吐趋势 (QPS/TPS)</h3>
          </div>
          <div class="h-64">
            <Line v-if="throughputChartData" :data="throughputChartData" :options="throughputChartOptions" />
            <div v-else class="flex h-full items-center justify-center text-gray-400">加载中...</div>
          </div>
        </div>

        <!-- Error Distribution -->
        <div class="rounded-2xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
          <div class="mb-6 flex items-center justify-between">
            <h3 class="text-sm font-black text-gray-900 dark:text-white uppercase tracking-wider">错误类型分布</h3>
          </div>
          <div class="flex h-64 gap-6">
            <div class="relative w-1/2">
              <Doughnut v-if="errorChartData" :data="errorChartData" :options="{ ...chartOptions, cutout: '70%' }" />
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
    </div>
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
