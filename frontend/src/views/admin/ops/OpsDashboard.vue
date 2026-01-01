<template>
  <AppLayout>
    <div class="space-y-6">
      <!-- Top Status Bar -->
      <div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div class="flex items-center gap-4">
            <!-- Health Score Ring -->
            <div
              class="relative flex h-16 w-16 items-center justify-center rounded-full border-4 bg-white dark:bg-dark-800"
              :class="healthRingBorderClass"
            >
              <svg class="absolute h-full w-full -rotate-90 transform" viewBox="0 0 36 36">
                <!-- Background Circle -->
                <path
                  class="text-gray-100 dark:text-dark-700"
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                />
                <!-- Progress Circle -->
                <path
                  class="drop-shadow-sm"
                  :class="healthRingProgressClass"
                  :stroke-dasharray="`${healthScoreRing}, 100`"
                  d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                  fill="none"
                  stroke="currentColor"
                  stroke-width="3"
                />
              </svg>
              <div class="flex flex-col items-center">
                <span class="text-sm font-bold text-gray-900 dark:text-white">{{ healthScoreText }}</span>
                <span class="text-[10px] text-gray-400">SCORE</span>
              </div>
            </div>

            <div>
              <h2 class="text-lg font-bold text-gray-900 dark:text-white">
                {{ healthStatusText }}
              </h2>
              <div class="flex items-center gap-2">
                <span class="relative flex h-2.5 w-2.5">
                  <span class="absolute inline-flex h-full w-full animate-ping rounded-full opacity-75" :class="healthDotPingClass"></span>
                  <span class="relative inline-flex h-2.5 w-2.5 rounded-full" :class="healthDotClass"></span>
                </span>
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.status.monitoring') }} • {{ lastUpdatedText }}
                </span>
              </div>
            </div>
          </div>

          <div class="flex items-center gap-3">
            <button
              class="btn btn-secondary btn-sm gap-2"
              @click="refreshData"
              :disabled="loading"
            >
              <svg
                class="h-4 w-4"
                :class="{ 'animate-spin': loading }"
                fill="none"
                viewBox="0 0 24 24"
                stroke="currentColor"
              >
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
              </svg>
              {{ t('common.refresh') }}
            </button>
          </div>
        </div>
      </div>

      <!-- Metrics Grid -->
      <div class="grid grid-cols-2 gap-4 lg:grid-cols-4">
        <StatCard
          v-for="card in statCards"
          :key="card.key"
          :title="card.title"
          :value="card.value"
          :icon="card.icon"
          :icon-variant="card.iconVariant"
        />
      </div>

      <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
        <!-- Main Chart: Error Trend -->
        <div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700 lg:col-span-2">
          <div class="mb-4 flex items-center justify-between">
            <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.charts.errorTrend') }}</h3>
            <Select v-model="timeRange" :options="timeRangeOptions" class="w-32 !text-xs" />
          </div>
          <div class="h-64">
            <Line v-if="chartData" :data="chartData" :options="chartOptions" />
          </div>
        </div>

        <!-- Secondary: Distribution -->
        <div class="rounded-xl bg-white p-5 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
           <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.charts.errorDistribution') }}</h3>
           <div class="space-y-4">
             <div v-for="(item, idx) in errorDistribution" :key="idx" class="space-y-1">
               <div class="flex justify-between text-xs">
                 <span class="font-medium text-gray-700 dark:text-gray-300">{{ item.label }}</span>
                 <span class="text-gray-500">{{ item.percentage }}%</span>
               </div>
               <div class="h-2 w-full overflow-hidden rounded-full bg-gray-100 dark:bg-dark-700">
                 <div
                    class="h-full rounded-full"
                    :class="item.color"
                    :style="{ width: `${item.percentage}%` }"
                 ></div>
               </div>
             </div>
           </div>
        </div>
      </div>

      <!-- Error Logs Table -->
      <div class="rounded-xl bg-white shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
        <div class="border-b border-gray-100 p-4 dark:border-dark-700">
          <div class="flex flex-wrap items-center justify-between gap-3">
             <div class="flex items-center gap-2">
               <div class="flex h-8 w-8 items-center justify-center rounded-lg bg-red-50 text-red-600 dark:bg-red-900/20 dark:text-red-400">
                 <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                   <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                 </svg>
               </div>
               <div>
                 <h3 class="text-sm font-semibold text-gray-900 dark:text-white">{{ t('admin.ops.errors.title') }}</h3>
                 <p class="text-xs text-gray-500 dark:text-gray-400">
                   {{ t('admin.ops.errors.subtitle') }} • {{ t('admin.ops.errors.count', { n: filteredErrors.length }) }}
                 </p>
               </div>
             </div>

             <div class="flex gap-2">
                <input
                  v-model="searchQuery"
                  class="input text-xs"
                  :placeholder="t('admin.ops.searchPlaceholder')"
                />
                <Select v-model="platformFilter" :options="platformOptions" class="w-28 !text-xs" />
                <Select v-model="phaseFilter" :options="phaseOptions" class="w-28 !text-xs" />
                <Select v-model="severityFilter" :options="severityOptions" class="w-28 !text-xs" />
             </div>
          </div>
        </div>

        <DataTable :columns="errorColumns" :data="filteredErrors" :loading="loading">
          <template #cell-created_at="{ value }">
            <span class="font-mono text-xs text-gray-500">
              {{ formatTimeOnly(value) }}
            </span>
          </template>

          <template #cell-severity="{ value }">
             <span :class="['inline-flex items-center rounded px-1.5 py-0.5 text-[10px] font-bold uppercase tracking-wide', severityBadgeClass(value)]">
                {{ value }}
             </span>
          </template>

          <template #cell-phase="{ value }">
            <span class="text-xs font-medium text-gray-700 dark:text-gray-300">
              {{ t(`admin.ops.phase.${value}`) }}
            </span>
          </template>

          <template #cell-platform="{ value }">
             <div class="flex items-center gap-1.5">
               <div class="h-1.5 w-1.5 rounded-full" :class="platformColorClass(value)"></div>
               <span class="text-xs font-medium text-gray-700 dark:text-gray-300">{{ platformLabel(value) }}</span>
             </div>
          </template>

          <template #cell-status_code="{ value }">
             <span class="font-mono text-xs font-bold" :class="statusColorClass(value)">
                {{ value }}
             </span>
          </template>

          <template #cell-latency_ms="{ value }">
            <span class="font-mono text-xs text-gray-600 dark:text-gray-400">
              {{ formatLatency(value) }}
            </span>
          </template>

          <template #cell-request_id="{ value }">
             <div class="group flex items-center gap-1">
               <span class="font-mono text-[10px] text-gray-500">{{ shortenId(value) }}</span>
                 <button
                   class="invisible p-0.5 text-gray-400 hover:text-gray-600 group-hover:visible dark:hover:text-gray-300"
                   @click.stop="copyToClipboard(value)"
                   :title="t('common.copy')"
                 >
                 <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                   <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                 </svg>
               </button>
             </div>
          </template>

          <template #cell-message="{ value }">
             <span class="line-clamp-1 max-w-[200px] text-xs font-medium text-gray-900 dark:text-gray-200" :title="value">
                {{ value }}
             </span>
          </template>

          <template #cell-actions="{ row }">
             <button
               class="flex items-center gap-1 rounded px-2 py-1 text-xs font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-gray-400 dark:hover:bg-dark-700 dark:hover:text-white"
               @click="openDetails(row)"
             >
               {{ t('common.details') }}
               <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                 <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7" />
               </svg>
             </button>
          </template>

          <template #empty>
            <div class="flex flex-col items-center">
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">
                {{ t('admin.ops.empty.title') }}
              </p>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.empty.subtitle') }}
              </p>
            </div>
          </template>
        </DataTable>
      </div>
    </div>

    <!-- Details Drawer (Slide-over) -->
    <div v-if="selectedError" class="fixed inset-0 z-50 overflow-hidden" aria-labelledby="slide-over-title" role="dialog" aria-modal="true">
      <div class="absolute inset-0 bg-gray-500 bg-opacity-75 transition-opacity" @click="closeDetails"></div>
      <div class="fixed inset-y-0 right-0 flex max-w-full pl-10">
        <div class="pointer-events-auto w-screen max-w-md transform transition-transform">
          <div class="flex h-full flex-col overflow-y-scroll bg-white shadow-xl dark:bg-dark-800">
            <div class="bg-gray-50 px-4 py-6 dark:bg-dark-900 sm:px-6">
              <div class="flex items-start justify-between">
                <h2 class="text-lg font-medium text-gray-900 dark:text-white" id="slide-over-title">
                  {{ t('admin.ops.details.title') }}
                </h2>
                <div class="ml-3 flex h-7 items-center">
                  <button type="button" class="rounded-md bg-transparent text-gray-400 hover:text-gray-500 focus:outline-none" @click="closeDetails">
                    <span class="sr-only">Close panel</span>
                    <svg class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                  </button>
                </div>
              </div>
              <div class="mt-1">
                 <p class="text-sm text-gray-500 dark:text-gray-400">
                   {{ t('admin.ops.details.requestId') }}:
                   <span class="font-mono select-all text-gray-900 dark:text-white">{{ selectedError.request_id }}</span>
                 </p>
              </div>
            </div>
            <div class="relative flex-1 px-4 py-6 sm:px-6">
              <!-- Content -->
              <div class="space-y-6">
                <!-- Status Badge -->
                <div class="flex items-center gap-3">
                   <span :class="['rounded px-2.5 py-1 text-sm font-bold', severityBadgeClass(selectedError.severity)]">
                      {{ selectedError.severity }}
                   </span>
                   <span :class="['rounded px-2.5 py-1 text-sm font-mono font-bold', statusColorClass(selectedError.status_code)]">
                      {{ selectedError.status_code }}
                   </span>
                   <span class="text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.type }}</span>
                </div>

                <!-- Error Message -->
                <div class="rounded-lg bg-red-50 p-4 dark:bg-red-900/10">
                   <h4 class="mb-1 text-xs font-bold uppercase tracking-wider text-red-800 dark:text-red-300">
                     {{ t('admin.ops.details.errorMessage') }}
                   </h4>
                   <p class="text-sm text-red-900 dark:text-red-200">{{ selectedError.message }}</p>
                </div>

                <!-- Metadata -->
                <dl class="grid grid-cols-2 gap-x-4 gap-y-4 rounded-lg border border-gray-100 p-4 dark:border-dark-700">
                  <div>
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.table.platform') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ platformLabel(selectedError.platform) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.table.model') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.model }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.table.latency') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ formatLatency(selectedError.latency_ms) }}</dd>
                  </div>
                  <div>
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.table.phase') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.phase }}</dd>
                  </div>
                  <div v-if="selectedError.request_path">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.requestPath') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.request_path }}</dd>
                  </div>
                  <div v-if="selectedError.client_ip">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.clientIp') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.client_ip }}</dd>
                  </div>
                  <div v-if="selectedError.user_id !== undefined && selectedError.user_id !== null">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.userId') }}</dt>
                    <dd class="mt-1 text-sm font-mono font-bold text-gray-900 dark:text-white">{{ selectedError.user_id }}</dd>
                  </div>
                  <div v-if="selectedError.api_key_id !== undefined && selectedError.api_key_id !== null">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.apiKeyId') }}</dt>
                    <dd class="mt-1 text-sm font-mono font-bold text-gray-900 dark:text-white">{{ selectedError.api_key_id }}</dd>
                  </div>
                  <div v-if="selectedError.group_id !== undefined && selectedError.group_id !== null">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.groupId') }}</dt>
                    <dd class="mt-1 text-sm font-mono font-bold text-gray-900 dark:text-white">{{ selectedError.group_id }}</dd>
                  </div>
                  <div v-if="selectedError.stream !== undefined">
                    <dt class="text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('admin.ops.details.stream') }}</dt>
                    <dd class="mt-1 text-sm font-medium text-gray-900 dark:text-white">{{ selectedError.stream ? t('common.yes') : t('common.no') }}</dd>
                  </div>
                </dl>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

  </AppLayout>
</template>

<script setup lang="ts">
import { computed, ref, h, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import StatCard from '@/components/common/StatCard.vue'
import DataTable from '@/components/common/DataTable.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import { formatNumber, formatRelativeTime } from '@/utils/format'
import { opsAPI, type OpsErrorLog, type OpsMetrics } from '@/api/admin/ops'
import { useAppStore } from '@/stores/app'

// Chart.js
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
} from 'chart.js'
import { Line } from 'vue-chartjs'

ChartJS.register(
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler
)

const { t } = useI18n()
const appStore = useAppStore()

type Platform = 'gemini' | 'openai' | 'anthropic' | 'antigravity'
type Severity = 'P0' | 'P1' | 'P2' | 'P3'
type OpsError = OpsErrorLog
type IconVariant = 'primary' | 'success' | 'warning' | 'danger'

// --- State ---
const loading = ref(false)
const selectedError = ref<OpsError | null>(null)
const timeRange = ref<'15m' | '1h' | '24h' | '7d'>('1h')
const severityFilter = ref<'all' | Severity>('all')
const platformFilter = ref<'all' | Platform>('all')
const phaseFilter = ref<
  | 'all'
  | 'auth'
  | 'concurrency'
  | 'billing'
  | 'scheduling'
  | 'network'
  | 'upstream'
  | 'response'
  | 'internal'
>('all')
const searchQuery = ref('')
const metrics = ref<OpsMetrics | null>(null)
const metricsHistory = ref<OpsMetrics[]>([])
const errors = ref<OpsError[]>([])

let autoRefreshTimer: number | null = null

const lookbackMinutes = (range: string): number => {
  switch (range) {
    case '15m':
      return 15
    case '1h':
      return 60
    case '24h':
      return 60 * 24
    case '7d':
      return 60 * 24 * 7
    default:
      return 60
  }
}

const hasTraffic = computed(() => (metrics.value?.request_count ?? 0) > 0)

const healthScoreValue = computed<number | null>(() => {
  if (!metrics.value || !hasTraffic.value) return null
  const raw = metrics.value.success_rate
  if (!Number.isFinite(raw)) return null
  return Math.round(Math.max(0, Math.min(100, raw)))
})

const healthScoreRing = computed(() => healthScoreValue.value ?? 0)
const healthScoreText = computed(() => (healthScoreValue.value === null ? '--' : String(healthScoreValue.value)))

const healthVariant = computed<'neutral' | 'success' | 'warning' | 'danger'>(() => {
  const score = healthScoreValue.value
  if (score === null) return 'neutral'
  if (score >= 99) return 'success'
  if (score >= 95) return 'warning'
  return 'danger'
})

const healthStatusText = computed(() => {
  if (healthVariant.value === 'neutral') return t('admin.ops.status.noData')
  if (healthVariant.value === 'success') return t('admin.ops.status.systemNormal')
  if (healthVariant.value === 'warning') return t('admin.ops.status.systemDegraded')
  return t('admin.ops.status.systemDown')
})

const healthRingBorderClass = computed(() => {
  switch (healthVariant.value) {
    case 'success':
      return 'border-emerald-50 dark:border-emerald-900/20'
    case 'warning':
      return 'border-amber-50 dark:border-amber-900/20'
    case 'danger':
      return 'border-red-50 dark:border-red-900/20'
    default:
      return 'border-gray-100 dark:border-dark-700'
  }
})

const healthRingProgressClass = computed(() => {
  switch (healthVariant.value) {
    case 'success':
      return 'text-emerald-500'
    case 'warning':
      return 'text-amber-500'
    case 'danger':
      return 'text-red-500'
    default:
      return 'text-gray-300 dark:text-dark-600'
  }
})

const healthDotClass = computed(() => {
  switch (healthVariant.value) {
    case 'success':
      return 'bg-emerald-500'
    case 'warning':
      return 'bg-amber-500'
    case 'danger':
      return 'bg-red-500'
    default:
      return 'bg-gray-400'
  }
})

const healthDotPingClass = computed(() => {
  switch (healthVariant.value) {
    case 'success':
      return 'bg-emerald-400'
    case 'warning':
      return 'bg-amber-400'
    case 'danger':
      return 'bg-red-400'
    default:
      return 'bg-gray-300'
  }
})

const lastUpdatedText = computed(() => {
  if (metrics.value?.updated_at) return formatRelativeTime(metrics.value.updated_at)
  return t('admin.ops.status.waiting')
})

const chartOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: true, position: 'top' as const, align: 'end' as const },
    tooltip: { mode: 'index' as const, intersect: false }
  },
  scales: {
    y: { beginAtZero: true, suggestedMax: 100, grid: { color: 'rgba(0,0,0,0.05)' }, ticks: { callback: (v: any) => `${v}%` } },
    y1: { beginAtZero: true, position: 'right' as const, grid: { display: false } },
    x: { grid: { display: false } }
  }
}

const chartData = computed(() => {
  const items = metricsHistory.value ?? []
  const labels = items.map((m) => formatTimeLabel(m.updated_at))
  const errorRate = items.map((m) => {
    if (!Number.isFinite(m.request_count) || m.request_count <= 0) return null
    if (!Number.isFinite(m.error_rate)) return null
    return m.error_rate
  })
  const reqCount = items.map((m) => (Number.isFinite(m.request_count) ? m.request_count : 0))

  return {
    labels,
    datasets: [
      {
        label: t('admin.ops.charts.errorRate'),
        data: errorRate,
        yAxisID: 'y',
        borderColor: '#ef4444',
        backgroundColor: 'rgba(239, 68, 68, 0.1)',
        tension: 0.35,
        fill: true
      },
      {
        label: t('admin.ops.charts.requestCount'),
        data: reqCount,
        yAxisID: 'y1',
        borderColor: '#10b981',
        backgroundColor: 'transparent',
        borderDash: [4, 4],
        tension: 0.35
      }
    ]
  }
})

const errorDistribution = computed(() => {
  const items = errors.value ?? []
  if (items.length === 0) return []

  const buckets = {
    rateLimit: 0,
    serverError: 0,
    clientError: 0,
    other: 0
  }

  for (const e of items) {
    if (e.status_code === 429) {
      buckets.rateLimit++
      continue
    }
    if (e.status_code >= 500) {
      buckets.serverError++
      continue
    }
    if (e.status_code >= 400) {
      buckets.clientError++
      continue
    }
    buckets.other++
  }

  const total = items.length
  const pct = (n: number) => Math.round((n / total) * 100)

  return [
    { label: t('admin.ops.charts.rateLimits'), percentage: pct(buckets.rateLimit), color: 'bg-amber-500' },
    { label: t('admin.ops.charts.serverErrors'), percentage: pct(buckets.serverError), color: 'bg-red-500' },
    { label: t('admin.ops.charts.clientErrors'), percentage: pct(buckets.clientError), color: 'bg-blue-500' },
    { label: t('admin.ops.charts.otherErrors'), percentage: pct(buckets.other), color: 'bg-gray-500' }
  ].filter((b) => b.percentage > 0)
})

// --- Columns ---
const errorColumns = computed<Column[]>(() => [
  { key: 'created_at', label: t('admin.ops.table.time'), sortable: true },
  { key: 'severity', label: t('admin.ops.table.severity') },
  { key: 'phase', label: t('admin.ops.table.phase') },
  { key: 'platform', label: t('admin.ops.table.platform') },
  { key: 'status_code', label: t('admin.ops.table.statusCode') },
  { key: 'latency_ms', label: t('admin.ops.table.latency') },
  { key: 'message', label: t('admin.ops.table.message') },
  { key: 'request_id', label: t('admin.ops.table.requestId') },
  { key: 'actions', label: '' }
])

// --- Helpers ---
const formatTimeOnly = (iso: string) => {
  return new Date(iso).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' })
}
const shortenId = (id: string) => id.length > 12 ? id.substring(0, 12) + '...' : id

const severityBadgeClass = (sev: Severity) => {
  const map: Record<string, string> = {
    P0: 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-400',
    P1: 'bg-orange-100 text-orange-700 dark:bg-orange-900/30 dark:text-orange-400',
    P2: 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400',
    P3: 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400'
  }
  return map[sev] || 'bg-gray-100 text-gray-700'
}

const statusColorClass = (code: number) => {
  if (code >= 500) return 'text-red-600 dark:text-red-400'
  if (code === 429) return 'text-amber-600 dark:text-amber-400'
  return 'text-gray-600 dark:text-gray-400'
}

const platformColorClass = (p: Platform) => {
  const map: Record<string, string> = {
    openai: 'bg-green-500',
    anthropic: 'bg-orange-500',
    gemini: 'bg-blue-500',
    antigravity: 'bg-purple-500'
  }
  return map[p] || 'bg-gray-400'
}

const platformLabel = (p: string) => p.charAt(0).toUpperCase() + p.slice(1)

const formatLatency = (latencyMs: number | null) => {
  if (latencyMs === null || latencyMs === undefined) return '—'
  return `${latencyMs} ms`
}

const formatTimeLabel = (iso: string | undefined): string => {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''

  if (timeRange.value === '24h' || timeRange.value === '7d') {
    return d.toLocaleString([], { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
  }
  return d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}

// --- Actions ---
const refreshData = async () => {
  if (loading.value) return
  loading.value = true
  const endTime = new Date()
  const minutes = lookbackMinutes(timeRange.value)
  const windowMinutes = timeRange.value === '7d' ? 5 : 1
  const historyLimit = Math.min(5000, Math.ceil(minutes / windowMinutes) + 5)
  const startTime = new Date(endTime.getTime() - minutes * 60 * 1000)

  try {
    const [latest, historyResp, errorsResp] = await Promise.all([
      opsAPI.getMetrics(),
      opsAPI.listMetricsHistory({ window_minutes: windowMinutes, minutes, limit: historyLimit }),
      opsAPI.listErrors({
        start_time: startTime.toISOString(),
        end_time: endTime.toISOString(),
        limit: 200,
        platform: platformFilter.value === 'all' ? undefined : platformFilter.value,
        phase: phaseFilter.value === 'all' ? undefined : phaseFilter.value,
        severity: severityFilter.value === 'all' ? undefined : severityFilter.value
      })
    ])

    metrics.value = latest
    metricsHistory.value = historyResp.items || []
    errors.value = errorsResp.items || []
  } catch (err) {
    const message =
      (err as { message?: string })?.message ||
      t('admin.ops.failedToLoad')
    appStore.showError(message)
  } finally {
    loading.value = false
  }
}

const openDetails = (err: OpsError) => {
  selectedError.value = err
}
const closeDetails = () => {
  selectedError.value = null
}

const copyToClipboard = async (text: string) => {
  try {
    await navigator.clipboard.writeText(text)
    appStore.showSuccess(t('common.copiedToClipboard'))
  } catch {
    appStore.showError(t('common.copyFailed'))
  }
}

const filteredErrors = computed(() => {
  return errors.value.filter(e => {
    const q = searchQuery.value.trim().toLowerCase()
    if (q) {
      const hay = `${e.message} ${e.request_id} ${e.model} ${e.type}`.toLowerCase()
      if (!hay.includes(q)) return false
    }
    if (severityFilter.value !== 'all' && e.severity !== severityFilter.value) return false
    if (platformFilter.value !== 'all' && e.platform !== platformFilter.value) return false
    if (phaseFilter.value !== 'all' && e.phase !== phaseFilter.value) return false
    return true
  })
})

const severityOptions = computed(() => [
  { value: 'all', label: t('admin.ops.filters.allSeverities') },
  { value: 'P0', label: t('admin.ops.filters.p0') },
  { value: 'P1', label: t('admin.ops.filters.p1') },
  { value: 'P2', label: t('admin.ops.filters.p2') },
  { value: 'P3', label: t('admin.ops.filters.p3') }
])

const platformOptions = computed(() => [
  { value: 'all', label: t('admin.ops.filters.allPlatforms') },
  { value: 'openai', label: t('admin.ops.platform.openai') },
  { value: 'gemini', label: t('admin.ops.platform.gemini') },
  { value: 'anthropic', label: t('admin.ops.platform.anthropic') },
  { value: 'antigravity', label: t('admin.ops.platform.antigravity') }
])

const phaseOptions = computed(() => [
  { value: 'all', label: t('admin.ops.filters.allPhases') },
  { value: 'auth', label: t('admin.ops.phase.auth') },
  { value: 'concurrency', label: t('admin.ops.phase.concurrency') },
  { value: 'billing', label: t('admin.ops.phase.billing') },
  { value: 'scheduling', label: t('admin.ops.phase.scheduling') },
  { value: 'network', label: t('admin.ops.phase.network') },
  { value: 'upstream', label: t('admin.ops.phase.upstream') },
  { value: 'response', label: t('admin.ops.phase.response') },
  { value: 'internal', label: t('admin.ops.phase.internal') }
])

const timeRangeOptions = computed(() => [
  { value: '15m', label: t('admin.ops.range.15m') },
  { value: '1h', label: t('admin.ops.range.1h') },
  { value: '24h', label: t('admin.ops.range.24h') },
  { value: '7d', label: t('admin.ops.range.7d') }
])

type StatCardItem = {
  key: string
  title: string
  value: string | number
  icon: any
  iconVariant: IconVariant
}

const statCards = computed<StatCardItem[]>(() => [
  {
    key: 'success',
    title: t('admin.ops.metrics.successRate'),
    value: hasTraffic.value ? `${(metrics.value?.success_rate ?? 0).toFixed(2)}%` : t('common.noData'),
    icon: { render: () => h('svg', {class: 'h-6 w-6 text-emerald-600', fill:'none', viewBox:'0 0 24 24', stroke:'currentColor'}, [h('path',{d:'M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z', 'stroke-width':2})]) },
    iconVariant: 'success'
  },
  {
    key: 'error_rate',
    title: t('admin.ops.metrics.errorRate'),
    value: hasTraffic.value ? `${(metrics.value?.error_rate ?? 0).toFixed(2)}%` : t('common.noData'),
    icon: { render: () => h('svg', {class: 'h-6 w-6 text-red-600', fill:'none', viewBox:'0 0 24 24', stroke:'currentColor'}, [h('path',{d:'M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z', 'stroke-width':2})]) },
    iconVariant: 'danger'
  },
  {
    key: 'p99',
    title: t('admin.ops.metrics.p99'),
    value: hasTraffic.value ? `${formatNumber(metrics.value?.p99_latency_ms ?? 0)} ms` : t('common.noData'),
    icon: { render: () => h('svg', {class: 'h-6 w-6 text-amber-600', fill:'none', viewBox:'0 0 24 24', stroke:'currentColor'}, [h('path',{d:'M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z', 'stroke-width':2})]) },
    iconVariant: 'warning'
  },
  {
    key: 'cpu',
    title: t('admin.ops.metrics.cpuUsage'),
    value: metrics.value?.cpu_usage_percent !== undefined ? `${metrics.value.cpu_usage_percent.toFixed(1)}%` : t('common.noData'),
    icon: { render: () => h('svg', {class: 'h-6 w-6 text-blue-600', fill:'none', viewBox:'0 0 24 24', stroke:'currentColor'}, [h('path',{d:'M9 3v2m6-2v2M9 19v2m6-2v2M5 9H3m2 6H3m18-6h-2m2 6h-2M8 9h8v6H8V9z', 'stroke-width':2})]) },
    iconVariant: 'primary'
  },
  {
    key: 'queue',
    title: t('admin.ops.metrics.queueDepth'),
    value: metrics.value?.concurrency_queue_depth !== undefined ? formatNumber(metrics.value.concurrency_queue_depth) : t('common.noData'),
    icon: { render: () => h('svg', {class: 'h-6 w-6 text-blue-600', fill:'none', viewBox:'0 0 24 24', stroke:'currentColor'}, [h('path',{d:'M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9', 'stroke-width':2})]) },
    iconVariant: 'primary'
  }
])

onMounted(() => {
  refreshData()
  autoRefreshTimer = window.setInterval(() => {
    refreshData()
  }, 30_000)
})

onUnmounted(() => {
  if (autoRefreshTimer) window.clearInterval(autoRefreshTimer)
  autoRefreshTimer = null
})

watch(timeRange, () => {
  refreshData()
})

watch([platformFilter, phaseFilter, severityFilter], () => {
  refreshData()
})
</script>

<style scoped>
/* Optional: Custom scrollbar for the drawer code block */
pre::-webkit-scrollbar {
  height: 4px;
}
pre::-webkit-scrollbar-thumb {
  background-color: #4b5563;
  border-radius: 4px;
}
</style>
