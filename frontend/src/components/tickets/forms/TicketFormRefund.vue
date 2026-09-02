<template>
 <div class="space-y-4">
 <div class="grid gap-4 md:grid-cols-2">
 <div>
 <label class="input-label">{{ t('tickets.fields.orderNo') }}</label>
 <input :value="stringValue('order_no')" :readonly="readonly" class="input" @input="updateField('order_no', ($event.target as HTMLInputElement).value)" />
 </div>
 <div>
 <label class="input-label">{{ t('tickets.fields.expectedAmount') }}</label>
 <input :value="refundAmount" :readonly="readonly" class="input" @input="updateField('refund_amount', ($event.target as HTMLInputElement).value)" />
 </div>
 </div>
 <div>
 <label class="input-label">{{ t('tickets.fields.reason') }}</label>
 <textarea :value="stringValue('reason')" :readonly="readonly" class="input min-h-[120px]" @input="updateField('reason', ($event.target as HTMLTextAreaElement).value)" />
 </div>
 <div>
 <label class="input-label">{{ t('tickets.fields.evidence') }}</label>
 <textarea :value="stringValue('evidence')" :readonly="readonly" class="input min-h-[90px]" @input="updateField('evidence', ($event.target as HTMLTextAreaElement).value)" />
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ modelValue: Record<string, unknown>; readonly?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()

function stringValue(key: string) {
 return String(props.modelValue?.[key] ?? '')
}

const refundAmount = computed(() => stringValue('refund_amount') || stringValue('expected_amount'))

function updateField(key: string, value: string) {
 emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>
