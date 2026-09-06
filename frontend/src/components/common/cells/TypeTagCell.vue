<template>
  <span class="tag" :class="toneClass">
    <slot name="icon" />
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

export type TypeTagTone = 'default' | 'accent' | 'success' | 'warning' | 'danger'

const props = withDefaults(
  defineProps<{
    label: string
    tone?: TypeTagTone
  }>(),
  {
    tone: 'default'
  }
)

const toneClass = computed(() => (props.tone === 'default' ? '' : `tag-${props.tone}`))
</script>
