<template>
  <Teleport to="body">
    <Transition name="ui-modal">
      <div
        v-if="open"
        class="ui-modal-overlay"
        role="presentation"
        @click.self="onOverlayClick"
      >
        <div
          ref="panelRef"
          class="ui-modal-panel glass-card-solid"
          role="dialog"
          aria-modal="true"
          tabindex="-1"
          :aria-labelledby="titleId"
          :style="panelStyle"
          @click.stop
        >
          <header class="ui-modal-header">
            <h2 :id="titleId" class="ui-modal-title">{{ title }}</h2>
            <button
              v-if="showClose"
              type="button"
              class="ui-modal-close"
              :aria-label="closeLabel"
              @click="emit('close')"
            >
              <Icon name="x" size="sm" />
            </button>
          </header>
          <div ref="bodyRef" class="ui-modal-body">
            <slot />
          </div>
          <footer v-if="$slots.footer" class="ui-modal-footer">
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
    width?: ModalWidth
    closeOnOverlay?: boolean
    closeOnEscape?: boolean
    showClose?: boolean
    closeLabel?: string
  }>(),
  {
    width: 'md',
    closeOnOverlay: true,
    closeOnEscape: true,
    showClose: true,
    closeLabel: 'Close'
  }
)

const emit = defineEmits<{ close: [] }>()

const titleId = `ui-modal-title-${useId()}`
const panelRef = ref<HTMLElement | null>(null)
const bodyRef = ref<HTMLElement | null>(null)
let previousFocus: HTMLElement | null = null
let overlayHeld = false

const panelStyle = computed(() => {
  const widths: Record<ModalWidth, string> = { sm: '420px', md: '520px', lg: '680px', xl: '840px' }
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

onBeforeUnmount(releaseOverlay)
</script>

<style scoped>
.ui-modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 60;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 16px;
  background: rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
}

.ui-modal-panel {
  width: 100%;
  max-height: min(90vh, 900px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  border-radius: var(--radius-hero);
}

.ui-modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px;
  border-bottom: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
}

.ui-modal-title {
  font-size: 16px;
  font-weight: 800;
  color: var(--foreground);
}

.ui-modal-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex-shrink: 0;
  border: 0;
  border-radius: var(--radius-btn);
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.ui-modal-close:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.ui-modal-body {
  padding: 16px 18px;
  overflow: auto;
}

.ui-modal-footer {
  padding: 12px 18px 16px;
  border-top: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.ui-modal-enter-active,
.ui-modal-leave-active {
  transition: opacity 0.18s ease;
}

.ui-modal-enter-from,
.ui-modal-leave-to {
  opacity: 0;
}
</style>
