<template>
  <div class="ui-progress" role="progressbar" :aria-valuenow="clamped" :aria-valuemin="0" :aria-valuemax="100">
    <div class="ui-progress-track">
      <div class="ui-progress-fill" :class="toneClass" :style="{ width: `${clamped}%` }" />
    </div>
    <span v-if="showLabel" class="ui-progress-label">{{ clamped }}%</span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(
  defineProps<{
    value: number
    showLabel?: boolean
  }>(),
  {
    showLabel: false
  }
)

const clamped = computed(() => Math.max(0, Math.min(100, Math.round(props.value))))

const toneClass = computed(() => {
  if (clamped.value >= 90) return 'ui-progress-danger'
  if (clamped.value >= 70) return 'ui-progress-warning'
  return 'ui-progress-accent'
})
</script>

<style scoped>
.ui-progress {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
}

.ui-progress-track {
  flex: 1;
  height: 6px;
  border-radius: 999px;
  background: color-mix(in oklch, var(--foreground) 8%, transparent);
  overflow: hidden;
}

.ui-progress-fill {
  height: 100%;
  border-radius: inherit;
  transition: width 0.2s ease;
}

.ui-progress-accent {
  background: var(--accent);
}

.ui-progress-warning {
  background: var(--warning);
}

.ui-progress-danger {
  background: var(--danger);
}

.ui-progress-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  min-width: 32px;
  text-align: right;
}
</style>
