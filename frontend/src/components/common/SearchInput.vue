<template>
  <div class="relative w-full">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
      <Icon name="search" size="md" class="text-gray-400" />
    </div>
    <input
      :value="modelValue"
      type="text"
      class="input pl-10"
      :placeholder="placeholder"
      @input="handleInput"
    />
  </div>
</template>

<script setup lang="ts">
import { onUnmounted } from 'vue'
import Icon from '@/components/icons/Icon.vue'

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

let searchTimer: ReturnType<typeof setTimeout> | null = null

const clearSearchTimer = () => {
  if (searchTimer !== null) {
    clearTimeout(searchTimer)
    searchTimer = null
  }
}

const handleInput = (event: Event) => {
  const value = (event.target as HTMLInputElement).value
  emit('update:modelValue', value)
  clearSearchTimer()
  searchTimer = setTimeout(() => {
    searchTimer = null
    emit('search', value)
  }, props.debounceMs)
}

onUnmounted(() => {
  clearSearchTimer()
})
</script>
