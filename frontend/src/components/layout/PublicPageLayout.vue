<template>
  <div class="public-page-layout public-page">
    <header v-if="$slots.nav" class="public-page-layout-nav">
      <slot name="nav" />
    </header>
    <div class="public-page-layout-content">
      <slot />
    </div>
  </div>
</template>

<script setup lang="ts">
/**
 * Skeleton for LandingLayout / Auth / Callback / Setup / NotFound pages.
 * Colour + surface come from the shared `.public-page` class (`--bg-public`);
 * this component only owns the centred content column and optional top nav.
 */
</script>

<style scoped>
.public-page-layout {
  display: flex;
  flex-direction: column;
  min-height: 100vh;
}

.public-page-layout-nav {
  flex: none;
}

.public-page-layout-content {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 100%;
}
</style>
