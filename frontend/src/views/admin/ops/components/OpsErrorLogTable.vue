<script setup lang="ts">
import { ref, reactive } from 'vue'
import { useI18n } from 'vue-i18n'
import { getSeverityClass, formatDateTime } from '../utils/opsFormatters'
import type { ErrorFilters } from '../types'
import { opsAPI, type OpsErrorLog, type OpsPlatform, type OpsSeverity } from '@/api/admin/ops'
import ElPagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'

interface Props {
  errorLogs: OpsErrorLog[]
  errorLogsTotal: number
  errorLogsLoading: boolean
  filters: ErrorFilters
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

const retryingIds = reactive(new Set<number>())

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

function updateFilter(key: keyof ErrorFilters, value: any) {
  emit('update:filters', { ...props.filters, [key]: value })
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
  if (msg.includes('context deadline exceeded')) return 'Timeout (Deadline Exceeded)'
  if (msg.includes('connection refused')) return 'Connection Refused'
  if (msg.includes('rate limit')) return 'Rate Limit Exceeded'

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
      <div class="relative">
        <select
          :value="filters.platforms.length > 0 ? filters.platforms[0] : ''"
          @change="updateFilter('platforms', ($event.target as HTMLSelectElement).value ? [($event.target as HTMLSelectElement).value] : [])"
          class="w-full appearance-none rounded-2xl border-gray-200 bg-gray-50/50 py-2.5 pl-4 pr-10 text-sm font-bold text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        >
          <option value="">{{ t('admin.ops.errors.allPlatforms') }}</option>
          <option v-for="platform in platformOptions" :key="platform" :value="platform">
            {{ platform.toUpperCase() }}
          </option>
        </select>
        <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3.5">
          <svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M19 9l-7 7-7-7" /></svg>
        </div>
      </div>

      <!-- Status Code Select -->
      <div class="relative">
        <select
          :value="filters.statusCodes.length > 0 ? filters.statusCodes[0] : ''"
          @change="updateFilter('statusCodes', ($event.target as HTMLSelectElement).value ? [Number(($event.target as HTMLSelectElement).value)] : [])"
          class="w-full appearance-none rounded-2xl border-gray-200 bg-gray-50/50 py-2.5 pl-4 pr-10 text-sm font-bold text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        >
          <option value="">{{ t('admin.ops.errors.allStatusCodes') }}</option>
          <option v-for="code in statusCodeOptions" :key="code" :value="code">
            {{ code }}
          </option>
        </select>
        <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3.5">
          <svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M19 9l-7 7-7-7" /></svg>
        </div>
      </div>

      <!-- Severity Select -->
      <div class="relative">
        <select
          :value="filters.severity"
          @change="updateFilter('severity', ($event.target as HTMLSelectElement).value)"
          class="w-full appearance-none rounded-2xl border-gray-200 bg-gray-50/50 py-2.5 pl-4 pr-10 text-sm font-bold text-gray-700 transition-all focus:border-blue-500 focus:bg-white focus:ring-4 focus:ring-blue-500/10 dark:border-dark-700 dark:bg-dark-900 dark:text-gray-300"
        >
          <option value="">{{ t('admin.ops.errors.allSeverities') }}</option>
          <option v-for="sev in severityOptions" :key="sev" :value="sev">
            {{ sev }}
          </option>
        </select>
        <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3.5">
          <svg class="h-4 w-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2.5" d="M19 9l-7 7-7-7" /></svg>
        </div>
      </div>
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

      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
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
              class="group cursor-pointer transition-all duration-200 hover:bg-gray-50/80 dark:hover:bg-dark-800/50"
              @click="emit('openErrorDetail', log)"
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
                  @click="handleRetry(log)"
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
        @update:page="handlePageChange"
        @update:pageSize="handleSizeChange"
      />
    </div>
  </div>
</template>

<style scoped>
/* Hidden Select default arrow */
select {
  background-image: none;
}
</style>

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
