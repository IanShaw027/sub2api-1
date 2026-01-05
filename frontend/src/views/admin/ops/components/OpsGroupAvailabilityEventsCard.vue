<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import Select from '@/components/common/Select.vue'
import { opsAPI } from '@/api/admin/ops'
import type { GroupAvailabilityEvent } from '../types'
import { formatDateTime } from '../utils/opsFormatters'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const events = ref<GroupAvailabilityEvent[]>([])

const limit = ref(50)
const limitOptions = computed(() => [
  { value: 50, label: '50' },
  { value: 100, label: '100' },
  { value: 200, label: '200' }
])

const status = ref<'all' | 'firing' | 'resolved'>('all')
const statusOptions = computed(() => [
  { value: 'all', label: t('admin.ops.availabilityEvents.status.all') },
  { value: 'firing', label: t('admin.ops.availabilityEvents.status.firing') },
  { value: 'resolved', label: t('admin.ops.availabilityEvents.status.resolved') }
])

function handleStatusChange(val: string | number | boolean | null) {
  const s = String(val || 'all')
  status.value = s === 'firing' || s === 'resolved' || s === 'all' ? s : 'all'
}

async function load() {
  loading.value = true
  try {
    events.value = await opsAPI.listGroupAvailabilityEvents({ limit: limit.value, status: status.value })
  } catch (err: any) {
    console.error('[OpsGroupAvailabilityEventsCard] Failed to load events', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.availabilityEvents.loadFailed'))
    events.value = []
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  load()
})

watch([limit, status], () => {
  load()
})

const empty = computed(() => events.value.length === 0 && !loading.value)

function statusBadgeClass(v: string | undefined): string {
  const s = String(v || '').trim().toLowerCase()
  if (s === 'firing') return 'bg-red-50 text-red-700 ring-red-600/20 dark:bg-red-900/30 dark:text-red-300 dark:ring-red-500/30'
  if (s === 'resolved') return 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/30 dark:text-green-300 dark:ring-green-500/30'
  return 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/30 dark:text-gray-300 dark:ring-gray-500/30'
}

function severityBadgeClass(v: string | undefined): string {
  const s = String(v || '').trim().toLowerCase()
  if (s === 'critical') return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  if (s === 'warning') return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  if (s === 'info') return 'bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300'
  return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-gray-300'
}
</script>

<template>
  <div class="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-900/5 dark:bg-dark-800 dark:ring-dark-700">
    <div class="mb-4 flex items-start justify-between gap-4">
      <div>
        <h3 class="text-sm font-bold text-gray-900 dark:text-white">{{ t('admin.ops.availabilityEvents.title') }}</h3>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.ops.availabilityEvents.description') }}</p>
      </div>

      <div class="flex items-center gap-2">
        <Select :model-value="status" :options="statusOptions" class="w-[120px]" @change="handleStatusChange" />
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
      {{ t('admin.ops.availabilityEvents.loading') }}
    </div>

    <div v-else-if="empty" class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
      {{ t('admin.ops.availabilityEvents.empty') }}
    </div>

    <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
      <div class="overflow-x-auto">
        <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
          <thead class="bg-gray-50 dark:bg-dark-900">
            <tr>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.time') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.group') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.status') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.severity') }}
              </th>
              <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.waterline') }}
              </th>
              <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {{ t('admin.ops.availabilityEvents.table.email') }}
              </th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
            <tr v-for="row in events" :key="row.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {{ formatDateTime(row.fired_at || row.created_at) }}
              </td>
              <td class="px-4 py-3 text-xs text-gray-700 dark:text-gray-200">
                <div class="font-semibold">{{ row.group?.name || `#${row.group_id}` }}</div>
                <div v-if="row.group?.platform" class="mt-0.5 text-[11px] text-gray-500 dark:text-gray-400">
                  {{ String(row.group.platform || '').toUpperCase() }}
                </div>
              </td>
              <td class="whitespace-nowrap px-4 py-3">
                <span class="inline-flex items-center rounded-full px-2 py-1 text-[10px] font-bold ring-1 ring-inset" :class="statusBadgeClass(row.status)">
                  {{ String(row.status || '-').toUpperCase() }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-3">
                <span class="rounded-full px-2 py-1 text-[10px] font-bold" :class="severityBadgeClass(String(row.severity || ''))">
                  {{ row.severity || '-' }}
                </span>
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                {{ row.available_accounts }} / {{ row.threshold_accounts }} ({{ row.total_accounts }})
              </td>
              <td class="whitespace-nowrap px-4 py-3 text-right text-xs">
                <span
                  class="inline-flex items-center rounded-full px-2 py-1 text-[10px] font-bold ring-1 ring-inset"
                  :class="row.email_sent ? 'bg-green-50 text-green-700 ring-green-600/20 dark:bg-green-900/30 dark:text-green-300 dark:ring-green-500/30' : 'bg-gray-50 text-gray-700 ring-gray-600/20 dark:bg-gray-900/30 dark:text-gray-300 dark:ring-gray-500/30'"
                >
                  {{ row.email_sent ? t('common.enabled') : t('common.disabled') }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
