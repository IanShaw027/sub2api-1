<template>
  <GlassCard variant="solid" padding="md" class="ui-endpoint-card">
    <div class="ui-endpoint-card-header">
      <div class="ui-endpoint-card-method" :class="`is-${method.toLowerCase()}`">{{ method }}</div>
      <code class="ui-endpoint-card-path">{{ path }}</code>
      <StatusBadge v-if="status" :tone="statusTone" :label="status" />
    </div>
    <p v-if="description" class="ui-endpoint-card-description">{{ description }}</p>
    <div v-if="$slots.default" class="ui-endpoint-card-body">
      <slot />
    </div>
  </GlassCard>
</template>

<script setup lang="ts">
import GlassCard from './GlassCard.vue'
import StatusBadge from './StatusBadge.vue'
import type { StatusBadgeTone } from './types'

withDefaults(
  defineProps<{
    method: string
    path: string
    description?: string
    status?: string
    statusTone?: StatusBadgeTone
  }>(),
  {
    statusTone: 'muted'
  }
)
</script>

<style scoped>
.ui-endpoint-card-header {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.ui-endpoint-card-method {
  height: 22px;
  padding: 0 8px;
  border-radius: 6px;
  font-size: 11px;
  font-weight: 800;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  background: color-mix(in oklch, var(--accent) 14%, transparent);
  color: var(--accent);
}

.ui-endpoint-card-method.is-post {
  background: color-mix(in oklch, var(--success) 16%, transparent);
  color: var(--success-text);
}

.ui-endpoint-card-method.is-get {
  background: color-mix(in oklch, var(--accent) 14%, transparent);
  color: var(--accent);
}

.ui-endpoint-card-path {
  font-family: var(--font-mono);
  font-size: 12px;
  color: var(--foreground);
}

.ui-endpoint-card-description {
  margin-top: 8px;
  font-size: 12px;
  color: var(--muted);
}

.ui-endpoint-card-body {
  margin-top: 12px;
}
</style>
