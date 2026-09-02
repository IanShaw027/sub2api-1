<template>
 <AppLayout>
 <PageHeader :title="t('payment.invoices.mine')" :description="invoice ? `#${invoice.id}` : t('nav.myInvoices')">
 <template #actions>
 <Button variant="secondary" @click="router.push('/invoices')">{{ t('common.back') }}</Button>
 <Button v-if="invoice?.status === 'APPLIED'" variant="danger" :disabled="actionLoading" @click="cancelInvoice">
 {{ t('payment.invoices.cancel') }}
 </Button>
 <Button v-if="invoice?.status === 'ISSUED' && invoice.has_file" :disabled="actionLoading" @click="download">
 {{ t('payment.invoices.download') }}
 </Button>
 </template>
 </PageHeader>

 <GlassCard v-if="invoice" class="space-y-4" padding="lg">
 <div class="grid gap-4 sm:grid-cols-2">
 <div>
 <p class="text-xs text-muted">{{ t('payment.invoices.id') }}</p>
 <p class="font-mono">#{{ invoice.id }}</p>
 </div>
 <div>
 <p class="text-xs text-muted">{{ t('payment.invoices.statusLabel') }}</p>
 <p>{{ statusLabel(invoice.status) }}</p>
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

 <div>
 <p class="mb-2 text-sm font-medium">{{ t('payment.invoices.orders') }}</p>
 <ul class="space-y-1 text-sm">
 <li v-for="item in invoice.orders || []" :key="item.order_id" class="flex justify-between">
 <span class="font-mono">#{{ item.order_id }} {{ item.out_trade_no }}</span>
 <span>{{ item.pay_amount_snapshot.toFixed(2) }}{{ item.currency ? ` ${item.currency}` : '' }}</span>
 </li>
 </ul>
 </div>
 </GlassCard>
 </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { paymentAPI } from '@/api/payment'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { Invoice } from '@/types/payment'
import AppLayout from '@/components/layout/AppLayout.vue'
import Button from '@/components/ui/Button.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const invoice = ref<Invoice | null>(null)
const actionLoading = ref(false)

function statusLabel(status: string) {
  return t(`payment.invoices.status.${String(status).toLowerCase()}`, status)
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

async function cancelInvoice() {
  if (!invoice.value || invoice.value.status !== 'APPLIED') return
  actionLoading.value = true
  try {
    const res = await paymentAPI.cancelInvoice(invoice.value.id)
    invoice.value = res.data
    appStore.showSuccess(t('common.success'))
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

async function download() {
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
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error')))
  } finally {
    actionLoading.value = false
  }
}

onMounted(load)
</script>
