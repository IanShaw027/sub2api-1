<template>
 <div>
 <label class="input-label">
 {{ t('payment.paymentMethod') }}
 </label>
 <div
 data-testid="payment-method-grid"
 class="method-row"
 >
 <button
 v-for="method in sortedMethods"
 :key="method.type"
 type="button"
 :title="methodTitle(method)"
 :disabled="!method.available"
 class="method-option"
 :class="[
 !method.available ? 'method-option--disabled' : '',
 selected === method.type ? 'method-option--active' : '',
 ]"
 @click="method.available && emit('select', method.type)"
 >
 <img :src="methodIcon(method.type)" :alt="methodLabel(method)" class="method-option-icon" />
 <span data-testid="payment-method-label" class="method-option-label">
 {{ methodLabel(method) }}
 </span>
 <span v-if="method.fee_rate > 0" class="method-option-fee">
 +{{ method.fee_rate }}%
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

function methodTitle(method: PaymentMethodOption): string {
 if (method.fee_rate > 0) {
 return `${methodLabel(method)} · ${t('payment.fee')} ${method.fee_rate}%`
 }
 return methodLabel(method)
}
</script>

<style scoped>
.method-row {
 display: flex;
 flex-wrap: wrap;
 gap: 8px;
}

.method-option {
 display: flex;
 height: 32px;
 align-items: center;
 gap: 6px;
 padding: 0 12px;
 border-radius: var(--radius-field);
 border: 1px solid var(--border);
 background: color-mix(in oklch, var(--surface) 85%, transparent);
 cursor: pointer;
 transition: transform 0.15s ease, box-shadow 0.15s ease, background 0.15s ease;
}

.method-option:hover:not(.method-option--disabled) {
 transform: translateY(-1px);
}

.method-option--active {
 border-color: var(--accent);
 background: color-mix(in oklch, var(--accent) 12%, transparent);
 box-shadow: 0 0 0 1px color-mix(in oklch, var(--accent) 40%, transparent);
}

.method-option--disabled {
 cursor: not-allowed;
 opacity: 0.5;
}

.method-option-icon {
 height: 16px;
 width: 16px;
 flex-shrink: 0;
 object-fit: contain;
}

.method-option-label {
 white-space: nowrap;
 font-size: 12.5px;
 font-weight: 600;
 color: var(--foreground);
}

.method-option-fee {
 font-size: 10.5px;
 letter-spacing: 0.02em;
 color: var(--muted);
}
</style>
