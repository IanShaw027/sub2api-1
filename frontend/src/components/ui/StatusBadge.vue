<template>
  <span :class="['badge-tone-' + tone, dot ? 'ui-status-badge-dot' : null]">
    <span
      v-if="dot"
      class="ui-status-badge-dot-mark"
      :class="{ 'ui-status-badge-dot-live': pulse }"
      aria-hidden="true"
    />
    <slot>{{ label }}</slot>
  </span>
</template>

<script setup lang="ts">
import type { StatusBadgeTone } from './types'

withDefaults(
  defineProps<{
    tone?: StatusBadgeTone
    label?: string
    dot?: boolean
    /** Real-time / live state: animates the dot with `s2a-pulse`. */
    pulse?: boolean
  }>(),
  {
    tone: 'muted',
    dot: false,
    pulse: false
  }
)
</script>

<style scoped>
.ui-status-badge-dot {
  gap: 6px;
}

.ui-status-badge-dot-mark {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: currentColor;
  flex: none;
}

.ui-status-badge-dot-live {
  animation: s2a-pulse 1.6s ease-in-out infinite;
}
</style>
