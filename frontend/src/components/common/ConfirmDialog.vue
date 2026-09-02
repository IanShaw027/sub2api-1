<template>
 <BaseDialog :show="show" :title="title" width="narrow" @close="handleCancel">
 <div class="space-y-4">
 <p class="text-sm text-muted">{{ message }}</p>
 <slot></slot>
 </div>

 <template #footer>
 <div class="flex justify-end space-x-3">
 <button
 @click="handleCancel"
 type="button"
 class="rounded-md border border-line bg-surface px-4 py-2 text-sm font-medium text-foreground hover:bg-surface-2 focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2"
 >
 {{ cancelText }}
 </button>
 <button
 @click="handleConfirm"
 type="button"
 :disabled="confirming"
 :class="[
 'rounded-md px-4 py-2 text-sm font-medium text-white focus:outline-none focus:ring-2 focus:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50',
 danger
 ? 'bg-red-600 hover:bg-red-700 focus:ring-red-500'
 : 'bg-accent hover:opacity-90 focus:ring-accent'
 ]"
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
