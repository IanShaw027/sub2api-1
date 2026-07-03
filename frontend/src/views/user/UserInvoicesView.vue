<template>
  <AppLayout>
    <OrdersTabBar />
    <div class="space-y-4">
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="filters.status" :options="statusOptions" class="w-36" @change="applyFilters" />
          <input
            v-model="filters.keyword"
            type="text"
            class="input flex-1 sm:max-w-72"
            :placeholder="t('payment.invoice.list.searchPlaceholder', '搜索发票…')"
            @keyup.enter="applyFilters"
          />
          <button class="btn btn-secondary" @click="loadInvoices" :disabled="loading" :title="t('common.refresh')">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <div class="card overflow-hidden">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-800">
              <tr>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">#</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.title') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.list.colOrderCount') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.amount') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.invoice.status') }}</th>
                <th class="px-4 py-3 text-left text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('payment.orders.createdAt') }}</th>
                <th class="px-4 py-3 text-right text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-200 bg-white dark:divide-dark-700 dark:bg-dark-900">
              <tr v-for="item in invoices" :key="item.id" class="cursor-pointer hover:bg-gray-50 dark:hover:bg-dark-800" @click="openDetail(item.id)">
                <td class="px-4 py-3 text-sm text-gray-900 dark:text-white">#{{ item.id }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ item.title }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ item.order_count }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatPaymentAmount(item.invoice_amount, item.currency) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ statusLabel(item.status) }}</td>
                <td class="px-4 py-3 text-sm text-gray-700 dark:text-gray-300">{{ formatDate(item.created_at) }}</td>
                <td class="px-4 py-3 text-right" @click.stop>
                  <button v-if="item.status === 'ISSUED'" class="btn btn-secondary btn-sm" :disabled="actionLoading" @click="downloadFile(item.id)">
                    <Icon name="download" size="sm" />
                    {{ t('payment.invoice.download') }}
                  </button>
                  <button v-if="item.status === 'APPLIED'" class="btn btn-secondary btn-sm ml-2" :disabled="actionLoading" @click="confirmCancel(item.id)">
                    {{ t('payment.invoice.cancel') }}
                  </button>
                </td>
              </tr>
              <tr v-if="!loading && invoices.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
                  {{ t('payment.invoice.list.empty') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination v-if="pagination.total > 0" :page="pagination.page" :total="pagination.total" :page-size="pagination.page_size" @update:page="onPage" @update:pageSize="onPageSize" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { paymentAPI } from '@/api/payment'
import type { Invoice } from '@/types/payment'
import { formatPaymentAmount } from '@/components/payment/currency'
import { useAppStore } from '@/stores'
import { extractI18nErrorMessage } from '@/utils/apiError'
import AppLayout from '@/components/layout/AppLayout.vue'
import OrdersTabBar from '@/components/user/orders/OrdersTabBar.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const invoices = ref<Invoice[]>([])
const filters = reactive({ status: '', keyword: '' })
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusOptions = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'APPLIED', label: t('payment.invoice.applied') },
  { value: 'ISSUED', label: t('payment.invoice.issued') },
  { value: 'CANCELLED', label: t('payment.invoice.cancelled') },
])

function statusLabel(s: string) {
  if (s === 'APPLIED') return t('payment.invoice.applied')
  if (s === 'ISSUED') return t('payment.invoice.issued')
  if (s === 'CANCELLED') return t('payment.invoice.cancelled')
  return s
}

function formatDate(value: string) { return new Date(value).toLocaleString() }

async function loadInvoices() {
  loading.value = true
  try {
    const res = await paymentAPI.listMyInvoices({
      page: pagination.page,
      page_size: pagination.page_size,
      status: filters.status || undefined,
      keyword: filters.keyword || undefined,
    })
    invoices.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function applyFilters() { pagination.page = 1; loadInvoices() }
function onPage(p: number) { pagination.page = p; loadInvoices() }
function onPageSize(s: number) { pagination.page_size = s; pagination.page = 1; loadInvoices() }
function openDetail(id: number) { router.push({ name: 'MyInvoiceDetail', params: { id: String(id) } }) }

async function downloadFile(id: number) {
  actionLoading.value = true
  try {
    const res = await paymentAPI.getInvoiceDownloadURL(id)
    if (res.data.url) window.open(res.data.url, '_blank', 'noopener')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function confirmCancel(id: number) {
  if (!window.confirm(t('payment.invoice.cancel'))) return
  actionLoading.value = true
  try {
    await paymentAPI.cancelInvoice(id)
    appStore.showSuccess(t('common.success'))
    await loadInvoices()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.invoice.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(loadInvoices)
</script>
