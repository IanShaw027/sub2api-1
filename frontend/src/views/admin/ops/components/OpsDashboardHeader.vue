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

const healthScoreColor = computed(() => {
  const score = props.overview?.health_score || 0
  if (score >= 90) return '#10b981' // green-500
  if (score >= 70) return '#f59e0b' // yellow-500
  return '#ef4444' // red-500
})

const healthScoreClass = computed(() => {
  const score = props.overview?.health_score || 0
  if (score >= 90) return 'text-green-500'
  if (score >= 70) return 'text-yellow-500'
  return 'text-red-500'
})

// SVG Circle properties for Health Score
const circleSize = 100
const strokeWidth = 8
const radius = (circleSize - strokeWidth) / 2
const circumference = 2 * Math.PI * radius
const dashOffset = computed(() => {
  const score = props.overview?.health_score || 0
  return circumference - (score / 100) * circumference
})

const displayRealTimeQPS = computed(() => {
  if (props.wsConnected && props.realTimeQPS > 0) return props.realTimeQPS
  return props.overview?.qps.current ?? 0
})

const displayRealTimeTPS = computed(() => {
  if (props.wsConnected && props.realTimeTPS > 0) return props.realTimeTPS
  return props.overview?.tps.current ?? 0
})

// Status helpers
const getStatusColor = (status: string | undefined) => {
  if (!status) return 'bg-gray-200 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  if (status === 'operational' || status === 'healthy' || status === 'running')
    return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  if (status === 'degraded' || status === 'warning')
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
}
</script>

<template>
  <div class="flex flex-col gap-4 rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <!-- Top Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 pb-4 dark:border-dark-700">
      <div>
        <h1 class="flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <svg class="h-6 w-6 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          {{ $t('admin.ops.title') }}
        </h1>
        <div class="mt-1 flex items-center gap-3 text-xs text-gray-500">
          <span class="flex items-center gap-1.5" :title="wsConnected ? $t('admin.ops.status.wsConnected') : $t('admin.ops.status.wsDisconnected')">
            <span class="relative flex h-2 w-2">
              <span v-if="wsConnected" class="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
              <span class="relative inline-flex h-2 w-2 rounded-full" :class="wsConnected ? 'bg-green-500' : 'bg-red-500'"></span>
            </span>
            {{ wsConnected ? $t('admin.ops.status.online') : $t('admin.ops.status.offline') }}
          </span>
          <span>·</span>
          <span>{{ $t('admin.ops.status.updatedAt') }} {{ lastUpdated.toLocaleTimeString() }}</span>
        </div>
      </div>

      <div class="flex items-center gap-3">
        <select
          :value="timeRange"
          @change="emit('update:timeRange', ($event.target as HTMLSelectElement).value)"
          class="rounded-lg border-gray-200 bg-gray-50 py-1.5 pl-3 pr-8 text-sm font-medium text-gray-700 focus:border-blue-500 focus:ring-blue-500 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        >
          <option value="5m">{{ $t('admin.ops.timeRange.5m') }}</option>
          <option value="30m">{{ $t('admin.ops.timeRange.30m') }}</option>
          <option value="1h">{{ $t('admin.ops.timeRange.1h') }}</option>
          <option value="6h">{{ $t('admin.ops.timeRange.6h') }}</option>
          <option value="24h">{{ $t('admin.ops.timeRange.24h') }}</option>
        </select>

        <button
          @click="emit('refresh')"
          :disabled="loading"
          class="flex h-9 w-9 items-center justify-center rounded-lg bg-gray-100 text-gray-500 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
          :title="$t('admin.ops.status.refresh')"
        >
          <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
        </button>
      </div>
    </div>

    <!-- Main Dashboard Grid -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-12">
      <!-- 1. Health Score (Col span 3) -->
      <div 
        class="flex flex-col items-center justify-center border-r border-gray-100 py-2 dark:border-dark-700 lg:col-span-3"
        :title="$t('admin.ops.tooltips.healthScore')"
      >
        <div class="relative flex items-center justify-center">
          <svg :width="circleSize" :height="circleSize" class="-rotate-90 transform">
            <circle
              :cx="circleSize / 2"
              :cy="circleSize / 2"
              :r="radius"
              :stroke-width="strokeWidth"
              fill="transparent"
              class="text-gray-100 dark:text-dark-700"
              stroke="currentColor"
            />
            <circle
              :cx="circleSize / 2"
              :cy="circleSize / 2"
              :r="radius"
              :stroke-width="strokeWidth"
              fill="transparent"
              :stroke="healthScoreColor"
              stroke-linecap="round"
              :stroke-dasharray="circumference"
              :stroke-dashoffset="dashOffset"
              class="transition-all duration-1000 ease-out"
            />
          </svg>
          <div class="absolute flex flex-col items-center">
            <span class="text-3xl font-black" :class="healthScoreClass">{{ overview?.health_score || '--' }}</span>
            <span class="text-[10px] font-bold uppercase tracking-wider text-gray-400">Health</span>
          </div>
        </div>
        <div class="mt-4 text-center">
          <div class="text-xs font-medium text-gray-500">{{ $t('admin.ops.status.healthCondition') }}</div>
          <div class="mt-1 text-xs font-bold" :class="healthScoreClass">
            {{ overview?.health_score && overview.health_score >= 90 ? $t('admin.ops.status.healthy') : $t('admin.ops.status.risky') }}
          </div>
        </div>
      </div>

      <!-- 2. Real-time Pulse (Col span 4) -->
      <div class="flex flex-col justify-center border-r border-gray-100 px-4 py-2 dark:border-dark-700 lg:col-span-5">
        <div class="mb-2 flex items-center gap-2">
          <div class="relative flex h-3 w-3">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-75"></span>
            <span class="relative inline-flex h-3 w-3 rounded-full bg-blue-500"></span>
          </div>
          <h3 class="text-xs font-bold uppercase tracking-wider text-gray-400">{{ $t('admin.ops.status.trafficPulse') }}</h3>
        </div>

        <div class="flex items-baseline gap-1" :title="$t('admin.ops.tooltips.realtimeQPS')">
          <span class="text-4xl font-black text-gray-900 dark:text-white">{{ displayRealTimeQPS.toFixed(1) }}</span>
          <span class="text-sm font-bold text-gray-500">QPS</span>
        </div>
        <div class="mt-1 flex items-center gap-4 text-xs font-medium text-gray-500">
          <span :title="$t('admin.ops.tooltips.realtimeTPS')">TPS: {{ formatNumber(displayRealTimeTPS) }}</span>
          <span class="h-1 w-1 rounded-full bg-gray-300 dark:bg-gray-600"></span>
          <span :title="$t('admin.ops.tooltips.peak1h')">{{ $t('admin.ops.status.peak1h') }}: {{ overview?.qps.peak_1h.toFixed(1) }}</span>
        </div>

        <!-- Animated Pulse Line (CSS/SVG Animation) -->
        <div class="mt-4 h-8 w-full overflow-hidden opacity-50">
           <svg class="h-full w-full" preserveAspectRatio="none">
             <path
               d="M0 16 Q 20 16, 40 16 T 80 16 T 120 10 T 160 22 T 200 16 T 240 16 T 280 16"
               fill="none"
               stroke="#3b82f6"
               stroke-width="2"
               vector-effect="non-scaling-stroke"
             >
               <animate attributeName="d" dur="2s" repeatCount="indefinite"
                 values="M0 16 Q 20 16, 40 16 T 80 16 T 120 10 T 160 22 T 200 16 T 240 16 T 280 16;
                         M0 16 Q 20 16, 40 16 T 80 16 T 120 16 T 160 16 T 200 10 T 240 22 T 280 16;
                         M0 16 Q 20 16, 40 16 T 80 16 T 120 16 T 160 16 T 200 16 T 240 16 T 280 16"
                 keyTimes="0;0.5;1"
               />
             </path>
           </svg>
        </div>
      </div>

      <!-- 3. Key Metrics Grid (Col span 5) -->
      <div class="grid grid-cols-2 gap-4 lg:col-span-4">
        <!-- SLA -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between" :title="$t('admin.ops.status.slaTooltip')">
            <span class="text-[10px] font-bold uppercase text-gray-400">SLA</span>
            <span class="h-1.5 w-1.5 rounded-full" :class="overview?.sla.current && overview.sla.current >= 99.9 ? 'bg-green-500' : 'bg-yellow-500'"></span>
          </div>
          <div class="mt-1 text-xl font-black text-gray-900 dark:text-white">{{ overview?.sla.current.toFixed(3) }}%</div>
          <div class="mt-1 h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
             <div class="h-full bg-green-500 transition-all" :style="{ width: `${Math.max((overview?.sla.current || 0) - 90, 0) * 10}%` }"></div>
          </div>
        </div>

        <!-- P99 Latency -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between" :title="$t('admin.ops.status.p99Tooltip')">
            <span class="text-[10px] font-bold uppercase text-gray-400">P99 Latency</span>
            <span class="text-[10px] text-gray-400">ms</span>
          </div>
          <div class="mt-1 text-xl font-black text-gray-900 dark:text-white">{{ overview?.latency.p99 }}</div>
          <div class="mt-1 text-[10px] font-medium text-gray-500">Avg: {{ overview?.latency.avg }}ms</div>
        </div>

        <!-- Error Rate -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between" :title="$t('admin.ops.status.errorRateTooltip')">
            <span class="text-[10px] font-bold uppercase text-gray-400">Error Rate</span>
          </div>
          <div class="mt-1 text-xl font-black" :class="overview?.errors.error_rate && overview.errors.error_rate > 1 ? 'text-red-500' : 'text-gray-900 dark:text-white'">
            {{ overview?.errors.error_rate.toFixed(2) }}%
          </div>
          <div class="mt-1 text-[10px] font-medium text-gray-500">5xx: {{ overview?.errors['5xx_count'] }}</div>
        </div>

        <!-- Active Conns -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between" :title="$t('admin.ops.status.dbConnsTooltip')">
            <span class="text-[10px] font-bold uppercase text-gray-400">DB Conns</span>
          </div>
          <div class="mt-1 text-xl font-black text-gray-900 dark:text-white">{{ overview?.resources.db_connections.active }}</div>
          <div class="mt-1 text-[10px] font-medium text-gray-500">/ {{ overview?.resources.db_connections.max }} Max</div>
        </div>
      </div>
    </div>

    <!-- System Status Bar -->
    <div class="mt-2 flex items-center gap-2 overflow-x-auto border-t border-gray-100 pt-4 scrollbar-hide dark:border-dark-700">
      <div
        v-for="(status, component) in overview?.system_status"
        :key="component"
        class="flex flex-shrink-0 items-center gap-2 rounded-full px-3 py-1.5 text-xs font-bold transition-colors"
        :class="getStatusColor(status)"
        :title="$t('admin.ops.tooltips.systemStatus')"
      >
        <div class="flex items-center justify-center">
          <!-- Simple Icons based on component name -->
          <svg v-if="component === 'redis'" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z" />
          </svg>
          <svg v-else-if="component === 'database'" class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 7v10c0 2.21 3.582 4 8 4s8-1.79 8-4V7M4 7c0 2.21 3.582 4 8 4s8-1.79 8-4M4 7c0-2.21 3.582-4 8-4s8 1.79 8 4m0 5c0 2.21-3.582 4-8 4s-8-1.79-8-4" />
          </svg>
          <svg v-else class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
             <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
             <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </div>
        <span class="uppercase">{{ component }}</span>
        <span class="uppercase opacity-70">{{ status }}</span>
      </div>

      <!-- Add Goroutines and Memory as generic status pills -->
      <div 
        class="flex flex-shrink-0 items-center gap-2 rounded-full bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-600 dark:bg-dark-700 dark:text-gray-400"
        :title="$t('admin.ops.tooltips.memory')"
      >
        <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
        <span>MEM {{ overview?.resources.memory_usage }}%</span>
      </div>
      <div 
        class="flex flex-shrink-0 items-center gap-2 rounded-full bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-600 dark:bg-dark-700 dark:text-gray-400"
        :title="$t('admin.ops.tooltips.cpu')"
      >
         <svg class="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
           <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M7 19h10a2 2 0 002-2V7a2 2 0 00-2-2H7a2 2 0 00-2 2v10a2 2 0 002 2zM9 9h6v6H9V9z" />
         </svg>
        <span>CPU {{ overview?.resources.cpu_usage }}%</span>
      </div>
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

.scrollbar-hide::-webkit-scrollbar {
    display: none;
}
.scrollbar-hide {
    -ms-overflow-style: none;
    scrollbar-width: none;
}
</style>
