<template>
  <GlassCard variant="glass" padding="lg" class="ui-settings-section">
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
.ui-settings-section + .ui-settings-section {
  margin-top: 16px;
}

.ui-settings-section-header {
  margin-bottom: 4px;
}

.ui-settings-section-title {
  font-size: 16px;
  font-weight: 800;
  color: var(--foreground);
}

.ui-settings-section-description {
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}

.ui-settings-section-rows {
  display: flex;
  flex-direction: column;
}
</style>
