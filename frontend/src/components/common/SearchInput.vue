<template>
  <div class="relative w-full">
    <div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3">
      <Icon name="search" size="md" class="text-ink-faint dark:text-dark-400" />
    </div>
    <input
      :value="modelValue"
      type="text"
      class="input pl-10"
      :placeholder="placeholderText"
      @input="handleInput"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  modelValue: string
  placeholder?: string
  debounceMs?: number
}>(), {
  debounceMs: 300
})

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void
  (e: 'search', value: string): void
}>()

let searchTimer: ReturnType<typeof setTimeout> | null = null

const placeholderText = computed(() => props.placeholder ?? t('common.searchPlaceholder'))

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
