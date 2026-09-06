<template>
  <GlassCard variant="glass" padding="md" class="ui-endpoint-card">
    <div class="ui-endpoint-card-top">
      <span class="ui-endpoint-card-label">{{ label }}</span>
      <span v-if="badge" class="ui-endpoint-card-badge" :class="badgeClass">{{ badge }}</span>
    </div>
    <div class="ui-endpoint-card-url-row">
      <code class="ui-endpoint-card-url">{{ url }}</code>
      <button
        type="button"
        class="ui-endpoint-card-copy"
        :aria-label="copyLabel"
        @click="$emit('copy', url)"
      >
        <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
          <path d="M8 8h12v12H8zM16 8V4H4v12h4" />
        </svg>
        {{ copyLabel }}
      </button>
    </div>
    <p v-if="description" class="ui-endpoint-card-description">{{ description }}</p>
    <div v-if="$slots.actions" class="ui-endpoint-card-actions"><slot name="actions" /></div>
  </GlassCard>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import GlassCard from './GlassCard.vue'
import type { StatusBadgeTone } from './types'

const props = withDefaults(
  defineProps<{
    label: string
    url: string
    description?: string
    badge?: string
    badgeTone?: StatusBadgeTone
    copyLabel?: string
  }>(),
  {
    badgeTone: 'accent',
    copyLabel: 'Copy'
  }
)

defineEmits<{
  copy: [url: string]
}>()

const badgeClass = computed(() => `badge-tone-${props.badgeTone}`)
</script>

<style scoped>
.ui-endpoint-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.ui-endpoint-card-label {
  font-size: 12px;
  font-weight: 600;
  color: var(--muted);
}

.ui-endpoint-card-badge {
  font-size: 11px;
  font-weight: 600;
  height: 22px;
}

.ui-endpoint-card-url-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 10px;
  margin-top: 8px;
}

.ui-endpoint-card-url {
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 13.5px;
  font-weight: 500;
  color: var(--foreground);
  overflow-wrap: anywhere;
}

.ui-endpoint-card-copy {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  height: 28px;
  padding: 0 10px;
  flex: none;
  border: 0;
  border-radius: 8px;
  background: var(--surface-secondary);
  color: var(--foreground);
  font-size: 12px;
  font-weight: 600;
  cursor: pointer;
}

.ui-endpoint-card-description {
  margin-top: 8px;
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.ui-endpoint-card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 8px;
}
</style>
