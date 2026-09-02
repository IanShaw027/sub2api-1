<template>
  <div class="segmented" role="tablist">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="tab"
      class="segmented-item"
      :class="{ 'segmented-item-active': option.value === modelValue }"
      :aria-selected="option.value === modelValue"
      :disabled="option.disabled"
      @click="select(option.value)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import type { SegmentedOption } from './types'

defineProps<{
  modelValue: T
  options: SegmentedOption<T>[]
}>()

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()

function select(value: T) {
  emit('update:modelValue', value)
}
</script>
