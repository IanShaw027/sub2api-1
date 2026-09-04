<template>
 <DataTable :columns="columns" :data="orders" :loading="loading">
 <template #cell-out_trade_no="{ value, row }">
 <div class="cell-primary">
 <span class="cell-primary-name">{{ value }}</span>
 <span class="cell-primary-meta">#{{ row.id }}</span>
 </div>
 </template>
 <template v-if="showUser" #cell-user_email="{ value, row }">
 <div class="cell-primary">
 <span class="cell-primary-name">{{ value || row.user_name || '#' + row.user_id }}</span>
 <span v-if="row.user_notes" class="cell-primary-meta">{{ row.user_notes }}</span>
 </div>
 </template>
 <template #cell-pay_amount="{ value, row }">
 <div class="cell-amount">
 <span class="cell-amount-value">{{ paymentAmountSymbol(row) }}{{ value.toFixed(2) }}</span>
 <span v-if="row.fee_rate > 0 || row.amount !== row.pay_amount" class="cell-amount-meta">
 <template v-if="row.fee_rate > 0">{{ t('payment.orders.fee') }} {{ row.fee_rate }}%</template>
 <template v-if="row.fee_rate > 0 && row.amount !== row.pay_amount"> · </template>
 <template v-if="row.amount !== row.pay_amount">
 {{ t('payment.orders.creditedAmount') }} {{ creditedAmountSymbol }}{{ row.amount.toFixed(2) }}
 </template>
 </span>
 </div>
 </template>
 <template #cell-payment_type="{ value }">
 <span class="tag">{{ t('payment.methods.' + value, value) }}</span>
 </template>
 <template #cell-status="{ value }">
 <OrderStatusBadge :status="value" />
 </template>
 <template #cell-created_at="{ value }">
 <span class="cell-time" :title="formatDateTime(value)">{{ formatRelativeTime(value) }}</span>
 </template>
 <template #cell-actions="{ row }">
 <slot name="actions" :row="row" />
 </template>
 </DataTable>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PaymentOrder } from '@/types/payment'
import type { Column } from '@/components/common/types'
import DataTable from '@/components/common/DataTable.vue'
import OrderStatusBadge from '@/components/payment/OrderStatusBadge.vue'
import { currencySymbol } from '@/components/payment/currency'
import { formatDateTime, formatRelativeTime } from '@/utils/format'

const { t } = useI18n()

const props = defineProps<{
 orders: PaymentOrder[]
 loading: boolean
 showUser?: boolean
}>()

const creditedAmountSymbol = currencySymbol('USD')

function paymentAmountSymbol(order: PaymentOrder): string {
 return currencySymbol(order.currency)
}

const columns = computed((): Column[] => {
 const cols: Column[] = [
 { key: 'out_trade_no', label: t('payment.orders.orderNo') },
 ]
 if (props.showUser) {
 cols.push({ key: 'user_email', label: t('payment.admin.colUser') })
 }
 cols.push(
 { key: 'pay_amount', label: t('payment.orders.payAmount') },
 { key: 'payment_type', label: t('payment.orders.paymentMethod') },
 { key: 'status', label: t('payment.orders.status') },
 { key: 'created_at', label: t('payment.orders.createdAt') },
 { key: 'actions', label: t('common.actions') },
 )
 return cols
})
</script>

<style scoped>
/* ListPage table geometry: thead 42px / rows 61px (matches AccountsView/KeysView recipe) */
:deep(.table-wrapper thead th) {
  height: 42px;
}

:deep(.table-wrapper tbody td) {
  height: 61px;
}

.cell-primary {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  line-height: 1.2;
}

.cell-primary-name {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-primary-meta {
  font-family: var(--font-mono);
  font-size: 11.5px;
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-amount {
  display: flex;
  flex-direction: column;
  gap: 2px;
  line-height: 1.2;
  font-variant-numeric: tabular-nums;
}

.cell-amount-value {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.cell-amount-meta {
  font-size: 11.5px;
  color: var(--muted);
}

.cell-time {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
</style>
