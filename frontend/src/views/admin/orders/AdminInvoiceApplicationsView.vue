<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card p-4">
        <div class="flex flex-wrap items-center gap-3">
          <input v-model="keyword" type="text" class="input sm:max-w-64" :placeholder="t('payment.invoices.search')" @input="debounceLoad" />
          <Select v-model="status" :options="statusFilters" class="w-36" @change="load" />
          <button class="btn btn-secondary" :disabled="loading" @click="load">
            <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
          </button>
        </div>
      </div>

      <DataTable :columns="columns" :data="invoices" :loading="loading">
        <template #cell-id="{ value, row }">
          <span class="font-mono text-sm">#{{ value }}</span>
          <span v-if="row.unread_by_admin && row.status === 'applied'" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500" />
        </template>
        <template #cell-status="{ value }">
          {{ t('payment.invoices.status.' + value, value) }}
        </template>
        <template #cell-invoice_amount="{ value, row }">
          {{ Number(value).toFixed(2) }}{{ row.currency ? ' ' + row.currency : '' }}
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button class="text-xs text-blue-600 hover:underline" @click="openDetail(row.id)">{{ t('common.view') }}</button>
            <button v-if="row.status === 'applied'" class="text-xs text-yellow-600 hover:underline" @click="cancelInvoice(row.id)">{{ t('common.cancel') }}</button>
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
    </div>

    <BaseDialog :show="!!detail" :title="t('payment.invoices.detail')" @close="detail = null">
      <div v-if="detail" class="space-y-4">
        <p class="text-sm">{{ detail.title }} · {{ detail.invoice_amount.toFixed(2) }}{{ detail.currency ? ' ' + detail.currency : '' }} · {{ t('payment.invoices.status.' + detail.status) }}</p>
        <p class="text-xs text-gray-500">{{ detail.email }} / {{ detail.tax_number }}</p>
        <ul class="text-sm">
          <li v-for="item in detail.orders || []" :key="item.order_id">#{{ item.order_id }} {{ item.out_trade_no }}</li>
        </ul>
        <div v-if="detail.status === 'applied'">
          <label class="input-label">{{ t('payment.invoices.uploadFile') }}</label>
          <input type="file" accept="application/pdf" @change="onFile" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-2">
          <button v-if="detail?.status === 'issued'" class="btn btn-secondary" @click="resend">{{ t('payment.invoices.resendEmail') }}</button>
          <button v-if="detail?.status === 'issued' && detail.has_file" class="btn btn-secondary" @click="download">{{ t('payment.invoices.download') }}</button>
          <button v-if="detail?.status === 'applied'" class="btn btn-primary" :disabled="!file || actionLoading" @click="issue">{{ t('payment.invoices.issue') }}</button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminPaymentAPI } from '@/api/admin/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
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
  { value: 'applied', label: t('payment.invoices.status.applied') },
  { value: 'issued', label: t('payment.invoices.status.issued') },
  { value: 'cancelled', label: t('payment.invoices.status.cancelled') },
])

const columns = computed((): Column[] => [
  { key: 'id', label: t('payment.invoices.id') },
  { key: 'user_email', label: t('payment.admin.colUser') },
  { key: 'title', label: t('payment.invoices.title') },
  { key: 'invoice_amount', label: t('payment.invoices.amount') },
  { key: 'status', label: t('payment.invoices.statusLabel') },
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
    const res = await adminPaymentAPI.issueInvoice(detail.value.id, file.value)
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
    const res = await adminPaymentAPI.getInvoiceDownloadGrant(detail.value.id)
    window.open(res.data.url, '_blank', 'noopener')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

onMounted(load)
</script>
