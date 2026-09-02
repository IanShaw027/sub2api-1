<template>
  <section class="ui-settings-section">
    <header v-if="title || description" class="ui-settings-section-header">
      <h2 v-if="title" class="ui-settings-section-title">{{ title }}</h2>
      <p v-if="description" class="ui-settings-section-description">{{ description }}</p>
    </header>
    <div class="ui-settings-section-rows">
      <slot />
    </div>
  </section>
</template>

<script setup lang="ts">
defineProps<{
  title?: string
  description?: string
}>()
</script>

<style scoped>
.ui-settings-section + .ui-settings-section {
  margin-top: 24px;
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
  margin-top: 12px;
  display: flex;
  flex-direction: column;
  gap: 0;
}
</style>
