<template>
  <span class="cell-mono"><slot>{{ value }}</slot></span>
</template>

<script setup lang="ts">
withDefaults(
  defineProps<{
    value?: string | number | null
  }>(),
  {
    value: null
  }
)
</script>

<style scoped>
.cell-mono {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
