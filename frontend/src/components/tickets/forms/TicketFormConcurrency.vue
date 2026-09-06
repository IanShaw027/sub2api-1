<template>
  <div class="space-y-4">
    <div class="grid gap-4 md:grid-cols-2">
      <TextInput
        :label="t('tickets.fields.currentConcurrency')"
        :model-value="stringValue('current_concurrency')"
        readonly
      />
      <TextInput
        :label="t('tickets.fields.targetConcurrency')"
        :model-value="stringValue('target_concurrency')"
        :readonly="readonly"
        @update:model-value="updateField('target_concurrency', String($event))"
      />
    </div>
    <TextArea
      :label="t('tickets.fields.usageScenario')"
      :model-value="stringValue('usage_scenario')"
      :readonly="readonly"
      :rows="5"
      @update:model-value="updateField('usage_scenario', $event)"
    />
    <TextInput
      :label="t('tickets.fields.peakWindow')"
      :model-value="stringValue('peak_window')"
      :readonly="readonly"
      @update:model-value="updateField('peak_window', String($event))"
    />
  </div>
</template>

<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import TextInput from '@/components/ui/TextInput.vue'
import TextArea from '@/components/common/TextArea.vue'

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
