<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { opsAPI, type OpsErrorLog } from '@/api/admin/ops'
import { formatDateTime, getSeverityClass } from '../utils/opsFormatters'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  show: boolean
  ip: string
  startTimeIso: string
  endTimeIso: string
}>()

const emit = defineEmits<{
  close: []
  openErrorDetail: [errorId: number]
}>()

const loading = ref(false)
const items = ref<OpsErrorLog[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(50)

const title = computed(() => t('admin.ops.ipErrors.modalTitle', { ip: props.ip || '-' }))

async function fetchData() {
  if (!props.show || !props.ip) return
  loading.value = true
  try {
    const res = await opsAPI.getErrorsByIP(props.ip, {
      start_time: props.startTimeIso,
      end_time: props.endTimeIso,
      page: page.value,
      page_size: pageSize.value
    })
    items.value = res.errors || []
    total.value = res.total || 0
  } catch (err: any) {
    console.error('[OpsErrorsByIPModal] Failed to load errors by IP', err)
    appStore.showError(err?.response?.data?.detail || t('admin.ops.ipErrors.loadFailed'))
    items.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

watch(
  () => props.show,
  (open) => {
    if (!open) return
    page.value = 1
    fetchData()
  }
)

watch(
  () => [props.ip, props.startTimeIso, props.endTimeIso],
  () => {
    if (!props.show) return
    page.value = 1
    fetchData()
  }
)

function close() {
  emit('close')
}

function handlePageChange(next: number) {
  page.value = next
  fetchData()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  fetchData()
}

function openErrorDetail(errorId: number) {
  emit('openErrorDetail', errorId)
}
</script>

<template>
  <BaseDialog :show="props.show" :title="title" width="extra-wide" @close="close">
    <div class="space-y-4">
      <div class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.ops.ipErrors.range') }}:
        <span class="ml-1 font-mono">{{ props.startTimeIso }}</span>
        <span class="mx-1">→</span>
        <span class="font-mono">{{ props.endTimeIso }}</span>
      </div>

      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <svg class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24">
          <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
          <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
        </svg>
        {{ t('common.loading') }}
      </div>

      <div v-else-if="items.length === 0" class="rounded-xl border border-dashed border-gray-200 p-8 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
        {{ t('admin.ops.ipErrors.empty') }}
      </div>

      <div v-else class="overflow-hidden rounded-xl border border-gray-200 dark:border-dark-700">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-900">
              <tr>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.time') }}
                </th>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.platform') }}
                </th>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.phase') }}
                </th>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.severity') }}
                </th>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.status') }}
                </th>
                <th class="px-4 py-3 text-left text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.requestId') }}
                </th>
                <th class="px-4 py-3 text-right text-[11px] font-bold uppercase tracking-wider text-gray-500 dark:text-gray-400">
                  {{ t('admin.ops.ipErrors.table.actions') }}
                </th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-800">
              <tr v-for="row in items" :key="row.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/50">
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  {{ formatDateTime(row.created_at) }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-xs font-medium text-gray-700 dark:text-gray-200">
                  {{ (row.platform || 'unknown').toUpperCase() }}
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  {{ row.phase || '-' }}
                </td>
                <td class="whitespace-nowrap px-4 py-3">
                  <span class="rounded-full px-2 py-1 text-[10px] font-bold" :class="getSeverityClass(row.severity)">
                    {{ row.severity }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-600 dark:text-gray-300">
                  {{ row.status_code ?? '-' }}
                </td>
                <td class="px-4 py-3">
                  <span class="max-w-[240px] truncate font-mono text-[11px] text-gray-700 dark:text-gray-200" :title="row.request_id || ''">
                    {{ row.request_id || '-' }}
                  </span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-right text-xs">
                  <button
                    class="rounded-lg bg-gray-100 px-3 py-1.5 text-xs font-bold text-gray-700 hover:bg-gray-200 dark:bg-dark-700 dark:text-gray-200 dark:hover:bg-dark-600"
                    @click="openErrorDetail(row.id)"
                  >
                    {{ t('admin.ops.ipErrors.view') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        v-if="total > 0"
        :page="page"
        :total="total"
        :page-size="pageSize"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>
  </BaseDialog>
</template>
