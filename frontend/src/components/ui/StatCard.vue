<template>
  <GlassCard :variant="variant" :hover="hover" :padding="padding" class="ui-stat-card ui-stat-card-pad">
    <div class="ui-stat-card-top">
      <p class="ui-stat-card-label">{{ label }}</p>
      <span v-if="delta" class="ui-stat-card-delta" :class="deltaToneClass">{{ delta }}</span>
    </div>
    <p class="ui-stat-card-value">{{ value }}</p>
    <div class="ui-stat-card-bottom">
      <p v-if="sub" class="ui-stat-card-sub">{{ sub }}</p>
      <div v-if="$slots.sparkline" class="ui-stat-card-sparkline">
        <slot name="sparkline" />
      </div>
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import GlassCard from './GlassCard.vue'
import type { GlassCardPadding, GlassCardVariant, StatDeltaTone } from './types'

const props = withDefaults(
  defineProps<{
    label: string
    value: string | number
    sub?: string
    delta?: string
    deltaTone?: StatDeltaTone
    variant?: GlassCardVariant
    padding?: GlassCardPadding
    hover?: boolean
  }>(),
  {
    deltaTone: 'neutral',
    variant: 'glass',
    padding: 'md',
    hover: true
  }
)

const deltaToneClass = computed(() => {
  if (props.deltaTone === 'up') return 'ui-stat-card-delta-up'
  if (props.deltaTone === 'down') return 'ui-stat-card-delta-down'
  if (props.deltaTone === 'warn') return 'ui-stat-card-delta-warn'
  return 'ui-stat-card-delta-neutral'
})
</script>

<style scoped>
.ui-stat-card-pad.ui-glass-card-pad-md {
  padding: 14px 16px;
}

.ui-stat-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.ui-stat-card-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}

.ui-stat-card-value {
  margin-top: 6px;
  font-family: var(--display);
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.04em;
  line-height: 1.05;
  font-variant-numeric: tabular-nums;
  color: var(--foreground);
}

.ui-stat-card-bottom {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 10px;
  margin-top: 6px;
}

.ui-stat-card-sub {
  font-size: 12px;
  color: var(--muted);
}

.ui-stat-card-delta {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 6px;
  border-radius: 6px;
}

.ui-stat-card-delta-up {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.ui-stat-card-delta-down {
  background: color-mix(in oklch, var(--danger) 14%, transparent);
  color: var(--danger-text);
}

.ui-stat-card-delta-warn {
  background: color-mix(in oklch, var(--warning) 16%, transparent);
  color: var(--warning-text);
}

.ui-stat-card-delta-neutral {
  background: var(--surface-secondary);
  color: var(--muted);
}

.ui-stat-card-sparkline {
  flex: none;
  width: 96px;
  height: 28px;
}
</style>
