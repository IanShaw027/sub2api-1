<script setup lang="ts">
import Select from '@/components/common/Select.vue'

defineOptions({ inheritAttrs: false })

withDefaults(
  defineProps<{
    modelValue?: string | number | null
    options: Array<{ label: string; value: string | number; disabled?: boolean }>
    placeholder?: string
    disabled?: boolean
    searchable?: boolean
  }>(),
  {
    disabled: false,
    searchable: false
  }
)

defineEmits<{
  'update:modelValue': [value: string | number | null]
}>()
</script>

<template>
  <Select
    v-bind="$attrs"
    :model-value="modelValue"
    :options="options"
    :placeholder="placeholder"
    :disabled="disabled"
    :searchable="searchable"
    class="ui-select"
    @update:model-value="$emit('update:modelValue', $event as string | number | null)"
  />
</template>

<style scoped>
.ui-select :deep(.select-trigger),
.ui-select :deep(button) {
  min-height: 36px;
  border-radius: var(--radius-field);
}
</style>
