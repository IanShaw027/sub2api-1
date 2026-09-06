<template>
  <UiStatCard
    layout="icon"
    :label="title"
    :value="formattedValue"
    :icon="icon"
    :icon-variant="iconVariant"
    :delta="formattedChange"
    :delta-tone="changeType"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { Component } from 'vue'
import UiStatCard from '@/components/ui/StatCard.vue'

const props = withDefaults(defineProps<{
  title: string
  value: number | string
  icon?: Component
  iconVariant?: 'primary' | 'success' | 'warning' | 'danger'
  change?: number
  changeType?: 'up' | 'down' | 'neutral'
  formatValue?: (value: number | string) => string
}>(), {
  changeType: 'neutral',
  iconVariant: 'primary'
})

const formattedValue = computed(() => {
  if (props.formatValue) return props.formatValue(props.value)
  return typeof props.value === 'number' ? props.value.toLocaleString() : props.value
})

const formattedChange = computed(() => props.change === undefined ? undefined : `${Math.abs(props.change)}%`)
</script>
