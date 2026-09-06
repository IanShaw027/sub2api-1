<template>
  <div class="dashboard-page-layout">
    <div v-if="$slots.header" class="dashboard-page-layout-header">
      <slot name="header" />
    </div>
    <div v-if="$slots.stats" class="dashboard-page-layout-stats">
      <slot name="stats" />
    </div>
    <div v-if="$slots.charts" class="dashboard-page-layout-charts">
      <slot name="charts" />
    </div>
    <div v-if="$slots.lists" class="dashboard-page-layout-lists">
      <slot name="lists" />
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
/**
 * DashboardPage skeleton (`/dashboard`, `/admin/dashboard`, `/admin/ops`, `/monitor`).
 * Page content padding (`8px 24px 24px 20px`) already comes from
 * `AppLayout`'s `.app-shell-content` — this component only owns the
 * vertical section rhythm (`#header #stats #charts #lists`, gap 14).
 */
</script>

<style scoped>
.dashboard-page-layout {
  display: flex;
  flex-direction: column;
  gap: 14px;
}
</style>
