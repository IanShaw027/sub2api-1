<template>
  <AppLayout>
    <OrdersTabBar />
    <div v-if="loading" class="card p-8 text-center text-ink-soft">{{ t('common.processing') }}</div>
    <div v-else-if="invoice" class="space-y-4">
      <!-- 顶部摘要 -->
      <div class="card p-6">
        <div class="flex flex-wrap items-center justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-ink dark:text-white">#{{ invoice.id }} · {{ invoice.title }}</h2>
            <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">
              {{ statusLabel(invoice.status) }} · {{ t('payment.invoice.list.colOrderCount') }} {{ invoice.order_count }} · {{ formatPaymentAmount(invoice.invoice_amount, invoice.currency) }}
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button class="btn btn-secondary" @click="$router.push('/orders/invoices')">{{ t('common.back') }}</button>
            <button v-if="invoice.status === 'APPLIED'" class="btn btn-danger" :disabled="actionLoading" @click="confirmCancel">
              {{ actionLoading ? t('common.processing') : t('payment.invoice.cancel') }}
            </button>
            <button v-if="invoice.status === 'ISSUED'" class="btn btn-primary" :disabled="actionLoading" @click="download">
              {{ actionLoading ? t('common.processing') : t('payment.invoice.download') }}
            </button>
          </div>
        </div>
      </div>

      <!-- 开票信息 -->
      <div class="card p-6">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div><p class="text-xs text-ink-soft">{{ t('payment.invoice.taxNumber') }}</p><p class="text-sm">{{ invoice.tax_number }}</p></div>
          <div><p class="text-xs text-ink-soft">{{ t('payment.invoice.email') }}</p><p class="text-sm">{{ invoice.email }}</p></div>
          <div><p class="text-xs text-ink-soft">{{ t('payment.invoice.contactName') }}</p><p class="text-sm">{{ invoice.contact_name || '-' }}</p></div>
          <div><p class="text-xs text-ink-soft">{{ t('payment.invoice.contactPhone') }}</p><p class="text-sm">{{ invoice.contact_phone || '-' }}</p></div>
        </div>
      </div>

      <!-- 关联订单 -->
      <div class="card overflow-hidden">
        <div class="border-b border-line px-6 py-3 text-sm font-medium text-ink-body dark:border-dark-700 dark:text-dark-200">
          {{ t('payment.invoice.detailPage.relatedOrders') }}
        </div>
        <table class="min-w-full divide-y divide-line dark:divide-dark-700">
          <thead class="bg-page dark:bg-dark-800">
            <tr>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-ink-soft">{{ t('payment.orders.orderNo') }}</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-ink-soft">{{ t('payment.orders.payAmount') }}</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-ink-soft">{{ t('payment.orders.paymentMethod') }}</th>
              <th class="px-4 py-3 text-left text-xs font-medium uppercase text-ink-soft">{{ t('payment.orders.createdAt') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-line bg-card dark:divide-dark-700 dark:bg-dark-900">
            <tr v-for="o in invoice.orders" :key="o.order_id">
              <td class="px-4 py-3 text-sm font-mono">{{ o.out_trade_no }}</td>
              <td class="px-4 py-3 text-sm">{{ formatPaymentAmount(o.pay_amount_snapshot, invoice.currency) }}</td>
              <td class="px-4 py-3 text-sm">{{ o.payment_type }}</td>
              <td class="px-4 py-3 text-sm">{{ formatDate(o.created_at) }}</td>
            </tr>
          </tbody>
        </table>
      </div>

      <!-- 文件区 -->
      <div class="card p-6">
        <p v-if="invoice.status === 'APPLIED'" class="text-sm text-ink-soft">{{ t('payment.invoice.detailPage.fileWaiting') }}</p>
        <p v-else-if="invoice.status === 'ISSUED'" class="text-sm text-ink-body dark:text-dark-200">
          {{ t('payment.invoice.fileName') }}: {{ invoice.file_name || '-' }}
        </p>
        <p v-else class="text-sm text-ink-faint">{{ statusLabel(invoice.status) }}</p>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import { paymentAPI } from '@/api/payment'
import type { Invoice } from '@/types/payment'
import { formatPaymentAmount } from '@/components/payment/currency'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import OrdersTabBar from '@/components/user/orders/OrdersTabBar.vue'

const { t } = useI18n()
const route = useRoute()
const appStore = useAppStore()

const loading = ref(true)
const actionLoading = ref(false)
const invoice = ref<Invoice | null>(null)

function statusLabel(s: string) {
  if (s === 'APPLIED') return t('payment.invoice.applied')
  if (s === 'ISSUED') return t('payment.invoice.issued')
  if (s === 'CANCELLED') return t('payment.invoice.cancelled')
  return s
}
function formatDate(v: string) { return new Date(v).toLocaleString() }

async function load() {
  const id = Number(route.params.id)
  if (!id) return
  loading.value = true
  try {
    const res = await paymentAPI.getInvoice(id)
    invoice.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

async function confirmCancel() {
  if (!invoice.value) return
  if (!window.confirm(t('payment.invoice.cancel'))) return
  actionLoading.value = true
  try {
    const res = await paymentAPI.cancelInvoice(invoice.value.id)
    invoice.value = res.data
    appStore.showSuccess(t('common.success'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function download() {
  if (!invoice.value) return
  actionLoading.value = true
  try {
    const res = await paymentAPI.getInvoiceDownloadURL(invoice.value.id)
    if (res.data.url) window.open(res.data.url, '_blank', 'noopener')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(load)
</script>
