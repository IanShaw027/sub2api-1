<template>
 <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
 <div class="space-y-4">
 <p class="text-sm text-muted">{{ message }}</p>
 <slot></slot>
 </div>

 <template #footer>
 <div class="flex justify-end gap-2">
 <button
 @click="handleCancel"
 type="button"
 class="btn-glass-secondary"
 >
 {{ cancelText }}
 </button>
 <button
 @click="handleConfirm"
 type="button"
 :disabled="confirming"
 :class="danger
 ? 'inline-flex h-[34px] items-center justify-center rounded-btn bg-danger px-3.5 text-[13px] font-semibold text-white disabled:cursor-not-allowed disabled:opacity-50'
 : 'btn-glass-primary disabled:cursor-not-allowed disabled:opacity-50'"
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

interface Props {
 show: boolean
 title: string
 message: string
 confirmText?: string
 cancelText?: string
 danger?: boolean
 confirming?: boolean
}

interface Emits {
 (e: 'confirm'): void
 (e: 'cancel'): void
}

const props = withDefaults(defineProps<Props>(), {
 danger: false,
 confirming: false
})

const confirmText = computed(() => props.confirmText || t('common.confirm'))
const cancelText = computed(() => props.cancelText || t('common.cancel'))

const emit = defineEmits<Emits>()

const handleConfirm = () => {
 if (props.confirming) return
 emit('confirm')
}

const handleCancel = () => {
 emit('cancel')
}
</script>
