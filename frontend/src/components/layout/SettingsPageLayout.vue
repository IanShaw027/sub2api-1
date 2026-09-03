<template>
  <div class="settings-page-layout">
    <div v-if="$slots.header" class="settings-page-layout-header">
      <slot name="header" />
    </div>
    <div class="settings-page-layout-grid">
      <div v-if="$slots.nav" class="settings-page-layout-nav">
        <slot name="nav" />
      </div>
      <div class="settings-page-layout-content">
        <slot name="content" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * SettingsPage skeleton (`/admin/settings`, `/profile`).
 * `#header` on top, then a `224px 1fr` grid (`#nav` section list + `#content`
 * settings cards), stacking nav above content below 768px.
 */
</script>

<style scoped>
.settings-page-layout {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.settings-page-layout-grid {
  display: grid;
  grid-template-columns: 224px minmax(0, 1fr);
  gap: 14px;
  align-items: start;
}

.settings-page-layout-content {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 12px;
}

@media (max-width: 767px) {
  .settings-page-layout-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
