<template>
  <StatusBadge :tone="resolvedTone" :label="label" :dot="dot" :pulse="pulse" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { StatusBadgeTone } from '@/components/ui/types'
import { statusTone } from './statusTone'

const props = withDefaults(
  defineProps<{
    status: string | null | undefined
    label: string
    /** Override the auto-resolved tone. */
    tone?: StatusBadgeTone | null
    dot?: boolean
    pulse?: boolean
  }>(),
  {
    tone: null,
    dot: true,
    pulse: false
  }
)

const resolvedTone = computed<StatusBadgeTone>(() => props.tone ?? statusTone(props.status))
</script>
