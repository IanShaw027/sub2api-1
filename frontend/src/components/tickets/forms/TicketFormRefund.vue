<template>
  <div class="space-y-4">
    <div class="grid gap-4 md:grid-cols-2">
      <TextInput
        :label="t('tickets.fields.orderNo')"
        :model-value="stringValue('order_no')"
        :readonly="readonly"
        @update:model-value="updateField('order_no', String($event))"
      />
      <TextInput
        :label="t('tickets.fields.expectedAmount')"
        :model-value="refundAmount"
        :readonly="readonly"
        @update:model-value="updateField('refund_amount', String($event))"
      />
    </div>
    <TextArea
      :label="t('tickets.fields.reason')"
      :model-value="stringValue('reason')"
      :readonly="readonly"
      :rows="5"
      @update:model-value="updateField('reason', $event)"
    />
    <TextArea
      :label="t('tickets.fields.evidence')"
      :model-value="stringValue('evidence')"
      :readonly="readonly"
      :rows="4"
      @update:model-value="updateField('evidence', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import TextInput from '@/components/ui/TextInput.vue'
import TextArea from '@/components/common/TextArea.vue'

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
