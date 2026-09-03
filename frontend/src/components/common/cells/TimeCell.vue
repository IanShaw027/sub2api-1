<template>
  <span class="cell-time" :title="absoluteText">{{ displayText }}</span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { formatRelativeTime, formatDateTimeToMinute } from '@/utils/format'

const props = withDefaults(
  defineProps<{
    value?: string | Date | null
    /** 'relative' shows "3d ago" with the absolute time in the title; 'absolute' shows the full timestamp. */
    mode?: 'relative' | 'absolute'
  }>(),
  {
    value: null,
    mode: 'relative'
  }
)

const absoluteText = computed(() => formatDateTimeToMinute(props.value))
const displayText = computed(() =>
  props.mode === 'absolute' ? absoluteText.value : formatRelativeTime(props.value)
)
</script>

<style scoped>
.cell-time {
  font-family: var(--font-mono);
  font-size: 12.5px;
  color: var(--muted);
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
</style>
