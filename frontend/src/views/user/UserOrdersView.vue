<template>
  <AppLayout>
    <PageHeader :title="t('payment.orders.title')" :description="t('nav.myOrders')">
      <template #actions>
        <Button v-if="selectedIds.length > 0" variant="secondary" @click="showInvoiceDialog = true">
          {{ t('payment.invoices.applySelected', { count: selectedIds.length }) }}
        </Button>
        <Button variant="secondary" @click="router.push('/invoices')">{{ t('payment.invoices.mine') }}</Button>
        <Button variant="secondary" :disabled="loading" :title="t('common.refresh')" @click="fetchOrders">
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </Button>
        <Button @click="router.push('/purchase')">{{ t('payment.result.backToRecharge') }}</Button>
      </template>
    </PageHeader>
    <div class="space-y-4">
      <div class="flex flex-col gap-3">
        <FilterBar>
          <template #search>
            <Select v-model="currentFilter" :options="statusFilters" class="w-36" @change="fetchOrders" />
          </template>
        </FilterBar>
        <ChipScroller
          :model-value="String(currentFilter || '')"
          :chips="statusFilters.map((opt) => ({ value: String(opt.value), label: opt.label }))"
          @update:model-value="(v) => { currentFilter = v || ''; fetchOrders() }"
        />
      </div>

      <OrderTable :orders="orders" :loading="loading">
        <template #actions="{ row }">
          <div class="flex items-center gap-2">
            <label v-if="canInvoice(row)" class="inline-flex items-center gap-1 text-xs text-muted">
              <input type="checkbox" :checked="selectedIds.includes(row.id)" @change="toggleSelect(row.id)" />
              {{ t('payment.invoices.select') }}
            </label>
            <button v-if="row.status === 'PENDING'" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-yellow-600 hover:bg-yellow-50" @click="handleCancel(row.id)">
              <Icon name="x" size="sm" />
              <span>{{ t('payment.orders.cancel') }}</span>
            </button>
            <button v-if="canRequestRefund(row)" class="inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium text-purple-600 hover:bg-purple-50" @click="openRefundDialog(row)">
              <Icon name="dollar" size="sm" />
              <span>{{ t('payment.orders.requestRefund') }}</span>
            </button>
          </div>
        </template>
      </OrderTable>

      <UiPagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />
    </div>

    <BaseDialog :show="!!cancelTargetId" :title="t('payment.orders.cancel')" width="narrow" @close="cancelTargetId = null">
      <p class="text-sm text-muted">{{ t('payment.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button variant="secondary" @click="cancelTargetId = null">{{ t('common.cancel') }}</Button>
          <Button variant="danger" :disabled="actionLoading" @click="confirmCancel">{{ actionLoading ? t('common.processing') : t('payment.orders.cancel') }}</Button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="!!refundTarget" :title="t('payment.orders.requestRefund')" @close="refundTarget = null">
      <div v-if="refundTarget" class="space-y-4">
        <GlassCard variant="solid" padding="sm">
          <div class="flex justify-between text-sm">
            <span class="text-muted">{{ t('payment.orders.orderId') }}</span>
            <span class="font-mono text-foreground">#{{ refundTarget.id }}</span>
          </div>
          <div class="mt-2 flex justify-between text-sm">
            <span class="text-muted">{{ t('payment.orders.amount') }}</span>
            <span class="text-foreground">${{ refundTarget.amount.toFixed(2) }}</span>
          </div>
        </GlassCard>
        <div>
          <label class="input-label">{{ t('payment.refundReason') }}</label>
          <textarea v-model="refundReason" rows="3" class="input mt-1 w-full" :placeholder="t('payment.refundReasonPlaceholder')" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button variant="secondary" @click="refundTarget = null">{{ t('common.cancel') }}</Button>
          <Button :disabled="actionLoading || !refundReason.trim()" @click="confirmRefund">{{ actionLoading ? t('common.processing') : t('payment.orders.requestRefund') }}</Button>
        </div>
      </template>
    </BaseDialog>

    <BaseDialog :show="showInvoiceDialog" :title="t('payment.invoices.apply')" @close="showInvoiceDialog = false">
      <div class="space-y-3">
        <p class="text-sm text-muted">{{ t('payment.invoices.applyHint', { count: selectedIds.length }) }}</p>
        <div>
          <label class="input-label">{{ t('payment.invoices.title') }}</label>
          <input v-model="invoiceForm.title" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoices.taxNumber') }} <span class="text-red-500">*</span></label>
          <input v-model="invoiceForm.tax_number" class="input mt-1 w-full" required />
          <p class="mt-1 text-xs text-muted">{{ t('payment.invoices.taxNumberRequired') }}</p>
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoices.email') }}</label>
          <input v-model="invoiceForm.email" type="email" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoices.contactName') }}</label>
          <input v-model="invoiceForm.contact_name" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoices.contactPhone') }}</label>
          <input v-model="invoiceForm.contact_phone" class="input mt-1 w-full" />
        </div>
        <div>
          <label class="input-label">{{ t('payment.invoices.note') }}</label>
          <textarea v-model="invoiceForm.request_note" rows="2" class="input mt-1 w-full" />
        </div>
      </div>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button variant="secondary" @click="showInvoiceDialog = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="actionLoading || !invoiceForm.title.trim() || !invoiceForm.tax_number.trim() || !invoiceForm.email.trim()" @click="confirmInvoice">
            {{ actionLoading ? t('common.processing') : t('payment.invoices.apply') }}
          </Button>
        </div>
      </template>
    </BaseDialog>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import type { PaymentOrder } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import Button from '@/components/ui/Button.vue'
import FilterBar from '@/components/ui/FilterBar.vue'
import ChipScroller from '@/components/ui/ChipScroller.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import UiPagination from '@/components/ui/UiPagination.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import OrderTable from '@/components/payment/OrderTable.vue'
import { useAuthStore } from '@/stores/auth'
import { loadInvoiceDraft, saveInvoiceDraft } from './invoiceDraft'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()

const loading = ref(false)
const actionLoading = ref(false)
const orders = ref<PaymentOrder[]>([])
const refundEligibleProviders = ref<Set<string>>(new Set())
const invoiceEligibleProviders = ref<Set<string>>(new Set())
const selectedIds = ref<number[]>([])
const showInvoiceDialog = ref(false)
const invoiceForm = reactive({
  title: '',
  tax_number: '',
  email: '',
  contact_name: '',
  contact_phone: '',
  request_note: '',
})
const currentFilter = ref('')
const cancelTargetId = ref<number | null>(null)
const refundTarget = ref<PaymentOrder | null>(null)
const refundReason = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

function hydrateInvoiceDraft() {
  if (authStore.user?.id) Object.assign(invoiceForm, loadInvoiceDraft(authStore.user.id, authStore.user.email))
}

function persistInvoiceDraft() {
  if (authStore.user?.id) saveInvoiceDraft(authStore.user.id, invoiceForm)
}

watch(invoiceForm, persistInvoiceDraft, { deep: true })

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

function canRequestRefund(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  return refundEligibleProviders.value.has(order.provider_instance_id)
}

function canInvoice(order: PaymentOrder): boolean {
  if (order.status !== 'COMPLETED') return false
  if (!order.provider_instance_id) return false
  return invoiceEligibleProviders.value.has(order.provider_instance_id)
}

function toggleSelect(id: number) {
  if (selectedIds.value.includes(id)) {
    selectedIds.value = selectedIds.value.filter((item) => item !== id)
    return
  }
  if (selectedIds.value.length >= 100) return
  selectedIds.value = [...selectedIds.value, id]
}

async function confirmInvoice() {
  if (!invoiceForm.title.trim() || !invoiceForm.tax_number.trim() || !invoiceForm.email.trim() || selectedIds.value.length === 0) return
  actionLoading.value = true
  try {
    persistInvoiceDraft()
    await paymentAPI.applyInvoice({
      order_ids: selectedIds.value,
      title: invoiceForm.title.trim(),
      tax_number: invoiceForm.tax_number.trim(),
      email: invoiceForm.email.trim(),
      contact_name: invoiceForm.contact_name.trim(),
      contact_phone: invoiceForm.contact_phone.trim(),
      request_note: invoiceForm.request_note.trim(),
    })
    appStore.showSuccess(t('common.success'))
    showInvoiceDialog.value = false
    selectedIds.value = []
    await fetchOrders()
    router.push('/invoices')
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
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
  } catch { /* ignore */ }
}

onMounted(() => { hydrateInvoiceDraft(); fetchOrders(); loadRefundEligibility(); loadInvoiceEligibility() })
</script>
