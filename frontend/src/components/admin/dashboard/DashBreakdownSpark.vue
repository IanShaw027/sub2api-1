<template>
  <HelpTooltip class="dash-breakdown-trigger" width-class="w-56">
    <template #trigger>
      <DashSparkline :path="path" />
    </template>
    <div class="dash-breakdown">
      <div v-for="item in items" :key="item.key" class="dash-breakdown-row">
        <span>{{ item.label }}</span>
        <span class="dash-breakdown-value" :class="item.textClass">${{ format(item.value) }}</span>
      </div>
    </div>
  </HelpTooltip>
</template>

<script setup lang="ts">
/**
 * Stat-card sparkline that doubles as the trigger for a cost-split tooltip,
 * so the card keeps the prototype's shape while retaining the breakdown data.
 */
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import DashSparkline from './DashSparkline.vue'
import type { BreakdownItem, SparkPath } from './types'

defineProps<{
  path: SparkPath
  items: BreakdownItem[]
  format: (value: number) => string
}>()
</script>

<style scoped>
.dash-breakdown-trigger {
  margin-left: 0 !important;
  width: 100%;
  height: 100%;
}

.dash-breakdown {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.dash-breakdown-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.dash-breakdown-value {
  font-weight: 600;
  font-variant-numeric: tabular-nums;
}

.dash-tone-success { color: var(--success-text); }
.dash-tone-warning { color: var(--warning-text); }
.dash-tone-muted { color: var(--muted); }
.dash-tone-accent { color: var(--accent); }
.dash-tone-danger { color: var(--danger-text); }
</style>
