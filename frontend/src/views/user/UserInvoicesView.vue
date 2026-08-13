<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="fetchInvoices" />
          <div class="flex flex-1 items-center justify-end gap-2">
            <button class="btn btn-secondary" :disabled="loading" @click="fetchInvoices">
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button class="btn btn-primary" @click="router.push('/orders')">{{ t('payment.invoices.fromOrders') }}</button>
          </div>
        </div>
      </div>

      <DataTable :columns="columns" :data="invoices" :loading="loading">
        <template #cell-id="{ value }">
          <span class="font-mono text-sm">#{{ value }}</span>
        </template>
        <template #cell-status="{ value }">
          <span class="text-sm">{{ t('payment.invoices.status.' + value, value) }}</span>
        </template>
        <template #cell-invoice_amount="{ value, row }">
          <span class="text-sm font-medium">{{ Number(value).toFixed(2) }}{{ row.currency ? ' ' + row.currency : '' }}</span>
        </template>
        <template #cell-created_at="{ value }">
          <span class="text-xs text-gray-500">{{ formatDate(value) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button class="text-xs text-blue-600 hover:underline" @click="router.push(`/invoices/${row.id}`)">{{ t('common.view') }}</button>
            <button
              v-if="row.status === 'applied'"
              class="text-xs text-yellow-600 hover:underline"
              @click="cancelInvoice(row.id)"
            >{{ t('common.cancel') }}</button>
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
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const invoices = ref<Invoice[]>([])
const currentFilter = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'applied', label: t('payment.invoices.status.applied') },
  { value: 'issued', label: t('payment.invoices.status.issued') },
  { value: 'cancelled', label: t('payment.invoices.status.cancelled') },
])

const columns = computed((): Column[] => [
  { key: 'id', label: t('payment.invoices.id') },
  { key: 'title', label: t('payment.invoices.title') },
  { key: 'invoice_amount', label: t('payment.invoices.amount') },
  { key: 'order_count', label: t('payment.invoices.orderCount') },
  { key: 'status', label: t('payment.invoices.statusLabel') },
  { key: 'created_at', label: t('payment.orders.createdAt') },
  { key: 'actions', label: t('common.actions') },
])

function formatDate(value: string) {
  return new Date(value).toLocaleString()
}

async function fetchInvoices() {
  loading.value = true
  try {
    const res = await paymentAPI.getMyInvoices({
      page: pagination.page,
      page_size: pagination.page_size,
      status: currentFilter.value || undefined,
    })
    invoices.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  fetchInvoices()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  fetchInvoices()
}

async function cancelInvoice(id: number) {
  try {
    await paymentAPI.cancelInvoice(id)
    appStore.showSuccess(t('common.success'))
    await fetchInvoices()
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

onMounted(fetchInvoices)
</script>
