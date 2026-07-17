<template>
  <AppLayout>
    <OrdersTabBar />
    <div class="space-y-4">
      <!-- Filters -->
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="fetchOrders" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button @click="fetchOrders" :disabled="loading" class="btn btn-secondary" :title="t('common.refresh')">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/purchase')">{{ t('payment.result.backToRecharge') }}</button>
          </div>
        </div>
      </div>

      <!-- Batch Actions Bar -->
      <div v-if="selectedCount > 0" class="card flex items-center gap-4 border-accent-200 bg-accent-50/80 p-3 dark:border-blue-900/40 dark:bg-blue-900/20">
        <span class="text-sm font-medium text-accent-700 dark:text-blue-300">
          {{ t('payment.invoice.create.selected', { count: selectedCount }) }}
        </span>
        <span class="text-sm text-accent-600 dark:text-blue-400">
          {{ t('payment.invoice.create.totalAmount') }}: {{ selectedInvoiceTotalLabel }}
        </span>
        <div class="ml-auto flex items-center gap-2">
          <button class="btn btn-primary btn-sm" @click="openCreateInvoiceDialog">
            {{ t('payment.invoice.create.action') }}
          </button>
          <button class="btn btn-secondary btn-sm" @click="clear">
            {{ t('payment.invoice.create.clearSelection') }}
          </button>
        </div>
      </div>

      <!-- Table -->
      <OrderTable
        :orders="orders"
        :loading="loading"
        :selectable="hasInvoiceEligible"
        :is-selected="isOrderSelected"
        :is-row-selectable="canApplyInvoice"
        :all-visible-selected="allEligibleSelected"
        :some-visible-selected="someEligibleSelected"
        @toggle-row="handleToggleRow"
        @toggle-all="handleToggleAll"
      >
        <template #actions="{ row }">
          <div class="flex items-center gap-2">
            <button v-if="row.status === 'PENDING'" @click="handleCancel(row.id)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50 dark:text-yellow-400 dark:hover:bg-yellow-900/20">
              <Icon name="x" size="sm" />
              <span>{{ t('payment.orders.cancel') }}</span>
            </button>
            <button v-if="canApplyInvoice(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-accent-600 hover:bg-accent-50 dark:text-blue-400 dark:hover:bg-blue-900/20" @click="quickApply(row)">
              <Icon name="document" size="sm" />
              <span>{{ t('payment.invoice.apply') }}</span>
            </button>
            <button v-else-if="row.invoice_id && row.invoice_status === 'APPLIED'" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-sky-600 hover:bg-sky-50 dark:text-sky-400 dark:hover:bg-sky-900/20" @click="goInvoiceDetail(row.invoice_id)">
              <Icon name="eye" size="sm" />
              <span>{{ t('payment.invoice.applied') }}</span>
            </button>
            <button v-else-if="row.invoice_id && row.invoice_status === 'ISSUED'" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-green-600 hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20" @click="goInvoiceDetail(row.invoice_id)">
              <Icon name="check" size="sm" />
              <span>{{ t('payment.invoice.issued') }}</span>
            </button>
            <button v-if="canRequestRefund(row)" @click="openRefundDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50 dark:text-purple-400 dark:hover:bg-purple-900/20">
              <Icon name="dollar" size="sm" />
              <span>{{ t('payment.orders.requestRefund') }}</span>
            </button>
          </div>
        </template>
      </OrderTable>

      <!-- Pagination -->
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <!-- Cancel Confirm Dialog -->
    <BaseDialog :show="!!cancelTargetId" :title="t('payment.orders.cancel')" width="narrow" @close="cancelTargetId = null">
      <p class="text-sm text-ink-body dark:text-ink-body">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Refund Dialog -->
    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="closeRefundDialog">
      <div v-if="refundTarget" class="space-y-4">
        <div class="rounded-xl bg-page p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-ink-soft dark:text-ink-soft">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-ink dark:text-white">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-ink-soft dark:text-ink-soft">{{ t('payment.orders.amount') }}</span>
            <span class="text-ink dark:text-white">{{ formatRefundMoney(refundTarget.amount) }}</span>
          </div>
        </div>
        <div class="rounded-xl border border-accent-100 bg-accent-50 p-4 text-sm dark:border-blue-900/60 dark:bg-blue-900/20">
          <div v-if="refundPreviewLoading" class="text-accent-700 dark:text-blue-300">正在计算可退款金额...</div>
          <div v-else-if="refundPreview" class="space-y-2 text-accent-800 dark:text-blue-200">
            <div class="flex justify-between">
              <span>订单剩余可退</span>
              <span class="font-medium">{{ formatRefundMoney(refundPreview.order_amount - refundPreview.already_refunded) }}</span>
            </div>
            <template v-if="refundPreview.order_type === 'balance'">
              <div class="flex justify-between">
                <span>当前可用余额</span>
                <span class="font-medium">{{ formatRefundMoney(refundPreview.balance_available || 0) }}</span>
              </div>
              <p class="text-xs text-accent-600 dark:text-blue-300">余额退款按订单剩余可退金额和当前可用余额取较小值。</p>
            </template>
            <template v-else>
              <div class="flex justify-between">
                <span>订阅已消耗额度</span>
                <span class="font-medium">{{ formatRefundMoney(refundPreview.usage_amount || 0) }}</span>
              </div>
              <div class="flex justify-between">
                <span>订阅倍率 / 退款倍率</span>
                <span class="font-medium">{{ formatNumber(refundPreview.subscription_rate_multiplier || 1) }} / {{ formatNumber(refundPreview.refund_rate_multiplier || 1) }}</span>
              </div>
              <div class="flex justify-between">
                <span>折算已使用金额</span>
                <span class="font-medium">{{ formatRefundMoney(refundPreview.used_refund_value || 0) }}</span>
              </div>
              <p class="text-xs text-accent-600 dark:text-blue-300">
                订阅退款按 消耗总额度 / 订阅倍率 * 退款倍率 折算已使用金额，再从订单剩余可退金额中扣除。
              </p>
            </template>
            <div class="border-t border-accent-200 pt-2 dark:border-blue-800">
              <div class="flex justify-between font-semibold">
                <span>实际可退款额</span>
                <span>{{ formatRefundMoney(refundPreview.max_refund_amount) }}</span>
              </div>
              <p class="mt-1 text-xs text-accent-600 dark:text-blue-300">
                {{ refundPreview.auto_refund ? '该订单提交后会自动退款到账。' : '该订单提交后需等待管理员审批。' }}
              </p>
            </div>
          </div>
          <div v-else class="text-red-600 dark:text-red-300">暂时无法获取可退款金额，请稍后重试。</div>
        </div>
        <div>
          <label class="input-label">退款金额</label>
          <input
            v-model.number="refundAmount"
            type="number"
            step="0.01"
            min="0.01"
            :max="refundPreview?.max_refund_amount || refundTarget.amount"
            class="input mt-1 w-full"
          />
          <p class="mt-1 text-xs text-ink-soft dark:text-ink-soft">
            最大可退 {{ formatRefundMoney(refundPreview?.max_refund_amount || 0) }}
          </p>
        </div>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="closeRefundDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || refundPreviewLoading || !refundPreview || refundAmount <= 0 || refundAmount > refundPreview.max_refund_amount || !refundReason.trim()" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Create Invoice Dialog -->
    <BaseDialog :show="showCreateInvoiceDialog" :title="t('payment.invoice.create.action')" @close="closeCreateInvoiceDialog">
      <div class="space-y-4">
        <div class="rounded-xl bg-page p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-ink-soft dark:text-ink-soft">{{ t('payment.invoice.create.selected', { count: selectedCount }) }}</span>
            <span class="font-medium text-ink dark:text-white">{{ t('payment.invoice.create.totalAmount') }}: {{ selectedInvoiceTotalLabel }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoice.title') }}</label>
          <input v-model="createInvoiceForm.title" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoice.taxNumber') }}</label>
          <input v-model="createInvoiceForm.tax_number" class="input mt-1 w-full" />
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.invoice.email') }}</label>
            <input v-model="createInvoiceForm.email" class="input mt-1 w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('payment.invoice.contactName') }}</label>
            <input v-model="createInvoiceForm.contact_name" class="input mt-1 w-full" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.invoice.contactPhone') }}</label>
            <input v-model="createInvoiceForm.contact_phone" class="input mt-1 w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('payment.invoice.note') }}</label>
            <input v-model="createInvoiceForm.request_note" class="input mt-1 w-full" />
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="closeCreateInvoiceDialog">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || !createInvoiceForm.title.trim() || !createInvoiceForm.tax_number.trim() || !createInvoiceForm.email.trim()" @click="confirmCreateInvoice">
            {{ actionLoading ? t('payment.invoice.create.submitting') : t('payment.invoice.create.submit') }}
          </button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useTableSelection } from '@/composables/useTableSelection'
import type { PaymentOrder, RefundPreview } from '@/types/payment'
import { formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import OrdersTabBar from '@/components/user/orders/OrdersTabBar.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const refundEligibleProviders = ref<Set<string>>(new Set())
const invoiceEligibleProviders = ref<Set<string>>(new Set())
const currentFilter = ref('')
const cancelTargetId = ref<number | null>(null)
const refundTarget = ref<PaymentOrder | null>(null)
const refundReason = ref('')
const refundAmount = ref(0)
const refundPreview = ref<RefundPreview | null>(null)
const refundPreviewLoading = ref(false)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

// Create invoice state
const showCreateInvoiceDialog = ref(false)
const createInvoiceForm = reactive({
  title: '',
  tax_number: '',
  email: '',
  contact_name: '',
  contact_phone: '',
  request_note: '',
})

// Selection
const { selectedIds, selectedCount, isSelected, toggle, clear, batchUpdate } = useTableSelection({
  rows: orders,
  getId: (o) => o.id,
})

const hasInvoiceEligible = computed(() => orders.value.some(canApplyInvoice))

const eligibleOrders = computed(() => orders.value.filter(canApplyInvoice))

const allEligibleSelected = computed(() => {
  if (eligibleOrders.value.length === 0) return false
  return eligibleOrders.value.every((o) => isSelected(o.id))
})

const someEligibleSelected = computed(() => {
  return eligibleOrders.value.some((o) => isSelected(o.id))
})

const selectedInvoiceTotals = computed(() => {
  const ids = new Set(selectedIds.value)
  const totals = new Map<string, number>()
  for (const order of orders.value) {
    if (!ids.has(order.id)) continue
    const currency = normalizePaymentCurrency(order.currency)
    totals.set(currency, (totals.get(currency) || 0) + Number(order.pay_amount || 0))
  }
  return Array.from(totals.entries()).map(([currency, amount]) => ({ currency, amount }))
})

const selectedInvoiceTotalLabel = computed(() => {
  return selectedInvoiceTotals.value
    .map(({ currency, amount }) => formatPaymentAmount(amount, currency))
    .join(' / ')
})

function isOrderSelected(row: PaymentOrder): boolean {
  return isSelected(row.id)
}

function handleToggleRow(row: PaymentOrder) {
  toggle(row.id)
}

function handleToggleAll() {
  if (allEligibleSelected.value) {
    batchUpdate((draft) => {
      eligibleOrders.value.forEach((o) => draft.delete(o.id))
    })
  } else {
    batchUpdate((draft) => {
      eligibleOrders.value.forEach((o) => draft.add(o.id))
    })
  }
}

// Create invoice
function openCreateInvoiceDialog() {
  createInvoiceForm.title = ''
  createInvoiceForm.tax_number = ''
  createInvoiceForm.email = ''
  createInvoiceForm.contact_name = ''
  createInvoiceForm.contact_phone = ''
  createInvoiceForm.request_note = ''
  showCreateInvoiceDialog.value = true
}

function closeCreateInvoiceDialog() {
  showCreateInvoiceDialog.value = false
}

async function confirmCreateInvoice() {
  if (selectedCount.value === 0) return
  actionLoading.value = true
  try {
    const res = await paymentAPI.createInvoice({
      order_ids: selectedIds.value,
      title: createInvoiceForm.title.trim(),
      tax_number: createInvoiceForm.tax_number.trim(),
      email: createInvoiceForm.email.trim(),
      contact_name: createInvoiceForm.contact_name.trim() || undefined,
      contact_phone: createInvoiceForm.contact_phone.trim() || undefined,
      request_note: createInvoiceForm.request_note.trim() || undefined,
    })
    appStore.showSuccess(t('payment.invoice.create.success'))
    showCreateInvoiceDialog.value = false
    clear()
    router.push({ name: 'MyInvoiceDetail', params: { id: String(res.data.id) } })
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function quickApply(row: PaymentOrder) {
  clear()
  toggle(row.id)
  openCreateInvoiceDialog()
}

function goInvoiceDetail(invoiceID: number) {
  router.push({ name: 'MyInvoiceDetail', params: { id: String(invoiceID) } })
}

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
])

async function fetchOrders() {
  loading.value = true
  clear()
  try {
    const res = await paymentAPI.getMyOrders({
      page: pagination.page,
      page_size: pagination.page_size,
      status: currentFilter.value || undefined,
    })
    orders.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) { pagination.page = page; fetchOrders() }
function handlePageSizeChange(size: number) { pagination.page_size = size; pagination.page = 1; fetchOrders() }

function handleCancel(orderId: number) { cancelTargetId.value = orderId }

async function confirmCancel() {
  if (!cancelTargetId.value) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelOrder(cancelTargetId.value)
    appStore.showSuccess(t('common.success'))
    cancelTargetId.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function openRefundDialog(order: PaymentOrder) {
  refundTarget.value = order
  refundReason.value = ''
  refundAmount.value = 0
  refundPreview.value = null
  refundPreviewLoading.value = true
  try {
    const res = await paymentAPI.getRefundPreview(order.id)
    refundPreview.value = res.data
    refundAmount.value = Math.max(0, Number(res.data.max_refund_amount || 0))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    refundPreviewLoading.value = false
  }
}

function closeRefundDialog() {
  refundTarget.value = null
  refundReason.value = ''
  refundAmount.value = 0
  refundPreview.value = null
}

async function confirmRefund() {
  if (!refundTarget.value || !refundPreview.value || !refundReason.value.trim()) return
  if (refundAmount.value <= 0 || refundAmount.value > refundPreview.value.max_refund_amount) return
  actionLoading.value = true
  try {
    await paymentAPI.requestRefund(refundTarget.value.id, {
      amount: refundAmount.value,
      reason: refundReason.value.trim(),
    })
    appStore.showSuccess(t('common.success'))
    closeRefundDialog()
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function formatNumber(value: number): string {
  return Number(value || 0).toFixed(4).replace(/0+$/, '').replace(/\.$/, '')
}

function formatRefundMoney(value: number): string {
  return formatPaymentAmount(Number(value || 0), refundTarget.value?.currency)
}

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  if (hasActiveInvoice(order)) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
}

function hasActiveInvoice(order: PaymentOrder): boolean {
  return Boolean(order.invoice_id && order.invoice_status !== 'CANCELLED')
}

function canApplyInvoice(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  if (!invoiceEligibleProviders.value.has(order.provider_instance_id)) return false
  return !order.invoice_status || order.invoice_status === 'CANCELLED'
}

async function loadRefundEligibility() {
  try {
    const res = await paymentAPI.getRefundEligibleProviders()
    refundEligibleProviders.value = new Set(res.data.provider_instance_ids || [])
  } catch { /* ignore — default to hiding refund button */ }
}

async function loadInvoiceEligibility() {
  try {
    const res = await paymentAPI.getInvoiceEligibleProviders()
    invoiceEligibleProviders.value = new Set(res.data.provider_instance_ids || [])
  } catch { /* ignore — default to hiding invoice button */ }
}

onMounted(() => { fetchOrders(); loadRefundEligibility(); loadInvoiceEligibility() })
</script>
