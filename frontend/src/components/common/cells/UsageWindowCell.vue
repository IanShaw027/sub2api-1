<template>
  <div class="cell-usage-window">
    <div v-for="window in visibleWindows" :key="window.label" class="cell-usage-window-row">
      <span class="cell-usage-window-label">{{ window.label }}</span>
      <div class="progress progress-thin cell-usage-window-track">
        <div
          class="progress-bar"
          :class="toneClass(window.percent)"
          :style="{ width: `${clamp(window.percent)}%` }"
        />
      </div>
      <span class="cell-usage-window-value">{{ formatPercent(window.percent) }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
export interface UsageWindow {
  /** Short window label, e.g. "5h" or "7d". */
  label: string
  /** 0-100 utilization percentage. Null/undefined renders an empty "-" row. */
  percent: number | null | undefined
}

const props = withDefaults(
  defineProps<{
    windows: UsageWindow[]
    /** >= this value renders the warning tone (default 70). */
    warningThreshold?: number
    /** >= this value renders the danger tone (default 90). */
    dangerThreshold?: number
  }>(),
  {
    warningThreshold: 70,
    dangerThreshold: 90
  }
)

const visibleWindows = props.windows

const clamp = (value: number | null | undefined) => {
  if (value === null || value === undefined || Number.isNaN(value)) return 0
  return Math.max(0, Math.min(100, value))
}

const toneClass = (value: number | null | undefined) => {
  const v = clamp(value)
  if (v >= props.dangerThreshold) return 'progress-bar-danger'
  if (v >= props.warningThreshold) return 'progress-bar-warning'
  return ''
}

const formatPercent = (value: number | null | undefined) => {
  if (value === null || value === undefined || Number.isNaN(value)) return '-'
  return `${Math.round(clamp(value))}%`
}
</script>

<style scoped>
.cell-usage-window {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 140px;
}

.cell-usage-window-row {
  display: flex;
  align-items: center;
  gap: 8px;
}

.cell-usage-window-label {
  flex: none;
  width: 20px;
  font-size: 11px;
  color: var(--muted);
}

.cell-usage-window-track {
  flex: 1;
}

.cell-usage-window-value {
  flex: none;
  width: 30px;
  font-size: 11px;
  color: var(--muted);
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
