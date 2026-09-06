<template>
  <div class="detail-page-layout">
    <div v-if="$slots.header" class="detail-page-layout-header">
      <slot name="header" />
    </div>
    <div class="detail-page-layout-grid">
      <div class="detail-page-layout-main">
        <slot name="main" />
        <div v-if="$slots.composer" class="detail-page-layout-composer">
          <slot name="composer" />
        </div>
      </div>
      <div v-if="$slots.side" class="detail-page-layout-side">
        <slot name="side" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * DetailPage skeleton (`/invoices/:id`, `/tickets/:id`, `/admin/tickets/:id`, …).
 * `#header` on top, then a `1fr 320px` grid (`#main` timeline + `#side` meta
 * card), collapsing to a single column below 1024px. `#composer` sits at the
 * bottom of `#main` when present (reply / message input area).
 */
</script>

<style scoped>
.detail-page-layout {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.detail-page-layout-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 320px;
  gap: 14px;
  align-items: start;
}

.detail-page-layout-main {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 14px;
}

@media (max-width: 1023px) {
  .detail-page-layout-grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
