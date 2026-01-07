<script setup lang="ts">
import { computed, ref, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatNumber } from '@/utils/format'
import type { OpsDashboardOverview } from '@/api/admin/ops'
import type { OpsWSStatus } from '@/api/admin/ops'
import { adminAPI } from '@/api/admin' // Import for group list
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const helpUrl = 'https://github.com/Wei-Shaw/sub2api#readme'

interface Props {
  overview: OpsDashboardOverview | null
  wsStatus: OpsWSStatus
  wsReconnectInMs?: number | null
  realTimeQPS: number
  realTimeTPS: number
  platform: string
  groupId: number | null
  timeRange: string
  loading: boolean
  lastUpdated: Date
}

interface Emits {
  (e: 'update:timeRange', value: string): void
  (e: 'refresh'): void
  // Add new filter events
  (e: 'update:platform', value: string): void
  (e: 'update:group', value: number | null): void
  (e: 'openRequestDetails', preset: RequestDetailsPreset): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

type RequestDetailsPreset = {
  title: string
  kind?: 'success' | 'error' | 'all'
  sort?: 'created_at_desc' | 'duration_desc'
  min_duration_ms?: number
  max_duration_ms?: number
}

// --- Global Filters ---
const groups = ref<Array<{ id: number, name: string }>>([])

// Platform options
const platformOptions = computed(() => [
  { value: '', label: t('admin.ops.filters.allPlatforms') },
  { value: 'openai', label: 'OpenAI' },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' }
])

// Time range options
const timeRangeOptions = computed(() => [
  { value: '5m', label: t('admin.ops.timeRange.5m') },
  { value: '30m', label: t('admin.ops.timeRange.30m') },
  { value: '1h', label: t('admin.ops.timeRange.1h') },
  { value: '6h', label: t('admin.ops.timeRange.6h') },
  { value: '24h', label: t('admin.ops.timeRange.24h') }
])

// Group options computed
const groupOptions = computed(() => [
  { value: null, label: t('admin.ops.filters.allGroups') },
  ...groups.value.map(g => ({ value: g.id, label: g.name }))
])

onMounted(async () => {
  try {
    // Fetch simple group list for filter
    const res = await adminAPI.groups.list(1, 100)
    groups.value = res.items.map(g => ({ id: g.id, name: g.name }))
  } catch (e) {
    console.error('Failed to load groups for filter', e)
  }
})

function handlePlatformChange(val: string | number | boolean | null) {
  emit('update:platform', String(val || ''))
}

function handleGroupChange(val: string | number | boolean | null) {
  const id = val ? Number(val) : null
  emit('update:group', id)
}

function handleTimeRangeChange(val: string | number | boolean | null) {
  emit('update:timeRange', String(val || '5m'))
}

// --- 视觉辅助逻辑 ---

const isSystemIdle = computed(() => {
  const ov = props.overview
  if (!ov) return true
  // 如果 QPS 为 0 且 错误率 为 0，视为待机
  // 注意：某些情况下 qps.current 可能是瞬时值，也可以结合 total_requests 判断，但这里用实时 QPS 比较直观
  return (props.realTimeQPS || ov.qps.current) === 0 && ov.errors.error_rate === 0
})

const healthScoreColor = computed(() => {
  if (isSystemIdle.value) return '#9ca3af' // gray-400 for Idle
  
  const score = props.overview?.health_score || 0
  if (score >= 90) return '#10b981' // green
  if (score >= 60) return '#f59e0b' // yellow
  return '#ef4444' // red
})

const healthScoreClass = computed(() => {
  if (isSystemIdle.value) return 'text-gray-400'
  
  const score = props.overview?.health_score || 0
  if (score >= 90) return 'text-green-500'
  if (score >= 60) return 'text-yellow-500'
  return 'text-red-500'
})

// SVG Circle properties
const circleSize = 100
const strokeWidth = 8
const radius = (circleSize - strokeWidth) / 2
const circumference = 2 * Math.PI * radius
const dashOffset = computed(() => {
  if (isSystemIdle.value) return 0 // Full circle for idle
  const score = props.overview?.health_score || 0
  return circumference - (score / 100) * circumference
})

// --- 智能诊断引擎 (Frontend Diagnosis) ---
// Tuned for LLM Gateway context
interface DiagnosisItem {
  type: 'critical' | 'warning' | 'info'
  message: string
  impact: string
}

const diagnosisReport = computed<DiagnosisItem[]>(() => {
  const ov = props.overview
  if (!ov) return []
  
  const report: DiagnosisItem[] = []

  // 0. 待机检测
  if (isSystemIdle.value) {
    report.push({
      type: 'info',
      message: t('admin.ops.diagnosis.items.idle.message'),
      impact: t('admin.ops.diagnosis.items.idle.impact')
    })
    return report
  }
  
  // 1. 错误率检查 (Upstream Error Rate)
  // LLM API 波动大，3% 以内可接受
  if (ov.errors.error_rate > 10) {
    report.push({
      type: 'critical',
      message: t('admin.ops.diagnosis.items.upstreamErrorVeryHigh.message'),
      impact: t('admin.ops.diagnosis.items.upstreamErrorVeryHigh.impact')
    })
  } else if (ov.errors.error_rate > 3) {
    report.push({
      type: 'warning',
      message: t('admin.ops.diagnosis.items.upstreamErrorHigh.message'),
      impact: t('admin.ops.diagnosis.items.upstreamErrorHigh.impact')
    })
  }

  // 2. SLA 检查 (User Success Rate)
  if (ov.sla.current < 90.0) {
    report.push({
      type: 'critical',
      message: t('admin.ops.diagnosis.items.slaCritical.message'),
      impact: t('admin.ops.diagnosis.items.slaCritical.impact')
    })
  } else if (ov.sla.current < 98.0) {
    report.push({
      type: 'warning',
      message: t('admin.ops.diagnosis.items.slaWarning.message'),
      impact: t('admin.ops.diagnosis.items.slaWarning.impact')
    })
  }

  // 3. 延迟检查
  // LLM 响应本来就慢，8s 以内算正常，超过 20s 才是异常
  if (ov.latency.p99 > 20000) {
    report.push({
      type: 'warning',
      message: t('admin.ops.diagnosis.items.p99VeryHigh.message'),
      impact: t('admin.ops.diagnosis.items.p99VeryHigh.impact')
    })
  } else if (ov.latency.p99 > 8000) {
    // 8s-20s 对于流式输出来说是可接受的，作为提示即可
    report.push({
      type: 'info',
      message: t('admin.ops.diagnosis.items.p99High.message'),
      impact: t('admin.ops.diagnosis.items.p99High.impact')
    })
  }

  // 4. 资源水位检查
  const dbUsage = ov.resources.db_connections.active / Math.max(ov.resources.db_connections.max, 1)
  if (dbUsage > 0.9) {
    report.push({
      type: 'critical',
      message: t('admin.ops.diagnosis.items.dbConnsExhausted.message'),
      impact: t('admin.ops.diagnosis.items.dbConnsExhausted.impact')
    })
  }

  // 5. 健康加分项
  if (report.length === 0) {
    report.push({
      type: 'info',
      message: t('admin.ops.diagnosis.items.stable.message'),
      impact: t('admin.ops.diagnosis.items.stable.impact')
    })
  }

  return report
})

const displayRealTimeQPS = computed(() => {
  if (props.wsStatus === 'connected' && props.realTimeQPS > 0) return props.realTimeQPS
  return props.overview?.qps.current ?? 0
})

const displayRealTimeTPS = computed(() => {
  if (props.wsStatus === 'connected' && props.realTimeTPS > 0) return props.realTimeTPS
  return props.overview?.tps.current ?? 0
})

const wsStatusLabel = computed(() => {
  if (props.wsStatus === 'connected') return t('admin.ops.status.online')
  if (props.wsStatus === 'reconnecting') return t('admin.ops.status.reconnecting')
  if (props.wsStatus === 'connecting') return t('admin.ops.status.connecting')
  if (props.wsStatus === 'offline') return t('admin.ops.status.offline')
  return t('admin.ops.status.offline')
})

const wsStatusTitle = computed(() => {
  if (props.wsStatus === 'connected') return t('admin.ops.status.wsConnected')
  if (props.wsStatus === 'reconnecting') return t('admin.ops.status.wsReconnecting')
  if (props.wsStatus === 'connecting') return t('admin.ops.status.wsConnecting')
  if (props.wsStatus === 'offline') return t('admin.ops.status.wsDisconnected')
  return t('admin.ops.status.wsDisconnected')
})

const wsDotClass = computed(() => {
  if (props.wsStatus === 'connected') return 'bg-green-500'
  if (props.wsStatus === 'reconnecting' || props.wsStatus === 'connecting') return 'bg-amber-500'
  return 'bg-red-500'
})

const getStatusColor = (status: string | undefined) => {
  if (!status) return 'bg-gray-200 text-gray-500 dark:bg-dark-700 dark:text-gray-400'
  const s = status.toLowerCase()
  if (s === 'operational' || s === 'healthy' || s === 'running')
    return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-400'
  if (s === 'degraded' || s === 'warning')
    return 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/30 dark:text-yellow-400'
  return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400'
}

const getComponentName = (component: string) => {
  const key = `admin.ops.systemComponents.${component}`
  return t(key) !== key ? t(key) : component.toUpperCase()
}

const getStatusName = (status: string) => {
  const key = `admin.ops.systemStatus.${status.toLowerCase()}`
  return t(key) !== key ? t(key) : status.toUpperCase()
}

function openDetails(preset: RequestDetailsPreset) {
  emit('openRequestDetails', preset)
}
</script>

<template>
  <div class="flex flex-col gap-4 rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <!-- Top Toolbar -->
    <div class="flex flex-wrap items-center justify-between gap-4 border-b border-gray-100 pb-4 dark:border-dark-700">
      <div>
        <h1 class="flex items-center gap-2 text-xl font-black text-gray-900 dark:text-white">
          <svg class="h-6 w-6 text-blue-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
          </svg>
          {{ $t('admin.ops.title') }}
        </h1>
        <div class="mt-1 flex items-center gap-3 text-xs text-gray-500">
          <span class="flex items-center gap-1.5" :title="wsStatusTitle">
            <span class="relative flex h-2 w-2">
              <span v-if="wsStatus === 'connected'" class="absolute inline-flex h-full w-full animate-ping rounded-full bg-green-400 opacity-75"></span>
              <span class="relative inline-flex h-2 w-2 rounded-full" :class="wsDotClass"></span>
            </span>
            {{ wsStatusLabel }}
            <span v-if="wsStatus === 'reconnecting' && wsReconnectInMs" class="text-[10px] font-semibold text-gray-400">
              ({{ Math.max(1, Math.round(wsReconnectInMs / 1000)) }}s)
            </span>
          </span>
          <span>·</span>
          <span>{{ $t('admin.ops.status.updatedAt') }} {{ lastUpdated.toLocaleTimeString() }}</span>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-3">
        <a
          :href="helpUrl"
          target="_blank"
          rel="noopener noreferrer"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
          :title="t('admin.ops.status.help')"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.02 2.575-2.5 2.9-.98.215-1.5.792-1.5 1.6V16m0 3h.01M12 21a9 9 0 110-18 9 9 0 010 18z" />
          </svg>
        </a>

        <Select
          :model-value="platform"
          :options="platformOptions"
          @change="handlePlatformChange"
          class="w-full sm:w-[140px]"
        />

        <Select
          :model-value="groupId"
          :options="groupOptions"
          @change="handleGroupChange"
          class="w-full sm:w-[140px]"
        />

        <div class="mx-1 hidden h-4 w-[1px] bg-gray-200 dark:bg-dark-700 sm:block"></div>

        <Select
          :model-value="timeRange"
          :options="timeRangeOptions"
          @change="handleTimeRangeChange"
          class="relative w-full sm:w-[140px]"
        />

        <button
          @click="emit('refresh')"
          :disabled="loading"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-gray-100 text-gray-500 transition-colors hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-400 dark:hover:bg-dark-600"
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
      <!-- 1. Health Score Diagnosis Card (Col span 3) -->
      <div 
        class="group relative flex flex-col items-center justify-center border-r border-gray-100 py-2 transition-all hover:bg-gray-50 dark:border-dark-700 dark:hover:bg-dark-700/50 lg:col-span-3 cursor-pointer rounded-xl"
      >
        <!-- Hover Popover: Diagnosis Report -->
        <div class="pointer-events-none absolute left-full top-0 z-50 ml-2 w-72 opacity-0 transition-opacity duration-200 group-hover:pointer-events-auto group-hover:opacity-100">
           <div class="rounded-xl bg-white p-4 shadow-xl ring-1 ring-black/5 dark:bg-gray-800 dark:ring-white/10">
             <h4 class="mb-3 border-b border-gray-100 pb-2 text-sm font-bold text-gray-900 dark:border-gray-700 dark:text-white">
               {{ t('admin.ops.diagnosis.title') }}
             </h4>
             <div class="space-y-3">
               <div v-for="(item, idx) in diagnosisReport" :key="idx" class="flex gap-3">
                 <div class="mt-0.5 shrink-0">
                   <svg v-if="item.type === 'critical'" class="h-4 w-4 text-red-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z" clip-rule="evenodd" /></svg>
                   <svg v-else-if="item.type === 'warning'" class="h-4 w-4 text-yellow-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M8.257 3.099c.765-1.36 2.722-1.36 3.486 0l5.58 9.92c.75 1.334-.213 2.98-1.742 2.98H4.42c-1.53 0-2.493-1.646-1.743-2.98l5.58-9.92zM11 13a1 1 0 11-2 0 1 1 0 012 0zm-1-8a1 1 0 00-1 1v3a1 1 0 002 0V6a1 1 0 00-1-1z" clip-rule="evenodd" /></svg>
                   <svg v-else-if="item.type === 'info'" class="h-4 w-4 text-blue-500" fill="currentColor" viewBox="0 0 20 20"><path fill-rule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7-4a1 1 0 11-2 0 1 1 0 012 0zM9 9a1 1 0 000 2v3a1 1 0 001 1h1a1 1 0 100-2v-3a1 1 0 00-1-1H9z" clip-rule="evenodd" /></svg>
                 </div>
                 <div>
                   <div class="text-xs font-medium text-gray-900 dark:text-white">{{ item.message }}</div>
                   <div class="text-[10px] text-gray-500 dark:text-gray-400">{{ item.impact }}</div>
                 </div>
               </div>
             </div>
             <div class="mt-3 border-t border-gray-100 pt-2 text-[10px] text-gray-400 dark:border-gray-700">
               {{ t('admin.ops.diagnosis.footer') }}
             </div>
           </div>
        </div>

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
            <span class="text-3xl font-black" :class="healthScoreClass">{{ isSystemIdle ? t('admin.ops.labels.idle') : (overview?.health_score || '--') }}</span>
            <span class="text-[10px] font-bold uppercase tracking-wider text-gray-400">{{ t('admin.ops.labels.health') }}</span>
          </div>
        </div>
        <div class="mt-4 text-center">
          <div class="flex items-center justify-center gap-1 text-xs font-medium text-gray-500">
            {{ $t('admin.ops.status.healthCondition') }}
            <!-- Hint Icon -->
            <svg class="h-3.5 w-3.5 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          </div>
          <div class="mt-1 text-xs font-bold" :class="healthScoreClass">
            {{ isSystemIdle ? t('admin.ops.status.idle') : (overview?.health_score && overview.health_score >= 90 ? t('admin.ops.status.healthy') : t('admin.ops.status.risky')) }}
          </div>
        </div>
      </div>

      <!-- 2. Real-time Pulse (Col span 4) -->
      <div class="flex flex-col justify-center border-r border-gray-100 px-4 py-2 dark:border-dark-700 lg:col-span-5">
        <div class="mb-2 flex items-center justify-between gap-2">
          <div class="relative flex h-3 w-3">
            <span class="absolute inline-flex h-full w-full animate-ping rounded-full bg-blue-400 opacity-75"></span>
            <span class="relative inline-flex h-3 w-3 rounded-full bg-blue-500"></span>
          </div>
          <h3 class="flex-1 text-xs font-bold uppercase tracking-wider text-gray-400">{{ $t('admin.ops.status.trafficPulse') }}</h3>
          <button
            class="text-[10px] font-bold text-blue-500 hover:underline"
            @click="openDetails({ title: t('admin.ops.status.trafficPulse'), kind: 'all', sort: 'created_at_desc' })"
          >
            {{ t('admin.ops.requestDetails.details') }}
          </button>
        </div>

        <div class="flex items-baseline gap-1">
          <span class="text-4xl font-black text-gray-900 dark:text-white">{{ displayRealTimeQPS.toFixed(1) }}</span>
          <span class="text-sm font-bold text-gray-500">QPS</span>
          <HelpTooltip :content="t('admin.ops.tooltips.realtimeQPS')" />
        </div>
        
        <div class="mt-1 flex items-center gap-4 text-xs font-medium text-gray-500">
          <span class="flex items-center">
            TPS: {{ formatNumber(displayRealTimeTPS) }}
            <HelpTooltip :content="t('admin.ops.tooltips.realtimeTPS')" />
          </span>
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
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1">
              <span class="text-[10px] font-bold uppercase text-gray-400">SLA</span>
              <HelpTooltip :content="t('admin.ops.status.slaTooltip')" />
            </div>
            <button
              class="text-[10px] font-bold text-blue-500 hover:underline"
              @click="openDetails({ title: t('admin.ops.metrics.sla'), kind: 'all', sort: 'created_at_desc' })"
            >
              {{ t('admin.ops.requestDetails.details') }}
            </button>
            <span class="h-1.5 w-1.5 rounded-full" :class="overview?.sla.current && overview.sla.current >= 99.9 ? 'bg-green-500' : 'bg-yellow-500'"></span>
          </div>
          <div class="mt-1 text-xl font-black text-gray-900 dark:text-white">{{ overview?.sla.current.toFixed(3) }}%</div>
          <div class="mt-1 h-1 w-full overflow-hidden rounded-full bg-gray-200 dark:bg-dark-700">
             <div class="h-full bg-green-500 transition-all" :style="{ width: `${Math.max((overview?.sla.current || 0) - 90, 0) * 10}%` }"></div>
          </div>
        </div>

        <!-- P99 Latency -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1">
              <span class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.labels.p99Latency') }}</span>
              <HelpTooltip :content="t('admin.ops.status.p99Tooltip')" />
            </div>
            <button
              class="text-[10px] font-bold text-blue-500 hover:underline"
              @click="
                openDetails({
                  title: t('admin.ops.labels.p99Latency'),
                  kind: 'all',
                  sort: 'duration_desc',
                  min_duration_ms: Math.max(Number(overview?.latency?.p99 ?? 0), 0)
                })
              "
            >
              {{ t('admin.ops.requestDetails.details') }}
            </button>
            <span class="text-[10px] text-gray-400">ms</span>
          </div>
          <div class="flex items-baseline gap-2 mt-1">
            <div class="text-xl font-black text-gray-900 dark:text-white">{{ overview?.latency.p99 }} <span class="text-[10px] text-gray-400 font-normal">P99</span></div>
          </div>
          <div class="mt-1 flex items-center justify-end gap-2 text-[10px] font-medium text-gray-500">
            <span>P95: {{ overview?.latency.p95 }}</span>
            <span class="text-gray-300">|</span>
            <span>P90: {{ overview?.latency.p50 }}</span>
          </div>
        </div>

        <!-- Error Rate -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1">
              <span class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.labels.errorRate') }}</span>
              <HelpTooltip :content="t('admin.ops.status.errorRateTooltip')" />
            </div>
            <button
              class="text-[10px] font-bold text-blue-500 hover:underline"
              @click="openDetails({ title: t('admin.ops.labels.errorRate'), kind: 'error', sort: 'created_at_desc' })"
            >
              {{ t('admin.ops.requestDetails.details') }}
            </button>
          </div>
          <div class="mt-1 text-xl font-black" :class="overview?.errors.error_rate && overview.errors.error_rate > 5 ? 'text-red-500' : 'text-gray-900 dark:text-white'">
            {{ overview?.errors.error_rate.toFixed(2) }}%
          </div>
          <div class="mt-1 text-[10px] font-medium text-gray-500">5xx: {{ overview?.errors['5xx_count'] }}</div>
        </div>

        <!-- Active Conns -->
        <div class="rounded-xl bg-gray-50 p-3 dark:bg-dark-900">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-1">
              <span class="text-[10px] font-bold uppercase text-gray-400">{{ t('admin.ops.labels.dbConns') }}</span>
              <HelpTooltip :content="t('admin.ops.status.dbConnsTooltip')" />
            </div>
          </div>
          <div class="mt-1 text-xl font-black text-gray-900 dark:text-white">{{ overview?.resources.db_connections.active }}</div>
          <div class="mt-1 text-[10px] font-medium text-gray-500">/ {{ overview?.resources.db_connections.max }} {{ t('admin.ops.labels.max') }}</div>
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
             <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37.996.608 2.296.07 2.572-1.065z" />
             <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
        </div>
        <span class="uppercase">{{ getComponentName(component) }}</span>
        <span class="uppercase opacity-70">{{ getStatusName(status) }}</span>
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
