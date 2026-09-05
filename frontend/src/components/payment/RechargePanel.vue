<template>
  <div class="glass-card p-5">
    <p class="text-xs font-medium text-muted">{{ t('payment.rechargeAccount') }}</p>
    <p class="mt-1 text-base font-semibold text-foreground">{{ username }}</p>
    <p class="mt-0.5 text-sm font-medium purchase-balance-text">{{ t('payment.currentBalance') }}: {{ balance }}</p>
  </div>
  <div v-if="enabledMethods.length === 0" class="glass-card py-16 text-center">
    <p class="text-muted">{{ t('payment.notAvailable') }}</p>
  </div>
  <template v-else>
    <div class="glass-card p-6">
      <AmountInput
        :model-value="amount"
        :amounts="[10, 20, 50, 100, 200, 500, 1000, 2000, 5000]"
        :min="globalMinAmount"
        :max="globalMaxAmount"
        @update:model-value="$emit('update:amount', $event)"
      />
      <p v-if="amountError" class="mt-2 text-xs purchase-warning-text">{{ amountError }}</p>
    </div>
    <div v-if="enabledMethods.length >= 1" class="glass-card p-6">
      <PaymentMethodSelector
        :methods="methodOptions"
        :selected="selectedMethod"
        @select="$emit('update:selectedMethod', $event)"
      />
    </div>
    <div v-if="validAmount > 0" class="glass-card p-6">
      <div class="space-y-2 text-sm">
        <div class="flex justify-between">
          <span class="text-muted">{{ t('payment.paymentAmount') }}</span>
          <span class="text-foreground">{{ formatAmount(validAmount) }}</span>
        </div>
        <div v-if="feeRate > 0" class="flex justify-between">
          <span class="text-muted">{{ t('payment.fee') }} ({{ feeRate }}%)</span>
          <span class="text-foreground">{{ formatAmount(feeAmount) }}</span>
        </div>
        <div v-if="feeRate > 0" class="flex justify-between border-t border-line pt-2">
          <span class="font-medium text-foreground">{{ t('payment.actualPay') }}</span>
          <span class="text-lg font-bold text-accent">{{ formatAmount(totalAmount) }}</span>
        </div>
        <div v-if="balanceRechargeMultiplier !== 1" class="flex justify-between" :class="{ 'border-t border-line pt-2': feeRate <= 0 }">
          <span class="text-muted">{{ t('payment.creditedBalance') }}</span>
          <span class="text-foreground">${{ creditedAmount.toFixed(2) }}</span>
        </div>
        <p v-if="balanceRechargeMultiplier !== 1" class="border-t border-line pt-2 text-xs text-muted">
          {{ t('payment.rechargeRatePreview', { currency: selectedCurrency, usd: balanceRechargeMultiplier.toFixed(2) }) }}
        </p>
      </div>
    </div>
    <button class="btn btn-lg w-full" :class="paymentButtonClass" :disabled="!canSubmit || submitting" @click="$emit('submit')">
      <span v-if="submitting" class="flex items-center justify-center gap-2">
        <span class="purchase-btn-spinner"></span>
        {{ t('common.processing') }}
      </span>
      <span v-else>{{ t('payment.createOrder') }} {{ formatAmount(totalAmount) }}</span>
    </button>
  </template>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import AmountInput from '@/components/payment/AmountInput.vue'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'

const { t } = useI18n()

defineProps<{
  username: string
  balance: string
  enabledMethods: string[]
  amount: number | null
  globalMinAmount: number
  globalMaxAmount: number
  amountError: string
  methodOptions: PaymentMethodOption[]
  selectedMethod: string
  validAmount: number
  feeRate: number
  feeAmount: number
  totalAmount: number
  balanceRechargeMultiplier: number
  creditedAmount: number
  selectedCurrency: string
  formatAmount: (value: number) => string
  paymentButtonClass: string
  canSubmit: boolean
  submitting: boolean
}>()

defineEmits<{
  'update:amount': [value: number | null]
  'update:selectedMethod': [value: string]
  submit: []
}>()
</script>

<style scoped>
.purchase-balance-text {
  color: var(--success-text);
}

.purchase-warning-text {
  color: var(--warning-text);
}

.purchase-btn-spinner {
  width: 16px;
  height: 16px;
  border-radius: 999px;
  border: 2px solid color-mix(in oklch, white 60%, transparent);
  border-top-color: transparent;
  animation: purchase-spin 0.6s linear infinite;
}

@keyframes purchase-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
