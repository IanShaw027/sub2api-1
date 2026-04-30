<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-80">
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.affiliates.searchPlaceholder')"
              class="input"
              @input="handleSearch"
            />
          </div>

          <div class="flex items-center gap-2">
            <span class="text-sm font-medium text-gray-700 dark:text-gray-300">
              {{ t('admin.affiliates.period') }}:
            </span>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="handleDateRangeChange"
            />
          </div>

          <button
            v-if="startDate || endDate"
            type="button"
            class="btn btn-secondary"
            @click="clearDateRange"
          >
            {{ t('admin.affiliates.clearRange') }}
          </button>

          <div class="flex flex-1 justify-end">
            <button
              type="button"
              class="btn btn-secondary"
              :disabled="loading"
              :title="t('common.refresh')"
              @click="loadAffiliates"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="affiliates"
          :loading="loading"
        >
          <template #cell-user="{ row }">
            <div class="min-w-56">
              <p class="font-medium text-gray-900 dark:text-white">
                {{ row.email || t('admin.affiliates.userId', { id: row.user_id }) }}
              </p>
              <p class="text-xs text-gray-500 dark:text-dark-400">
                <span v-if="row.username">{{ row.username }}</span>
                <span v-if="row.username" class="mx-1">/</span>
                <span>{{ t('admin.affiliates.userId', { id: row.user_id }) }}</span>
              </p>
            </div>
          </template>

          <template #cell-aff_code="{ value }">
            <code class="font-mono text-sm text-gray-900 dark:text-gray-100">
              {{ value || '-' }}
            </code>
          </template>

          <template #cell-aff_count="{ row, value }">
            <button
              type="button"
              class="font-medium text-primary-600 transition hover:text-primary-700 disabled:cursor-not-allowed disabled:text-gray-400 dark:text-primary-400 dark:hover:text-primary-300 dark:disabled:text-dark-500"
              :disabled="inviteesLoading && activeInviteeOwner?.user_id === row.user_id"
              @click="openInvitees(row)"
            >
              {{ formatCount(value) }}
            </button>
          </template>

          <template #cell-aff_quota="{ value }">
            <span class="font-medium text-green-600 dark:text-green-400">
              {{ formatCurrency(value) }}
            </span>
          </template>

          <template #cell-aff_history_quota="{ value }">
            <span class="font-medium text-gray-900 dark:text-white">
              {{ formatCurrency(value) }}
            </span>
          </template>

          <template #cell-period_rebate_amount="{ value }">
            <span class="font-medium text-blue-600 dark:text-blue-400">
              {{ formatCurrency(value) }}
            </span>
          </template>
        </DataTable>
      </template>

      <template #pagination>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BaseDialog
      :show="inviteesOpen"
      :title="inviteesDialogTitle"
      width="extra-wide"
      :close-on-click-outside="true"
      @close="closeInvitees"
    >
      <div v-if="inviteesLoading" class="py-8 text-center text-sm text-gray-500 dark:text-dark-400">
        {{ t('common.loading') }}
      </div>
      <div
        v-else-if="invitees.length === 0"
        class="rounded-xl border border-dashed border-gray-300 p-6 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-dark-400"
      >
        {{ t('admin.affiliates.invitees.empty') }}
      </div>
      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[720px] text-left text-sm">
          <thead>
            <tr class="border-b border-gray-200 text-gray-500 dark:border-dark-700 dark:text-dark-400">
              <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.invitees.columns.email') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.invitees.columns.username') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.invitees.columns.joinedAt') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.invitees.columns.consumed') }}</th>
              <th class="px-3 py-2 font-medium">{{ t('admin.affiliates.invitees.columns.rebate') }}</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in invitees"
              :key="item.user_id"
              class="border-b border-gray-100 last:border-b-0 dark:border-dark-800"
            >
              <td class="px-3 py-3 text-gray-900 dark:text-white">{{ item.email || '-' }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ item.username || '-' }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) || '-' }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatCurrency(item.total_consumed || 0) }}</td>
              <td class="px-3 py-3 text-gray-700 dark:text-gray-300">{{ formatCurrency(item.total_rebate || 0) }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { AdminAffiliateSummary } from '@/api/admin'
import type { AffiliateInvitee } from '@/types'
import { useAppStore } from '@/stores/app'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'
import { formatCurrency, formatDateTime } from '@/utils/format'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const affiliates = ref<AdminAffiliateSummary[]>([])
const loading = ref(false)
const searchQuery = ref('')
const startDate = ref('')
const endDate = ref('')
const inviteesOpen = ref(false)
const inviteesLoading = ref(false)
const invitees = ref<AffiliateInvitee[]>([])
const activeInviteeOwner = ref<AdminAffiliateSummary | null>(null)

const pagination = reactive({
  page: 1,
  page_size: getPersistedPageSize(),
  total: 0,
})

const columns = computed<Column[]>(() => [
  { key: 'user', label: t('admin.affiliates.columns.user') },
  { key: 'aff_code', label: t('admin.affiliates.columns.affCode') },
  { key: 'aff_count', label: t('admin.affiliates.columns.invitedCount') },
  { key: 'rebated_invitee_count', label: t('admin.affiliates.columns.rebatedInviteeCount') },
  { key: 'aff_quota', label: t('admin.affiliates.columns.availableQuota') },
  { key: 'aff_history_quota', label: t('admin.affiliates.columns.historyQuota') },
  { key: 'period_invited_count', label: t('admin.affiliates.columns.periodInvitedCount') },
  { key: 'period_rebate_amount', label: t('admin.affiliates.columns.periodRebateAmount') },
])

let abortController: AbortController | null = null
let searchTimeout: ReturnType<typeof setTimeout> | null = null
let activeInviteesRequestID = 0

const inviteesDialogTitle = computed(() => {
  if (!activeInviteeOwner.value) {
    return t('admin.affiliates.invitees.title')
  }
  return t('admin.affiliates.invitees.titleWithUser', {
    user: activeInviteeOwner.value.email || t('admin.affiliates.userId', { id: activeInviteeOwner.value.user_id }),
  })
})

const formatCount = (value: number) => Number(value || 0).toLocaleString()

const loadAffiliates = async () => {
  if (abortController) {
    abortController.abort()
  }

  const currentController = new AbortController()
  abortController = currentController
  loading.value = true

  try {
    const response = await adminAPI.affiliate.list(
      pagination.page,
      pagination.page_size,
      {
        search: searchQuery.value.trim() || undefined,
        start_date: startDate.value || undefined,
        end_date: endDate.value || undefined,
      },
      { signal: currentController.signal },
    )

    if (currentController.signal.aborted || abortController !== currentController) return

    affiliates.value = response.items
    pagination.total = response.total
  } catch (error: any) {
    if (
      currentController.signal.aborted ||
      abortController !== currentController ||
      error?.name === 'AbortError' ||
      error?.code === 'ERR_CANCELED'
    ) {
      return
    }
    appStore.showError(t('admin.affiliates.failedToLoad'))
    console.error('Error loading affiliate summaries:', error)
  } finally {
    if (abortController === currentController) {
      loading.value = false
      abortController = null
    }
  }
}

const resetAndLoad = () => {
  pagination.page = 1
  loadAffiliates()
}

const handleSearch = () => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(resetAndLoad, 300)
}

const handleDateRangeChange = () => {
  resetAndLoad()
}

const clearDateRange = () => {
  startDate.value = ''
  endDate.value = ''
  resetAndLoad()
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadAffiliates()
}

const handlePageSizeChange = (pageSize: number) => {
  pagination.page_size = pageSize
  pagination.page = 1
  loadAffiliates()
}

const openInvitees = async (row: AdminAffiliateSummary) => {
  const requestID = ++activeInviteesRequestID
  activeInviteeOwner.value = row
  inviteesOpen.value = true
  inviteesLoading.value = true
  invitees.value = []

  try {
    const items = await adminAPI.affiliate.listInvitees(row.user_id)
    if (requestID !== activeInviteesRequestID) {
      return
    }
    invitees.value = items
  } catch (error) {
    if (requestID !== activeInviteesRequestID) {
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('admin.affiliates.invitees.failedToLoad')))
  } finally {
    if (requestID === activeInviteesRequestID) {
      inviteesLoading.value = false
    }
  }
}

const closeInvitees = () => {
  activeInviteesRequestID++
  inviteesOpen.value = false
  inviteesLoading.value = false
  invitees.value = []
  activeInviteeOwner.value = null
}

onMounted(() => {
  loadAffiliates()
})

onUnmounted(() => {
  if (abortController) {
    abortController.abort()
  }
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  closeInvitees()
})
</script>
