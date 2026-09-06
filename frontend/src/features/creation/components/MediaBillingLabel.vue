<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { RotateCcw } from '@lucide/vue'
import type { MediaCost } from '../localMedia'
import { mediaMessages } from './mediaMessages'

const props = defineProps<{ cost?: MediaCost | null; loading?: boolean; canRefresh?: boolean }>()
const emit = defineEmits<{ refresh: [] }>()
const { t, locale } = useI18n({ useScope: 'local', messages: mediaMessages })
const label = computed(() => {
  if (props.loading) return t('pricingLoading')
  const cost = props.cost
  if (cost?.status === 'not_billed' && cost.amount === 0) return t('notBilled')
  if (cost?.status === 'pending') return t('billingPending')
  if (!cost || !['estimated', 'settled'].includes(cost.status) || typeof cost.amount !== 'number' || !Number.isFinite(cost.amount)) return t('actualSettlement')
  const amount = cost.amount > 0 && cost.amount < 0.00000001 ? '< $0.00000001' : new Intl.NumberFormat(locale.value, { style: 'currency', currency: 'USD', maximumFractionDigits: 8 }).format(cost.amount)
  const target = cost.billing_target === 'balance' ? t('billingBalance') : cost.billing_target === 'subscription' ? t('billingSubscription') : ''
  return `${t(cost.status === 'estimated' ? 'estimatedCost' : 'actualCost')}: ${amount}${target ? ` · ${target}` : ''}`
})
</script>

<template>
  <div class="media-billing" :data-billing-status="cost?.status || 'unavailable'" role="status"><span>{{ label }}</span><button v-if="canRefresh && !['settled', 'not_billed'].includes(cost?.status || '')" type="button" :title="t('refreshBilling')" :aria-label="t('refreshBilling')" :disabled="loading" @click="emit('refresh')"><RotateCcw :size="12" /></button></div>
</template>

<style scoped>
.media-billing { display: flex; align-items: center; flex-wrap: wrap; gap: 6px; color: var(--muted); font-size: 11px; line-height: 1.5; overflow-wrap: anywhere; }
.media-billing button { display: inline-flex; padding: 3px; color: var(--muted); background: none; border: 0; }
.media-billing[data-billing-status='settled'] { color: var(--foreground); }
</style>
