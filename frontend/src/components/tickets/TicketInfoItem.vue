<template>
  <div class="min-w-0 rounded-xl bg-gray-50 px-3 py-3 dark:bg-dark-700/40">
    <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ label }}</p>
    <p class="mt-1 break-words text-sm font-medium text-gray-900 dark:text-white">{{ value }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  value: string
}>()
</script>
