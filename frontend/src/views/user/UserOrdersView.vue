<template>
  <AppLayout>
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

      <!-- Table -->
      <OrderTable :orders="orders" :loading="loading">
        <template #actions="{ row }">
          <div class="flex items-center gap-2">
            <button v-if="row.status === 'PENDING'" @click="handleCancel(row.id)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50 dark:text-yellow-400 dark:hover:bg-yellow-900/20">
              <Icon name="x" size="sm" />
              <span>{{ t('payment.orders.cancel') }}</span>
            </button>
            <button v-if="canApplyInvoice(row)" @click="openInvoiceApplyDialog(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/20">
              <Icon name="document" size="sm" />
              <span>{{ t('payment.invoice.apply') }}</span>
            </button>
            <button v-else-if="row.invoice_status === 'APPLIED'" @click="openInvoiceDetail(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-sky-600 hover:bg-sky-50 dark:text-sky-400 dark:hover:bg-sky-900/20">
              <Icon name="eye" size="sm" />
              <span>{{ t('payment.invoice.applied') }}</span>
            </button>
            <button v-else-if="row.invoice_status === 'ISSUED'" @click="downloadInvoice(row.id)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-green-600 hover:bg-green-50 dark:text-green-400 dark:hover:bg-green-900/20">
              <Icon name="download" size="sm" />
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
      <p class="text-sm text-gray-600 dark:text-gray-300">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</button>
        </div>
      </template>
    </BaseDialog>

    <!-- Refund Dialog -->
    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="refundTarget = null">
      <div v-if="refundTarget" class="space-y-4">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.amount') }}</span>
            <span class="text-gray-900 dark:text-white">${{ refundTarget.amount.toFixed(2) }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="refundTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || !refundReason.trim()" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!invoiceApplyTarget" :title="t('payment.invoice.apply')" @close="invoiceApplyTarget = null">
      <div v-if="invoiceApplyTarget" class="space-y-4">
        <div class="rounded-xl bg-gray-50 p-4 dark:bg-dark-800">
          <div class="flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</span>
            <span class="font-mono text-gray-900 dark:text-white">{{ invoiceApplyTarget.out_trade_no }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-gray-500 dark:text-gray-400">{{ t('payment.invoice.amount') }}</span>
            <span class="text-gray-900 dark:text-white">¥{{ invoiceApplyTarget.pay_amount.toFixed(2) }}</span>
          </div>
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoice.title') }}</label>
          <input v-model="invoiceForm.title" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoice.taxNumber') }}</label>
          <input v-model="invoiceForm.tax_number" class="input mt-1 w-full" />
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.invoice.email') }}</label>
            <input v-model="invoiceForm.email" class="input mt-1 w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('payment.invoice.contactName') }}</label>
            <input v-model="invoiceForm.contact_name" class="input mt-1 w-full" />
          </div>
        </div>
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div>
            <label class="input-label">{{ t('payment.invoice.contactPhone') }}</label>
            <input v-model="invoiceForm.contact_phone" class="input mt-1 w-full" />
          </div>
          <div>
            <label class="input-label">{{ t('payment.invoice.note') }}</label>
            <input v-model="invoiceForm.request_note" class="input mt-1 w-full" />
          </div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="invoiceApplyTarget = null">{{ t('common.cancel') }}</button>
          <button class="btn btn-primary" :disabled="actionLoading || !invoiceForm.title.trim() || !invoiceForm.tax_number.trim() || !invoiceForm.email.trim()" @click="confirmApplyInvoice">{{ actionLoading ? t('common.processing') : t('payment.invoice.apply') }}</button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!invoiceDetail" :title="t('payment.invoice.detail')" @close="invoiceDetail = null">
      <div v-if="invoiceDetail" class="space-y-4">
        <div class="grid grid-cols-2 gap-4">
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.status') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceStatusLabel(invoiceDetail.status) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.amount') }}</p><p class="text-sm text-gray-900 dark:text-white">¥{{ invoiceDetail.invoice_amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.title') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.title }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.taxNumber') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.tax_number }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.email') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.email }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.contactName') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.contact_name || '-' }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.contactPhone') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.contact_phone || '-' }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.fileName') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceDetail.file_name || '-' }}</p></div>
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button v-if="invoiceDetail?.status === 'APPLIED'" class="btn btn-danger" :disabled="actionLoading" @click="confirmCancelInvoice">{{ actionLoading ? t('common.processing') : t('payment.invoice.cancel') }}</button>
          <button v-if="invoiceDetail?.status === 'ISSUED'" class="btn btn-primary" :disabled="actionLoading" @click="downloadInvoice(invoiceDetail.order_id)">{{ t('payment.invoice.download') }}</button>
          <button class="btn btn-secondary" @click="invoiceDetail = null">{{ t('common.close') }}</button>
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
import type { InvoiceApplication, PaymentOrder } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'

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
const invoiceApplyTarget = ref<PaymentOrder | null>(null)
const invoiceDetail = ref<InvoiceApplication | null>(null)
const invoiceForm = reactive({
  title: '',
  tax_number: '',
  email: '',
  contact_name: '',
  contact_phone: '',
  request_note: '',
})
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'PENDING', label: t('payment.status.pending') },
  { value: 'COMPLETED', label: t('payment.status.completed') },
  { value: 'FAILED', label: t('payment.status.failed') },
  { value: 'REFUNDED', label: t('payment.status.refunded') },
])

async function fetchOrders() {
  loading.value = true
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

function openRefundDialog(order: PaymentOrder) { refundTarget.value = order; refundReason.value = '' }
function openInvoiceApplyDialog(order: PaymentOrder) {
  invoiceApplyTarget.value = order
  invoiceForm.title = ''
  invoiceForm.tax_number = ''
  invoiceForm.email = ''
  invoiceForm.contact_name = ''
  invoiceForm.contact_phone = ''
  invoiceForm.request_note = ''
}

async function confirmRefund() {
  if (!refundTarget.value || !refundReason.value.trim()) return
  actionLoading.value = true
  try {
    await paymentAPI.requestRefund(refundTarget.value.id, { reason: refundReason.value.trim() })
    appStore.showSuccess(t('common.success'))
    refundTarget.value = null
    refundReason.value = ''
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function confirmApplyInvoice() {
  if (!invoiceApplyTarget.value) return
  actionLoading.value = true
  try {
    await paymentAPI.applyOrderInvoice(invoiceApplyTarget.value.id, {
      title: invoiceForm.title.trim(),
      tax_number: invoiceForm.tax_number.trim(),
      email: invoiceForm.email.trim(),
      contact_name: invoiceForm.contact_name.trim() || undefined,
      contact_phone: invoiceForm.contact_phone.trim() || undefined,
      request_note: invoiceForm.request_note.trim() || undefined,
    })
    appStore.showSuccess(t('common.success'))
    invoiceApplyTarget.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function openInvoiceDetail(order: PaymentOrder) {
  actionLoading.value = true
  try {
    const res = await paymentAPI.getOrderInvoice(order.id)
    invoiceDetail.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function confirmCancelInvoice() {
  if (!invoiceDetail.value) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelOrderInvoice(invoiceDetail.value.order_id)
    appStore.showSuccess(t('common.success'))
    invoiceDetail.value = null
    await fetchOrders()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function downloadInvoice(orderID: number) {
  actionLoading.value = true
  try {
    const res = await paymentAPI.getOrderInvoiceDownloadURL(orderID)
    const url = res.data.url
    if (url) window.open(url, '_blank', 'noopener')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
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

function invoiceStatusLabel(status: string) {
  if (status === 'APPLIED') return t('payment.invoice.applied')
  if (status === 'ISSUED') return t('payment.invoice.issued')
  if (status === 'CANCELLED') return t('payment.invoice.cancelled')
  return status
}

onMounted(() => { fetchOrders(); loadRefundEligibility(); loadInvoiceEligibility() })
</script>
