<template>
  <div class="space-y-4">
    <div class="grid gap-4 md:grid-cols-2">
      <div>
        <label class="input-label">{{ t('tickets.fields.currentRate') }}</label>
        <input :value="stringValue('current_rate')" :readonly="readonly" class="input" @input="updateField('current_rate', ($event.target as HTMLInputElement).value)" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.fields.targetRate') }}</label>
        <input :value="stringValue('target_rate')" :readonly="readonly" class="input" @input="updateField('target_rate', ($event.target as HTMLInputElement).value)" />
      </div>
    </div>
    <div>
      <label class="input-label">{{ t('tickets.fields.targetScope') }}</label>
      <input :value="stringValue('target_scope')" :readonly="readonly" class="input" @input="updateField('target_scope', ($event.target as HTMLInputElement).value)" />
    </div>
    <div>
      <label class="input-label">{{ t('tickets.fields.usageScenario') }}</label>
      <textarea :value="stringValue('usage_scenario')" :readonly="readonly" class="input min-h-[120px]" @input="updateField('usage_scenario', ($event.target as HTMLTextAreaElement).value)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

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
