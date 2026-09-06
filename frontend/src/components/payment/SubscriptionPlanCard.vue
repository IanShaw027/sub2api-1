<template>
 <div
 :class="[
 'plan-card group relative flex flex-col overflow-hidden border transition-all',
 'hover:shadow-pop hover:-translate-y-0.5',
 borderClass,
 'bg-surface',
 { 'glass-ring': selected },
 ]"
 >
 <!-- Colored top accent bar -->
 <div :class="['h-1.5', accentClass]" />

 <div class="plan-card-body flex flex-1 flex-col">
 <!-- Header: name + badge + price -->
 <div class="mb-3 flex items-start justify-between gap-2">
 <div class="min-w-0 flex-1">
 <h3
 :title="plan.name"
 class="h-12 min-w-0 break-words [overflow-wrap:anywhere] text-base font-bold leading-6 text-foreground line-clamp-2"
 >
 {{ plan.name }}
 </h3>
 <p v-if="plan.description" class="mt-0.5 text-xs leading-relaxed text-muted line-clamp-2">
 {{ plan.description }}
 </p>
 </div>
 <div class="shrink-0 text-right">
 <div class="flex items-baseline gap-1">
 <span class="text-xs text-muted">{{ planCurrencySymbol }}</span>
 <span :class="['plan-price', textClass]">{{ plan.price }}</span>
 <span v-if="plan.currency" class="text-xs font-medium text-muted">{{ plan.currency }}</span>
 </div>
 <div class="flex items-center justify-end gap-1">
 <span :class="['inline-flex shrink-0 rounded-full px-2 py-0.5 text-[11px] font-medium', badgeLightClass]">
 {{ pLabel }}
 </span>
 <span class="text-[11px] text-muted">/ {{ validitySuffix }}</span>
 </div>
 <div v-if="plan.original_price" class="mt-0.5 flex items-center justify-end gap-1.5">
 <span class="text-xs text-muted line-through">{{ planCurrencySymbol }}{{ plan.original_price }}<template v-if="plan.currency"> {{ plan.currency }}</template></span>
 <span :class="['rounded px-1 py-0.5 text-[10px] font-semibold', discountClass]">{{ discountText }}</span>
 </div>
 </div>
 </div>

 <!-- Group quota info (compact) -->
 <div class="mb-3 grid grid-cols-2 gap-x-3 gap-y-1 rounded-lg bg-surface-2 px-3 py-2 text-xs">
 <div class="flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.rate') }}</span>
 <span class="font-medium text-foreground">{{ rateDisplay }}</span>
 </div>
 <div v-if="hasPeakRate" class="col-span-2 flex items-center justify-between gap-2">
 <span class="text-muted">{{ t('payment.planCard.peakRate') }}</span>
 <span class="text-right font-medium text-warning-text">{{ peakRateDisplay }}</span>
 </div>
 <div v-if="plan.daily_limit_usd != null" class="flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.dailyLimit') }}</span>
 <span class="font-medium text-foreground">${{ plan.daily_limit_usd }}</span>
 </div>
 <div v-if="plan.weekly_limit_usd != null" class="flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.weeklyLimit') }}</span>
 <span class="font-medium text-foreground">${{ plan.weekly_limit_usd }}</span>
 </div>
 <div v-if="plan.monthly_limit_usd != null" class="flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.monthlyLimit') }}</span>
 <span class="font-medium text-foreground">${{ plan.monthly_limit_usd }}</span>
 </div>
 <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null" class="flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.quota') }}</span>
 <span class="font-medium text-foreground">{{ t('payment.planCard.unlimited') }}</span>
 </div>
 <div v-if="modelScopeLabels.length > 0" class="col-span-2 flex items-center justify-between">
 <span class="text-muted">{{ t('payment.planCard.models') }}</span>
 <div class="flex flex-wrap justify-end gap-1">
 <span v-for="scope in modelScopeLabels" :key="scope"
 class="rounded bg-surface-2/80 px-1.5 py-0.5 text-[10px] font-medium text-muted">
 {{ scope }}
 </span>
 </div>
 </div>
 </div>

 <!-- Features list (compact) -->
 <div v-if="plan.features.length > 0" class="mb-3 space-y-1">
 <div v-for="feature in plan.features" :key="feature" class="flex items-start gap-1.5">
 <svg class="plan-feature-icon mt-0.5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5">
 <path stroke-linecap="round" stroke-linejoin="round" d="M4.5 12.75l6 6 9-13.5" />
 </svg>
 <span class="plan-feature-text text-muted">{{ feature }}</span>
 </div>
 </div>

 <div class="flex-1" />

 <!-- Subscribe Button -->
 <button
 type="button"
 :class="['plan-card-cta w-full text-sm font-semibold transition-all active:scale-[0.98]', btnClass]"
 @click="emit('select', plan)"
 >
 {{ isRenewal ? t('payment.renewNow') : t('payment.subscribeNow') }}
 </button>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SubscriptionPlan } from '@/types/payment'
import type { UserSubscription } from '@/types'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'
import { planValiditySuffix } from './validity'
import { currencySymbol } from '@/components/payment/currency'
import {
 platformAccentBarClass,
 platformBadgeLightClass,
 platformBorderClass,
 platformTextClass,
 platformButtonClass,
 platformDiscountClass,
 platformLabel,
} from '@/utils/platformColors'

const props = defineProps<{ plan: SubscriptionPlan; activeSubscriptions?: UserSubscription[]; selected?: boolean }>()
const emit = defineEmits<{ select: [plan: SubscriptionPlan] }>()
const { t } = useI18n()

const platform = computed(() => props.plan.group_platform || '')
const isRenewal = computed(() =>
 props.activeSubscriptions?.some(s => s.group_id === props.plan.group_id && s.status === 'active') ?? false
)

// Derived color classes from central config
const accentClass = computed(() => platformAccentBarClass(platform.value))
const borderClass = computed(() => platformBorderClass(platform.value))
const badgeLightClass = computed(() => platformBadgeLightClass(platform.value))
const textClass = computed(() => platformTextClass(platform.value))
const btnClass = computed(() => platformButtonClass(platform.value))
const discountClass = computed(() => platformDiscountClass(platform.value))
const pLabel = computed(() => platformLabel(platform.value))

const discountText = computed(() => {
 if (!props.plan.original_price || props.plan.original_price <= 0) return ''
 const pct = Math.round((1 - props.plan.price / props.plan.original_price) * 100)
 return pct > 0 ? `-${pct}%` : ''
})

const rateDisplay = computed(() => {
 const rate = props.plan.rate_multiplier ?? 1
 return `×${Number(rate.toPrecision(10))}`
})

const appStore = useAppStore()
const planCurrencySymbol = computed(() => currencySymbol(props.plan.currency || 'USD'))

const hasPeakRate = computed(() => groupHasPeakRate(props.plan))

const peakRateDisplay = computed(() => {
 return formatPeakRateWindow(props.plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
})

const MODEL_SCOPE_LABELS: Record<string, string> = {
 claude: 'Claude',
 gemini_text: 'Gemini',
 gemini_image: 'Imagen',
}

const modelScopeLabels = computed(() => {
 if (platform.value !== 'antigravity') return []
 const scopes = props.plan.supported_model_scopes
 if (!scopes || scopes.length === 0) return []
 return scopes.map(s => MODEL_SCOPE_LABELS[s] || s)
})

const validitySuffix = computed(() => planValiditySuffix(props.plan, t))
</script>

<style scoped>
.plan-card {
  border-radius: var(--radius-hero);
}

.plan-card-body {
  padding: 20px;
}

.plan-price {
  font-size: 28px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.01em;
}

.plan-feature-text {
  font-size: 13px;
}

.plan-feature-icon {
  width: 12px;
  height: 12px;
  color: var(--success-text);
}

.plan-card-cta {
  height: 42px;
  border-radius: var(--radius-field);
}
</style>
