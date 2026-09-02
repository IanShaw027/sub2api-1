<template>
  <AppLayout>
    <PageHeader :title="t('nav.ticketManagement')" :description="t('tickets.description')">
      <template #actions>
        <Button variant="secondary" @click="resetFilters">{{ t('common.reset') }}</Button>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadTickets">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <FilterBar :search-placeholder="t('tickets.filters.adminSearch')">
            <template #search>
              <SearchInput
                v-model="filters.search"
                :placeholder="t('tickets.filters.adminSearch')"
                class="w-full"
                @search="applyFilters"
              />
            </template>
            <template #filters>
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
              <label class="flex items-center gap-2 text-sm">
                <input v-model="filters.unread_only" type="checkbox" @change="applyFilters" />
                {{ t('tickets.unreadOnly') }}
              </label>
            </template>
          </FilterBar>
        </div>
      </template>

      <template #table>
        <DataTable
          :columns="columns"
          :data="items"
          :loading="showInitialLoading"
        >
          <template #cell-category="{ row }">
            <span class="text-sm text-foreground">
              {{ t(`tickets.categories.${row.category}`) }}
            </span>
          </template>

          <template #cell-title="{ row }">
            <div class="max-w-[28rem] whitespace-normal break-words text-sm text-foreground">
              {{ row.title }}
              <span v-if="row.unread_by_admin" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500 align-middle" />
            </div>
          </template>

          <template #cell-user_name="{ row }">
            <div class="max-w-[14rem] whitespace-normal break-words text-sm text-foreground">
              {{ row.user_name || row.user_email }}
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(row.status)">
              {{ t(`tickets.statuses.${row.status}`) }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-muted">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="text-sm text-muted">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button class="btn-glass-secondary text-sm" @click="router.push(`/admin/tickets/${row.id}`)">{{ t('tickets.actions.view') }}</button>
            </div>
          </template>

          <template #empty>
            <div class="flex flex-col items-center">
              <Icon
                name="inbox"
                size="xl"
                class="mb-4 h-12 w-12 text-muted"
              />
              <p class="text-lg font-medium text-foreground">
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
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import Pagination from '@/components/common/Pagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { adminTicketsAPI } from '@/api/admin/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions } from '@/utils/tickets'
import type { SupportTicket, TicketCategory, TicketStatus } from '@/types/ticket'

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
const filters = reactive<{ search: string; category: TicketCategory | ''; status: TicketStatus | ''; start_date: string; end_date: string; unread_only: boolean }>({
  search: '',
  category: '',
  status: '',
  start_date: '',
  end_date: '',
  unread_only: false,
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

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
}

async function loadTickets() {
  const requestID = ++loadTicketsRequestID
  try {
    loading.value = true
    const res = await adminTicketsAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: filters.search.trim() || undefined,
      category: filters.category || undefined,
      status: filters.status || undefined,
      unread_only: filters.unread_only || undefined,
      start_date: filters.start_date || undefined,
      end_date: filters.end_date || undefined,
    })
    if (requestID !== loadTicketsRequestID) {
      return
    }
    items.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    if (requestID !== loadTicketsRequestID) {
      return
    }
    appStore.showError(ticketError(err))
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
  filters.unread_only = false
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
