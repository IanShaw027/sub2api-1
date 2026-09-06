<template>
  <Teleport to="body">
    <Transition name="modal">
      <div
        v-if="open"
        class="modal-overlay ui-modal-overlay"
        :style="{ zIndex }"
        role="presentation"
        @click.self="onOverlayClick"
      >
        <div
          ref="panelRef"
          class="modal-content ui-modal-panel glass-card-solid"
          role="dialog"
          aria-modal="true"
          tabindex="-1"
          :aria-labelledby="titleId"
          :style="panelStyle"
          @click.stop
        >
          <header class="modal-header ui-modal-header">
            <div class="ui-modal-heading">
              <h2 :id="titleId" class="modal-title ui-modal-title">{{ title }}</h2>
              <p v-if="subtitle || $slots.subtitle" class="modal-subtitle">
                <slot name="subtitle">{{ subtitle }}</slot>
              </p>
            </div>
            <button
              v-if="showClose"
              type="button"
              class="modal-close ui-modal-close"
              :aria-label="closeLabel"
              @click="emit('close')"
            >
              <Icon name="x" size="sm" />
            </button>
          </header>
          <div ref="bodyRef" class="modal-body ui-modal-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="modal-footer ui-modal-footer">
            <slot name="footer" />
          </footer>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, useId, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { focusFirst, trapFocus } from './focusTrap'
import { acquireOverlayLock, popOverlay, pushOverlay, releaseOverlayLock } from './overlayLock'
import type { ModalWidth } from './types'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    subtitle?: string
    width?: ModalWidth
    closeOnOverlay?: boolean
    closeOnEscape?: boolean
    showClose?: boolean
    closeLabel?: string
    zIndex?: number
  }>(),
  {
    width: 'md',
    closeOnOverlay: true,
    closeOnEscape: true,
    showClose: true,
    closeLabel: 'Close',
    zIndex: 60
  }
)

const emit = defineEmits<{ close: [] }>()

const titleId = `ui-modal-title-${useId()}`
const panelRef = ref<HTMLElement | null>(null)
const bodyRef = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
let overlayHeld = false

/** Panel widths only ever come from this fixed sm/md/lg/xl scale (440/560/720/960). */
const panelStyle = computed(() => {
  const widths: Record<ModalWidth, string> = { sm: '440px', md: '560px', lg: '720px', xl: '960px' }
  return { maxWidth: widths[props.width] }
})

function onOverlayClick() {
  if (props.closeOnOverlay) emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && props.closeOnEscape) {
    emit('close')
    return
  }
  trapFocus(event, panelRef.value)
}

function holdOverlay() {
  if (overlayHeld) return
  acquireOverlayLock()
  pushOverlay(onKeydown)
  overlayHeld = true
}

function releaseOverlay() {
  if (!overlayHeld) return
  popOverlay(onKeydown)
  releaseOverlayLock()
  overlayHeld = false
}

watch(
  () => props.open,
  async (open, wasOpen) => {
    if (open) {
      previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
      holdOverlay()
      await nextTick()
      if (!props.open || !overlayHeld) return
      if (bodyRef.value) bodyRef.value.scrollTop = 0
      focusFirst(panelRef.value)
      return
    }
    if (wasOpen) {
      releaseOverlay()
      previousFocus?.focus?.()
      previousFocus = null
    }
  },
  { immediate: true }
)

onBeforeUnmount(() => {
  releaseOverlay()
  previousFocus?.focus?.()
  previousFocus = null
})
</script>

<style scoped>
.ui-modal-heading {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

@media (max-width: 767px) {
  .ui-modal-close {
    position: relative;
  }

  .ui-modal-close::after {
    content: '';
    position: absolute;
    inset: -8px;
  }
}

</style>
