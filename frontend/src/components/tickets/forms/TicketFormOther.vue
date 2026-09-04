<template>
  <div class="space-y-4">
    <TextArea
      :label="t('tickets.fields.details')"
      :model-value="stringValue('details')"
      :readonly="readonly"
      :rows="6"
      @update:model-value="updateField('details', $event)"
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
