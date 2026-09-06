<template>
 <Teleport to="body">
 <Transition name="modal">
 <div
 v-if="show"
 class="modal-overlay"
 :style="zIndexStyle"
 :aria-labelledby="dialogId"
 role="dialog"
 aria-modal="true"
 @click.self="handleClose"
 >
 <!-- Modal panel -->
 <div ref="dialogRef" class="modal-content" :style="widthStyle" @click.stop>
 <!-- Header -->
 <div class="modal-header">
 <h3 :id="dialogId" class="modal-title">
 {{ title }}
 </h3>
 <button
 v-if="showCloseButton"
 type="button"
 @click="emit('close')"
 class="modal-close"
 aria-label="Close modal"
 >
 <Icon name="x" size="sm" :stroke-width="2" />
 </button>
 </div>

 <!-- Body -->
 <div ref="modalBodyRef" class="modal-body">
 <slot></slot>
 </div>

 <!-- Footer -->
 <div v-if="$slots.footer" class="modal-footer">
 <slot name="footer"></slot>
 </div>
 </div>
 </div>
 </Transition>
 </Teleport>
</template>

<script setup lang="ts">
import { computed, watch, onUnmounted, ref, nextTick } from 'vue'
import Icon from '@/components/icons/Icon.vue'
import { acquireOverlayLock, popOverlay, pushOverlay, releaseOverlayLock } from '@/components/ui/overlayLock'

// 生成唯一ID以避免多个对话框时ID冲突
let dialogIdCounter = 0
const dialogId = `modal-title-${++dialogIdCounter}`

// 焦点管理
const dialogRef = ref<HTMLElement | null>(null)
const modalBodyRef = ref<HTMLElement | null>(null)
let previousActiveElement: HTMLElement | null = null
let overlayHeld = false

type DialogWidth = 'narrow' | 'normal' | 'wide' | 'extra-wide' | 'full'

interface Props {
 show: boolean
 title: string
 width?: DialogWidth
 closeOnEscape?: boolean
 closeOnClickOutside?: boolean
 showCloseButton?: boolean
 zIndex?: number
}

interface Emits {
 (e: 'close'): void
}

const props = withDefaults(defineProps<Props>(), {
 width: 'normal',
 closeOnEscape: true,
 closeOnClickOutside: false,
 showCloseButton: true,
 zIndex: 50
})

const emit = defineEmits<Emits>()

// Custom z-index style (overrides the default z-50 from CSS)
const zIndexStyle = computed(() => {
 return props.zIndex !== 50 ? { zIndex: props.zIndex } : undefined
})

// `BaseDialog` is a compatible wrapper over `UiModal`'s look: its five legacy
// width names map onto the same fixed sm/md/lg/xl scale (440/560/720/960px)
// — no other panel widths are allowed by the glass-ui spec.
const widthStyle = computed(() => {
 const widths: Record<DialogWidth, string> = {
 narrow: '440px',
 normal: '560px',
 wide: '720px',
 'extra-wide': '960px',
 full: '960px'
 }
 return { maxWidth: widths[props.width] }
})

const handleClose = () => {
 if (props.closeOnClickOutside) {
 emit('close')
 }
}

const handleEscape = (event: KeyboardEvent) => {
 if (props.closeOnEscape && event.key === 'Escape') {
 emit('close')
 }
}

function holdOverlay() {
 if (overlayHeld) return
 acquireOverlayLock()
 pushOverlay(handleEscape)
 overlayHeld = true
}

function releaseOverlay() {
 if (!overlayHeld) return
 popOverlay(handleEscape)
 releaseOverlayLock()
 overlayHeld = false
}

// Prevent body scroll when modal is open and manage focus
watch(
 () => props.show,
 async (isOpen) => {
 if (isOpen) {
 // 保存当前焦点元素
 previousActiveElement = document.activeElement as HTMLElement
 holdOverlay()

 // 等待DOM更新后设置焦点到对话框
 await nextTick()
 if (modalBodyRef.value) {
 modalBodyRef.value.scrollTop = 0
 }
 if (dialogRef.value) {
 const firstFocusable = dialogRef.value.querySelector<HTMLElement>(
 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
 )
 firstFocusable?.focus()
 }
 } else {
 releaseOverlay()
 // 恢复之前的焦点
 if (previousActiveElement && typeof previousActiveElement.focus === 'function') {
 previousActiveElement.focus()
 }
 previousActiveElement = null
 }
 },
 { immediate: true }
)

onUnmounted(releaseOverlay)
</script>
