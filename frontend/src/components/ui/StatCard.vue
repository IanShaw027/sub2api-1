<template>
  <GlassCard :variant="variant" :padding="padding" class="ui-stat-card">
    <div class="ui-stat-card-body">
      <div class="ui-stat-card-copy">
        <p class="ui-stat-card-label">{{ label }}</p>
        <p class="ui-stat-card-value">{{ value }}</p>
        <p v-if="sub" class="ui-stat-card-sub">{{ sub }}</p>
        <p v-if="delta" class="ui-stat-card-delta" :class="deltaToneClass">{{ delta }}</p>
      </div>
      <div v-if="$slots.sparkline" class="ui-stat-card-sparkline">
        <slot name="sparkline" />
      </div>
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import GlassCard from './GlassCard.vue'
import type { GlassCardPadding, GlassCardVariant } from './types'

const props = withDefaults(
  defineProps<{
    label: string
    value: string | number
    sub?: string
    delta?: string
    deltaTone?: 'up' | 'down' | 'neutral'
    variant?: GlassCardVariant
    padding?: GlassCardPadding
  }>(),
  {
    deltaTone: 'neutral',
    variant: 'solid',
    padding: 'md'
  }
)

const deltaToneClass = computed(() => {
  if (props.deltaTone === 'up') return 'ui-stat-card-delta-up'
  if (props.deltaTone === 'down') return 'ui-stat-card-delta-down'
  return 'ui-stat-card-delta-neutral'
})
</script>

<style scoped>
.ui-stat-card-body {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 12px;
}

.ui-stat-card-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}

.ui-stat-card-value {
  margin-top: 4px;
  font-size: 24px;
  font-weight: 800;
  line-height: 1.1;
  color: var(--foreground);
}

.ui-stat-card-sub {
  margin-top: 4px;
  font-size: 12px;
  color: var(--muted);
}

.ui-stat-card-delta {
  margin-top: 6px;
  font-size: 11px;
  font-weight: 700;
}

.ui-stat-card-delta-up {
  color: var(--success-text);
}

.ui-stat-card-delta-down {
  color: var(--danger-text);
}

.ui-stat-card-delta-neutral {
  color: var(--muted);
}

.ui-stat-card-sparkline {
  flex: none;
  width: 72px;
  height: 28px;
}
</style>
