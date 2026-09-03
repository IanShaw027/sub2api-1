<template>
  <div
    class="summary-chip !cursor-default !flex-col !items-start !justify-start gap-1.5 !p-3.5"
    :title="title || undefined"
  >
    <span class="summary-chip-label">
      <span
        v-if="state"
        class="summary-chip-dot"
        :class="dotClass"
        aria-hidden="true"
      ></span>
      {{ label }}
    </span>
    <strong
      class="summary-chip-value block w-full overflow-visible !text-clip !whitespace-normal"
      :class="stateClass"
    >{{ value }}</strong>
    <div
      v-if="detailParts.length > 1"
      class="flex flex-wrap gap-x-2 gap-y-0.5 text-[11px] leading-snug text-muted"
    >
      <span
        v-for="(part, index) in detailParts"
        :key="`${index}:${part}`"
        class="whitespace-nowrap tabular-nums"
      >{{ part }}</span>
    </div>
    <small
      v-else-if="detail"
      class="block text-[11px] leading-snug text-muted"
    >{{ detail }}</small>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { HealthState } from '@/api/channelMonitorV2'

const props = defineProps<{
  label: string
  value: string
  detail: string
  state?: HealthState
  /** Exact numeric tooltip (e.g. uncompacted RPM/TPM). */
  title?: string
}>()

/** Split "AVG 475ms · P90 800ms" into chips so nothing is ellipsized. */
const detailParts = computed(() => {
  const raw = (props.detail || '').trim()
  if (!raw || raw === '-') return []
  return raw
    .split(/\s*[·|]\s*/)
    .map((part) => part.trim())
    .filter(Boolean)
})

const stateClass = computed(() => {
  if (!props.state) return 'text-foreground'
  if (props.state === 'healthy') return 'text-success-text'
  if (props.state === 'warning') return 'text-warning-text'
  if (props.state === 'critical') return 'text-danger-text'
  return 'text-muted'
})

const dotClass = computed(() => {
  if (props.state === 'healthy') return 'bg-success'
  if (props.state === 'warning') return 'bg-warning'
  if (props.state === 'critical') return 'bg-danger'
  return 'bg-surface-3'
})
</script>
