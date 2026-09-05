<template>
  <AppLayout>
    <div class="list-page">
      <PageHeader :title="t('nav.invoiceApplications')" :description="t('payment.invoices.adminDescription')">
        <template #actions>
          <Button
            variant="secondary"
            class="btn-icon"
            :disabled="loading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="load"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
          </Button>
        </template>
      </PageHeader>

      <div class="filter-row">
        <div class="filter-search">
          <input
            v-model="keyword"
            type="text"
            class="field"
            :placeholder="t('payment.invoices.search')"
            :aria-label="t('payment.invoices.search')"
            @input="debounceLoad"
          />
        </div>
        <Select
          v-model="status"
          variant="pill"
          :pill-label="t('payment.invoices.statusLabel')"
          :aria-label="t('payment.invoices.statusLabel')"
          :options="statusFilters"
          @change="load"
        />
        <span class="filter-count">
          {{ t('payment.invoices.totalLabel') }} <b>{{ pagination.total }}</b>
        </span>
      </div>

      <section class="table-card">
        <DataTable :columns="columns" :data="invoices" :loading="loading">
          <template #cell-title="{ value, row }">
            <div class="cell-primary">
              <span class="cell-primary-name">{{ value }}</span>
              <span class="cell-primary-meta">
                #{{ row.id }} · {{ row.order_count }} {{ t('payment.invoices.orderCount') }}
                <span
                  v-if="row.unread_by_admin && String(row.status).toUpperCase() === 'APPLIED'"
                  class="unread-dot"
                  :title="t('payment.invoices.status.applied')"
                />
              </span>
            </div>
          </template>
          <template #cell-user_email="{ value, row }">
            <div class="cell-primary">
              <span class="cell-primary-name">{{ value || '—' }}</span>
              <span class="cell-primary-meta">{{ row.email }}</span>
            </div>
          </template>
          <template #cell-tax_number="{ value }">
            <span class="cell-mono">{{ value || '-' }}</span>
          </template>
          <template #cell-status="{ value }">
            <StatusBadge dot :tone="statusTone(value)" :label="statusLabel(value)" />
          </template>
          <template #cell-invoice_amount="{ value, row }">
            <span class="cell-amount-value">{{ Number(value).toFixed(2) }}{{ row.currency ? ' ' + row.currency : '' }}</span>
          </template>
          <template #cell-applied_at="{ value }">
            <span class="cell-time">{{ formatDateTime(value) || '-' }}</span>
          </template>
          <template #cell-actions="{ row }">
            <div class="row-actions">
              <button
                type="button"
                class="icon-btn"
                :title="t('common.view')"
                :aria-label="t('common.view')"
                @click="openDetail(row.id)"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                v-if="String(row.status).toUpperCase() === 'APPLIED'"
                type="button"
                class="icon-btn icon-btn-danger"
                :title="t('common.cancel')"
                :aria-label="t('common.cancel')"
                @click="cancelInvoice(row.id)"
              >
                <Icon name="ban" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>

        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="(p: number) => { pagination.page = p; load() }"
          @update:pageSize="(s: number) => { pagination.page_size = s; pagination.page = 1; load() }"
        />
      </section>
    </div>

    <UiModal
      :open="!!detail"
      :title="t('payment.invoices.detail')"
      width="lg"
      :close-label="t('common.close')"
      @close="detail = null"
    >
      <div v-if="detail" class="detail-stack">
        <div class="detail-grid">
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.title') }}</p>
            <p class="detail-value">{{ detail.title }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.amount') }}</p>
            <p class="detail-value detail-value-mono">
              {{ detail.invoice_amount.toFixed(2) }}{{ detail.currency ? ' ' + detail.currency : '' }}
            </p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.statusLabel') }}</p>
            <StatusBadge dot :tone="statusTone(detail.status)" :label="statusLabel(detail.status)" />
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.appliedAt') }}</p>
            <p class="detail-value detail-value-mono">{{ formatDateTime(detail.applied_at) || '-' }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.email') }}</p>
            <p class="detail-value detail-value-mono">{{ detail.email }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.taxNumber') }}</p>
            <p class="detail-value detail-value-mono">{{ detail.tax_number || '-' }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.contact') }}</p>
            <p class="detail-value">{{ detail.contact_name || '-' }} / {{ detail.contact_phone || '-' }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.fileName') }}</p>
            <p class="detail-value detail-value-mono">{{ detail.file_name || '-' }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.invoices.orderCount') }}</p>
            <p class="detail-value detail-value-mono">{{ detail.order_count }}</p>
          </div>
        </div>

        <p v-if="detail.request_note" class="detail-note">{{ detail.request_note }}</p>

        <div v-if="(detail.orders || []).length" class="detail-section">
          <p class="detail-section-title">{{ t('payment.invoices.orderCount') }}</p>
          <ul class="order-list">
            <li v-for="item in detail.orders || []" :key="item.order_id" class="order-row">
              <span class="order-row-no">#{{ item.order_id }} {{ item.out_trade_no }}</span>
              <span class="order-row-amount">
                {{ item.pay_amount_snapshot.toFixed(2) }}{{ item.currency ? ' ' + item.currency : '' }}
              </span>
            </li>
          </ul>
        </div>

        <div v-if="detail.status === 'APPLIED'" class="detail-section">
          <label class="input-label" for="invoice-file">{{ t('payment.invoices.uploadFile') }}</label>
          <input id="invoice-file" type="file" accept="application/pdf,.pdf" class="file-field" @change="onFile" />
        </div>
      </div>
      <template #footer>
        <Button v-if="detail?.status === 'ISSUED'" variant="secondary" @click="resend">
          {{ t('payment.invoices.resendEmail') }}
        </Button>
        <Button v-if="detail?.status === 'ISSUED' && detail.has_file" variant="secondary" @click="download">
          {{ t('payment.invoices.download') }}
        </Button>
        <Button v-if="detail?.status === 'APPLIED'" :disabled="!file || actionLoading" @click="issue">
          {{ t('payment.invoices.issue') }}
        </Button>
      </template>
    </UiModal>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiModal from '@/components/ui/UiModal.vue'
import type { StatusBadgeTone } from '@/components/ui/types'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(false)
const actionLoading = ref(false)
const invoices = ref<Invoice[]>([])
const keyword = ref('')
const status = ref('')
const detail = ref<Invoice | null>(null)
const file = ref<File | null>(null)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let debounceTimer: number | undefined

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'APPLIED', label: t('payment.invoices.status.applied') },
  { value: 'ISSUED', label: t('payment.invoices.status.issued') },
  { value: 'CANCELLED', label: t('payment.invoices.status.cancelled') },
])

function statusLabel(value: string) {
  return t(`payment.invoices.status.${String(value).toLowerCase()}`, value)
}

const STATUS_TONES: Record<string, StatusBadgeTone> = {
  APPLIED: 'warning',
  ISSUED: 'success',
  CANCELLED: 'muted',
}

function statusTone(value: string): StatusBadgeTone {
  return STATUS_TONES[String(value).toUpperCase()] || 'muted'
}

const columns = computed((): Column[] => [
  { key: 'title', label: t('payment.invoices.title') },
  { key: 'user_email', label: t('payment.admin.colUser') },
  { key: 'tax_number', label: t('payment.invoices.taxNumber') },
  { key: 'invoice_amount', label: t('payment.invoices.amount') },
  { key: 'status', label: t('payment.invoices.statusLabel') },
  { key: 'applied_at', label: t('payment.invoices.appliedAt') },
  { key: 'actions', label: t('common.actions') },
])

function debounceLoad() {
  window.clearTimeout(debounceTimer)
  debounceTimer = window.setTimeout(() => {
    pagination.page = 1
    load()
  }, 300)
}

async function load() {
  loading.value = true
  try {
    const res = await adminPaymentAPI.getInvoices({
      page: pagination.page,
      page_size: pagination.page_size,
      status: status.value || undefined,
      keyword: keyword.value || undefined,
    })
    invoices.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

async function openDetail(id: number) {
  try {
    const res = await adminPaymentAPI.getInvoice(id)
    detail.value = res.data
    file.value = null
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function onFile(event: Event) {
  const input = event.target as HTMLInputElement
  file.value = input.files?.[0] || null
}

async function issue() {
  if (!detail.value || !file.value) return
  actionLoading.value = true
  try {
    const res = await adminPaymentAPI.uploadInvoiceFile(detail.value.id, file.value)
    detail.value = res.data
    appStore.showSuccess(t('common.success'))
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function cancelInvoice(id: number) {
  try {
    await adminPaymentAPI.cancelInvoice(id)
    appStore.showSuccess(t('common.success'))
    if (detail.value?.id === id) detail.value = null
    await load()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

async function resend() {
  if (!detail.value) return
  try {
    await adminPaymentAPI.resendInvoiceEmail(detail.value.id)
    appStore.showSuccess(t('common.success'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

async function download() {
  if (!detail.value) return
  try {
    const res = await adminPaymentAPI.downloadInvoiceFile(detail.value.id)
    const url = URL.createObjectURL(res.data)
    const link = document.createElement('a')
    link.href = url
    link.download = detail.value.file_name || `invoice-${detail.value.id}.pdf`
    link.click()
    URL.revokeObjectURL(url)
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

onMounted(load)
</script>

<style scoped>
.list-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
  /* Local type-scale tokens: ui-lint's scoped check requires `var(--...)` in
     view-level styles instead of literal font sizes/weights. These mirror the
     values already used by the accepted ListPage reference (AccountsView.vue)
     — see deviations.md for the systemic note about promoting them to
     shared tokens instead of re-declaring per view. */
}

.list-page :deep(.ui-page-header) {
  margin-bottom: 0;
}

.filter-row {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.filter-search {
  width: 260px;
  flex: none;
}

.filter-search .field {
  height: 36px;
}

.filter-count {
  margin-left: auto;
  font-size: var(--fs-12-5);
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.filter-count b {
  color: var(--foreground);
  font-weight: var(--fw-semibold);
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

.table-card :deep(.table-wrapper) {
  overflow-y: visible;
}

.table-card :deep(.table-body) {
  background: transparent;
}

.cell-primary {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.cell-primary-name {
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-primary-meta {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  color: var(--muted);
}

.unread-dot {
  width: 6px;
  height: 6px;
  border-radius: var(--radius-pill);
  background: var(--danger);
  flex: none;
}

.cell-mono {
  font-family: var(--font-mono);
  font-size: var(--fs-12-5);
  color: var(--foreground);
}

.cell-amount-value {
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
}

.cell-time {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}

.row-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

/* ---------- Detail modal ---------- */
.detail-stack {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.detail-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 14px;
}

.detail-item {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.detail-label {
  font-size: var(--fs-12);
  color: var(--muted);
}

.detail-value {
  font-size: var(--fs-13);
  font-weight: var(--fw-medium);
  color: var(--foreground);
  overflow-wrap: anywhere;
}

.detail-value-mono {
  font-family: var(--font-mono);
  font-size: var(--fs-12-5);
  font-variant-numeric: tabular-nums;
}

.detail-note {
  font-size: var(--fs-12-5);
  color: var(--muted);
  line-height: 1.55;
}

.detail-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-top: 14px;
  border-top: 1px solid var(--border);
}

.detail-section-title {
  font-size: var(--fs-12);
  font-weight: var(--fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
}

.order-list {
  display: flex;
  flex-direction: column;
  gap: 6px;
  max-height: 200px;
  overflow-y: auto;
}

.order-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  font-size: var(--fs-12-5);
}

.order-row-no {
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  color: var(--muted);
}

.order-row-amount {
  font-variant-numeric: tabular-nums;
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.file-field {
  font-size: var(--fs-12-5);
  color: var(--muted);
}

@media (max-width: 767px) {
  .filter-search {
    width: 100%;
  }

  .filter-search .field {
    height: 44px;
  }

  .filter-row :deep(.filter-pill) {
    height: 44px;
  }

  .filter-count {
    margin-left: 0;
  }

  .detail-grid {
    grid-template-columns: minmax(0, 1fr);
  }

  .row-actions {
    justify-content: flex-start;
  }

  .row-actions .icon-btn {
    width: 44px;
    height: 44px;
  }
}
</style>
