<template>
  <GlassCard v-if="items?.length" variant="glass" padding="sm" class="ui-mini-stat-group">
    <div class="ui-mini-stat-grid">
      <div v-for="(item, index) in items" :key="index" class="ui-mini-stat">
        <p class="ui-mini-stat-label">{{ item.label }}</p>
        <p class="ui-mini-stat-value">{{ item.value }}</p>
        <p v-if="item.sub" class="ui-mini-stat-sub">{{ item.sub }}</p>
      </div>
    </div>
  </GlassCard>
  <GlassCard v-else variant="glass" padding="sm" class="ui-mini-stat-card">
    <p class="ui-mini-stat-label">{{ label }}</p>
    <p class="ui-mini-stat-value">{{ value }}</p>
    <p v-if="sub" class="ui-mini-stat-sub">{{ sub }}</p>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'
import type { MiniStatItem } from './types'

defineProps<{
  label?: string
  value?: string | number
  sub?: string
  items?: MiniStatItem[]
}>()
</script>

<style scoped>
.ui-mini-stat-card.glass-card,
.ui-mini-stat-group.glass-card {
  padding: 10px 12px;
}

.ui-mini-stat-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 10px;
  align-items: center;
}

.ui-mini-stat-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
}

.ui-mini-stat-value {
  margin-top: 2px;
  font-family: var(--display);
  font-size: 20px;
  font-weight: 800;
  letter-spacing: -0.03em;
  line-height: 1.1;
  font-variant-numeric: tabular-nums;
  color: var(--foreground);
}

.ui-mini-stat-sub {
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted);
}
</style>
