<template>
  <Teleport to="body">
    <Transition name="ui-drawer-overlay">
      <div v-if="open" class="ui-drawer-overlay" @click="onOverlayClick" />
    </Transition>
    <Transition name="ui-drawer-panel">
      <aside
        v-if="open"
        ref="panelRef"
        class="ui-drawer-panel"
        role="dialog"
        aria-modal="true"
        :aria-label="title"
      >
        <header class="ui-drawer-header">
          <h2 class="ui-drawer-title">{{ title }}</h2>
          <button type="button" class="ui-drawer-close" :aria-label="closeLabel" @click="emit('close')">
            ×
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
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    side?: 'right' | 'left'
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
let previousFocus: HTMLElement | null = null
let bodyLocked = false

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

function lockBody(lock: boolean) {
  if (lock && !bodyLocked) {
    document.body.style.overflow = 'hidden'
    bodyLocked = true
  } else if (!lock && bodyLocked) {
    document.body.style.overflow = ''
    bodyLocked = false
  }
}

function onOverlayClick() {
  if (props.closeOnOverlay) emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (!props.open) return
  if (event.key === 'Escape' && props.closeOnEscape) {
    emit('close')
    return
  }
  if (event.key !== 'Tab' || !panelRef.value) return
  const nodes = Array.from(panelRef.value.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
    (el) => !el.hasAttribute('disabled')
  )
  if (nodes.length === 0) return
  const first = nodes[0]
  const last = nodes[nodes.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

watch(
  () => props.open,
  async (open, wasOpen) => {
    if (open) {
      previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
      lockBody(true)
      window.addEventListener('keydown', onKeydown)
      await nextTick()
      const first = panelRef.value?.querySelector<HTMLElement>(FOCUSABLE)
      first?.focus()
      return
    }
    if (wasOpen) {
      lockBody(false)
      window.removeEventListener('keydown', onKeydown)
      previousFocus?.focus?.()
      previousFocus = null
    }
  }
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  lockBody(false)
})
</script>

<style scoped>
.ui-drawer-overlay {
  position: fixed;
  inset: 0;
  z-index: 55;
  background: rgba(0, 0, 0, 0.28);
  backdrop-filter: blur(2px);
}

.ui-drawer-panel {
  position: fixed;
  top: 0;
  right: 0;
  z-index: 56;
  width: min(420px, 100vw);
  height: 100%;
  display: flex;
  flex-direction: column;
  background: color-mix(in oklch, var(--background) 88%, transparent);
  backdrop-filter: blur(28px);
  border-left: 1px solid color-mix(in oklch, var(--border) 80%, transparent);
  box-shadow: -12px 0 40px rgba(0, 0, 0, 0.12);
}

.ui-drawer-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 16px 18px;
  border-bottom: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
}

.ui-drawer-title {
  font-size: 15px;
  font-weight: 800;
  color: var(--foreground);
}

.ui-drawer-close {
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--muted);
  font-size: 22px;
  cursor: pointer;
}

.ui-drawer-body {
  flex: 1;
  overflow: auto;
  padding: 16px 18px;
}

.ui-drawer-footer {
  padding: 12px 18px 16px;
  border-top: 1px solid color-mix(in oklch, var(--border) 70%, transparent);
}

.ui-drawer-overlay-enter-active,
.ui-drawer-overlay-leave-active,
.ui-drawer-panel-enter-active,
.ui-drawer-panel-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}

.ui-drawer-overlay-enter-from,
.ui-drawer-overlay-leave-to {
  opacity: 0;
}

.ui-drawer-panel-enter-from,
.ui-drawer-panel-leave-to {
  transform: translateX(100%);
}
</style>
