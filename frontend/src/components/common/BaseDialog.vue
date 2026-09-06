<template>
  <UiModal
    :open="show"
    :title="title"
    :width="modalWidth"
    :close-on-escape="closeOnEscape"
    :close-on-overlay="closeOnClickOutside"
    :show-close="showCloseButton"
    :z-index="zIndex"
    close-label="Close modal"
    @close="emit('close')"
  >
    <slot />
    <template v-if="$slots.footer" #footer><slot name="footer" /></template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import UiModal from '@/components/ui/UiModal.vue'
import type { ModalWidth } from '@/components/ui/types'

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

const props = withDefaults(defineProps<{
  show: boolean
  title: string
  width?: DialogWidth
  closeOnEscape?: boolean
  closeOnClickOutside?: boolean
  showCloseButton?: boolean
  zIndex?: number
}>(), {
  width: 'normal',
  closeOnEscape: true,
  closeOnClickOutside: false,
  showCloseButton: true,
  zIndex: 50
})

const emit = defineEmits<{ close: [] }>()

const modalWidth = computed<ModalWidth>(() => ({
  narrow: 'sm',
  normal: 'md',
  wide: 'lg',
  'extra-wide': 'xl',
  full: 'xl'
} as const)[props.width])
</script>
