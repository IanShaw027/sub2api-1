<template>
  <AppLayout>
    <PageHeader :title="t('tickets.title')" :description="t('tickets.description')">
      <template #actions>
        <Button variant="secondary" @click="resetFilters">{{ t('common.reset') }}</Button>
        <Button
          variant="icon"
          :disabled="loading"
          :title="t('common.refresh')"
          :aria-label="t('common.refresh')"
          @click="loadTickets"
        >
          <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button class="tickets-create-desktop" to="/tickets/new">
          <Icon name="plus" size="md" />
          {{ t('tickets.create') }}
        </Button>
      </template>
    </PageHeader>

    <TablePageLayout>
      <template #filters>
        <div class="filter-row">
          <div class="filter-search">
            <SearchInput
              v-model="filters.search"
              :placeholder="t('tickets.filters.search')"
              @search="applyFilters"
            />
          </div>
          <Select
            v-model="filters.category"
            class="w-[112px]"
            :options="categoryFilterOptions"
            :searchable="false"
            @change="applyFilters"
          />
          <Select
            v-model="filters.status"
            class="w-[128px]"
            :options="statusFilterOptions"
            :searchable="false"
            @change="applyFilters"
          />
          <DateRangePicker
            v-model:start-date="filters.start_date"
            v-model:end-date="filters.end_date"
            @change="applyFilters"
          />
          <span class="filter-count">
            {{ t('common.total') }} <b>{{ pagination.total }}</b>
          </span>
        </div>
      </template>

      <template #table>
        <div class="tickets-table-scroll">
          <DataTable
            :columns="columns"
            :data="items"
            :loading="showInitialLoading"
          >
            <template #cell-title="{ row }">
              <div class="cell-stack">
                <span class="cell-title font-semibold">
                  {{ row.title }}
                  <span v-if="row.unread_by_user" class="unread-dot rounded-full" :title="t('tickets.unreadOnly')" />
                </span>
                <span class="cell-meta text-[11.5px]">#{{ row.ticket_no }} · {{ t(`tickets.categories.${row.category}`) }}</span>
              </div>
            </template>

            <template #cell-status="{ row }">
              <StatusBadge dot :tone="ticketStatusTone(row.status)" :label="t(`tickets.statuses.${row.status}`)" />
            </template>

            <template #cell-created_at="{ value }">
              <span class="cell-time text-[12.5px]" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            </template>

            <template #cell-updated_at="{ value }">
              <span class="cell-time text-[12.5px]" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <ActionsCell
                :show-edit="row.status === 'withdrawn'"
                :edit-label="t('tickets.actions.edit')"
                :more-label="t('common.more')"
                :items="getTicketActionItems(row)"
                @edit="openEdit(row)"
              >
                <template #extra>
                  <button
                    type="button"
                    class="icon-btn"
                    :title="t('tickets.actions.view')"
                    :aria-label="t('tickets.actions.view')"
                    @click="router.push(`/tickets/${row.id}`)"
                  >
                    <Icon name="eye" size="sm" />
                  </button>
                </template>
              </ActionsCell>
            </template>

            <template #empty>
              <div class="empty-state">
                <Icon name="inbox" class="empty-state-icon" :stroke-width="1.6" aria-hidden="true" />
                <p class="empty-state-title">{{ t('tickets.empty') }}</p>
              </div>
            </template>
          </DataTable>
        </div>

        <div v-if="pagination.total > 0" class="tickets-table-footer">
          <UiPagination
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </div>
      </template>
    </TablePageLayout>

    <Fab class="tickets-fab" :label="t('tickets.create')" @click="router.push('/tickets/new')">
      <Icon name="plus" size="md" />
    </Fab>
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
import StatusBadge from '@/components/ui/StatusBadge.vue'
import Fab from '@/components/ui/Fab.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import { ActionsCell, type ActionsCellItem } from '@/components/common/cells'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { ticketsAPI } from '@/api/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { ticketCategoryOptions, ticketStatusOptions } from '@/utils/tickets'
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
const filters = reactive<{ search: string; category: TicketCategory | ''; status: TicketStatus | ''; start_date: string; end_date: string }>({
  search: '',
  category: '',
  status: '',
  start_date: '',
  end_date: '',
})

const columns = computed<Column[]>(() => [
  { key: 'title', label: t('tickets.table.title'), class: 'min-w-[18rem] whitespace-normal' },
  { key: 'status', label: t('tickets.table.status'), class: 'w-32' },
  { key: 'created_at', label: t('tickets.table.createdAt'), class: 'w-28' },
  { key: 'updated_at', label: t('tickets.table.updatedAt'), class: 'w-28' },
  { key: 'actions', label: t('tickets.table.actions'), class: 'w-24 text-right' },
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

const statusToneMap: Record<TicketStatus, 'success' | 'warning' | 'danger' | 'muted' | 'accent'> = {
  submitted: 'accent',
  processing: 'warning',
  waiting_user: 'muted',
  waiting_admin: 'accent',
  resolved: 'success',
  closed: 'muted',
  withdrawn: 'danger',
}

function ticketStatusTone(status: TicketStatus) {
  return statusToneMap[status] ?? 'muted'
}

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
  return status !== 'withdrawn' && status !== 'closed'
}

function getTicketActionItems(ticket: SupportTicket): ActionsCellItem[] {
  const items: ActionsCellItem[] = []
  if (canCloseTicket(ticket.status)) {
    items.push({
      label: t('tickets.actions.close'),
      icon: 'xCircle',
      danger: true,
      onClick: () => closeTicketItem(ticket.id),
    })
  }
  return items
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

function openEdit(ticket: SupportTicket) {
  if (ticket.status === 'withdrawn') {
    router.push({ path: `/tickets/${ticket.id}`, query: { edit: '1' } })
    return
  }
  router.push(`/tickets/${ticket.id}`)
}

onMounted(async () => {
  await loadTickets()
})
</script>
<style scoped>
.tickets-fab {
  display: none;
}

.tickets-table-scroll {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}

.tickets-table-footer {
  flex-shrink: 0;
  padding: 10px 16px;
  border-top: 1px solid var(--border);
}

.cell-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.cell-title {
  line-height: 1.4;
  color: var(--foreground);
  white-space: normal;
  word-break: break-word;
}

.cell-meta {
  line-height: 1.4;
  font-family: var(--font-mono);
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-time {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.unread-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-left: 6px;
  background: var(--danger);
  vertical-align: middle;
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
