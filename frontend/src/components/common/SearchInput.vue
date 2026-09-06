<template>
  <div class="search-input relative w-full">
    <span class="search-input-icon" aria-hidden="true">
      <svg
        width="15"
        height="15"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.8"
        stroke-linecap="round"
        stroke-linejoin="round"
      >
        <path d="M11 19a8 8 0 1 0 0-16 8 8 0 0 0 0 16zM21 21l-4.35-4.35" />
      </svg>
    </span>
    <input
      :value="modelValue"
      type="text"
      class="field search-input-field"
      :placeholder="placeholder"
      @input="handleInput"
    />
  </div>
</template>

<script setup lang="ts">
import { useDebounceFn } from '@vueuse/core'

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  debounceMs?: number
}>(), {
  placeholder: 'Search...',
  debounceMs: 300
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'search', value: string): void
}>()

const debouncedEmitSearch = useDebounceFn((value: string) => {
  emit('search', value)
}, props.debounceMs)

const handleInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
  debouncedEmitSearch(value)
}
</script>

<style scoped>
/* 36px `.field` with a 15px search glyph inset 12px from the left. */
.search-input-icon {
  position: absolute;
  left: 12px;
  top: 0;
  bottom: 0;
  display: inline-flex;
  align-items: center;
  color: var(--muted);
  pointer-events: none;
}

.search-input-field {
  padding-left: 36px;
}
</style>
