<template>
  <span :class="['badge', 'badge-dot', toneClass]">
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
  label: string
}>()

/** Same variant mapping as before, expressed with the Glass badge tones. */
const toneClass = computed(() => {
  switch (props.status) {
    case 'active':
    case 'success':
      return 'badge-tone-success'
    case 'disabled':
    case 'inactive':
    case 'warning':
      return 'badge-tone-warning'
    case 'error':
    case 'danger':
      return 'badge-tone-danger'
    case 'accent':
    case 'primary':
    case 'info':
      return 'badge-tone-accent'
    default:
      return 'badge-tone-muted'
  }
})
</script>
