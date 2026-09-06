<template>
  <UiStatusBadge :tone="tone" :label="label" dot />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiStatusBadge from '@/components/ui/StatusBadge.vue'

const props = defineProps<{
  status: string
  label: string
}>()

/** Same variant mapping as before, expressed with the Glass badge tones. */
const tone = computed(() => {
  switch (props.status) {
    case 'active':
    case 'success':
      return 'success'
    case 'disabled':
    case 'inactive':
    case 'warning':
      return 'warning'
    case 'error':
    case 'danger':
      return 'danger'
    case 'accent':
    case 'primary':
    case 'info':
      return 'accent'
    default:
      return 'muted'
  }
})
</script>
