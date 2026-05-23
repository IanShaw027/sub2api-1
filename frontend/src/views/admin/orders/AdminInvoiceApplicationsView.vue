<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <div class="flex-1 sm:max-w-72">
            <input v-model="filters.keyword" type="text" :placeholder="t('payment.invoice.searchPlaceholder')" class="input" @keyup.enter="applyFilters" />
          </div>
          <Select v-model="filters.status" :options="statusOptions" class="w-40" @change="applyFilters" />
          <div class="flex flex-1 justify-end gap-2">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset') }}</button>
            <button class="btn btn-secondary" @click="loadInvoices" :disabled="loading">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.title') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.email') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.amount') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.status') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.orders.createdAt') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in invoices" :key="item.id">
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-white">{{ item.order_out_trade_no }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ item.title }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ item.email }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">¥{{ item.invoice_amount.toFixed(2) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ invoiceStatusLabel(item.status) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatDateTime(item.created_at) }}</td>
                <td class="px-4 py-3 text-right">
                  <button class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-blue-600 hover:bg-blue-50 dark:text-blue-400 dark:hover:bg-blue-900/20" @click="openDetail(item.id)">
                    <Icon name="eye" size="sm" />
                    {{ t('common.view') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!loading && invoices.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.noData') }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="handlePageChange" @update:pageSize="handlePageSizeChange" />
    </div>

    <BaseDialog :show="!!detail" :title="t('payment.invoice.detail')" width="wide" @close="closeDetail">
      <div v-if="detail" class="space-y-4">
        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2">
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.orders.orderNo') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.order_out_trade_no }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.status') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ invoiceStatusLabel(detail.status) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.amount') }}</p><p class="text-sm text-gray-900 dark:text-white">¥{{ detail.invoice_amount.toFixed(2) }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.fileName') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.file_name || '-' }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.title') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.title }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.taxNumber') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.tax_number }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.email') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.email }}</p></div>
          <div><p class="text-xs text-gray-500 dark:text-gray-400">{{ t('payment.invoice.contactPhone') }}</p><p class="text-sm text-gray-900 dark:text-white">{{ detail.contact_phone || '-' }}</p></div>
        </div>

        <div v-if="detail.status === 'APPLIED'" class="rounded-xl border border-dashed border-gray-300 p-4 dark:border-dark-600">
          <label class="input-label">{{ t('payment.invoice.uploadFile') }}</label>
          <input type="file" class="mt-2 block w-full text-sm text-gray-700 dark:text-gray-300" accept=".pdf,.ofd,.xml,.zip" @change="handleFileChange" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <button class="btn btn-secondary" @click="closeDetail">{{ t('common.close') }}</button>
          <button v-if="detail?.status === 'APPLIED'" class="btn btn-primary" :disabled="submitting || !selectedFile" @click="uploadFile">{{ submitting ? t('common.processing') : t('payment.invoice.markIssued') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminPaymentAPI } from '@/api/admin/payment'
import type { InvoiceApplication } from '@/types/payment'
import { useAppStore } from '@/stores/app'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { formatOrderDateTime } from '@/components/payment/orderUtils'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const submitting = ref(false)
const invoices = ref<InvoiceApplication[]>([])
const detail = ref<InvoiceApplication | null>(null)
const selectedFile = ref<File | null>(null)
const filters = reactive({ status: '', keyword: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let invoiceListReqSeq = 0
let invoiceDetailReqSeq = 0
let activeInvoiceDetailId: number | null = null

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'APPLIED', label: t('payment.invoice.applied') },
  { value: 'ISSUED', label: t('payment.invoice.issued') },
  { value: 'CANCELLED', label: t('payment.invoice.cancelled') },
])

function invoiceStatusLabel(status: string) {
  if (status === 'APPLIED') return t('payment.invoice.applied')
  if (status === 'ISSUED') return t('payment.invoice.issued')
  if (status === 'CANCELLED') return t('payment.invoice.cancelled')
  return status
}

function formatDateTime(value?: string) {
  return value ? formatOrderDateTime(value) : '-'
}

async function loadInvoices() {
  const seq = ++invoiceListReqSeq
  loading.value = true
  try {
    const res = await adminPaymentAPI.getInvoices({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status || undefined,
      keyword: filters.keyword || undefined,
    })
    if (seq !== invoiceListReqSeq) return
    invoices.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    if (seq !== invoiceListReqSeq) return
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    if (seq === invoiceListReqSeq) loading.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  loadInvoices()
}

function resetFilters() {
  filters.status = ''
  filters.keyword = ''
  pagination.page = 1
  loadInvoices()
}

function handlePageChange(page: number) {
  pagination.page = page
  loadInvoices()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadInvoices()
}

async function openDetail(id: number) {
  const seq = ++invoiceDetailReqSeq
  activeInvoiceDetailId = id
  detail.value = null
  selectedFile.value = null
  try {
    const res = await adminPaymentAPI.getInvoice(id)
    if (seq !== invoiceDetailReqSeq || activeInvoiceDetailId !== id) return
    detail.value = res.data
  } catch (err: unknown) {
    if (seq !== invoiceDetailReqSeq || activeInvoiceDetailId !== id) return
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

function closeDetail() {
  activeInvoiceDetailId = null
  detail.value = null
  selectedFile.value = null
}

function handleFileChange(event: Event) {
  const input = event.target as HTMLInputElement
  selectedFile.value = input.files?.[0] || null
}

async function uploadFile() {
  if (!detail.value || !selectedFile.value) return
  submitting.value = true
  try {
    const res = await adminPaymentAPI.uploadInvoiceFile(detail.value.id, selectedFile.value)
    detail.value = res.data
    appStore.showSuccess(t('common.success'))
    selectedFile.value = null
    await loadInvoices()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    submitting.value = false
  }
}

onMounted(loadInvoices)
</script>
