<template>
  <label :for="htmlFor" class="ui-field-label">
    <span class="ui-field-label-text">
      <slot />
      <span v-if="required" class="ui-field-label-required" aria-hidden="true">*</span>
    </span>
    <span v-if="hint" class="ui-field-label-hint">{{ hint }}</span>
  </label>
</template>

<script setup lang="ts">
defineProps<{
  htmlFor?: string
  hint?: string
  required?: boolean
}>()
</script>

<style scoped>
.ui-field-label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-bottom: 6px;
}

.ui-field-label-text {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
}

.ui-field-label-hint {
  font-size: 11px;
  color: var(--muted);
  font-weight: 500;
}

.ui-field-label-required {
  color: var(--danger);
  margin-left: 2px;
}
</style>
