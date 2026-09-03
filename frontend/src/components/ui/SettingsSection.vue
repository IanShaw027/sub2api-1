<template>
  <GlassCard variant="glass" padding="sm" class="ui-settings-section">
    <header v-if="title || description || $slots.header" class="ui-settings-section-header">
      <slot name="header">
        <h2 v-if="title" class="ui-settings-section-title">{{ title }}</h2>
        <p v-if="description" class="ui-settings-section-description">{{ description }}</p>
      </slot>
    </header>
    <div class="ui-settings-section-rows">
      <slot />
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'

defineProps<{
  title?: string
  description?: string
}>()
</script>

<style scoped>
.ui-settings-section.ui-glass-card-pad-sm {
  padding: 0;
  overflow: hidden;
}

.ui-settings-section + .ui-settings-section {
  margin-top: 12px;
}

.ui-settings-section-header {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
}

.ui-settings-section-title {
  font-family: var(--font-body);
  font-size: 15px;
  font-weight: 600;
  letter-spacing: 0;
  color: var(--foreground);
}

.ui-settings-section-description {
  font-size: 12.5px;
  color: var(--muted);
}

.ui-settings-section-rows {
  display: flex;
  flex-direction: column;
}
</style>
