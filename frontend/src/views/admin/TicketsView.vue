<template>
  <AppLayout>
    <div class="list-page">
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
          variant="pill"
          :pill-label="t('tickets.table.category')"
          :options="categoryFilterOptions"
          :searchable="false"
          @change="applyFilters"
        />
        <Select
          v-model="filters.status"
          variant="pill"
          :pill-label="t('tickets.table.status')"
          :options="statusFilterOptions"
          :searchable="false"
          @change="applyFilters"
        />
        <input
          v-model="filters.start_date"
          type="date"
          class="field filter-date"
          :aria-label="t('tickets.table.createdAt')"
          @change="applyFilters"
        />
        <span class="filter-dash" aria-hidden="true">–</span>
        <input
          v-model="filters.end_date"
          type="date"
          class="field filter-date"
          :aria-label="t('tickets.table.updatedAt')"
          @change="applyFilters"
        />
        <button
          type="button"
          class="filter-pill"
          :class="{ 'is-active': filters.unread_only }"
          :aria-pressed="filters.unread_only"
          @click="toggleUnreadOnly"
        >
          <span class="filter-pill-label">{{ t('tickets.unreadOnly') }}</span>
          <span class="filter-pill-value">{{ filters.unread_only ? t('common.yes') : t('common.no') }}</span>
        </button>
        <span class="filter-count">
          {{ t('tickets.filters.unreadSummary') }} <b>{{ unreadCount }}</b> / {{ pagination.total }}
        </span>
      </div>

      <div class="table-card">
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
              <span class="cell-title">
                {{ row.title }}
                <span v-if="row.unread_by_admin" class="unread-dot" :title="t('tickets.unreadOnly')" />
              </span>
              <span class="cell-meta">#{{ row.ticket_no }} · {{ t(`tickets.categories.${row.category}`) }}</span>
            </div>
          </template>

          <template #cell-user_name="{ row }">
            <div class="cell-stack">
              <span class="cell-title">{{ row.user_name || row.user_email || '—' }}</span>
              <span v-if="row.user_name && row.user_email" class="cell-meta">{{ row.user_email }}</span>
            </div>
          </template>

          <template #cell-status="{ row }">
            <StatusBadge dot :tone="ticketStatusTone(row.status)" :label="t(`tickets.statuses.${row.status}`)" />
          </template>

          <template #cell-created_at="{ value }">
            <span class="cell-time">{{ formatRelativeWithDateTime(value) }}</span>
          </template>

          <template #cell-updated_at="{ value }">
            <span class="cell-time">{{ formatRelativeWithDateTime(value) }}</span>
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

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
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
  { key: 'title', label: t('tickets.table.title'), class: 'min-w-[18rem] whitespace-normal' },
  { key: 'category', label: t('tickets.table.category'), class: 'w-36' },
  { key: 'user_name', label: t('tickets.table.user'), class: 'min-w-[14rem] whitespace-normal' },
  { key: 'status', label: t('tickets.table.status'), class: 'w-40' },
  { key: 'created_at', label: t('tickets.table.createdAt'), class: 'w-44' },
  { key: 'updated_at', label: t('tickets.table.updatedAt'), class: 'w-44' },
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
.list-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.summary-row {
  display: grid;
  grid-template-columns: repeat(5, 1fr);
  gap: 10px;
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

.filter-date {
  width: 150px;
  flex: none;
}

.filter-dash {
  font-size: 12.5px;
  color: var(--muted);
}

.filter-count {
  margin-left: auto;
  font-size: 12.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.filter-count b {
  color: var(--foreground);
}

.table-card {
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-radius: var(--radius-card);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  background: color-mix(in oklch, var(--surface) 70%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow: var(--shadow);
  overflow: hidden;
}

.cell-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.cell-title {
  font-weight: 600;
  color: var(--foreground);
  white-space: normal;
  word-break: break-word;
}

.cell-meta {
  font-size: 11.5px;
  font-family: var(--font-mono);
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-time {
  font-size: 12.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.unread-dot {
  display: inline-block;
  width: 8px;
  height: 8px;
  margin-left: 6px;
  border-radius: 999px;
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

  .table-card {
    border: 0;
    background: transparent;
    box-shadow: none;
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
    overflow: visible;
  }

  .row-actions .icon-btn {
    width: 44px;
    height: 44px;
  }
}
</style>
