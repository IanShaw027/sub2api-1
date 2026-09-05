<template>
  <AppLayout>
    <PageHeader :title="t('payment.invoices.mine')" :description="t('nav.myInvoices')">
      <template #actions>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="fetchInvoices">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button @click="router.push('/orders')">{{ t('payment.invoices.fromOrders') }}</Button>
      </template>
    </PageHeader>
    <div class="invoices-page">
      <div class="flex flex-col gap-3">
        <FilterBar
          :search-placeholder="t('payment.invoices.search')"
          :filter-label="t('common.filter')"
          @open-filters="showMobileFilters = !showMobileFilters"
        >
          <template #search>
            <input v-model="keyword" type="search" class="input" :placeholder="t('payment.invoices.search')" @keyup.enter="applyFilters" />
          </template>
          <template #filters>
            <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="applyFilters" />
          </template>
        </FilterBar>
        <div v-if="showMobileFilters" class="invoices-mobile-filters">
          <Select v-model="currentFilter" :options="statusFilters" class="w-full" @change="applyFilters" />
        </div>
        <ChipScroller
          :model-value="String(currentFilter || '')"
          :chips="statusFilters.map((opt) => ({ value: String(opt.value), label: opt.label }))"
          @update:model-value="(v) => { currentFilter = v || ''; applyFilters() }"
        />
      </div>

      <DataTable class="invoices-table" :columns="columns" :data="invoices" :loading="loading">
        <template #cell-id="{ value }">
          <span class="font-mono text-sm">#{{ value }}</span>
        </template>
        <template #cell-status="{ value }">
          <StatusBadge
            :tone="String(value).toUpperCase() === 'ISSUED' ? 'success' : String(value).toUpperCase() === 'CANCELLED' ? 'muted' : 'warning'"
            :label="statusLabel(value)"
            dot
          />
        </template>
        <template #cell-invoice_amount="{ value, row }">
          <span class="text-sm font-medium">{{ Number(value).toFixed(2) }}{{ row.currency ? ' ' + row.currency : '' }}</span>
        </template>
        <template #cell-applied_at="{ value }">
          <span v-if="value" class="text-xs text-muted" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
          <span v-else class="text-xs text-muted">-</span>
        </template>
        <template #cell-actions="{ row }">
          <div class="flex items-center gap-2">
            <button class="text-xs text-accent hover:underline" @click="router.push(`/invoices/${row.id}`)">{{ t('common.view') }}</button>
            <button
              v-if="String(row.status).toUpperCase() === 'APPLIED'"
              class="text-xs text-warning-text hover:underline"
              @click="cancelInvoice(row.id)"
            >{{ t('common.cancel') }}</button>
          </div>
        </template>
      </DataTable>

      <UiPagination
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
import { formatDateTime, formatRelativeTime } from '@/utils/format'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import ChipScroller from '@/components/ui/ChipScroller.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import DataTable from '@/components/common/DataTable.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const invoices = ref<Invoice[]>([])
const currentFilter = ref('')
const keyword = ref('')
const showMobileFilters = ref(false)
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  { value: 'APPLIED', label: t('payment.invoices.status.applied') },
  { value: 'ISSUED', label: t('payment.invoices.status.issued') },
  { value: 'CANCELLED', label: t('payment.invoices.status.cancelled') },
])

function statusLabel(value: string) {
  return t(`payment.invoices.status.${String(value).toLowerCase()}`, value)
}

const columns = computed((): Column[] => [
  { key: 'id', label: t('payment.invoices.id') },
  { key: 'title', label: t('payment.invoices.title') },
  { key: 'tax_number', label: t('payment.invoices.taxNumber') },
  { key: 'email', label: t('payment.invoices.email') },
  { key: 'invoice_amount', label: t('payment.invoices.amount') },
  { key: 'order_count', label: t('payment.invoices.orderCount') },
  { key: 'status', label: t('payment.invoices.statusLabel') },
  { key: 'applied_at', label: t('payment.invoices.appliedAt') },
  { key: 'actions', label: t('common.actions') },
])

async function fetchInvoices() {
  loading.value = true
  try {
    const res = await paymentAPI.getMyInvoices({
      page: pagination.page,
      page_size: pagination.page_size,
      status: currentFilter.value || undefined,
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

function applyFilters() {
  pagination.page = 1
  fetchInvoices()
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

<style scoped>
.invoices-page {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.invoices-table :deep(.table-wrapper thead th) {
  height: 42px;
}

.invoices-table :deep(.table-wrapper tbody td) {
  height: 61px;
}

.invoices-mobile-filters {
  display: none;
}

@media (max-width: 767px) {
  .invoices-mobile-filters {
    display: block;
  }
}
</style>
