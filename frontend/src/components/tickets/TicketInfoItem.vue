<template>
 <div class="min-w-0 rounded-xl bg-surface-2 px-3 py-3">
 <p class="text-xs font-medium uppercase tracking-wide text-muted">{{ label }}</p>
 <p class="mt-1 break-words text-sm font-medium text-foreground">{{ value }}</p>
 </div>
</template>

<script setup lang="ts">
defineProps<{
 label: string
 value: string
}>()
</script>
