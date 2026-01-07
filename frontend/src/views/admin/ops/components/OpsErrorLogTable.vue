<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSeverityClass, formatDateTime, parseTimeRangeMinutes } from '../utils/opsFormatters'
import type { ErrorFilters, IPErrorStats } from '../types'
import { opsAPI, type OpsErrorLog, type OpsPlatform, type OpsSeverity } from '@/api/admin/ops'
import ElPagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import Select from '@/components/common/Select.vue'
import { useVirtualList } from '@vueuse/core'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import OpsErrorsByIPModal from './OpsErrorsByIPModal.vue'

interface Props {
  errorLogs: OpsErrorLog[]
  errorLogsTotal: number
  errorLogsLoading: boolean
  filters: ErrorFilters
  timeRange: string
  page?: number
  pageSize?: number
}

interface Emits {
  (e: 'update:filters', value: ErrorFilters): void
  (e: 'update:page', value: number): void
  (e: 'update:pageSize', value: number): void
  (e: 'openErrorDetail', log: OpsErrorLog): void
}

const { t } = useI18n()
const appStore = useAppStore()
const props = withDefaults(defineProps<Props>(), {
  page: 1,
  pageSize: 50
})
const emit = defineEmits<Emits>()

const platformOptions: OpsPlatform[] = ['openai', 'anthropic', 'gemini', 'antigravity']
const statusCodeOptions = [400, 401, 403, 404, 429, 500, 502, 503, 504]
const severityOptions: OpsSeverity[] = ['P0', 'P1', 'P2', 'P3']

// Select component options
const platformSelectOptions = computed(() => [
  { value: '', label: t('admin.ops.errors.allPlatforms') },
  ...platformOptions.map(p => ({ value: p, label: p.toUpperCase() }))
])

const statusCodeSelectOptions = computed(() => [
  { value: '', label: t('admin.ops.errors.allStatusCodes') },
  ...statusCodeOptions.map(c => ({ value: c, label: String(c) }))
])

const severitySelectOptions = computed(() => [
  { value: '', label: t('admin.ops.errors.allSeverities') },
  ...severityOptions.map(s => ({ value: s, label: s }))
])

const retryingIds = reactive(new Set<number>())
const showRetryConfirm = ref(false)
const pendingRetryLog = ref<OpsErrorLog | null>(null)

// Top Error IPs quick filter
const topIpLoading = ref(false)
const topIpItems = ref<IPErrorStats[]>([])
const topIpLimit = 30
const ipModalOpen = ref(false)
const ipModalSelectedIP = ref('')
const ipModalStartTimeIso = ref('')
const ipModalEndTimeIso = ref('')

function computeTimeRangeIso(): { start: string, end: string } {
  const minutes = parseTimeRangeMinutes(props.timeRange)
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return { start: start.toISOString(), end: end.toISOString() }
}

async function loadTopErrorIps() {
  topIpLoading.value = true
  try {
    const { start, end } = computeTimeRangeIso()
    ipModalStartTimeIso.value = start
    ipModalEndTimeIso.value = end
    const res = await opsAPI.getErrorStatsByIP({
      start_time: start,
      end_time: end,
      limit: topIpLimit,
      sort_by: 'error_count',
      sort_order: 'desc'
    })
    topIpItems.value = res.data || []
  } catch (err: any) {
    console.error('[OpsErrorLogTable] Failed to load top error IPs', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.ipErrors.loadFailed'))
    topIpItems.value = []
  } finally {
    topIpLoading.value = false
  }
}

const topIpSelectOptions = computed(() => {
  const options = [
    { value: '', label: t('common.all') }
  ] as Array<{ value: string, label: string }>

  const seen = new Set<string>()
  for (const row of topIpItems.value) {
    if (!row?.client_ip) continue
    seen.add(row.client_ip)
    options.push({
      value: row.client_ip,
      label: `${row.client_ip} · ${row.error_count}`
    })
  }

  if (props.filters.clientIp && !seen.has(props.filters.clientIp)) {
    options.push({ value: props.filters.clientIp, label: props.filters.clientIp })
  }

  return options
})

function handleTopIpChange(val: string | number | boolean | null) {
  // Clear "quick filter" highlight when using IP filter.
  activeQuickFilter.value = ''
  updateFilter('clientIp', String(val || ''))
}

function openIpModal(ip: string) {
  if (!ip) return
  const { start, end } = computeTimeRangeIso()
  ipModalStartTimeIso.value = start
  ipModalEndTimeIso.value = end
  ipModalSelectedIP.value = ip
  ipModalOpen.value = true
}

function closeIpModal() {
  ipModalOpen.value = false
  ipModalSelectedIP.value = ''
}

function handleIpModalOpenErrorDetail(errorId: number) {
  // OpsDashboard only needs `id` to open the error detail modal.
  emit('openErrorDetail', { id: errorId } as unknown as OpsErrorLog)
}

// Enable virtualization earlier when the page size is increased (e.g. 200/500/1000),
// and also guard against backends returning more than requested.
const virtualEnabled = computed(() => props.pageSize >= 200 || props.errorLogs.length >= 100)
const errorLogsRef = computed(() => props.errorLogs)
const { list: virtualRows, containerProps, wrapperProps } = useVirtualList(errorLogsRef, {
  itemHeight: 76,
  overscan: 10
})

async function handleRetry(log: OpsErrorLog) {
  if (retryingIds.has(log.id)) return
  retryingIds.add(log.id)
  try {
    await opsAPI.retryErrorRequest(log.id)
    appStore.showSuccess(t('admin.ops.details.retryInfo'))
  } catch (e: any) {
    appStore.showError(e?.response?.data?.detail || t('admin.ops.details.retryFailed'))
  } finally {
    retryingIds.delete(log.id)
  }
}

function confirmRetry(log: OpsErrorLog) {
  pendingRetryLog.value = log
  showRetryConfirm.value = true
}

async function runConfirmedRetry() {
  if (!pendingRetryLog.value) return
  const log = pendingRetryLog.value
  showRetryConfirm.value = false
  pendingRetryLog.value = null
  await handleRetry(log)
}

function cancelRetry() {
  showRetryConfirm.value = false
  pendingRetryLog.value = null
}

function updateFilter(key: keyof ErrorFilters, value: any) {
  emit('update:filters', { ...props.filters, [key]: value })
}

function handlePlatformChange(val: string | number | boolean | null) {
  updateFilter('platforms', val ? [String(val)] : [])
}

function handleStatusCodeChange(val: string | number | boolean | null) {
  updateFilter('statusCodes', val ? [Number(val)] : [])
}

function handleSeverityChange(val: string | number | boolean | null) {
  updateFilter('severity', String(val || ''))
}

function handlePageChange(page: number) {
  emit('update:page', page)
}

function handleSizeChange(pageSize: number) {
  emit('update:pageSize', pageSize)
}

// Quick Filters
const activeQuickFilter = ref<string>('')

function applyQuickFilter(type: string) {
  if (activeQuickFilter.value === type) {
    // Toggle off
    activeQuickFilter.value = ''
    emit('update:filters', {
      platforms: [],
      groupId: null,
      statusCodes: [],
      clientIp: '',
      severity: '',
      searchText: ''
    })
    return
  }

  activeQuickFilter.value = type
  const newFilters: ErrorFilters = {
    platforms: [],
    groupId: null,
    statusCodes: [],
    clientIp: '',
    severity: '',
    searchText: ''
  }

  if (type === 'critical') {
    newFilters.severity = 'P0'
  } else if (type === '5xx') {
    newFilters.statusCodes = [500, 502, 503, 504]
  } else if (type === 'timeout') {
    newFilters.searchText = 'timeout'
  }

  emit('update:filters', newFilters)
}

// Smart Message Formatter
function formatSmartMessage(msg: string): string {
  if (!msg) return ''
  
  // Try to detect JSON
  if (msg.startsWith('{') || msg.startsWith('[')) {
    try {
      const obj = JSON.parse(msg)
      if (obj.error?.message) return obj.error.message
      if (obj.message) return obj.message
      if (obj.detail) return obj.detail
      if (typeof obj === 'object') return JSON.stringify(obj).substring(0, 150)
    } catch (e) {
      // ignore parse error
    }
  }

  // Common patterns
  if (msg.includes('context deadline exceeded')) return t('admin.ops.errors.smartMessage.timeoutDeadlineExceeded')
  if (msg.includes('connection refused')) return t('admin.ops.errors.smartMessage.connectionRefused')
  if (msg.includes('rate limit')) return t('admin.ops.errors.smartMessage.rateLimitExceeded')

  // Truncate if still too long
  return msg.length > 200 ? msg.substring(0, 200) + '...' : msg
}

function getLatencyClass(latency: number | null): string {
  if (!latency) return 'text-gray-400'
  if (latency > 10000) return 'text-red-600 font-black'
  if (latency > 5000) return 'text-red-500 font-bold'
  if (latency > 2000) return 'text-orange-500 font-medium'
  return 'text-gray-600 dark:text-gray-400'
}

function getStatusClass(code: number): string {
  if (code >= 500) return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-400 dark:ring-red-500/30'
  if (code === 429) return 'bg-purple-50 text-purple-700 ring-purple-600/20 dark:bg-purple-900/30 dark:text-purple-400 dark:ring-purple-500/30'
  if (code >= 400) return 'bg-amber-50 text-amber-700 ring-amber-600/20 dark:bg-amber-900/30 dark:text-amber-400 dark:ring-amber-500/30'
  return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/30 dark:text-gray-400 dark:ring-gray-500/30'
}

onMounted(() => {
  loadTopErrorIps()
})

watch(
  () => props.timeRange,
  () => {
    loadTopErrorIps()
  }
)
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <!-- Header Section -->
    <div class="mb-6 flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
      <div class="flex items-center gap-4">
        <div class="flex h-10 w-10 items-center justify-center rounded-2xl bg-orange-50 dark:bg-orange-900/20">
          <svg class="h-6 w-6 text-orange-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
        </div>
        <div>
          <h3 class="text-base font-black text-gray-900 dark:text-white">
            {{ t('admin.ops.errors.trackingTitle') }}
          </h3>
          <p class="text-xs font-medium text-gray-500 dark:text-dark-400">
            {{ t('admin.ops.errors.recentCount', { n: errorLogsTotal }) }}
          </p>
        </div>
      </div>
      
      <!-- Quick Filters -->
      <div class="flex flex-wrap gap-2">
        <button
          @click="applyQuickFilter('critical')"
          class="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs font-bold transition-all border shadow-sm"
          :class="activeQuickFilter === 'critical' 
            ? 'bg-red-500 border-red-500 text-white dark:bg-red-600 dark:border-red-600' 
            : 'bg-white border-gray-200 text-gray-700 hover:border-red-300 hover:bg-red-50 dark:bg-dark-800 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-red-900/20'"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-current"></span>
          {{ t('admin.ops.errors.quickFilters.critical') }}
        </button>
        <button
          @click="applyQuickFilter('5xx')"
          class="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs font-bold transition-all border shadow-sm"
          :class="activeQuickFilter === '5xx' 
            ? 'bg-orange-500 border-orange-500 text-white dark:bg-orange-600 dark:border-orange-600' 
            : 'bg-white border-gray-200 text-gray-700 hover:border-orange-300 hover:bg-orange-50 dark:bg-dark-800 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-orange-900/20'"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-current"></span>
          {{ t('admin.ops.errors.quickFilters.fiveXX') }}
        </button>
        <button
          @click="applyQuickFilter('timeout')"
          class="inline-flex items-center gap-1.5 rounded-xl px-3 py-2 text-xs font-bold transition-all border shadow-sm"
          :class="activeQuickFilter === 'timeout' 
            ? 'bg-blue-500 border-blue-500 text-white dark:bg-blue-600 dark:border-blue-600' 
            : 'bg-white border-gray-200 text-gray-700 hover:border-blue-300 hover:bg-blue-50 dark:bg-dark-800 dark:border-dark-700 dark:text-gray-300 dark:hover:bg-blue-900/20'"
        >
          <span class="h-1.5 w-1.5 rounded-full bg-current"></span>
          {{ t('admin.ops.errors.quickFilters.timeout') }}
        </button>
      </div>
    </div>

    <!-- Top Error IPs -->
    <div class="mb-5 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div class="flex flex-col gap-2 sm:flex-row sm:items-center">
        <div class="text-xs font-black text-gray-500 dark:text-dark-400">
          {{ t('admin.ops.ipErrors.title') }}
        </div>
        <div class="flex items-center gap-2">
          <Select
            class="w-[260px]"
            :model-value="filters.clientIp"
            :options="topIpSelectOptions"
            :disabled="topIpLoading || topIpSelectOptions.length <= 1"
            searchable
            @change="handleTopIpChange"
          />
          <button
            class="inline-flex items-center gap-1.5 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-bold text-gray-700 shadow-sm transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
            :disabled="topIpLoading"
            @click="loadTopErrorIps"
          >
            <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': topIpLoading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            {{ t('common.refresh') }}
          </button>
          <button
            class="inline-flex items-center gap-1.5 rounded-xl border border-gray-200 bg-white px-3 py-2 text-xs font-bold text-gray-700 shadow-sm transition-colors hover:bg-gray-50 disabled:cursor-not-allowed disabled:opacity-50 dark:border-dark-700 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
            :disabled="!filters.clientIp"
            @click="openIpModal(filters.clientIp)"
          >
            {{ t('admin.ops.ipErrors.details') }}
          </button>
        </div>
      </div>
      <div v-if="filters.clientIp" class="text-[11px] font-medium text-gray-500 dark:text-dark-400">
        client_ip:
        <span class="ml-1 font-mono font-bold text-gray-700 dark:text-gray-200">{{ filters.clientIp }}</span>
      </div>
    </div>

    <!-- Filters Bar -->
    <div class="mb-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-5">
      <!-- Search Input -->
      <div class="lg:col-span-2">
        <div class="relative group">
          <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <svg class="h-4 w-4 text-gray-400 group-focus-within:text-blue-500 transition-colors" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <input
            :value="filters.searchText"
            @input="updateFilter('searchText', ($event.target as HTMLInputElement).value)"
            type="text"
            :placeholder="t('admin.ops.errors.searchPlaceholder')"
            class="w-full rounded-2xl border-gray-200 bg-gray-50/50 py-2.5 pl-10 pr-4 text-sm font-medium text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300 dark:focus:bg-dark-800"
          />
        </div>
      </div>

      <!-- Platform Select -->
      <Select
        :model-value="filters.platforms.length > 0 ? filters.platforms[0] : ''"
        :options="platformSelectOptions"
        @change="handlePlatformChange"
      />

      <!-- Status Code Select -->
      <Select
        :model-value="filters.statusCodes.length > 0 ? filters.statusCodes[0] : ''"
        :options="statusCodeSelectOptions"
        @change="handleStatusCodeChange"
      />

      <!-- Severity Select -->
      <Select
        :model-value="filters.severity"
        :options="severitySelectOptions"
        @change="handleSeverityChange"
      />
    </div>

    <!-- Error Logs Table Area -->
    <div class="relative overflow-hidden rounded-2xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-900 shadow-sm">
      <!-- Loading Overlay -->
      <div v-if="errorLogsLoading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/60 backdrop-blur-sm dark:bg-dark-900/60">
        <div class="flex flex-col items-center gap-3">
          <div class="relative">
            <div class="h-10 w-10 rounded-full border-4 border-gray-200 dark:border-dark-700"></div>
            <div class="absolute top-0 h-10 w-10 animate-spin rounded-full border-4 border-blue-500 border-t-transparent"></div>
          </div>
          <span class="text-xs font-black uppercase tracking-widest text-gray-500 dark:text-dark-400">{{ t('admin.ops.errors.loading') }}</span>
        </div>
      </div>

      <!-- Mobile card list -->
      <div class="sm:hidden">
        <div v-if="errorLogs.length === 0 && !errorLogsLoading" class="py-10 text-center">
          <div class="flex flex-col items-center gap-3">
            <div class="flex h-14 w-14 items-center justify-center rounded-2xl bg-gray-50 text-gray-300 dark:bg-dark-800">
              <svg class="h-9 w-9" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4" />
              </svg>
            </div>
            <p class="text-sm font-bold text-gray-400 dark:text-dark-500">{{ t('admin.ops.errors.emptyTable') }}</p>
          </div>
        </div>

        <div v-else class="divide-y divide-gray-100 dark:divide-dark-700">
          <button
            v-for="log in errorLogs"
            :key="log.id"
            type="button"
            class="w-full p-4 text-left transition-colors hover:bg-gray-50 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:hover:bg-dark-800/40 dark:focus:ring-offset-dark-900"
            @click="emit('openErrorDetail', log)"
          >
            <div class="flex items-start justify-between gap-3">
              <div class="min-w-0">
                <div class="flex items-center gap-2">
                  <span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-bold uppercase tracking-tight text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ log.platform }}
                  </span>
                  <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-[11px] font-black ring-1 ring-inset shadow-sm', getStatusClass(log.status_code)]">
                    {{ log.status_code }}
                  </span>
                  <span v-if="log.severity !== 'P3'" :class="['rounded-md px-2 py-0.5 text-[10px] font-black shadow-sm', getSeverityClass(log.severity)]">
                    {{ log.severity }}
                  </span>
                </div>
                <div class="mt-2 text-xs font-semibold text-gray-700 dark:text-gray-300">
                  {{ formatSmartMessage(log.message) }}
                </div>
                <div class="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] text-gray-500 dark:text-dark-400">
                  <span class="font-mono">{{ formatDateTime(log.created_at).split(' ')[1] }}</span>
                  <span v-if="log.latency_ms" class="font-mono">{{ Math.round(log.latency_ms) }}ms</span>
                  <span v-if="log.client_ip" class="font-mono">{{ log.client_ip }}</span>
                </div>
              </div>

              <div class="flex flex-col items-end gap-2" @click.stop>
                <button
                  type="button"
                  @click="confirmRetry(log)"
                  :disabled="retryingIds.has(log.id)"
                  class="btn btn-xs btn-secondary"
                >
                  {{ t('admin.ops.details.retry') }}
                </button>
              </div>
            </div>
          </button>
        </div>
      </div>

      <div class="hidden overflow-x-auto sm:block">
        <!-- Virtual list (avoids heavy DOM when page size is large) -->
        <div v-if="virtualEnabled" class="min-w-[980px]">
          <div class="grid grid-cols-[150px_140px_140px_1fr_120px_120px] gap-0 bg-gray-50/50 dark:bg-dark-800/50">
            <div class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.errors.table.timeId') }}
            </div>
            <div class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.errors.table.context') }}
            </div>
            <div class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.errors.table.status') }}
            </div>
            <div class="px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.errors.table.message') }}
            </div>
            <div class="whitespace-nowrap px-6 py-4 text-right text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('admin.ops.errors.table.latency') }}
            </div>
            <div class="whitespace-nowrap px-6 py-4 text-right text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
              {{ t('common.actions') }}
            </div>
          </div>

          <div v-if="errorLogs.length === 0 && !errorLogsLoading" class="py-24 text-center">
            <div class="flex flex-col items-center gap-3">
              <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-50 text-gray-300 dark:bg-dark-800">
                <svg class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                  <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </div>
              <p class="text-sm font-bold text-gray-400 dark:text-dark-500">{{ t('admin.ops.errors.emptyTable') }}</p>
            </div>
          </div>

          <div
            v-else
            v-bind="containerProps"
            class="h-[60vh] overflow-y-auto border-b border-gray-100 dark:border-dark-700 sm:h-[640px]"
            role="table"
            :aria-label="t('admin.ops.errors.trackingTitle')"
          >
            <div v-bind="wrapperProps" class="divide-y divide-gray-100 dark:divide-dark-700">
              <div
                v-for="{ data: log } in virtualRows"
                :key="log.id"
                class="group grid grid-cols-[150px_140px_140px_1fr_120px_120px] cursor-pointer items-center transition-all duration-200 hover:bg-gray-50/80 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:hover:bg-dark-800/50 dark:focus:ring-offset-dark-900"
                @click="emit('openErrorDetail', log)"
                @keydown.enter.prevent="emit('openErrorDetail', log)"
                @keydown.space.prevent="emit('openErrorDetail', log)"
                tabindex="0"
                role="button"
              >
                <!-- Time & ID -->
                <div class="px-6 py-4">
                  <div class="flex flex-col gap-0.5">
                    <span class="font-mono text-xs font-bold text-gray-900 dark:text-gray-200">
                      {{ formatDateTime(log.created_at).split(' ')[1] }}
                    </span>
                    <span class="font-mono text-[10px] text-gray-400 transition-colors group-hover:text-blue-500" :title="log.request_id">
                      {{ log.request_id.substring(0, 12) }}
                    </span>
                  </div>
                </div>

                <!-- Context -->
                <div class="px-6 py-4">
                  <div class="flex flex-col items-start gap-1.5">
                    <span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-bold uppercase tracking-tight text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                      {{ log.platform }}
                    </span>
                    <span v-if="log.model" class="max-w-[140px] truncate font-mono text-[10px] text-gray-500 dark:text-dark-400" :title="log.model">
                      {{ log.model }}
                    </span>
                  </div>
                </div>

                <!-- Status & Severity -->
                <div class="px-6 py-4">
                  <div class="flex flex-wrap items-center gap-2">
                    <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset shadow-sm', getStatusClass(log.status_code)]">
                      {{ log.status_code }}
                    </span>
                    <span v-if="log.severity !== 'P3'" :class="['rounded-md px-2 py-0.5 text-[10px] font-black shadow-sm', getSeverityClass(log.severity)]">
                      {{ log.severity }}
                    </span>
                  </div>
                </div>

                <!-- Message -->
                <div class="px-6 py-4">
                  <div class="max-w-md lg:max-w-2xl">
                    <p class="truncate text-xs font-semibold text-gray-700 dark:text-gray-300" :title="log.message">
                      {{ formatSmartMessage(log.message) }}
                    </p>
                    <div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1">
                      <div v-if="log.phase" class="flex items-center gap-1">
                        <span class="h-1 w-1 rounded-full bg-gray-300"></span>
                        <span class="text-[9px] font-black uppercase tracking-tighter text-gray-400">{{ log.phase }}</span>
                      </div>
                      <div v-if="log.client_ip" class="flex items-center gap-1">
                        <span class="h-1 w-1 rounded-full bg-gray-300"></span>
                        <span class="text-[9px] font-mono font-bold text-gray-400">{{ log.client_ip }}</span>
                      </div>
                    </div>
                  </div>
                </div>

                <!-- Latency -->
                <div class="px-6 py-4 text-right">
                  <div class="flex flex-col items-end">
                    <span class="font-mono text-xs font-black" :class="getLatencyClass(log.latency_ms)">
                      {{ log.latency_ms ? Math.round(log.latency_ms) + 'ms' : '--' }}
                    </span>
                  </div>
                </div>

                <!-- Actions -->
                <div class="px-6 py-4 text-right" @click.stop>
                  <button
                    @click="confirmRetry(log)"
                    :disabled="retryingIds.has(log.id)"
                    class="btn btn-xs btn-secondary inline-flex items-center gap-1"
                    :title="t('admin.ops.details.retry')"
                  >
                    <svg v-if="retryingIds.has(log.id)" class="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                    <svg v-else class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                    {{ t('admin.ops.details.retry') }}
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>

        <!-- Standard table (keeps full semantics for smaller pages) -->
        <table v-else class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <caption class="sr-only">{{ t('admin.ops.errors.trackingTitle') }}</caption>
          <thead>
            <tr class="bg-gray-50/50 dark:bg-dark-800/50">
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.errors.table.timeId') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.errors.table.context') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.errors.table.status') }}
              </th>
              <th scope="col" class="px-6 py-4 text-left text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.errors.table.message') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-right text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('admin.ops.errors.table.latency') }}
              </th>
              <th scope="col" class="whitespace-nowrap px-6 py-4 text-right text-xs font-bold uppercase tracking-wider text-gray-500 dark:text-dark-400">
                {{ t('common.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
            <tr v-if="errorLogs.length === 0 && !errorLogsLoading">
              <td colspan="5" class="py-24 text-center">
                <div class="flex flex-col items-center gap-3">
                  <div class="flex h-16 w-16 items-center justify-center rounded-2xl bg-gray-50 text-gray-300 dark:bg-dark-800">
                    <svg class="h-10 w-10" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.172 16.172a4 4 0 015.656 0M9 10h.01M15 10h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                  </div>
                  <p class="text-sm font-bold text-gray-400 dark:text-dark-500">{{ t('admin.ops.errors.emptyTable') }}</p>
                </div>
              </td>
            </tr>
            <tr
              v-for="log in errorLogs"
              :key="log.id"
              class="group cursor-pointer transition-all duration-200 hover:bg-gray-50/80 focus:outline-none focus:ring-2 focus:ring-blue-500 focus:ring-offset-2 dark:hover:bg-dark-800/50 dark:focus:ring-offset-dark-900"
              @click="emit('openErrorDetail', log)"
              @keydown.enter.prevent="emit('openErrorDetail', log)"
              @keydown.space.prevent="emit('openErrorDetail', log)"
              tabindex="0"
              role="button"
            >
              <!-- Time & ID -->
              <td class="px-6 py-4">
                <div class="flex flex-col gap-0.5">
                  <span class="font-mono text-xs font-bold text-gray-900 dark:text-gray-200">
                    {{ formatDateTime(log.created_at).split(' ')[1] }}
                  </span>
                  <span class="font-mono text-[10px] text-gray-400 group-hover:text-blue-500 transition-colors" :title="log.request_id">
                    {{ log.request_id.substring(0, 12) }}
                  </span>
                </div>
              </td>

              <!-- Context (Platform/Model) -->
              <td class="px-6 py-4">
                <div class="flex flex-col items-start gap-1.5">
                  <span class="inline-flex items-center rounded-md bg-gray-100 px-2 py-0.5 text-[10px] font-bold uppercase tracking-tight text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ log.platform }}
                  </span>
                  <span v-if="log.model" class="max-w-[140px] truncate font-mono text-[10px] text-gray-500 dark:text-dark-400" :title="log.model">
                    {{ log.model }}
                  </span>
                </div>
              </td>

              <!-- Status & Severity -->
              <td class="px-6 py-4">
                <div class="flex flex-wrap items-center gap-2">
                  <span :class="['inline-flex items-center rounded-lg px-2 py-1 text-xs font-black ring-1 ring-inset shadow-sm', getStatusClass(log.status_code)]">
                    {{ log.status_code }}
                  </span>
                  <span v-if="log.severity !== 'P3'" :class="['rounded-md px-2 py-0.5 text-[10px] font-black shadow-sm', getSeverityClass(log.severity)]">
                    {{ log.severity }}
                  </span>
                </div>
              </td>

              <!-- Message -->
              <td class="px-6 py-4">
                <div class="max-w-md lg:max-w-2xl">
                  <p class="truncate text-xs font-semibold text-gray-700 dark:text-gray-300" :title="log.message">
                    {{ formatSmartMessage(log.message) }}
                  </p>
                  <div class="mt-1.5 flex flex-wrap gap-x-3 gap-y-1">
                    <div v-if="log.phase" class="flex items-center gap-1">
                      <span class="h-1 w-1 rounded-full bg-gray-300"></span>
                      <span class="text-[9px] font-black uppercase tracking-tighter text-gray-400">{{ log.phase }}</span>
                    </div>
                    <div v-if="log.client_ip" class="flex items-center gap-1">
                      <span class="h-1 w-1 rounded-full bg-gray-300"></span>
                      <span class="text-[9px] font-mono font-bold text-gray-400">{{ log.client_ip }}</span>
                    </div>
                  </div>
                </div>
              </td>

              <!-- Latency -->
              <td class="px-6 py-4 text-right">
                <div class="flex flex-col items-end">
                  <span class="font-mono text-xs font-black" :class="getLatencyClass(log.latency_ms)">
                    {{ log.latency_ms ? Math.round(log.latency_ms) + 'ms' : '--' }}
                  </span>
                </div>
              </td>

              <!-- Actions (Retry) -->
              <td class="px-6 py-4 text-right" @click.stop>
                <button
                  @click="confirmRetry(log)"
                  :disabled="retryingIds.has(log.id)"
                  class="btn btn-xs btn-secondary inline-flex items-center gap-1"
                  :title="t('admin.ops.details.retry')"
                >
                  <svg v-if="retryingIds.has(log.id)" class="h-3 w-3 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path></svg>
                  <svg v-else class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" /></svg>
                  {{ t('admin.ops.details.retry') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <!-- Pagination -->
    <div class="mt-6 flex items-center justify-between border-t border-gray-100 pt-6 dark:border-dark-700">
      <div class="text-xs font-medium text-gray-500 dark:text-dark-400">
        {{ t('pagination.showing') }} {{ (page - 1) * pageSize + 1 }} {{ t('pagination.to') }} {{ Math.min(page * pageSize, errorLogsTotal) }} {{ t('pagination.of') }} {{ errorLogsTotal }}
      </div>
      <ElPagination
        :total="errorLogsTotal"
        :page="page"
        :page-size="pageSize"
        :page-size-options="[20, 50, 100, 200, 500, 1000]"
        @update:page="handlePageChange"
        @update:pageSize="handleSizeChange"
      />
    </div>
  </div>

  <ConfirmDialog
    :show="showRetryConfirm"
    :title="t('admin.ops.errors.retryConfirmTitle')"
    :message="t('admin.ops.errors.retryConfirmMessage')"
    @confirm="runConfirmedRetry"
    @cancel="cancelRetry"
  />

  <OpsErrorsByIPModal
    :show="ipModalOpen"
    :ip="ipModalSelectedIP"
    :start-time-iso="ipModalStartTimeIso"
    :end-time-iso="ipModalEndTimeIso"
    @close="closeIpModal"
    @openErrorDetail="handleIpModalOpenErrorDetail"
  />
</template>
