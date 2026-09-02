<template>
  <div class="ui-chip-scroller" role="tablist">
    <button
      v-for="chip in chips"
      :key="chip.value"
      type="button"
      role="tab"
      class="ui-chip"
      :class="{ 'ui-chip-active': chip.value === modelValue }"
      :aria-selected="chip.value === modelValue"
      :disabled="chip.disabled"
      @click="$emit('update:modelValue', chip.value)"
    >
      {{ chip.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="T extends string">
import type { SegmentedOption } from './types'

defineProps<{
  modelValue: T
  chips: SegmentedOption<T>[]
}>()

defineEmits<{
  'update:modelValue': [value: T]
}>()
</script>

<style scoped>
.ui-chip-scroller {
  display: flex;
  gap: 8px;
  overflow-x: auto;
  padding-bottom: 2px;
  scrollbar-width: none;
}

.ui-chip-scroller::-webkit-scrollbar {
  display: none;
}

.ui-chip {
  flex: none;
  height: 30px;
  padding: 0 12px;
  border-radius: 10px;
  border: 1px solid var(--border);
  background: color-mix(in oklch, var(--surface) 85%, transparent);
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease, color 0.15s ease, border-color 0.15s ease;
}

.ui-chip-active {
  background: var(--foreground);
  color: var(--background);
  border-color: var(--foreground);
}

.ui-chip:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
</style>
