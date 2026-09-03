<template>
  <svg class="dash-spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
    <path class="dash-spark-area" :d="path.area" />
    <path class="dash-spark-line" :d="path.line" />
  </svg>
</template>

<script setup lang="ts">
/**
 * 96×28 area sparkline used by the admin dashboard stat cards.
 * Geometry is pre-computed by the view; this component only paints it.
 */
import type { SparkPath } from './types'

defineProps<{ path: SparkPath }>()
</script>

<style scoped>
.dash-spark {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.dash-spark-area {
  fill: color-mix(in oklch, var(--accent) 14%, transparent);
}

.dash-spark-line {
  fill: none;
  stroke: var(--accent);
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
</style>
