<template>
  <Teleport to="body">
    <Transition name="ui-drawer-overlay">
      <div v-if="open" class="ui-drawer-overlay" @click="onOverlayClick" />
    </Transition>
    <Transition :name="panelTransition">
      <aside
        v-if="open"
        ref="panelRef"
        class="ui-drawer-panel"
        :class="side === 'left' ? 'is-left' : 'is-right'"
        role="dialog"
        aria-modal="true"
        tabindex="-1"
        :aria-label="title"
      >
        <header class="ui-drawer-header">
          <h2 class="ui-drawer-title">{{ title }}</h2>
          <button type="button" class="ui-drawer-close" :aria-label="closeLabel" @click="emit('close')">
            <Icon name="x" size="sm" />
          </button>
        </header>
        <div class="ui-drawer-body">
          <slot />
        </div>
        <footer v-if="$slots.footer" class="ui-drawer-footer">
          <slot name="footer" />
        </footer>
      </aside>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { focusFirst, trapFocus } from './focusTrap'
import { acquireOverlayLock, popOverlay, pushOverlay, releaseOverlayLock } from './overlayLock'
import type { DrawerSide } from './types'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    side?: DrawerSide
    closeOnOverlay?: boolean
    closeOnEscape?: boolean
    closeLabel?: string
  }>(),
  {
    side: 'right',
    closeOnOverlay: true,
    closeOnEscape: true,
    closeLabel: 'Close'
  }
)

const emit = defineEmits<{ close: [] }>()

const panelRef = ref<HTMLElement | null>(null)
const panelTransition = computed(() =>
  props.side === 'left' ? 'ui-drawer-panel-left' : 'ui-drawer-panel-right'
)
let previousFocus: HTMLElement | null = null
let overlayHeld = false

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
.ui-drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 55;
  background: var(--scrim);
  backdrop-filter: blur(2px);
  -webkit-backdrop-filter: blur(2px);
}

.ui-drawer-panel {
  position: fixed;
  top: 0;
  bottom: 0;
  z-index: 56;
  width: min(420px, 100vw);
  display: flex;
  flex-direction: column;
  background: color-mix(in oklch, var(--background) 88%, transparent);
  backdrop-filter: blur(28px);
  -webkit-backdrop-filter: blur(28px);
  color: var(--foreground);
}

.ui-drawer-panel.is-right {
  right: 0;
  border-left: 1px solid var(--border);
  box-shadow: -30px 0 60px -30px rgba(0, 0, 0, 0.5);
}

.ui-drawer-panel.is-left {
  left: 0;
  border-right: 1px solid var(--border);
  box-shadow: 30px 0 60px -30px rgba(0, 0, 0, 0.5);
}

.ui-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 20px 12px;
  border-bottom: 1px solid var(--border);
}

.ui-drawer-title {
  font-family: var(--display);
  font-size: 16px;
  font-weight: 800;
  letter-spacing: -0.02em;
  color: var(--foreground);
}

.ui-drawer-close {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 9px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.ui-drawer-close:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.ui-drawer-body {
  flex: 1;
  overflow: auto;
  padding: 16px 20px;
}

.ui-drawer-footer {
  padding: 12px 20px 16px;
  border-top: 1px solid var(--border);
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

.ui-drawer-overlay-enter-active,
.ui-drawer-overlay-leave-active {
  transition: opacity 0.2s ease;
}

.ui-drawer-overlay-enter-from,
.ui-drawer-overlay-leave-to {
  opacity: 0;
}

.ui-drawer-panel-right-enter-active,
.ui-drawer-panel-right-leave-active,
.ui-drawer-panel-left-enter-active,
.ui-drawer-panel-left-leave-active {
  transition: transform 0.25s ease;
}

.ui-drawer-panel-right-enter-from,
.ui-drawer-panel-right-leave-to {
  transform: translateX(100%);
}

.ui-drawer-panel-left-enter-from,
.ui-drawer-panel-left-leave-to {
  transform: translateX(-100%);
}
</style>
