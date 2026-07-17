<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-chip px-2.5 py-0.5 text-xs font-semibold"
    :class="statusClass"
  >
    <span class="h-1.5 w-1.5 shrink-0 rounded-full" :class="dotClass" aria-hidden="true" />
    {{ statusLabel }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OrderStatus } from '@/types/payment'

const props = defineProps<{
  status: OrderStatus
}>()

const { t } = useI18n()

const statusMap: Record<OrderStatus, { key: string; class: string; dot: string }> = {
  PENDING: { key: 'payment.status.pending', class: 'bg-warning-soft text-warning dark:bg-yellow-900/30 dark:text-yellow-400', dot: 'bg-warning' },
  PAID: { key: 'payment.status.paid', class: 'bg-accent-50 text-accent-700 dark:bg-blue-900/30 dark:text-blue-400', dot: 'bg-accent' },
  RECHARGING: { key: 'payment.status.recharging', class: 'bg-accent-50 text-accent-700 dark:bg-blue-900/30 dark:text-blue-400', dot: 'bg-accent' },
  COMPLETED: { key: 'payment.status.completed', class: 'bg-success-soft text-success dark:bg-green-900/30 dark:text-green-400', dot: 'bg-success' },
  EXPIRED: { key: 'payment.status.expired', class: 'bg-ink-faint/10 text-ink-soft dark:bg-dark-900/30 dark:text-dark-400', dot: 'bg-ink-faint' },
  CANCELLED: { key: 'payment.status.cancelled', class: 'bg-ink-faint/10 text-ink-soft dark:bg-dark-900/30 dark:text-dark-400', dot: 'bg-ink-faint' },
  FAILED: { key: 'payment.status.failed', class: 'bg-danger-soft text-danger dark:bg-red-900/30 dark:text-red-400', dot: 'bg-danger' },
  REFUND_REQUESTED: { key: 'payment.status.refund_requested', class: 'bg-warning-soft text-warning dark:bg-orange-900/30 dark:text-orange-400', dot: 'bg-warning' },
  REFUNDING: { key: 'payment.status.refunding', class: 'bg-warning-soft text-warning dark:bg-orange-900/30 dark:text-orange-400', dot: 'bg-warning' },
  REFUND_PENDING: { key: 'payment.status.refund_pending', class: 'bg-warning-soft text-warning dark:bg-orange-900/30 dark:text-orange-400', dot: 'bg-warning' },
  REFUNDED: { key: 'payment.status.refunded', class: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400', dot: 'bg-purple-500' },
  PARTIALLY_REFUNDED: { key: 'payment.status.partially_refunded', class: 'bg-purple-100 text-purple-800 dark:bg-purple-900/30 dark:text-purple-400', dot: 'bg-purple-500' },
  REFUND_FAILED: { key: 'payment.status.refund_failed', class: 'bg-danger-soft text-danger dark:bg-red-900/30 dark:text-red-400', dot: 'bg-danger' },
}

const statusLabel = computed(() => {
  const entry = statusMap[props.status]
  return entry ? t(entry.key) : props.status
})

const statusClass = computed(() => {
  const entry = statusMap[props.status]
  return entry?.class ?? 'bg-ink-faint/10 text-ink-soft dark:bg-dark-900/30 dark:text-dark-400'
})

const dotClass = computed(() => {
  const entry = statusMap[props.status]
  return entry?.dot ?? 'bg-ink-faint'
})
</script>
