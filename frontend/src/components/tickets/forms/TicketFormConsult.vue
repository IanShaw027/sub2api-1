<template>
  <div class="space-y-4">
    <TextArea
      :label="t('tickets.fields.question')"
      :model-value="stringValue('question')"
      :readonly="readonly"
      :rows="5"
      @update:model-value="updateField('question', $event)"
    />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import TextArea from '@/components/common/TextArea.vue'

const props = defineProps<{ modelValue: Record<string, unknown>; readonly?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()

function stringValue(key: string) {
  return String(props.modelValue?.[key] ?? '')
}

function updateField(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}
</script>
