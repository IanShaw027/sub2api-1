<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, useTemplateRef, nextTick } from 'vue'

const props = withDefaults(defineProps<{
 content?: string
 trigger?: 'hover' | 'click'
 widthClass?: string
}>(), {
 trigger: 'hover',
 widthClass: 'w-64',
})

const show = ref(false)
const triggerRef = useTemplateRef<HTMLElement>('trigger')
const tooltipRef = useTemplateRef<HTMLElement>('tooltip')
const tooltipStyle = ref({ top: '0px', left: '0px' })

function openTooltip() {
 show.value = true
 nextTick(updatePosition)
}

function closeTooltip() {
 show.value = false
}

function onEnter() {
 if (props.trigger !== 'hover') return
 openTooltip()
}

function onLeave() {
 if (props.trigger !== 'hover') return
 closeTooltip()
}

function onClick(event: MouseEvent) {
 if (props.trigger !== 'click') return
 event.stopPropagation()
 if (show.value) {
 closeTooltip()
 return
 }
 openTooltip()
}

function onDocumentClick(event: MouseEvent) {
 if (props.trigger !== 'click' || !show.value) return
 const target = event.target as Node | null
 if (!target) return
 if (triggerRef.value?.contains(target) || tooltipRef.value?.contains(target)) return
 closeTooltip()
}

function onDocumentKeydown(event: KeyboardEvent) {
 if (props.trigger !== 'click') return
 if (event.key === 'Escape') {
 closeTooltip()
 }
}

function onViewportChange() {
 if (!show.value) return
 updatePosition()
}

function updatePosition() {
 const el = triggerRef.value
 if (!el) return
 const rect = el.getBoundingClientRect()
 tooltipStyle.value = {
 top: `${rect.top + window.scrollY}px`,
 left: `${rect.left + rect.width / 2 + window.scrollX}px`,
 }
}

onMounted(() => {
 document.addEventListener('click', onDocumentClick, true)
 document.addEventListener('keydown', onDocumentKeydown)
 window.addEventListener('resize', onViewportChange)
 window.addEventListener('scroll', onViewportChange, true)
})

onBeforeUnmount(() => {
 document.removeEventListener('click', onDocumentClick, true)
 document.removeEventListener('keydown', onDocumentKeydown)
 window.removeEventListener('resize', onViewportChange)
 window.removeEventListener('scroll', onViewportChange, true)
})
</script>

<template>
  <div
    ref="trigger"
    class="group relative ml-1 inline-flex items-center align-middle"
    @mouseenter="onEnter"
    @mouseleave="onLeave"
    @click="onClick"
  >
    <!-- Trigger · 15px circle, 1.5px muted ring, 10px/700 "?" -->
    <slot name="trigger">
      <span class="help-tooltip-mark" aria-hidden="true">?</span>
    </slot>

    <!-- Teleport to body to escape modal overflow clipping -->
    <Teleport to="body">
      <div
        ref="tooltip"
        v-show="show"
        role="tooltip"
        :class="['help-tooltip-bubble tooltip-bubble', props.widthClass]"
        :style="{ top: `calc(${tooltipStyle.top} - 8px)`, left: tooltipStyle.left }"
      >
        <button
          v-if="props.trigger === 'click'"
          type="button"
          class="help-tooltip-close"
          aria-label="Close"
          @click.stop="closeTooltip"
        >
          <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
            <path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
        <slot>{{ content }}</slot>
        <span class="help-tooltip-arrow" aria-hidden="true"></span>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
.help-tooltip-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 15px;
  height: 15px;
  flex: none;
  border-radius: 999px;
  border: 1.5px solid var(--muted);
  color: var(--muted);
  font-size: 10px;
  font-weight: 700;
  line-height: 1;
  cursor: help;
  transition: color 0.15s ease, border-color 0.15s ease;
}

.group:hover .help-tooltip-mark {
  color: var(--accent);
  border-color: var(--accent);
}

/* Bubble geometry comes from the global `.tooltip-bubble` recipe
   (foreground bg / background text, 11.5/500, padding 6px 10px, radius 8). */
.help-tooltip-bubble {
  position: fixed;
  z-index: 99999;
  transform: translate(-50%, -100%);
}

.help-tooltip-arrow {
  position: absolute;
  bottom: -3px;
  left: 50%;
  width: 6px;
  height: 6px;
  transform: translateX(-50%) rotate(45deg);
  background: var(--foreground);
}

.help-tooltip-close {
  position: absolute;
  right: 4px;
  top: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  border-radius: 6px;
  color: color-mix(in oklch, var(--background) 70%, transparent);
  transition: background 0.15s ease, color 0.15s ease;
}

.help-tooltip-close:hover {
  background: color-mix(in oklch, var(--background) 14%, transparent);
  color: var(--background);
}
</style>
