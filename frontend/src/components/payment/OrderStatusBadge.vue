<template>
 <StatusBadge :tone="statusTone" :label="statusLabel" dot />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { OrderStatus } from '@/types/payment'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { StatusBadgeTone } from '@/components/ui/types'

const props = defineProps<{
 status: OrderStatus
}>()

const { t } = useI18n()

const statusMap: Record<OrderStatus, { key: string; tone: StatusBadgeTone }> = {
 PENDING: { key: 'payment.status.pending', tone: 'warning' },
 PAID: { key: 'payment.status.paid', tone: 'accent' },
 RECHARGING: { key: 'payment.status.recharging', tone: 'accent' },
 COMPLETED: { key: 'payment.status.completed', tone: 'success' },
 EXPIRED: { key: 'payment.status.expired', tone: 'muted' },
 CANCELLED: { key: 'payment.status.cancelled', tone: 'muted' },
 FAILED: { key: 'payment.status.failed', tone: 'danger' },
 REFUND_REQUESTED: { key: 'payment.status.refund_requested', tone: 'warning' },
 REFUNDING: { key: 'payment.status.refunding', tone: 'warning' },
 REFUND_PENDING: { key: 'payment.status.refund_pending', tone: 'warning' },
 REFUNDED: { key: 'payment.status.refunded', tone: 'accent' },
 PARTIALLY_REFUNDED: { key: 'payment.status.partially_refunded', tone: 'accent' },
 REFUND_FAILED: { key: 'payment.status.refund_failed', tone: 'danger' },
}

const statusLabel = computed(() => {
 const entry = statusMap[props.status]
 return entry ? t(entry.key) : props.status
})

const statusTone = computed<StatusBadgeTone>(() => statusMap[props.status]?.tone ?? 'muted')
</script>
