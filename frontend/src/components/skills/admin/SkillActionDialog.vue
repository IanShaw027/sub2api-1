<template>
  <BaseDialog :show="show" :title="dialogTitle" width="normal" @close="handleClose">
    <div class="space-y-4">
      <div class="rounded-card border border-line bg-page p-4 dark:border-dark-700 dark:bg-dark-900/60">
        <p class="text-xs font-medium uppercase tracking-[0.16em] text-ink-soft dark:text-ink-soft">{{ t('skills.admin.review.actionDialog.subjectLabel') }}</p>
        <p class="mt-2 text-sm font-medium text-ink dark:text-white">{{ subject || t('skills.admin.review.actionDialog.noSubject') }}</p>
        <p class="mt-2 text-sm text-ink-body dark:text-ink-soft">{{ dialogDescription }}</p>
      </div>

      <TextArea
        v-model="note"
        :label="textareaLabel"
        :placeholder="textareaPlaceholder"
        :hint="textareaHint"
        :error="validationError"
        :rows="4"
      />
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="handleClose">{{ t('common.cancel') }}</button>
        <button
          type="button"
          :class="confirmButtonClass"
          :disabled="loading"
          @click="handleSubmit"
        >
          {{ loading ? t('common.submitting') : confirmLabel }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import TextArea from '@/components/common/TextArea.vue'
import type { SkillAdminAction } from '@/api/admin/skills'

const { t } = useI18n()

const props = withDefaults(defineProps<{
  show: boolean
  action: SkillAdminAction | null
  subject?: string | null
  loading?: boolean
}>(), {
  action: null,
  subject: '',
  loading: false
})

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: { note: string }): void
}>()

const note = ref('')
const validationError = ref('')

const isDestructive = computed(() => props.action === 'reject' || props.action === 'disable' || props.action === 'force-private')

const dialogTitle = computed(() => {
  return {
    approve: t('skills.admin.review.actionDialog.titleApprove'),
    reject: t('skills.admin.review.actionDialog.titleReject'),
    disable: t('skills.admin.review.actionDialog.titleDisable'),
    'force-private': t('skills.admin.review.actionDialog.titleForcePrivate')
  }[props.action || 'approve']
})

const dialogDescription = computed(() => {
  return {
    approve: t('skills.admin.review.actionDialog.descApprove'),
    reject: t('skills.admin.review.actionDialog.descReject'),
    disable: t('skills.admin.review.actionDialog.descDisable'),
    'force-private': t('skills.admin.review.actionDialog.descForcePrivate')
  }[props.action || 'approve']
})

const textareaLabel = computed(() => (isDestructive.value
  ? t('skills.admin.review.actionDialog.notesLabelDestructive')
  : t('skills.admin.review.actionDialog.notesLabelApprove')))
const textareaPlaceholder = computed(() => {
  return {
    approve: t('skills.admin.review.actionDialog.placeholderApprove'),
    reject: t('skills.admin.review.actionDialog.placeholderReject'),
    disable: t('skills.admin.review.actionDialog.placeholderDisable'),
    'force-private': t('skills.admin.review.actionDialog.placeholderForcePrivate')
  }[props.action || 'approve']
})
const textareaHint = computed(() => (isDestructive.value
  ? t('skills.admin.review.actionDialog.hintDestructive')
  : t('skills.admin.review.actionDialog.hintApprove')))

const confirmLabel = computed(() => {
  return {
    approve: t('skills.admin.review.actionDialog.confirmApprove'),
    reject: t('skills.admin.review.actionDialog.confirmReject'),
    disable: t('skills.admin.review.actionDialog.confirmDisable'),
    'force-private': t('skills.admin.review.actionDialog.confirmForcePrivate')
  }[props.action || 'approve']
})

const confirmButtonClass = computed(() => {
  if (props.action === 'approve') return 'btn btn-primary'
  return 'btn btn-danger'
})

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      note.value = ''
      validationError.value = ''
    }
  }
)

watch(
  () => props.action,
  () => {
    note.value = ''
    validationError.value = ''
  }
)

function handleClose() {
  emit('close')
}

function handleSubmit() {
  const trimmed = note.value.trim()
  if (isDestructive.value && !trimmed) {
    validationError.value = t('skills.admin.review.actionDialog.validationRequired')
    return
  }
  validationError.value = ''
  emit('submit', { note: trimmed })
}
</script>
