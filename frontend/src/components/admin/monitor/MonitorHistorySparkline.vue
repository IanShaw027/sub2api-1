<template>
  <svg class="monitor-spark" viewBox="0 0 100 28" preserveAspectRatio="none" aria-hidden="true">
    <path class="monitor-spark-area" :d="path.area" :class="toneClass" />
    <path class="monitor-spark-line" :d="path.line" :class="toneClass" />
  </svg>
</template>

<script setup lang="ts">
/**
 * 96x28 latency sparkline for a single monitor model's check history.
 * Geometry mirrors components/admin/dashboard/DashSparkline.vue's
 * buildSpark() algorithm (replicated, not imported — that helper is
 * private to the admin dashboard view and out of this task's scope).
 */
import { computed } from 'vue'

const props = defineProps<{
  /** Latency samples in chronological order (oldest first), ms. Nulls are dropped. */
  values: (number | null)[]
  /** Tone drives the stroke/fill color; mirrors StatusBadgeTone semantics. */
  tone?: 'success' | 'warning' | 'danger' | 'muted'
}>()

const SPARK_W = 100
const SPARK_H = 28

const path = computed(() => {
  const points = props.values.filter((v): v is number => v != null)
  const series = points.length >= 2 ? points : [0, 0]
  const min = Math.min(...series)
  const max = Math.max(...series)
  const span = max - min
  const coords = series.map((value, index) => {
    const ratio = span > 0 ? (value - min) / span : 0.5
    const norm = 0.18 + 0.82 * ratio
    return [(index * SPARK_W) / (series.length - 1), SPARK_H - norm * (SPARK_H - 4)] as const
  })
  const line = `M${coords.map(([x, y]) => `${x.toFixed(1)},${y.toFixed(1)}`).join(' L')}`
  return { line, area: `${line} L${SPARK_W},${SPARK_H} L0,${SPARK_H} Z` }
})

const toneClass = computed(() => `monitor-spark-${props.tone ?? 'muted'}`)
</script>

<style scoped>
.monitor-spark {
  display: block;
  width: 100%;
  height: 100%;
  overflow: visible;
}

.monitor-spark-line {
  fill: none;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}

.monitor-spark-area.monitor-spark-success { fill: color-mix(in oklch, var(--success) 14%, transparent); }
.monitor-spark-area.monitor-spark-warning { fill: color-mix(in oklch, var(--warning) 14%, transparent); }
.monitor-spark-area.monitor-spark-danger { fill: color-mix(in oklch, var(--danger) 14%, transparent); }
.monitor-spark-area.monitor-spark-muted { fill: color-mix(in oklch, var(--muted) 14%, transparent); }

.monitor-spark-line.monitor-spark-success { stroke: var(--success); }
.monitor-spark-line.monitor-spark-warning { stroke: var(--warning); }
.monitor-spark-line.monitor-spark-danger { stroke: var(--danger); }
.monitor-spark-line.monitor-spark-muted { stroke: var(--muted); }
</style>
