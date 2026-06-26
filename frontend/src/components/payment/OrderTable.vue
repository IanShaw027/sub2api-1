<template>
  <DataTable :columns="computedColumns" :data="orders" :loading="loading">
    <template v-if="selectable" #header-select>
      <input
        type="checkbox"
        :checked="allVisibleSelected"
        :indeterminate="someVisibleSelected && !allVisibleSelected"
        class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 dark:border-dark-600 dark:bg-dark-800"
        @change="emit('toggle-all')"
      >
    </template>
    <template v-if="selectable" #cell-select="{ row }">
      <input
        type="checkbox"
        :checked="isSelected?.(row) ?? false"
        :disabled="!(isRowSelectable?.(row) ?? true)"
        class="h-4 w-4 rounded border-gray-300 text-blue-600 focus:ring-blue-500 disabled:cursor-not-allowed disabled:opacity-40 dark:border-dark-600 dark:bg-dark-800"
        @change="emit('toggle-row', row)"
      >
    </template>
    <template #cell-id="{ value }">
      <span class="font-mono text-sm">#{{ value }}</span>
    </template>
    <template #cell-out_trade_no="{ value }">
      <span class="text-sm text-gray-900 dark:text-white">{{ value }}</span>
    </template>
    <template v-if="showUser" #cell-user_email="{ value, row }">
      <div class="text-sm">
        <span class="text-gray-900 dark:text-white">{{ value || row.user_name || '#' + row.user_id }}</span>
        <span v-if="row.user_notes" class="ml-1 text-xs text-gray-400">({{ row.user_notes }})</span>
      </div>
    </template>
    <template #cell-pay_amount="{ value, row }">
      <div class="text-sm">
        <span class="font-medium text-gray-900 dark:text-white">{{ paymentAmountSymbol(row) }}{{ value.toFixed(2) }}</span>
        <span v-if="row.fee_rate > 0" class="ml-1 text-xs text-gray-400" :title="t('payment.orders.fee') + ': ' + row.fee_rate + '%'">
          ({{ t('payment.orders.fee') }} {{ row.fee_rate }}%)
        </span>
        <div v-if="row.amount !== row.pay_amount" class="text-xs text-gray-500">
          {{ t('payment.orders.creditedAmount') }}: {{ creditedAmountSymbol }}{{ row.amount.toFixed(2) }}
        </div>
      </div>
    </template>
    <template #cell-payment_type="{ value }">
      <span class="text-sm text-gray-700 dark:text-gray-300">{{ t(paymentMethodDisplayKey(value), value) }}</span>
    </template>
    <template #cell-status="{ value }">
      <OrderStatusBadge :status="value" />
    </template>
    <template #cell-created_at="{ value }">
      <span class="text-xs text-gray-500 dark:text-gray-400">{{ formatDate(value) }}</span>
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
import { paymentMethodDisplayKey } from '@/utils/i18n'
import { currencySymbol } from '@/components/payment/currency'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  orders: PaymentOrder[]
  loading: boolean
  showUser?: boolean
  selectable?: boolean
  isSelected?: (row: PaymentOrder) => boolean
  isRowSelectable?: (row: PaymentOrder) => boolean
  allVisibleSelected?: boolean
  someVisibleSelected?: boolean
}>(), {
  selectable: false,
  allVisibleSelected: false,
  someVisibleSelected: false,
})

const emit = defineEmits<{
  'toggle-row': [row: PaymentOrder]
  'toggle-all': []
}>()

function formatDate(dateStr: string) { return new Date(dateStr).toLocaleString() }

const computedColumns = computed((): Column[] => {
  const cols: Column[] = []
  if (props.selectable) {
    cols.push({ key: 'select', label: '' })
  }
  cols.push(
    { key: 'id', label: t('payment.orders.orderId') },
    { key: 'out_trade_no', label: t('payment.orders.orderNo') }
  )
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

const creditedAmountSymbol = currencySymbol('USD')

function paymentAmountSymbol(order: PaymentOrder): string {
  return currencySymbol(order.currency)
}
</script>
