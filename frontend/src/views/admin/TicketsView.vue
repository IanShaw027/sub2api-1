<template>
  <AppLayout>
    <PageHeader :title="t('nav.ticketManagement')" :description="t('tickets.description')">
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
      </template>
    </PageHeader>

    <TablePageLayout>
      <template #filters>
        <div class="summary-row">
          <button
            v-for="chip in summaryChips"
            :key="chip.key"
            type="button"
            class="summary-chip"
            :class="{ 'is-active': filters.status === chip.status && filters.unread_only === chip.unreadOnly }"
            @click="applySummaryChip(chip)"
          >
            <span class="summary-chip-label">
              <span class="summary-chip-dot" :style="{ background: chip.color }" />
              {{ chip.label }}
            </span>
            <span class="summary-chip-value">{{ chip.value }}</span>
          </button>
        </div>

        <div class="filter-row">
          <div class="filter-search">
            <SearchInput
              v-model="filters.search"
              :placeholder="t('tickets.filters.adminSearch')"
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
          <label class="filter-toggle">
            <span class="filter-toggle-label text-[13px]">{{ t('tickets.unreadOnly') }}</span>
            <ToggleSwitch :model-value="filters.unread_only" @update:model-value="toggleUnreadOnly" />
          </label>
          <span class="filter-count text-[12.5px]">
            {{ t('tickets.filters.unreadSummary') }} <b>{{ unreadCount }}</b> / {{ pagination.total }}
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
            <template #cell-category="{ row }">
              <span class="tag">{{ t(`tickets.categories.${row.category}`) }}</span>
            </template>

            <template #cell-title="{ row }">
              <div class="cell-stack">
                <span class="cell-title font-semibold">
                  {{ row.title }}
                  <span v-if="row.unread_by_admin" class="unread-dot rounded-full" :title="t('tickets.unreadOnly')" />
                </span>
                <span class="cell-meta text-[11.5px]">#{{ row.ticket_no }} · {{ t(`tickets.categories.${row.category}`) }}</span>
              </div>
            </template>

            <template #cell-user_name="{ row }">
              <div class="cell-stack">
                <span class="cell-title font-semibold">{{ row.user_name || row.user_email || '—' }}</span>
                <span v-if="row.user_name && row.user_email" class="cell-meta text-[11.5px]">{{ row.user_email }}</span>
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
              <div class="row-actions">
                <button
                  type="button"
                  class="icon-btn"
                  :title="t('tickets.actions.view')"
                  :aria-label="t('tickets.actions.view')"
                  @click="router.push(`/admin/tickets/${row.id}`)"
                >
                  <Icon name="eye" size="sm" />
                </button>
              </div>
            </template>

            <template #empty>
              <div class="empty-state">
                <Icon name="inbox" class="empty-state-icon" :stroke-width="1.6" aria-hidden="true" />
                <p class="empty-state-title">{{ t('tickets.empty') }}</p>
              </div>
            </template>
          </DataTable>
        </div>

      </template>
      <template v-if="pagination.total > 0" #pagination>
          <Pagination
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
import StatusBadge from '@/components/ui/StatusBadge.vue'
import Pagination from '@/components/common/Pagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Select from '@/components/common/Select.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import ToggleSwitch from '@/components/ui/ToggleSwitch.vue'
import type { Column } from '@/components/common/types'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores'
import { adminTicketsAPI } from '@/api/admin/tickets'
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
const unreadCount = ref(0)
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
  { key: 'title', label: t('tickets.table.title'), class: 'min-w-[14rem] whitespace-normal' },
  { key: 'category', label: t('tickets.table.category'), class: 'w-24' },
  { key: 'user_name', label: t('tickets.table.user'), class: 'min-w-[11rem] whitespace-normal' },
  { key: 'status', label: t('tickets.table.status'), class: 'w-32' },
  { key: 'created_at', label: t('tickets.table.createdAt'), class: 'w-28' },
  { key: 'updated_at', label: t('tickets.table.updatedAt'), class: 'w-28' },
  { key: 'actions', label: t('tickets.table.actions'), class: 'w-20 text-right' },
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

type SummaryChip = {
  key: string
  label: string
  color: string
  value: number | string
  status: TicketStatus | ''
  unreadOnly: boolean
}

const pageStatusCount = (status: TicketStatus) => items.value.filter((item) => item.status === status).length

const summaryChips = computed<SummaryChip[]>(() => [
  { key: 'all', label: t('tickets.filters.allStatuses'), color: 'var(--muted)', value: pagination.total, status: '', unreadOnly: false },
  { key: 'unread', label: t('tickets.unreadOnly'), color: 'var(--danger)', value: unreadCount.value, status: '', unreadOnly: true },
  { key: 'submitted', label: t('tickets.statuses.submitted'), color: 'var(--accent)', value: pageStatusCount('submitted'), status: 'submitted', unreadOnly: false },
  { key: 'processing', label: t('tickets.statuses.processing'), color: 'var(--warning)', value: pageStatusCount('processing'), status: 'processing', unreadOnly: false },
  { key: 'resolved', label: t('tickets.statuses.resolved'), color: 'var(--success)', value: pageStatusCount('resolved'), status: 'resolved', unreadOnly: false },
])

function applySummaryChip(chip: SummaryChip) {
  filters.status = chip.status
  filters.unread_only = chip.unreadOnly
  applyFilters()
}

function toggleUnreadOnly() {
  filters.unread_only = !filters.unread_only
  applyFilters()
}

let loadTicketsRequestID = 0

function ticketError(err: unknown) {
  return extractI18nErrorMessage(err, t, 'tickets.errors', t('common.unknownError'))
}

async function loadUnreadCount() {
  try {
    const res = await adminTicketsAPI.unreadCount()
    unreadCount.value = res.data?.count ?? 0
  } catch {
    /* the unread badge is decorative — keep the previous value */
  }
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

onMounted(() => {
  loadTickets()
  loadUnreadCount()
})
</script>

<style scoped>
.summary-row {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
  margin-bottom: 14px;
}

.filter-row {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-search {
  width: 260px;
  flex: none;
}

.filter-toggle {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  height: 36px;
  padding: 0 12px;
  border-radius: var(--radius-field);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  border: 1px solid var(--border);
  box-shadow: var(--field-shadow);
  cursor: pointer;
  flex: none;
}

.filter-toggle-label {
  color: var(--foreground);
  white-space: nowrap;
}

.filter-count {
  margin-left: auto;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.filter-count b {
  color: var(--foreground);
}

.tickets-table-scroll {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
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

.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

@media (max-width: 767px) {
  .summary-row {
    grid-template-columns: repeat(2, 1fr);
  }

  .filter-search {
    width: 100%;
  }

  .filter-count {
    margin-left: 0;
  }

  .row-actions .icon-btn {
    width: 44px;
    height: 44px;
  }
}
</style>
