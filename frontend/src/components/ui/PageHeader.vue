<template>
  <header :class="['ui-page-header', variant === 'hero' ? 'ui-page-header-hero' : 'ui-page-header-compact']">
    <div class="ui-page-header-main">
      <p v-if="eyebrow" class="ui-page-header-eyebrow">{{ eyebrow }}</p>
      <h1 class="ui-page-header-title">{{ title }}</h1>
      <p v-if="subtitle" class="ui-page-header-subtitle">{{ subtitle }}</p>
    </div>
    <div v-if="$slots.actions" class="ui-page-header-actions">
      <slot name="actions" />
    </div>
  </header>
</template>

<script setup lang="ts">
import type { PageHeaderVariant } from './types'

withDefaults(
  defineProps<{
    title: string
    subtitle?: string
    eyebrow?: string
    variant?: PageHeaderVariant
  }>(),
  {
    variant: 'compact'
  }
)
</script>

<style scoped>
.ui-page-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 16px;
  margin-bottom: 16px;
}

.ui-page-header-compact .ui-page-header-title {
  font-size: 20px;
  font-weight: 800;
  line-height: 1.2;
  color: var(--foreground);
}

.ui-page-header-hero .ui-page-header-title {
  font-size: 28px;
  font-weight: 800;
  line-height: 1.15;
  color: var(--foreground);
}

.ui-page-header-eyebrow {
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
  margin-bottom: 6px;
}

.ui-page-header-subtitle {
  margin-top: 6px;
  font-size: 13px;
  color: var(--muted);
}

.ui-page-header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: none;
}
</style>
