<template>
  <div class="space-y-4">
    <div class="grid gap-4 md:grid-cols-2">
      <div>
        <label class="input-label">{{ t('tickets.fields.currentConcurrency') }}</label>
        <input :value="stringValue('current_concurrency')" readonly class="input" />
      </div>
      <div>
        <label class="input-label">{{ t('tickets.fields.targetConcurrency') }}</label>
        <input :value="stringValue('target_concurrency')" :readonly="readonly" class="input" @input="updateField('target_concurrency', ($event.target as HTMLInputElement).value)" />
      </div>
    </div>
    <div>
      <label class="input-label">{{ t('tickets.fields.usageScenario') }}</label>
      <textarea :value="stringValue('usage_scenario')" :readonly="readonly" class="input min-h-[120px]" @input="updateField('usage_scenario', ($event.target as HTMLTextAreaElement).value)" />
    </div>
    <div>
      <label class="input-label">{{ t('tickets.fields.peakWindow') }}</label>
      <input :value="stringValue('peak_window')" :readonly="readonly" class="input" @input="updateField('peak_window', ($event.target as HTMLInputElement).value)" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{ modelValue: Record<string, unknown>; readonly?: boolean; userConcurrency?: number | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: Record<string, unknown>] }>()
const { t } = useI18n()

function stringValue(key: string) {
  return String(props.modelValue?.[key] ?? '')
}

function updateField(key: string, value: string) {
  emit('update:modelValue', { ...props.modelValue, [key]: value })
}

watch(
  () => [props.readonly, props.userConcurrency, props.modelValue?.current_concurrency] as const,
  ([readonly, userConcurrency, currentValue]) => {
    if (readonly || userConcurrency == null || String(currentValue ?? '').trim()) {
      return
    }
    emit('update:modelValue', {
      ...props.modelValue,
      current_concurrency: String(userConcurrency),
    })
  },
  { immediate: true },
)
</script>
