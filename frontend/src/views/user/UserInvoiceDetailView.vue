<template>
  <AppLayout>
    <PageHeader :title="t('payment.invoices.detail')" :description="invoice ? `#${invoice.id}` : t('nav.myInvoices')">
      <template #actions>
        <Button variant="secondary" @click="router.push('/invoices')">{{ t('common.back') }}</Button>
      </template>
    </PageHeader>

    <GlassCard v-if="!invoice" class="detail-loading" padding="lg">
      {{ t('common.loading') }}
    </GlassCard>

    <DetailPageLayout v-else>
      <template #main>
        <GlassCard padding="lg">
          <div class="grid gap-4 sm:grid-cols-2">
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.id') }}</p>
              <p class="font-mono">#{{ invoice.id }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.title') }}</p>
              <p>{{ invoice.title }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.amount') }}</p>
              <p>{{ invoice.invoice_amount.toFixed(2) }}{{ invoice.currency ? ' ' + invoice.currency : '' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.taxNumber') }}</p>
              <p>{{ invoice.tax_number || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.email') }}</p>
              <p>{{ invoice.email }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.contactName') }}</p>
              <p>{{ invoice.contact_name || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.contactPhone') }}</p>
              <p>{{ invoice.contact_phone || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.note') }}</p>
              <p>{{ invoice.request_note || '-' }}</p>
            </div>
            <div>
              <p class="text-xs text-muted">{{ t('payment.invoices.fileName') }}</p>
              <p>{{ invoice.file_name || '-' }}</p>
            </div>
          </div>
        </GlassCard>

        <GlassCard padding="lg">
          <p class="mb-2 text-sm font-medium">{{ t('payment.invoices.orders') }}</p>
          <ul class="space-y-1 text-sm">
            <li v-for="item in invoice.orders || []" :key="item.order_id" class="flex justify-between">
              <span class="font-mono">#{{ item.order_id }} {{ item.out_trade_no }}</span>
              <span>{{ item.pay_amount_snapshot.toFixed(2) }}{{ item.currency ? ` ${item.currency}` : '' }}</span>
            </li>
          </ul>
        </GlassCard>
      </template>

      <template #side>
        <GlassCard padding="lg" class="invoice-status-card">
          <p class="invoice-status-label">{{ t('payment.invoices.statusLabel') }}</p>
          <StatusBadge :tone="statusTone(invoice.status)" :label="statusLabel(invoice.status)" dot />

          <div v-if="canCancel || canDownload" class="invoice-actions">
            <Button v-if="canCancel" variant="danger" :disabled="actionLoading" @click="showCancelConfirm = true">
              {{ t('payment.invoices.cancel') }}
            </Button>
            <Button v-if="canDownload" :disabled="actionLoading" @click="showDownloadConfirm = true">
              {{ t('payment.invoices.download') }}
            </Button>
          </div>
        </GlassCard>
      </template>
    </DetailPageLayout>

    <UiModal
      :open="showCancelConfirm"
      :title="t('payment.invoices.cancel')"
      width="sm"
      :close-label="t('common.close')"
      :close-on-overlay="false"
      @close="showCancelConfirm = false"
    >
      <p class="text-sm text-muted">{{ t('payment.invoices.confirmCancel') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button variant="secondary" @click="showCancelConfirm = false">{{ t('common.cancel') }}</Button>
          <Button variant="danger" :disabled="actionLoading" @click="confirmCancelInvoice">
            {{ actionLoading ? t('common.processing') : t('payment.invoices.cancel') }}
          </Button>
        </div>
      </template>
    </UiModal>

    <UiModal
      :open="showDownloadConfirm"
      :title="t('payment.invoices.download')"
      width="sm"
      :close-label="t('common.close')"
      :close-on-overlay="false"
      @close="showDownloadConfirm = false"
    >
      <p class="text-sm text-muted">{{ t('payment.invoices.confirmDownload') }}</p>
      <template #footer>
        <div class="flex justify-end gap-3">
          <Button variant="secondary" @click="showDownloadConfirm = false">{{ t('common.cancel') }}</Button>
          <Button :disabled="actionLoading" @click="confirmDownload">
            {{ actionLoading ? t('common.processing') : t('payment.invoices.download') }}
          </Button>
        </div>
      </template>
    </UiModal>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import DetailPageLayout from '@/components/layout/DetailPageLayout.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import UiModal from '@/components/ui/UiModal.vue'
import type { StatusBadgeTone } from '@/components/ui/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const invoice = ref<Invoice | null>(null)
const actionLoading = ref(false)
const showCancelConfirm = ref(false)
const showDownloadConfirm = ref(false)

const canCancel = computed(() => invoice.value?.status === 'APPLIED')
const canDownload = computed(() => invoice.value?.status === 'ISSUED' && !!invoice.value.has_file)

function statusLabel(status: string) {
  return t(`payment.invoices.status.${String(status).toLowerCase()}`, status)
}

function statusTone(status: string): StatusBadgeTone {
  const upper = String(status).toUpperCase()
  if (upper === 'ISSUED') return 'success'
  if (upper === 'CANCELLED') return 'muted'
  return 'warning'
}

async function load() {
  const id = Number(route.params.id)
  try {
    const res = await paymentAPI.getInvoice(id)
    invoice.value = res.data
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  }
}

async function confirmCancelInvoice() {
  if (!invoice.value || invoice.value.status !== 'APPLIED') return
  actionLoading.value = true
  try {
    const res = await paymentAPI.cancelInvoice(invoice.value.id)
    invoice.value = res.data
    appStore.showSuccess(t('common.success'))
    showCancelConfirm.value = false
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function confirmDownload() {
  if (!invoice.value) return
  actionLoading.value = true
  try {
    const res = await paymentAPI.downloadInvoiceFile(invoice.value.id)
    const url = URL.createObjectURL(res.data)
    const link = document.createElement('a')
    link.href = url
    link.download = invoice.value.file_name || `invoice-${invoice.value.id}.pdf`
    link.click()
    URL.revokeObjectURL(url)
    showDownloadConfirm.value = false
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(load)
</script>

<style scoped>
.detail-loading {
  text-align: center;
}

.invoice-status-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.invoice-status-label {
  font-size: var(--fs-12);
  color: var(--muted);
}

.invoice-actions {
  display: flex;
  flex-direction: column;
  gap: 8px;
}
</style>
