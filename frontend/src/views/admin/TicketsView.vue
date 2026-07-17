<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3 rounded-card border border-line bg-card p-3 shadow-xs dark:border-dark-700 dark:bg-dark-800/50">
          <SearchInput
            v-model="filters.search"
            :placeholder="t('tickets.filters.adminSearch')"
            class="w-full sm:w-80"
            @search="applyFilters"
          />
          <Select
            v-model="filters.category"
            :options="categoryFilterOptions"
            :searchable="false"
            class="w-full sm:w-40"
            @change="applyFilters"
          />
          <Select
            v-model="filters.status"
            :options="statusFilterOptions"
            :searchable="false"
            class="w-full sm:w-40"
            @change="applyFilters"
          />
          <input
            v-model="filters.start_date"
            type="date"
            class="input w-full sm:w-40"
            @change="applyFilters"
          />
          <input
            v-model="filters.end_date"
            type="date"
            class="input w-full sm:w-40"
            @change="applyFilters"
          />

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset') }}</button>
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="loadTickets">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="items"
          :loading="showInitialLoading"
        >
          <template #cell-category="{ row }">
            <span class="text-sm text-ink dark:text-white">
              {{ t(`tickets.categories.${row.category}`) }}
            </span>
          </template>

          <template #cell-title="{ row }">
            <div class="max-w-[28rem] whitespace-normal break-words text-sm text-ink dark:text-white">
              {{ row.title }}
            </div>
          </template>

          <template #cell-user_name="{ row }">
            <div class="max-w-[14rem] whitespace-normal break-words text-sm text-ink dark:text-white">
              {{ row.user_name }}
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(row.status)">
              {{ t(`tickets.statuses.${row.status}`) }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-ink-soft dark:text-dark-400">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="text-sm text-ink-soft dark:text-dark-400">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button class="btn btn-secondary btn-sm" @click="router.push(`/admin/tickets/${row.id}`)">{{ t('tickets.actions.view') }}</button>
            </div>
          </template>

          <template #empty>
            <div class="flex flex-col items-center">
              <Icon
                name="inbox"
                size="xl"
                class="mb-4 h-12 w-12 text-ink-faint dark:text-dark-500"
              />
              <p class="text-lg font-medium text-ink">
                {{ t('tickets.empty') }}
              </p>
            </div>
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import adminTicketsAPI from '@/api/adminTickets'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions } from '@/utils/tickets'
import type { SupportTicket, TicketCategory, TicketStatus } from '@/types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const hasLoadedTickets = ref(false)
const items = ref<SupportTicket[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})
const filters = reactive<{ search: string; category: TicketCategory | ''; status: TicketStatus | ''; start_date: string; end_date: string }>({
  search: '',
  category: '',
  status: '',
  start_date: '',
  end_date: '',
})

const columns = computed<Column[]>(() => [
  { key: 'category', label: t('tickets.table.category'), class: 'w-36' },
  { key: 'title', label: t('tickets.table.title'), class: 'min-w-[18rem] whitespace-normal' },
  { key: 'user_name', label: t('tickets.table.user'), class: 'min-w-[14rem] whitespace-normal' },
  { key: 'status', label: t('tickets.table.status'), class: 'w-40' },
  { key: 'created_at', label: t('tickets.table.createdAt'), class: 'w-44' },
  { key: 'updated_at', label: t('tickets.table.updatedAt'), class: 'w-44' },
  { key: 'actions', label: t('tickets.table.actions'), class: 'w-32' },
])
const categoryFilterOptions = computed(() => [
  { value: '', label: t('tickets.filters.allCategories') },
  ...ticketCategoryOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
])
const statusFilterOptions = computed(() => [
  { value: '', label: t('tickets.filters.allStatuses') },
  ...ticketStatusOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
])
const showInitialLoading = computed(() => loading.value && !hasLoadedTickets.value)

let loadTicketsRequestID = 0

function buildListParams() {
  const params: Record<string, string | number> = {
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
    page: pagination.page,
    page_size: pagination.page_size,
  }
  const trimmedSearch = filters.search.trim()
  if (trimmedSearch) {
    params.search = trimmedSearch
  }
  if (filters.category) {
    params.category = filters.category
  }
  if (filters.status) {
    params.status = filters.status
  }
  if (filters.start_date) {
    params.start_date = filters.start_date
  }
  if (filters.end_date) {
    params.end_date = filters.end_date
  }
  return params
}

async function loadTickets() {
  const requestID = ++loadTicketsRequestID
  try {
    loading.value = true
    const data = await adminTicketsAPI.listAdminTickets(buildListParams())
    if (requestID !== loadTicketsRequestID) {
      return
    }
    items.value = data.items
    pagination.total = data.total
  } catch (err: any) {
    if (requestID !== loadTicketsRequestID) {
      return
    }
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    if (requestID === loadTicketsRequestID) {
      loading.value = false
      hasLoadedTickets.value = true
    }
  }
}

function applyFilters() {
  pagination.page = 1
  loadTickets()
}

function resetFilters() {
  filters.search = ''
  filters.category = ''
  filters.status = ''
  filters.start_date = ''
  filters.end_date = ''
  pagination.page = 1
  loadTickets()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadTickets()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadTickets()
}

onMounted(loadTickets)
</script>
