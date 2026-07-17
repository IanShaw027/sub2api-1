<template>
  <div>
    <label class="mb-2 block text-sm font-medium text-ink dark:text-dark-300">
      {{ t('payment.paymentMethod') }}
    </label>
    <div class="grid grid-cols-2 gap-3 sm:flex">
      <button
        v-for="method in sortedMethods"
        :key="method.type"
        type="button"
        :disabled="!method.available"
        :class="[
          'relative flex h-[60px] flex-col items-center justify-center rounded-control border px-3 transition-all duration-150 sm:flex-1',
          !method.available
            ? 'cursor-not-allowed border-line bg-page opacity-50 dark:border-dark-700 dark:bg-dark-800/50'
            : selected === method.type
              ? methodSelectedClass(method.type)
              : 'border-line bg-card text-ink-body hover:border-divider dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:border-dark-500',
        ]"
        @click="method.available && emit('select', method.type)"
      >
        <span class="flex items-center gap-2">
          <img :src="methodIcon(method.type)" :alt="t(paymentMethodDisplayKey(method.type), method.type)" class="h-7 w-7 object-contain" />
          <span class="flex flex-col items-start leading-none">
            <span class="text-base font-semibold">{{ t(paymentMethodDisplayKey(method.type), method.type) }}</span>
            <span
              v-if="method.fee_rate > 0"
              class="text-[10px] tracking-wide text-ink-soft dark:text-dark-400"
            >
              {{ t('payment.fee') }} {{ method.fee_rate }}%
            </span>
          </span>
        </span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { METHOD_ORDER } from './providerConfig'
import alipayIcon from '@/assets/icons/alipay.svg'
import wxpayIcon from '@/assets/icons/wxpay.svg'
import stripeIcon from '@/assets/icons/stripe.svg'
import airwallexIcon from '@/assets/icons/airwallex.svg'
import { paymentMethodDisplayKey } from '@/utils/i18n'

export interface PaymentMethodOption {
  type: string
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
  if (type.includes('alipay')) return METHOD_ICONS.alipay
  if (type.includes('wxpay')) return METHOD_ICONS.wxpay
  if (type === 'airwallex') return METHOD_ICONS.airwallex
  return METHOD_ICONS[type] || alipayIcon
}

function methodSelectedClass(type: string): string {
  if (type.includes('alipay')) return 'border-[#02A9F1] bg-accent-50 text-ink shadow-xs dark:bg-blue-950 dark:text-dark-100'
  if (type.includes('wxpay')) return 'border-[#09BB07] bg-green-50 text-ink shadow-xs dark:bg-green-950 dark:text-dark-100'
  if (type === 'stripe') return 'border-[#676BE5] bg-indigo-50 text-ink shadow-xs dark:bg-indigo-950 dark:text-dark-100'
  if (type === 'airwallex') return 'border-[#FF6B3D] bg-orange-50 text-ink shadow-xs dark:border-[#FF8E3C] dark:bg-orange-950 dark:text-dark-100'
  return 'border-brand-500 bg-brand-50 text-ink shadow-xs dark:bg-brand-950 dark:text-dark-100'
}
</script>
