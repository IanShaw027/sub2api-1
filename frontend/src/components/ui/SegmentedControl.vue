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
      :tabindex="option.value === tabStopValue ? 0 : -1"
      :disabled="option.disabled"
      @click="select(option)"
    >
      {{ option.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import { computed, nextTick } from 'vue'
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

const enabledOptions = computed(() => props.options.filter((option) => !option.disabled))
const tabStopValue = computed(() =>
  enabledOptions.value.find((option) => option.value === props.modelValue)?.value
  ?? enabledOptions.value[0]?.value
)

function select(option: SegmentedOption<T>) {
  if (option.disabled) return
  emit('update:modelValue', option.value)
}

function onKeydown(event: KeyboardEvent) {
  if (!['ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) return
  const enabled = enabledOptions.value
  if (enabled.length === 0) return
  const root = event.currentTarget as HTMLElement
  const buttons = Array.from(root.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'))
  const focusedIndex = buttons.indexOf(event.target as HTMLButtonElement)
  const currentIndex = focusedIndex >= 0
    ? focusedIndex
    : Math.max(0, enabled.findIndex((option) => option.value === props.modelValue))
  event.preventDefault()
  const delta = event.key === 'ArrowRight' || event.key === 'ArrowDown' ? 1 : -1
  const nextIndex = event.key === 'Home' ? 0
    : event.key === 'End' ? enabled.length - 1
    : (currentIndex + delta + enabled.length) % enabled.length
  emit('update:modelValue', enabled[nextIndex].value)
  void nextTick(() => buttons[nextIndex]?.focus())
}
</script>
