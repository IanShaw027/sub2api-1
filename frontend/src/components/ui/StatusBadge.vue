<template>
  <span :class="['badge-tone-' + tone, dot ? 'ui-status-badge-dot' : null]">
    <span v-if="dot" class="ui-status-badge-dot-mark" aria-hidden="true" />
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
  }>(),
  {
    tone: 'muted',
    dot: false
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
</style>
