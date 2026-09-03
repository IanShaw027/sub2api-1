<template>
  <div class="segmented" role="radiogroup">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="segmented-item"
      :class="{ 'segmented-item-active': option.value === modelValue }"
      :aria-checked="option.value === modelValue"
      :disabled="option.disabled"
      @click="select(option)"
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

function select(option: SegmentedOption<T>) {
  if (option.disabled) return
  emit('update:modelValue', option.value)
}
</script>
