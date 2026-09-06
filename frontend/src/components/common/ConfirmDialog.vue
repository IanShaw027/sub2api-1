<template>
 <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
 <div class="confirm-dialog-body">
 <div class="confirm-dialog-icon" :class="`confirm-dialog-icon-${tone}`" aria-hidden="true">
 <svg v-if="tone === 'accent'" width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
 <circle cx="12" cy="12" r="9" />
 <path d="M12 16v-4M12 8h.01" />
 </svg>
 <svg v-else width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">
 <path d="M10.29 3.86L1.82 18a2 2 0 001.71 3h16.94a2 2 0 001.71-3L13.71 3.86a2 2 0 00-3.42 0z" />
 <path d="M12 9v4M12 17h.01" />
 </svg>
 </div>
 <div class="confirm-dialog-copy">
 <p class="confirm-dialog-message">{{ message }}</p>
 <slot></slot>
 </div>
 </div>

 <template #footer>
 <div class="flex justify-end gap-2">
 <button
 @click="handleCancel"
 type="button"
 class="btn btn-secondary btn-sm"
 >
 {{ cancelText }}
 </button>
 <button
 @click="handleConfirm"
 type="button"
 :disabled="confirming || confirmDisabled"
 :class="['btn btn-sm disabled:cursor-not-allowed disabled:opacity-50', confirmButtonClass]"
 >
 {{ confirmText }}
 </button>
 </div>
 </template>
 </BaseDialog>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from './BaseDialog.vue'

const { t } = useI18n()

type ConfirmTone = 'danger' | 'warning' | 'accent'

interface Props {
 show: boolean
 title: string
 message: string
 confirmText?: string
 cancelText?: string
 /** @deprecated use `tone="danger"` instead */
 danger?: boolean
 tone?: ConfirmTone
 confirming?: boolean
 confirmDisabled?: boolean
}

interface Emits {
 (e: 'confirm'): void
 (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
 danger: false,
 tone: undefined,
 confirming: false,
 confirmDisabled: false
})

const tone = computed<ConfirmTone>(() => props.tone ?? (props.danger ? 'danger' : 'accent'))

const confirmButtonClass = computed(() => {
 if (tone.value === 'danger') return 'btn-danger'
 if (tone.value === 'warning') return 'btn-warning'
 return 'btn-primary'
})

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()

const handleConfirm = () => {
 if (props.confirming || props.confirmDisabled) return
 emit('confirm')
}

const handleCancel = () => {
 emit('cancel')
}
</script>

<style scoped>
.confirm-dialog-body {
 display: flex;
 align-items: flex-start;
 gap: 12px;
}

.confirm-dialog-icon {
 display: flex;
 align-items: center;
 justify-content: center;
 width: 44px;
 height: 44px;
 flex: none;
 border-radius: 999px;
}

.confirm-dialog-icon-danger {
 background: color-mix(in oklch, var(--danger) 14%, transparent);
 color: var(--danger-text);
}

.confirm-dialog-icon-warning {
 background: color-mix(in oklch, var(--warning) 18%, transparent);
 color: var(--warning-text);
}

.confirm-dialog-icon-accent {
 background: color-mix(in oklch, var(--accent) 12%, transparent);
 color: var(--accent);
}

.confirm-dialog-copy {
 display: flex;
 flex-direction: column;
 gap: 8px;
 min-width: 0;
}

.confirm-dialog-message {
 font-size: 13px;
 line-height: 1.6;
 color: var(--muted);
}
</style>
