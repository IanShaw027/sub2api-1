<template>
  <AppLayout>
    <PageHeader :title="t('tickets.title')" :description="t('tickets.description')">
      <template #actions>
        <Button variant="secondary" @click="resetFilters">{{ t('common.reset') }}</Button>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="loadTickets">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button class="tickets-create-desktop" @click="openCreateDialog">
          <Icon name="plus" size="md" />
          {{ t('tickets.create') }}
        </Button>
      </template>
    </PageHeader>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-col gap-3">
          <FilterBar :search-placeholder="t('tickets.filters.search')">
            <template #search>
              <SearchInput
                v-model="filters.search"
                :placeholder="t('tickets.filters.search')"
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
            </template>
          </FilterBar>
          <ChipScroller
            :model-value="String(filters.status || '')"
            :chips="statusFilterOptions.map((opt) => ({ value: String(opt.value), label: opt.label }))"
            @update:model-value="onStatusChipChange"
          />
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
              <span v-if="row.unread_by_user" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500 align-middle" />
            </div>
          </template>

          <template #cell-status="{ row }">
            <StatusBadge
              :tone="row.status === 'resolved' ? 'success' : row.status === 'withdrawn' ? 'danger' : row.status === 'closed' ? 'muted' : row.status === 'submitted' ? 'accent' : 'warning'"
              :label="t(`tickets.statuses.${row.status}`)"
              dot
            />
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
              <Button variant="secondary" @click="router.push(`/tickets/${row.id}`)">{{ t('tickets.actions.view') }}</Button>
              <Button v-if="row.status === 'withdrawn'" variant="secondary" @click="openEdit(row)">{{ t('tickets.actions.edit') }}</Button>
              <Button v-if="canCloseTicket(row.status)" variant="secondary" :disabled="row.status === 'closed'" @click="closeTicketItem(row.id)">{{ t('tickets.actions.close') }}</Button>
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
        <UiPagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>
    <Fab class="tickets-fab" :label="t('tickets.create')" @click="openCreateDialog">
      <Icon name="plus" size="md" />
      {{ t('tickets.create') }}
    </Fab>

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
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import ChipScroller from '@/components/ui/ChipScroller.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import Fab from '@/components/ui/Fab.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
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
import { ticketCategoryOptions, ticketStatusOptions, validateTicketPayload } from '@/utils/tickets'
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

function onStatusChipChange(value: string) {
  filters.status = (value || '') as TicketStatus | ''
  applyFilters()
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
<style scoped>
.tickets-fab {
  display: none;
}
@media (max-width: 767px) {
  .tickets-create-desktop {
    display: none;
  }
  .tickets-fab {
    display: inline-flex;
  }
}
</style>
