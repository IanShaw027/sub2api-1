<template>
  <div class="glass-card glass-ring p-5">
    <div class="mb-3 flex flex-wrap items-center gap-2">
      <span :class="['rounded-md border px-2 py-0.5 text-xs font-medium', planBadgeClass]">
        {{ platformLabel(plan.group_platform || '') }}
      </span>
      <h3 class="text-lg font-bold text-foreground">{{ plan.name }}</h3>
    </div>
    <div class="flex items-baseline gap-2">
      <span v-if="plan.original_price" class="text-sm text-muted line-through">
        {{ formatSubscriptionAmount(plan.original_price) }}
      </span>
      <span :class="['confirm-price', planTextClass]">{{ formatSubscriptionAmount(plan.price) }}</span>
      <span class="text-sm text-muted">/ {{ validitySuffix }}</span>
    </div>
    <p v-if="plan.description" class="mt-2 text-sm leading-relaxed text-muted">
      {{ plan.description }}
    </p>
    <div class="mt-3 grid grid-cols-2 gap-3">
      <div>
        <span class="text-xs text-muted">{{ t('payment.planCard.rate') }}</span>
        <div class="flex items-baseline">
          <span :class="['text-lg font-bold', planTextClass]">×{{ plan.rate_multiplier ?? 1 }}</span>
        </div>
      </div>
      <div v-if="hasPeakRate">
        <span class="text-xs text-muted">{{ t('payment.planCard.peakRate') }}</span>
        <div class="text-sm font-semibold confirm-peak-text">
          {{ peakRateLabel }}
        </div>
      </div>
      <div v-if="plan.daily_limit_usd != null">
        <span class="text-xs text-muted">{{ t('payment.planCard.dailyLimit') }}</span>
        <div class="font-semibold text-lg text-foreground">${{ plan.daily_limit_usd }}</div>
      </div>
      <div v-if="plan.weekly_limit_usd != null">
        <span class="text-xs text-muted">{{ t('payment.planCard.weeklyLimit') }}</span>
        <div class="font-semibold text-lg text-foreground">${{ plan.weekly_limit_usd }}</div>
      </div>
      <div v-if="plan.monthly_limit_usd != null">
        <span class="text-xs text-muted">{{ t('payment.planCard.monthlyLimit') }}</span>
        <div class="font-semibold text-lg text-foreground">${{ plan.monthly_limit_usd }}</div>
      </div>
      <div v-if="plan.daily_limit_usd == null && plan.weekly_limit_usd == null && plan.monthly_limit_usd == null">
        <span class="text-xs text-muted">{{ t('payment.planCard.quota') }}</span>
        <div class="font-semibold text-lg text-foreground">{{ t('payment.planCard.unlimited') }}</div>
      </div>
    </div>
  </div>
  <div v-if="enabledMethods.length >= 1" class="glass-card p-6">
    <PaymentMethodSelector
      :methods="methodOptions"
      :selected="selectedMethod"
      @select="$emit('update:selectedMethod', $event)"
    />
  </div>
  <div v-if="feeRate > 0 && plan.price > 0" class="glass-card p-6">
    <div class="space-y-2 text-sm">
      <div class="flex justify-between">
        <span class="text-muted">{{ t('payment.amountLabel') }}</span>
        <span class="text-foreground">{{ formatAmount(subPaymentAmount) }}</span>
      </div>
      <div class="flex justify-between">
        <span class="text-muted">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
        <span class="text-foreground">{{ formatAmount(subFeeAmount) }}</span>
      </div>
      <div class="flex justify-between border-t border-line pt-2">
        <span class="font-medium text-foreground">{{ t('payment.actualPay') }}</span>
        <span class="text-lg font-bold text-accent">{{ formatAmount(subTotalAmount) }}</span>
      </div>
    </div>
  </div>
  <button class="btn btn-lg w-full" :class="paymentButtonClass" :disabled="!canSubmit || submitting" @click="$emit('submit')">
    <span v-if="submitting" class="flex items-center justify-center gap-2">
      <span class="confirm-btn-spinner"></span>
      {{ t('common.processing') }}
    </span>
    <span v-else>{{ t('payment.createOrder') }} {{ formatAmount(subTotalAmount) }}</span>
  </button>
  <button class="btn-glass-secondary w-full" @click="$emit('cancel')">{{ t('common.cancel') }}</button>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { platformLabel } from '@/utils/platformColors'
import type { SubscriptionPlan } from '@/types/payment'

const { t } = useI18n()

defineProps<{
  plan: SubscriptionPlan
  planBadgeClass: string
  planTextClass: string
  validitySuffix: string
  hasPeakRate: boolean
  peakRateLabel: string
  formatSubscriptionAmount: (value: number) => string
  formatAmount: (value: number) => string
  enabledMethods: string[]
  methodOptions: PaymentMethodOption[]
  selectedMethod: string
  feeRate: number
  subPaymentAmount: number
  subFeeAmount: number
  subTotalAmount: number
  paymentButtonClass: string
  canSubmit: boolean
  submitting: boolean
}>()

defineEmits<{
  'update:selectedMethod': [value: string]
  submit: []
  cancel: []
}>()
</script>

<style scoped>
.confirm-price {
  font-size: 28px;
  font-weight: 800;
  font-variant-numeric: tabular-nums;
}

.confirm-peak-text {
  color: var(--warning-text);
}

.confirm-btn-spinner {
  width: 16px;
  height: 16px;
  border-radius: 999px;
  border: 2px solid color-mix(in oklch, white 60%, transparent);
  border-top-color: transparent;
  animation: confirm-spin 0.6s linear infinite;
}

@keyframes confirm-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
