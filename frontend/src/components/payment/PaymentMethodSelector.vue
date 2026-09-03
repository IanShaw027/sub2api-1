<template>
 <div>
 <label class="input-label">
 {{ t('payment.paymentMethod') }}
 </label>
 <div
 data-testid="payment-method-grid"
 class="method-grid"
 >
 <button
 v-for="method in sortedMethods"
 :key="method.type"
 type="button"
 :title="methodLabel(method)"
 :disabled="!method.available"
 class="method-option glass-card"
 :class="[
 !method.available ? 'method-option--disabled' : '',
 selected === method.type ? 'glass-ring method-option--active' : '',
 ]"
 @click="method.available && emit('select', method.type)"
 >
 <img :src="methodIcon(method.type)" :alt="methodLabel(method)" class="method-option-icon" />
 <span class="method-option-body">
 <span data-testid="payment-method-label" class="method-option-label">
 {{ methodLabel(method) }}
 </span>
 <span
 v-if="method.fee_rate > 0"
 class="method-option-fee"
 >
 {{ t('payment.fee') }} {{ method.fee_rate }}%
 </span>
 </span>
 </button>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { METHOD_ORDER, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import paymentIcon from '@/assets/icons/payment.svg'

export interface PaymentMethodOption {
 type: string
 display_name?: string
 fee_rate: number
 available: boolean
}

const props = defineProps<{
 methods: PaymentMethodOption[]
 selected: string
}>()

const emit = defineEmits<{
 select: [type: string]
}>()

const { t } = useI18n()

const METHOD_ICONS: Record<string, string> = {
 alipay: alipayIcon,
 wxpay: wxpayIcon,
 stripe: stripeIcon,
 airwallex: airwallexIcon,
 credit_card: paymentIcon,
}

const sortedMethods = computed(() => {
 const order: readonly string[] = METHOD_ORDER
 return [...props.methods].sort((a, b) => {
 const ai = order.indexOf(a.type)
 const bi = order.indexOf(b.type)
 return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
 })
})

function methodIcon(type: string): string {
 if (isBuiltInAlipayMethod(type)) return METHOD_ICONS.alipay
 if (isBuiltInWxpayMethod(type)) return METHOD_ICONS.wxpay
 if (type === 'airwallex') return METHOD_ICONS.airwallex
 return METHOD_ICONS[type] || paymentIcon
}

function methodLabel(method: PaymentMethodOption): string {
 return method.display_name || t(`payment.methods.${method.type}`, method.type)
}
</script>

<style scoped>
.method-grid {
 display: grid;
 grid-template-columns: repeat(2, 1fr);
 gap: 10px;
}

@media (min-width: 640px) {
 .method-grid {
 grid-template-columns: repeat(3, 1fr);
 }
}

@media (min-width: 1024px) {
 .method-grid {
 grid-template-columns: repeat(4, 1fr);
 }
}

.method-option {
 display: flex;
 min-width: 0;
 height: 58px;
 align-items: center;
 justify-content: center;
 gap: 8px;
 padding: 0 12px;
 cursor: pointer;
 transition: transform 0.15s ease, box-shadow 0.15s ease;
}

.method-option:hover:not(.method-option--disabled) {
 transform: translateY(-1px);
}

.method-option--active {
 background: color-mix(in oklch, var(--accent) 8%, transparent);
}

.method-option--disabled {
 cursor: not-allowed;
 opacity: 0.5;
}

.method-option-icon {
 height: 26px;
 width: 26px;
 flex-shrink: 0;
 object-fit: contain;
}

.method-option-body {
 display: flex;
 min-width: 0;
 flex-direction: column;
 align-items: flex-start;
 line-height: 1.2;
}

.method-option-label {
 display: block;
 width: 100%;
 overflow: hidden;
 text-overflow: ellipsis;
 white-space: nowrap;
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
}

.method-option-fee {
 font-size: 10.5px;
 letter-spacing: 0.02em;
 color: var(--muted);
}
</style>
