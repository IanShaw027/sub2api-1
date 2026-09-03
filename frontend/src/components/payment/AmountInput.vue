<template>
 <div class="space-y-3">
 <!-- Quick Amount Buttons -->
 <div>
 <label class="input-label">
 {{ t('payment.quickAmounts') }}
 </label>
 <div class="quick-amounts">
 <button
 v-for="amt in filteredAmounts"
 :key="amt"
 type="button"
 class="chip-filter num"
 :class="{ 'is-active': modelValue === amt }"
 @click="selectAmount(amt)"
 >
 {{ amt }}
 </button>
 </div>
 </div>

 <!-- Custom Amount Input -->
 <div>
 <label class="input-label">
 {{ t('payment.customAmount') }}
 </label>
 <div class="amount-field">
 <span class="amount-prefix">$</span>
 <input
 type="text"
 inputmode="decimal"
 :value="customText"
 :placeholder="placeholderText"
 class="field input-lg amount-field-input num"
 @input="handleInput"
 />
 </div>
 </div>
 </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
 amounts?: number[]
 modelValue: number | null
 min?: number
 max?: number
}>(), {
 amounts: () => [10, 20, 50, 100, 200, 500, 1000, 2000, 5000],
 min: 0,
 max: 0,
})

const emit = defineEmits<{
 'update:modelValue': [value: number | null]
}>()

const { t } = useI18n()

const customText = ref('')

// 0 = no limit
const filteredAmounts = computed(() =>
 props.amounts.filter((a) => (props.min <= 0 || a >= props.min) && (props.max <= 0 || a <= props.max))
)

const placeholderText = computed(() => {
 if (props.min > 0 && props.max > 0) return `${props.min} - ${props.max}`
 if (props.min > 0) return `≥ ${props.min}`
 if (props.max > 0) return `≤ ${props.max}`
 return t('payment.enterAmount')
})

const AMOUNT_PATTERN = /^\d*(\.\d{0,2})?$/

function selectAmount(amt: number) {
 customText.value = String(amt)
 emit('update:modelValue', amt)
}

function handleInput(e: Event) {
 const val = (e.target as HTMLInputElement).value
 if (!AMOUNT_PATTERN.test(val)) return
 customText.value = val
 if (val === '') {
 emit('update:modelValue', null)
 return
 }
 const num = parseFloat(val)
 if (!isNaN(num) && num > 0) {
 emit('update:modelValue', num)
 } else {
 emit('update:modelValue', null)
 }
}

watch(() => props.modelValue, (v) => {
 if (v !== null && String(v) !== customText.value) {
 customText.value = String(v)
 }
}, { immediate: true })
</script>

<style scoped>
.quick-amounts {
 display: flex;
 flex-wrap: wrap;
 gap: 8px;
}

.quick-amounts .chip-filter {
 height: 28px;
}

.amount-field {
 position: relative;
}

.amount-prefix {
 position: absolute;
 left: 12px;
 top: 50%;
 transform: translateY(-50%);
 font-size: 13px;
 color: var(--muted);
 pointer-events: none;
}

.amount-field-input {
 padding-left: 26px;
 width: 100%;
}
</style>
