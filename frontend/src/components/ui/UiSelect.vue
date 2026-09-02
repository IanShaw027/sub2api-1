<script setup lang="ts">
import Select from '@/components/common/Select.vue'
import type { SelectOption } from '@/components/common/Select.vue'

defineOptions({ inheritAttrs: false })

defineProps<{
  modelValue?: string | number | boolean | null
  options: SelectOption[] | Array<Record<string, unknown>>
  placeholder?: string
  disabled?: boolean
  error?: boolean
  searchable?: boolean | 'auto'
}>()

defineEmits<{
  'update:modelValue': [value: string | number | boolean | null]
}>()
</script>

<template>
  <Select
    v-bind="$attrs"
    class="ui-select"
    :model-value="modelValue"
    :options="options"
    :placeholder="placeholder"
    :disabled="disabled"
    :error="error"
    :searchable="searchable"
    @update:model-value="$emit('update:modelValue', $event)"
  >
    <template v-for="(_, name) in $slots" #[name]="slotData">
      <slot :name="name" v-bind="slotData ?? {}" />
    </template>
  </Select>
</template>

<style scoped>
.ui-select :deep(.select-trigger) {
  height: 36px;
  min-height: 36px;
  padding: 0 12px;
  border-radius: var(--radius-field);
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  color: var(--foreground);
  font-size: 13px;
  box-shadow: var(--field-shadow);
}

.ui-select :deep(.select-trigger-open),
.ui-select :deep(.select-trigger:focus) {
  border-color: var(--accent);
  box-shadow: var(--field-shadow), 0 0 0 3px color-mix(in oklch, var(--accent) 18%, transparent);
}

.ui-select :deep(.select-trigger-error) {
  border-color: var(--danger);
}
</style>
