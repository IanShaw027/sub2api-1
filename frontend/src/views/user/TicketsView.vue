<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <SearchInput
            v-model="filters.search"
            :placeholder="t('tickets.filters.search')"
            class="w-full sm:w-64"
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
            <button class="btn btn-primary" @click="openCreateDialog">
              <Icon name="plus" size="md" class="mr-1" />
              {{ t('tickets.create') }}
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
            <span class="text-sm text-gray-900 dark:text-white">
              {{ t(`tickets.categories.${row.category}`) }}
            </span>
          </template>

          <template #cell-title="{ row }">
            <div class="max-w-[28rem] whitespace-normal break-words text-sm text-gray-900 dark:text-white">
              {{ row.title }}
              <span v-if="row.unread_by_user" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500 align-middle" />
            </div>
          </template>

          <template #cell-status="{ row }">
            <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(row.status)">
              {{ t(`tickets.statuses.${row.status}`) }}
            </span>
          </template>

          <template #cell-created_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="text-sm text-gray-500 dark:text-dark-400">
              {{ formatRelativeWithDateTime(value) }}
            </span>
          </template>

          <template #cell-actions="{ row }">
            <div class="flex flex-wrap items-center gap-2">
              <button class="btn btn-secondary btn-sm" @click="router.push(`/tickets/${row.id}`)">{{ t('tickets.actions.view') }}</button>
              <button v-if="row.status === 'withdrawn'" class="btn btn-secondary btn-sm" @click="openEdit(row)">{{ t('tickets.actions.edit') }}</button>
              <button v-if="canCloseTicket(row.status)" class="btn btn-secondary btn-sm" :disabled="row.status === 'closed'" @click="closeTicketItem(row.id)">{{ t('tickets.actions.close') }}</button>
            </div>
          </template>

          <template #empty>
            <div class="flex flex-col items-center">
              <Icon
                name="inbox"
                size="xl"
                class="mb-4 h-12 w-12 text-gray-400 dark:text-dark-500"
              />
              <p class="text-lg font-medium text-gray-900 dark:text-gray-100">
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

    <TicketCreateDialog
      :show="showCreateDialog"
      :submitting="creating"
      :user-concurrency="authStore.user?.concurrency ?? null"
      :rate-groups="rateGroups"
      @close="closeCreateDialog"
      @submit="createTicket"
    />
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
import TicketCreateDialog from '@/components/tickets/TicketCreateDialog.vue'
import { useAppStore, useAuthStore } from '@/stores'
import { ticketsAPI } from '@/api/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions, validateTicketPayload } from '@/utils/tickets'
import type { SupportTicket, TicketCategory, TicketRateGroupOption, TicketStatus } from '@/types/ticket'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(false)
const hasLoadedTickets = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const items = ref<SupportTicket[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})
const rateGroups = ref<TicketRateGroupOption[]>([])
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
  { key: 'status', label: t('tickets.table.status'), class: 'w-40' },
  { key: 'created_at', label: t('tickets.table.createdAt'), class: 'w-44' },
  { key: 'updated_at', label: t('tickets.table.updatedAt'), class: 'w-44' },
  { key: 'actions', label: t('tickets.table.actions'), class: 'w-48' },
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
    const res = await ticketsAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      keyword: filters.search.trim() || undefined,
      category: filters.category || undefined,
      status: filters.status || undefined,
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
  pagination.page = 1
  loadTickets()
}

async function loadTicketContext() {
  try {
    const res = await ticketsAPI.rateGroups()
    rateGroups.value = res.data || []
  } catch {
    rateGroups.value = []
  }
}

async function closeTicketItem(id: number) {
  try {
    await ticketsAPI.close(id)
    appStore.showSuccess(t('tickets.messages.closed'))
    await loadTickets()
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  }
}

function canCloseTicket(status: TicketStatus) {
  return status !== 'withdrawn'
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

function openCreateDialog() {
  showCreateDialog.value = true
}

function closeCreateDialog() {
  showCreateDialog.value = false
}

function openEdit(ticket: SupportTicket) {
  if (ticket.status === 'withdrawn') {
    router.push({ path: `/tickets/${ticket.id}`, query: { edit: '1' } })
    return
  }
  router.push(`/tickets/${ticket.id}`)
}

async function createTicket(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    creating.value = true
    const created = await ticketsAPI.create(form)
    showCreateDialog.value = false
    appStore.showSuccess(t('tickets.messages.created'))
    router.push(`/tickets/${created.data.id}`)
  } catch (err: unknown) {
    appStore.showError(ticketError(err))
  } finally {
    creating.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadTickets(), loadTicketContext()])
})
</script>
