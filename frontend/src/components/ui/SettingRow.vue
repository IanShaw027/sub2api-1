<template>
  <div class="ui-setting-row">
    <div class="ui-setting-row-label">
      <div :id="titleId" class="ui-setting-row-title">
        <slot name="label">
          <component :is="labelFor ? 'label' : 'span'" :for="labelFor">{{ label }}</component>
        </slot>
      </div>
      <div v-if="description || $slots.description" class="ui-setting-row-description">
        <slot name="description">{{ description }}</slot>
      </div>
    </div>
    <div class="ui-setting-row-control" :aria-labelledby="titleId">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useId } from 'vue'

defineProps<{
  label?: string
  labelFor?: string
  description?: string
}>()

const titleId = `ui-setting-row-${useId()}`
</script>

<style scoped>
.ui-setting-row {
  display: grid;
  grid-template-columns: 240px minmax(0, 1fr);
  gap: 12px 24px;
  align-items: center;
  padding: 14px 20px;
  border-bottom: 1px solid var(--border);
}

.ui-setting-row:last-child {
  border-bottom: 0;
}

.ui-setting-row-title {
  font-size: 13px;
  line-height: 19px;
  font-weight: 600;
  color: var(--foreground);
}

.ui-setting-row-description {
  margin-top: 2px;
  font-size: 12px;
  line-height: 17px;
  color: var(--muted);
}

.ui-setting-row-control {
  min-width: 0;
}

.ui-setting-row-control :deep(.field),
.ui-setting-row-control :deep(.input) {
  max-width: 420px;
}

.ui-setting-row-control :deep(input[type="number"].input),
.ui-setting-row-control :deep(input[type="number"].field) {
  max-width: 120px;
}

.ui-setting-row-control :deep(textarea.input),
.ui-setting-row-control :deep(textarea.field) {
  max-width: 100%;
}

@media (max-width: 900px) {
  .ui-setting-row {
    grid-template-columns: 1fr;
    gap: 8px;
  }
}
</style>
