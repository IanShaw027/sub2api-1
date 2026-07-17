<template>
  <label class="flex cursor-pointer flex-col items-center gap-0.5">
    <span class="whitespace-nowrap text-xs text-ink-soft">{{ label }}</span>
    <button
      type="button"
      role="switch"
      :aria-checked="checked"
      @click="emit('toggle')"
      :class="[
        'relative inline-flex h-5 w-9 shrink-0 rounded-full border-2 border-transparent transition-colors duration-150',
        checked ? 'bg-brand-500' : 'bg-line dark:bg-dark-600',
      ]"
    >
      <span :class="[
        'pointer-events-none inline-block h-4 w-4 rounded-full bg-white shadow-xs transition-transform duration-150',
        checked ? 'translate-x-4' : 'translate-x-0',
      ]" />
    </button>
  </label>
</template>

<script setup lang="ts">
defineProps<{ label: string; checked: boolean }>()
const emit = defineEmits<{ toggle: [] }>()
</script>
