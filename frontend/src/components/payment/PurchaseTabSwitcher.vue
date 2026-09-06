<template>
  <div class="purchase-tabs">
    <button
      v-for="tab in tabs"
      :key="tab.key"
      type="button"
      class="purchase-tab"
      :class="{ 'purchase-tab-active': modelValue === tab.key }"
      @click="$emit('update:modelValue', tab.key)"
    >
      {{ tab.label }}
    </button>
  </div>
</template>

<script setup lang="ts" generic="K extends string">
defineProps<{
  tabs: { key: K; label: string }[]
  modelValue: K
}>()

defineEmits<{
  'update:modelValue': [value: K]
}>()
</script>

<style scoped>
.purchase-tabs {
  display: flex;
  gap: 4px;
  border-radius: var(--radius-hero);
  background: var(--surface-secondary);
  padding: 4px;
}

.purchase-tab {
  flex: 1;
  border-radius: var(--radius-field);
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 500;
  color: var(--muted);
  transition: all 0.15s ease;
}

.purchase-tab-active {
  background: var(--surface);
  color: var(--foreground);
  box-shadow: var(--shadow);
}
</style>
