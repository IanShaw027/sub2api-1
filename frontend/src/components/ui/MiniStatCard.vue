<template>
  <GlassCard variant="solid" padding="sm" class="ui-mini-stat">
    <p class="ui-mini-stat-label">{{ label }}</p>
    <p class="ui-mini-stat-value">{{ value }}</p>
    <p v-if="sub" class="ui-mini-stat-sub">{{ sub }}</p>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'

defineProps<{
  label: string
  value: string | number
  sub?: string
}>()
</script>

<style scoped>
.ui-mini-stat-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
}

.ui-mini-stat-value {
  margin-top: 4px;
  font-size: 20px;
  font-weight: 800;
  line-height: 1.1;
  color: var(--foreground);
}

.ui-mini-stat-sub {
  margin-top: 2px;
  font-size: 11px;
  color: var(--muted);
}
</style>
