<script setup lang="ts">
import { computed } from 'vue'
import { formatNumber } from '@/utils/format'
import type { OpsDashboardOverview } from '@/api/admin/ops'

interface Props {
  overview: OpsDashboardOverview | null
  wsConnected: boolean
  realTimeQPS: number
  realTimeTPS: number
  timeRange: string
  loading: boolean
  lastUpdated: Date
}

interface Emits {
  (e: 'update:timeRange', value: string): void
  (e: 'refresh'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const healthScoreClass = computed(() => {
  const score = props.overview?.health_score || 0
  if (score >= 90) return 'text-green-500 border-green-500'
  if (score >= 70) return 'text-yellow-500 border-yellow-500'
  return 'text-red-500 border-red-500'
})

const displayRealTimeQPS = computed(() => {
  if (props.wsConnected && props.realTimeQPS > 0) return props.realTimeQPS
  return props.overview?.qps.current ?? props.realTimeQPS
})

const displayRealTimeTPS = computed(() => {
  if (props.wsConnected && props.realTimeTPS > 0) return props.realTimeTPS
  return props.overview?.tps.current ?? props.realTimeTPS
})
</script>

<template>
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

      <select
        :value="timeRange"
        @change="emit('update:timeRange', ($event.target as HTMLSelectElement).value)"
        class="rounded-lg border-gray-200 bg-gray-50 py-1.5 pl-3 pr-8 text-sm font-medium text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
      >
        <option value="5m">5 分钟</option>
        <option value="30m">30 分钟</option>
        <option value="1h">1 小时</option>
        <option value="24h">24 小时</option>
      </select>

      <button @click="emit('refresh')" :disabled="loading" class="flex h-9 w-9 items-center justify-center rounded-lg bg-gray-100 text-gray-500 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400">
        <svg class="h-5 w-5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>
    </div>
  </div>

  <!-- Core Metrics Grid -->
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
