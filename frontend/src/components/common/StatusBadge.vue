<template>
  <span
    :class="[
      'inline-flex items-center gap-1.5 rounded-chip px-2.5 py-0.5 text-xs font-semibold',
      pillClass
    ]"
  >
    <span
      :class="['inline-block h-1.5 w-1.5 rounded-full', dotClass]"
      aria-hidden="true"
    ></span>
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  status: string
  label: string
}>()

const pillClass = computed(() => {
  switch (props.status) {
    case 'active':
    case 'success':
      return 'bg-success/10 text-success dark:bg-emerald-900/30 dark:text-emerald-400'
    case 'disabled':
    case 'inactive':
    case 'warning':
      return 'bg-warning/10 text-warning dark:bg-amber-900/30 dark:text-amber-400'
    case 'error':
    case 'danger':
      return 'bg-danger/10 text-danger dark:bg-red-900/30 dark:text-red-400'
    default:
      return 'bg-ink-faint/10 text-ink-soft dark:bg-dark-700 dark:text-dark-300'
  }
})

const dotClass = computed(() => {
  switch (props.status) {
    case 'active':
    case 'success':
      return 'bg-success'
    case 'disabled':
    case 'inactive':
    case 'warning':
      return 'bg-warning'
    case 'error':
    case 'danger':
      return 'bg-danger'
    default:
      return 'bg-ink-faint'
  }
})
</script>
