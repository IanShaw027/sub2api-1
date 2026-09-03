<template>
  <div class="segmented" :class="{ 'segmented-sm': size === 'sm' }" role="radiogroup" @keydown="onKeydown">
    <button
      v-for="option in options"
      :key="option.value"
      type="button"
      role="radio"
      class="segmented-item"
      :class="{ 'segmented-item-active': option.value === modelValue }"
      :aria-checked="option.value === modelValue"
      :tabindex="option.value === modelValue ? 0 : -1"
      :disabled="option.disabled"
      @click="select(option)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import type { SegmentedOption } from './types'

const props = withDefaults(
  defineProps<{
    modelValue: T
    options: SegmentedOption<T>[]
    size?: 'md' | 'sm'
  }>(),
  {
    size: 'md'
  }
)

const emit = defineEmits<{
  'update:modelValue': [value: T]
}>()

function select(option: SegmentedOption<T>) {
  if (option.disabled) return
  emit('update:modelValue', option.value)
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
  const enabled = props.options.filter((option) => !option.disabled)
  if (enabled.length === 0) return
  event.preventDefault()
  const currentIndex = enabled.findIndex((option) => option.value === props.modelValue)
  const delta = event.key === 'ArrowRight' ? 1 : -1
  const nextIndex = (currentIndex + delta + enabled.length) % enabled.length
  emit('update:modelValue', enabled[nextIndex].value)
}
</script>
