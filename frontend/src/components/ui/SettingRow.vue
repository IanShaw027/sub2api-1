<template>
  <div class="ui-setting-row">
    <div class="ui-setting-row-label">
      <p class="ui-setting-row-title">{{ label }}</p>
      <p v-if="description" class="ui-setting-row-description">{{ description }}</p>
    </div>
    <div class="ui-setting-row-control">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  description?: string
}>()
</script>

<style scoped>
.ui-setting-row {
  display: grid;
  grid-template-columns: minmax(0, 240px) minmax(0, 1fr);
  gap: 16px;
  align-items: center;
  padding: 14px 0;
  border-bottom: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
}

.ui-setting-row:last-child {
  border-bottom: 0;
}

.ui-setting-row-title {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.ui-setting-row-description {
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}

@media (max-width: 767px) {
  .ui-setting-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
