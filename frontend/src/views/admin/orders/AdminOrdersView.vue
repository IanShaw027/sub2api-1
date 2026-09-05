<template>
  <AppLayout>
    <div class="list-page">
      <PageHeader :title="t('nav.orderManagement')" :description="t('payment.admin.ordersDescription')">
        <template #actions>
          <Button
            variant="secondary"
            class="btn-icon"
            :disabled="ordersLoading"
            :title="t('common.refresh')"
            :aria-label="t('common.refresh')"
            @click="loadOrders"
          >
            <Icon name="refresh" size="sm" :class="ordersLoading ? 'animate-spin' : ''" />
          </Button>
        </template>
      </PageHeader>

      <div class="filter-row">
        <div class="filter-search">
          <input
            v-model="orderSearch"
            type="text"
            :placeholder="t('payment.admin.searchOrders')"
            :aria-label="t('payment.admin.searchOrders')"
            class="field"
            @input="debounceLoadOrders"
          />
        </div>
        <Select
          v-model="orderFilters.status"
          variant="pill"
          :pill-label="t('payment.orders.status')"
          :aria-label="t('payment.admin.allStatuses')"
          :options="statusFilterOptions"
          @change="loadOrders"
        />
        <Select
          v-model="orderFilters.payment_type"
          variant="pill"
          :pill-label="t('payment.orders.paymentMethod')"
          :aria-label="t('payment.admin.allPaymentTypes')"
          :options="paymentTypeFilterOptions"
          @change="loadOrders"
        />
        <Select
          v-model="orderFilters.order_type"
          variant="pill"
          :pill-label="t('payment.admin.orderType')"
          :aria-label="t('payment.admin.allOrderTypes')"
          :options="orderTypeFilterOptions"
          @change="loadOrders"
        />
        <span class="filter-count">
          {{ t('payment.admin.totalOrdersLabel') }} <b>{{ orderPagination.total }}</b>
        </span>
      </div>

      <section class="table-card">
        <DataTable :columns="orderColumns" :data="orders" :loading="ordersLoading">
          <template #cell-out_trade_no="{ row }">
            <div class="cell-primary">
              <span class="cell-primary-name">{{ row.out_trade_no || '—' }}</span>
              <span class="cell-primary-meta">#{{ row.id }} · {{ orderTypeLabel(row.order_type) }}</span>
            </div>
          </template>

          <template #cell-user_email="{ value, row }">
            <div class="cell-primary">
              <span class="cell-primary-name">{{ value || row.user_name || '#' + row.user_id }}</span>
              <span class="cell-primary-meta">
                #{{ row.user_id }}<template v-if="row.user_notes"> · {{ row.user_notes }}</template>
              </span>
            </div>
          </template>

          <template #cell-pay_amount="{ value, row }">
            <div class="cell-amount">
              <span class="cell-amount-value">{{ paymentAmountSymbol(row) }}{{ value.toFixed(2) }}</span>
              <span v-if="row.fee_rate > 0 || row.amount !== row.pay_amount" class="cell-amount-meta">
                <template v-if="row.fee_rate > 0">{{ t('payment.orders.fee') }} {{ row.fee_rate }}%</template>
                <template v-if="row.fee_rate > 0 && row.amount !== row.pay_amount"> · </template>
                <template v-if="row.amount !== row.pay_amount">
                  {{ t('payment.orders.creditedAmount') }} {{ creditedAmountSymbol }}{{ row.amount.toFixed(2) }}
                </template>
              </span>
            </div>
          </template>

          <template #cell-payment_type="{ value }">
            <span class="tag">{{ t('payment.methods.' + value, value) }}</span>
          </template>

          <template #cell-status="{ value }">
            <StatusBadge dot :tone="statusTone(value)" :label="t('payment.status.' + String(value).toLowerCase(), value)" />
          </template>

          <template #cell-created_at="{ value }">
            <span class="cell-time">{{ formatDateTime(value) }}</span>
          </template>

          <template #cell-actions="{ row }">
            <div class="row-actions">
              <span v-if="row.status === 'REFUND_REQUESTED' && row.refund_amount" class="tag tag-warning">
                {{ creditedAmountSymbol }}{{ row.refund_amount.toFixed(2) }}
              </span>
              <button
                type="button"
                class="icon-btn"
                :title="t('common.view')"
                :aria-label="t('common.view')"
                @click="showOrderDetail(row)"
              >
                <Icon name="eye" size="sm" />
              </button>
              <button
                v-if="row.status === 'PENDING'"
                type="button"
                class="icon-btn"
                :title="t('payment.orders.cancel')"
                :aria-label="t('payment.orders.cancel')"
                @click="handleCancelOrder(row)"
              >
                <Icon name="x" size="sm" />
              </button>
              <button
                v-else-if="row.status === 'FAILED'"
                type="button"
                class="icon-btn"
                :title="t('payment.admin.retry')"
                :aria-label="t('payment.admin.retry')"
                @click="handleRetryOrder(row)"
              >
                <Icon name="refresh" size="sm" />
              </button>
              <button
                v-else-if="row.status === 'REFUND_REQUESTED'"
                type="button"
                class="icon-btn"
                :title="t('payment.admin.approveRefund')"
                :aria-label="t('payment.admin.approveRefund')"
                @click="openRefundDialog(row)"
              >
                <Icon name="check" size="sm" />
              </button>
              <button
                v-else-if="row.status === 'REFUND_FAILED'"
                type="button"
                class="icon-btn"
                :title="t('payment.admin.retryRefund')"
                :aria-label="t('payment.admin.retryRefund')"
                @click="openRefundDialog(row)"
              >
                <Icon name="refresh" size="sm" />
              </button>
              <button
                v-else-if="row.status === 'REFUND_PENDING'"
                type="button"
                class="icon-btn"
                :disabled="refundQueryingIds.has(row.id)"
                :title="t('payment.admin.queryRefundStatus')"
                :aria-label="t('payment.admin.queryRefundStatus')"
                @click="handleQueryRefund(row)"
              >
                <Icon name="refresh" size="sm" :class="refundQueryingIds.has(row.id) ? 'animate-spin' : ''" />
              </button>
              <button
                v-else-if="row.status === 'COMPLETED' || row.status === 'PARTIALLY_REFUNDED'"
                type="button"
                class="icon-btn icon-btn-danger"
                :title="t('payment.admin.refund')"
                :aria-label="t('payment.admin.refund')"
                @click="openRefundDialog(row)"
              >
                <Icon name="dollar" size="sm" />
              </button>
            </div>
          </template>
        </DataTable>

        <Pagination
          v-if="orderPagination.total > 0"
          :page="orderPagination.page"
          :total="orderPagination.total"
          :page-size="orderPagination.page_size"
          @update:page="handleOrderPageChange"
          @update:pageSize="handleOrderPageSizeChange"
        />
      </section>
    </div>

    <!-- Order Detail Dialog -->
    <UiModal
      :open="showDetailDialog"
      :title="t('payment.admin.orderDetail')"
      width="lg"
      :close-label="t('common.close')"
      @close="showDetailDialog = false"
    >
      <div v-if="selectedOrder" class="detail-stack">
        <div class="detail-grid">
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.orderId') }}</p>
            <p class="detail-value detail-value-mono">#{{ selectedOrder.id }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.orderNo') }}</p>
            <p class="detail-value detail-value-mono">{{ selectedOrder.out_trade_no }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.status') }}</p>
            <StatusBadge
              dot
              :tone="statusTone(selectedOrder.status)"
              :label="t('payment.status.' + String(selectedOrder.status).toLowerCase(), selectedOrder.status)"
            />
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.amount') }}</p>
            <p class="detail-value">{{ creditedAmountSymbol }}{{ selectedOrder.amount.toFixed(2) }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.payAmount') }}</p>
            <p class="detail-value">{{ paymentAmountSymbol(selectedOrder) }}{{ selectedOrder.pay_amount.toFixed(2) }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.paymentMethod') }}</p>
            <p class="detail-value">{{ t('payment.methods.' + selectedOrder.payment_type, selectedOrder.payment_type) }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.admin.feeRate') }}</p>
            <p class="detail-value">{{ selectedOrder.fee_rate }}%</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.orders.createdAt') }}</p>
            <p class="detail-value detail-value-mono">{{ formatDateTime(selectedOrder.created_at) }}</p>
          </div>
          <div class="detail-item">
            <p class="detail-label">{{ t('payment.admin.expiresAt') }}</p>
            <p class="detail-value detail-value-mono">{{ formatDateTime(selectedOrder.expires_at) }}</p>
          </div>
          <div v-if="selectedOrder.paid_at" class="detail-item">
            <p class="detail-label">{{ t('payment.admin.paidAt') }}</p>
            <p class="detail-value detail-value-mono">{{ formatDateTime(selectedOrder.paid_at) }}</p>
          </div>
          <div v-if="selectedOrder.refund_amount" class="detail-item">
            <p class="detail-label">{{ t('payment.admin.refundAmount') }}</p>
            <p class="detail-value detail-value-danger">{{ creditedAmountSymbol }}{{ selectedOrder.refund_amount.toFixed(2) }}</p>
          </div>
          <div v-if="selectedOrder.refund_reason" class="detail-item detail-item-wide">
            <p class="detail-label">{{ t('payment.admin.refundReason') }}</p>
            <p class="detail-value">{{ selectedOrder.refund_reason }}</p>
          </div>
        </div>

        <!-- Refund request info -->
        <div v-if="selectedOrder.refund_requested_at" class="detail-section">
          <p class="detail-section-title">{{ t('payment.admin.refundRequestInfo') }}</p>
          <div class="detail-grid">
            <div class="detail-item">
              <p class="detail-label">{{ t('payment.admin.refundRequestedAt') }}</p>
              <p class="detail-value detail-value-mono">{{ formatDateTime(selectedOrder.refund_requested_at) }}</p>
            </div>
            <div class="detail-item">
              <p class="detail-label">{{ t('payment.admin.refundRequestedBy') }}</p>
              <p class="detail-value detail-value-mono">#{{ selectedOrder.refund_requested_by }}</p>
            </div>
            <div class="detail-item detail-item-wide">
              <p class="detail-label">{{ t('payment.admin.refundRequestReason') }}</p>
              <p class="detail-value">{{ selectedOrder.refund_request_reason }}</p>
            </div>
          </div>
        </div>

        <!-- Audit Logs -->
        <div v-if="orderAuditLogs.length > 0" class="detail-section">
          <p class="detail-section-title">{{ t('payment.admin.auditLogs') }}</p>
          <div class="audit-list">
            <div v-for="log in orderAuditLogs" :key="log.id" class="audit-item">
              <div class="audit-item-head">
                <span class="audit-item-action">{{ log.action }}</span>
                <span class="audit-item-time">{{ formatDateTime(log.created_at) }}</span>
              </div>
              <p v-if="log.detail" class="audit-item-detail">{{ log.detail }}</p>
              <p v-if="log.operator" class="audit-item-operator">{{ t('payment.admin.operator') }}: {{ log.operator }}</p>
            </div>
          </div>
        </div>
      </div>
    </UiModal>

    <AdminRefundDialog
      :show="showRefundDialog"
      :order="selectedOrder"
      :submitting="refundSubmitting"
      :require-force="refundRequireForce"
      :warning="refundWarning"
      @confirm="handleRefund"
      @cancel="closeRefundDialog"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatOrderDateTime } from '@/components/payment/orderUtils'
import type { PaymentOrder } from '@/types/payment'
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
import AdminRefundDialog from '@/components/admin/payment/AdminRefundDialog.vue'
import { currencySymbol } from '@/components/payment/currency'

interface AuditLog {
  id: number
  action: string
  detail: string | null
  operator: string | null
  created_at: string
}

const { t } = useI18n()
const appStore = useAppStore()

const ordersLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const orderSearch = ref('')
const orderFilters = reactive({ status: '', payment_type: '', order_type: '' })
const orderPagination = reactive({ page: 1, page_size: 20, total: 0 })
const selectedOrder = ref<PaymentOrder | null>(null)
const showDetailDialog = ref(false)
const showRefundDialog = ref(false)
const refundSubmitting = ref(false)
const refundRequireForce = ref(false)
const refundWarning = ref('')
const refundQueryingIds = ref(new Set<number>())
const orderAuditLogs = ref<AuditLog[]>([])
const creditedAmountSymbol = currencySymbol('USD')

const orderColumns = computed((): Column[] => [
  { key: 'out_trade_no', label: t('payment.orders.orderNo') },
  { key: 'user_email', label: t('payment.admin.colUser') },
  { key: 'pay_amount', label: t('payment.orders.payAmount') },
  { key: 'payment_type', label: t('payment.orders.paymentMethod') },
  { key: 'status', label: t('payment.orders.status') },
  { key: 'created_at', label: t('payment.orders.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

const STATUS_TONES: Record<string, StatusBadgeTone> = {
  PENDING: 'warning',
  PAID: 'accent',
  RECHARGING: 'accent',
  COMPLETED: 'success',
  EXPIRED: 'muted',
  CANCELLED: 'muted',
  FAILED: 'danger',
  REFUND_REQUESTED: 'warning',
  REFUNDING: 'warning',
  REFUND_PENDING: 'warning',
  PARTIALLY_REFUNDED: 'warning',
  REFUNDED: 'accent',
  REFUND_FAILED: 'danger',
}

function statusTone(status: string): StatusBadgeTone {
  return STATUS_TONES[status] || 'muted'
}

function orderTypeLabel(orderType: string): string {
  return t('payment.admin.' + orderType + 'Order', orderType)
}

function paymentAmountSymbol(order: PaymentOrder | null | undefined): string {
  return currencySymbol(order?.currency)
}

let debounceTimer: ReturnType<typeof setTimeout> | null = null
function debounceLoadOrders() {
  if (debounceTimer) clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => loadOrders(), 300)
}

async function loadOrders() {
  ordersLoading.value = true
  try {
    const res = await adminPaymentAPI.getOrders({
      page: orderPagination.page, page_size: orderPagination.page_size,
      keyword: orderSearch.value || undefined, status: orderFilters.status || undefined,
      payment_type: orderFilters.payment_type || undefined, order_type: orderFilters.order_type || undefined,
    })
    orders.value = res.data.items || []
    orderPagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally { ordersLoading.value = false }
}

function handleOrderPageChange(page: number) { orderPagination.page = page; loadOrders() }
function handleOrderPageSizeChange(size: number) { orderPagination.page_size = size; orderPagination.page = 1; loadOrders() }

const statusFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allStatuses') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'PAID', label: t('payment.status.paid') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'EXPIRED', label: t('payment.status.expired') },
  { value: 'CANCELLED', label: t('payment.status.cancelled') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
  { value: 'REFUND_REQUESTED', label: t('payment.status.refund_requested') },
  { value: 'REFUND_PENDING', label: t('payment.status.refund_pending') },
  { value: 'REFUND_FAILED', label: t('payment.status.refund_failed') },
])

const paymentTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allPaymentTypes') },
  { value: 'alipay', label: t('payment.methods.alipay') },
  { value: 'wxpay', label: t('payment.methods.wxpay') },
  { value: 'stripe', label: t('payment.methods.stripe') },
  { value: 'airwallex', label: t('payment.methods.airwallex') },
])

const orderTypeFilterOptions = computed(() => [
  { value: '', label: t('payment.admin.allOrderTypes') },
  { value: 'balance', label: t('payment.admin.balanceOrder') },
  { value: 'subscription', label: t('payment.admin.subscriptionOrder') },
])

async function showOrderDetail(order: PaymentOrder) {
  selectedOrder.value = order
  orderAuditLogs.value = []
  showDetailDialog.value = true
  try {
    const res = await adminPaymentAPI.getOrder(order.id)
    const data = res.data as unknown as Record<string, unknown>
    if (data.order) selectedOrder.value = data.order as PaymentOrder
    orderAuditLogs.value = ((data.auditLogs || data.audit_logs || []) as unknown) as AuditLog[]
  } catch (_err: unknown) { /* keep cached order data */ }
}

async function handleCancelOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.cancelOrder(order.id); appStore.showSuccess(t('payment.admin.orderCancelled')); loadOrders() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

async function handleRetryOrder(order: PaymentOrder) {
  try { await adminPaymentAPI.retryRecharge(order.id); appStore.showSuccess(t('payment.admin.retrySuccess')); loadOrders() }
  catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
}

function openRefundDialog(order: PaymentOrder) {
  selectedOrder.value = order
  refundRequireForce.value = false
  refundWarning.value = ''
  showRefundDialog.value = true
}

function closeRefundDialog() {
  showRefundDialog.value = false
  refundRequireForce.value = false
  refundWarning.value = ''
}

function isRefundPendingWarning(warning: string | undefined): boolean {
  return /pending|处理中|待/.test(String(warning || '').toLowerCase())
}

async function handleRefund(data: { amount: number; reason: string; deduct_balance: boolean; force: boolean }) {
  if (!selectedOrder.value) return
  refundSubmitting.value = true
  try {
    const res = await adminPaymentAPI.refundOrder(selectedOrder.value.id, { amount: data.amount, reason: data.reason, deduct_balance: data.deduct_balance, force: data.force })
    if (res.data.success) {
      appStore.showSuccess(t('payment.admin.refundSuccess'))
      closeRefundDialog()
      loadOrders()
      return
    }
    if (isRefundPendingWarning(res.data.warning)) {
      appStore.showSuccess(t('payment.admin.refundPending'))
      closeRefundDialog()
      loadOrders()
      return
    }
    if (res.data.require_force) {
      // Backend needs an explicit force confirmation (e.g. the user spent their
      // balance after requesting the refund). Keep the dialog open and surface
      // the force checkbox instead of dropping the admin back to the list.
      refundRequireForce.value = true
      refundWarning.value = /issued invoice|红冲|INVOICE_ISSUED/i.test(res.data.warning || '')
        ? t('payment.admin.invoiceRefundIssuedWarning')
        : (res.data.warning || '')
      return
    }
    appStore.showError(res.data.warning || t('common.error'))
  } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
  finally { refundSubmitting.value = false }
}

async function handleQueryRefund(order: PaymentOrder) {
  refundQueryingIds.value = new Set(refundQueryingIds.value).add(order.id)
  try {
    const res = await adminPaymentAPI.queryRefund(order.id)
    if (res.data.success) {
      appStore.showSuccess(t('payment.admin.refundSuccess'))
    } else if (isRefundPendingWarning(res.data.warning)) {
      appStore.showSuccess(t('payment.admin.refundPending'))
    } else {
      appStore.showError(res.data.warning || t('common.error'))
    }
    loadOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    const next = new Set(refundQueryingIds.value)
    next.delete(order.id)
    refundQueryingIds.value = next
  }
}

function formatDateTime(dateStr: string): string { return formatOrderDateTime(dateStr) }

onMounted(() => loadOrders())
</script>

<style scoped>
/* ---------- ListPage shell · 14px rhythm (原型 04) ---------- */
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

/* ---------- Filter row · 36px controls, gap 8 ---------- */
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

/* ---------- Table card · radius 14, glass ---------- */
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

/* Primary cell · 名称 600 + #id · meta 11.5 mono muted */
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
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-amount {
  display: flex;
  flex-direction: column;
  gap: 2px;
  font-variant-numeric: tabular-nums;
}

.cell-amount-value {
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.cell-amount-meta {
  font-size: var(--fs-11-5);
  color: var(--muted);
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

.row-actions .icon-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
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

.detail-item-wide {
  grid-column: span 2;
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

.detail-value-danger {
  color: var(--danger-text);
  font-weight: var(--fw-semibold);
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

.audit-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 192px;
  overflow-y: auto;
}

.audit-item {
  padding: 10px 12px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--surface-secondary);
}

.audit-item-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
}

.audit-item-action {
  font-size: var(--fs-12-5);
  font-weight: var(--fw-semibold);
  color: var(--foreground);
}

.audit-item-time {
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  color: var(--muted);
}

.audit-item-detail,
.audit-item-operator {
  margin-top: 4px;
  font-size: var(--fs-12);
  color: var(--muted);
  overflow-wrap: anywhere;
}

/* ---------- Mobile ≤767 ---------- */
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

  .detail-item-wide {
    grid-column: span 1;
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
