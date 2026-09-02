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
              ×
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

const props = withDefaults(
  defineProps<{
    open: boolean
    title: string
    width?: 'sm' | 'md' | 'lg' | 'xl'
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

const panelStyle = computed(() => {
  const widths = { sm: '420px', md: '520px', lg: '680px', xl: '840px' }
  return { maxWidth: widths[props.width] }
})

const FOCUSABLE = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'

function onOverlayClick() {
  if (props.closeOnOverlay) emit('close')
}

function onKeydown(event: KeyboardEvent) {
  if (!props.open || !props.closeOnEscape) return
  if (event.key === 'Escape') {
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
      document.body.classList.add('modal-open')
      window.addEventListener('keydown', onKeydown)
      await nextTick()
      bodyRef.value && (bodyRef.value.scrollTop = 0)
      const first = panelRef.value?.querySelector<HTMLElement>(FOCUSABLE)
      first?.focus()
      return
    }
    if (wasOpen) {
      document.body.classList.remove('modal-open')
      window.removeEventListener('keydown', onKeydown)
      previousFocus?.focus?.()
      previousFocus = null
    }
  }
)

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown)
  document.body.classList.remove('modal-open')
})
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
}

.ui-modal-panel {
  width: 100%;
  max-height: min(90vh, 900px);
  display: flex;
  flex-direction: column;
  overflow: hidden;
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
  width: 32px;
  height: 32px;
  border: 0;
  border-radius: 10px;
  background: transparent;
  color: var(--muted);
  font-size: 22px;
  line-height: 1;
  cursor: pointer;
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
