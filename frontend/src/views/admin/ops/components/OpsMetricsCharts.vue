<script setup lang="ts">
import { computed } from 'vue'
import { Bar, Doughnut, Line } from 'vue-chartjs'
import { formatNumber } from '@/utils/format'
import { sumNumbers, formatHistoryLabel, formatByteRate } from '../utils/opsFormatters'
import type { ChartState } from '../types'
import type {
  ProviderHealthData,
  LatencyHistogramResponse,
  ErrorDistributionResponse,
  OpsMetrics,
  OpsDashboardOverview
} from '@/api/admin/ops'

interface Props {
  hasLoadedOnce: boolean
  loading: boolean
  timeRange: string
  providers: ProviderHealthData[]
  latencyData: LatencyHistogramResponse | null
  errorDistribution: ErrorDistributionResponse | null
  latestMetrics: OpsMetrics | null
  metricsHistory: OpsMetrics[]
  overview: OpsDashboardOverview | null
}

const props = defineProps<Props>()

const emptyRequestHintText = '当前时间段内无请求记录'
const emptyErrorHintText = '当前时间段内无错误记录'

const isDarkMode = computed(() => {
  return document.documentElement.classList.contains('dark')
})

const lineColors = computed(() => ({
  text: isDarkMode.value ? '#e5e7eb' : '#374151',
  grid: isDarkMode.value ? '#374151' : '#e5e7eb'
}))

// Chart Data: Latency Distribution
const latencyHasData = computed(() => (props.latencyData?.total_requests ?? 0) > 0)
const latencyChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (latencyHasData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const latencyChartData = computed(() => {
  if (!props.latencyData || !latencyHasData.value) return null
  return {
    labels: props.latencyData.buckets.map(b => b.range),
    datasets: [
      {
        label: '请求数量',
        data: props.latencyData.buckets.map(b => b.count),
        backgroundColor: '#3b82f6',
        borderRadius: 4
      }
    ]
  }
})

// Chart Data: Error Distribution
const errorTotalCount = computed(() => sumNumbers(props.errorDistribution?.items?.map(i => i.count) ?? []))
const errorChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (errorTotalCount.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const errorChartData = computed(() => {
  if (!props.errorDistribution || errorTotalCount.value <= 0) return null
  return {
    labels: props.errorDistribution.items.map(i => i.code),
    datasets: [
      {
        data: props.errorDistribution.items.map(i => i.count),
        backgroundColor: [
          '#ef4444', '#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ec4899'
        ]
      }
    ]
  }
})

// Chart Data: Provider SLA
const providerTotalRequests = computed(() => sumNumbers(props.providers.map(p => p.request_count)))
const providerChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (props.providers.length > 0 && providerTotalRequests.value > 0) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})
const providerChartData = computed(() => {
  if (!props.providers.length || providerTotalRequests.value <= 0) return null
  return {
    labels: props.providers.map(p => p.name),
    datasets: [
      {
        label: 'SLA (%)',
        data: props.providers.map(p => p.success_rate),
        backgroundColor: props.providers.map(p => p.success_rate > 99.5 ? '#10b981' : p.success_rate > 98 ? '#f59e0b' : '#ef4444'),
        borderRadius: 4
      }
    ]
  }
})

// Chart Data: Throughput Trend
const throughputChartData = computed(() => {
  const totalRequests = sumNumbers(props.metricsHistory.map(m => m.request_count))
  if (!props.metricsHistory.length || totalRequests <= 0) return null
  return {
    labels: props.metricsHistory.map(m => formatHistoryLabel(m.updated_at, props.timeRange)),
    datasets: [
      {
        label: 'QPS',
        data: props.metricsHistory.map(m => m.qps ?? 0),
        borderColor: '#3b82f6',
        backgroundColor: '#3b82f620',
        fill: true,
        tension: 0.3,
        pointRadius: 0
      },
      {
        label: 'TPS (K)',
        data: props.metricsHistory.map(m => (m.tps ?? 0) / 1000),
        borderColor: '#10b981',
        backgroundColor: '#10b98120',
        fill: true,
        tension: 0.3,
        pointRadius: 0
      }
    ]
  }
})

const throughputChartState = computed<ChartState>(() => {
  if (!props.hasLoadedOnce) return 'loading'
  if (throughputChartData.value) return 'ready'
  if (props.loading) return 'loading'
  return 'empty'
})

const networkIOText = computed(() => {
  const m = props.latestMetrics
  if (!m) return null
  const rx = formatByteRate(m.network_in_bytes ?? 0, m.window_minutes || 1)
  const tx = formatByteRate(m.network_out_bytes ?? 0, m.window_minutes || 1)
  return `RX ${rx} · TX ${tx}`
})

const diskIOText = computed(() => {
  const m = props.latestMetrics
  if (!m) return null
  const hasReadWrite = (m.disk_read_bytes ?? 0) > 0 || (m.disk_write_bytes ?? 0) > 0
  if (hasReadWrite) {
    const read = formatByteRate(m.disk_read_bytes ?? 0, m.window_minutes || 1)
    const write = formatByteRate(m.disk_write_bytes ?? 0, m.window_minutes || 1)
    return `Read ${read} · Write ${write}`
  }
  return `${formatNumber(m.disk_iops ?? 0)} IOPS`
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
</template>
