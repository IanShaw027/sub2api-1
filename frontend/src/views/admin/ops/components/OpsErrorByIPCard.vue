<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores/app'
import { opsAPI } from '@/api/admin/ops'
import type { IPErrorStats } from '../types'
import { parseTimeRangeMinutes, formatDateTime } from '../utils/opsFormatters'
import OpsErrorsByIPModal from './OpsErrorsByIPModal.vue'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  timeRange: string
}>()

const emit = defineEmits<{
  openErrorDetail: [errorId: number]
}>()

const loading = ref(false)
const items = ref<IPErrorStats[]>([])

const limit = ref(50)
const limitOptions = computed(() => [
  { value: 20, label: '20' },
  { value: 50, label: '50' },
  { value: 100, label: '100' },
  { value: 200, label: '200' }
])

const sortBy = ref<'error_count' | 'last_error_time'>('error_count')
const sortOptions = computed(() => [
  { value: 'error_count', label: t('admin.ops.ipErrors.sort.errorCount') },
  { value: 'last_error_time', label: t('admin.ops.ipErrors.sort.lastSeen') }
])

function handleSortChange(val: string | number | boolean | null) {
  const next = String(val || 'error_count')
  sortBy.value = next === 'last_error_time' || next === 'error_count' ? next : 'error_count'
}

const showModal = ref(false)
const selectedIP = ref('')
const startTimeIso = ref('')
const endTimeIso = ref('')

function computeTimeRange(): { start: string, end: string } {
  const minutes = parseTimeRangeMinutes(props.timeRange)
  const end = new Date()
  const start = new Date(end.getTime() - minutes * 60 * 1000)
  return { start: start.toISOString(), end: end.toISOString() }
}

async function load() {
  loading.value = true
  try {
    const { start, end } = computeTimeRange()
    startTimeIso.value = start
    endTimeIso.value = end
    const res = await opsAPI.getErrorStatsByIP({
      start_time: start,
      end_time: end,
      limit: limit.value,
      sort_by: sortBy.value,
      sort_order: 'desc'
    })
    items.value = res.data || []
  } catch (err: any) {
    console.error('[OpsErrorByIPCard] Failed to load IP error stats', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.ipErrors.loadFailed'))
    items.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})

watch(
  () => props.timeRange,
  () => {
    load()
  }
)

watch([limit, sortBy], () => {
  load()
})

function openIP(ip: string) {
  selectedIP.value = ip
  showModal.value = true
}

function closeModal() {
  showModal.value = false
  selectedIP.value = ''
}

const empty = computed(() => items.value.length === 0 && !loading.value)
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.ipErrors.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.ipErrors.description') }}</p>
      </div>

      <div class="flex items-center gap-2">
        <Select :model-value="sortBy" :options="sortOptions" class="w-[160px]" @change="handleSortChange" />
        <Select :model-value="limit" :options="limitOptions" class="w-[88px]" @change="limit = Number($event || 50)" />
        <button
          class="flex items-center gap-1.5 rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 transition-colors hover:bg-gray-200 disabled:cursor-not-allowed disabled:opacity-50 dark:bg-dark-700 dark:text-gray-300 dark:hover:bg-dark-600"
          :disabled="loading"
          @click="load"
        >
          <svg class="h-3.5 w-3.5" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {{ t('common.refresh') }}
        </button>
      </div>
    </div>

    <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
      <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
      </svg>
      {{ t('admin.ops.ipErrors.loading') }}
    </div>

    <div v-else-if="empty" class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
      {{ t('admin.ops.ipErrors.empty') }}
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-900">
            <tr>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.ipErrors.table.ip') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.ipErrors.table.count') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.ipErrors.table.firstSeen') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.ipErrors.table.lastSeen') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.ipErrors.table.actions') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="row in items" :key="row.client_ip" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
              <td class="whitespace-nowrap px-4 py-3 font-mono text-xs text-gray-700 dark:text-gray-200">
                {{ row.client_ip }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs font-bold text-gray-900 dark:text-white">
                {{ row.error_count }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {{ formatDateTime(row.first_error_time) }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {{ formatDateTime(row.last_error_time) }}
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs">
                <button
                  class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
                  @click="openIP(row.client_ip)"
                >
                  {{ t('admin.ops.ipErrors.details') }}
                </button>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <OpsErrorsByIPModal
      :show="showModal"
      :ip="selectedIP"
      :start-time-iso="startTimeIso"
      :end-time-iso="endTimeIso"
      @close="closeModal"
      @openErrorDetail="emit('openErrorDetail', $event)"
    />
  </div>
</template>
