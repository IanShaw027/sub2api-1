<template>
  <div class="min-w-0 rounded-xl bg-page px-3 py-3 dark:bg-dark-700/40">
    <p class="text-xs font-medium uppercase tracking-wide text-ink-soft">{{ label }}</p>
    <p class="mt-1 break-words text-sm font-medium text-ink dark:text-white">{{ value }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  value: string
}>()
</script>
