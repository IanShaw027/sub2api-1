<template>
  <UiModal
    :open="open"
    :title="t('admin.redeem.generatedSuccessfully')"
    width="md"
    :close-label="t('common.close')"
    @close="handleClose"
  >
    <p class="mb-4 text-sm text-muted">{{ t('admin.redeem.codesCreated', { count: codes.length }) }}</p>
    <textarea
      readonly
      :value="generatedCodesText"
      :style="{ height: textareaHeight }"
      class="code-block generated-codes"
    ></textarea>

    <template #footer>
      <button
        type="button"
        :class="['btn', copiedAll ? 'btn-success' : 'btn-secondary']"
        @click="copyGeneratedCodes"
      >
        <Icon :name="copiedAll ? 'check' : 'copy'" size="sm" :stroke-width="2" />
        {{ copiedAll ? t('admin.redeem.copied') : t('admin.redeem.copyAll') }}
      </button>
      <button type="button" class="btn btn-primary" @click="downloadGeneratedCodes">
        <Icon name="download" size="sm" :stroke-width="2" />
        {{ t('admin.redeem.download') }}
      </button>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import type { RedeemCode } from '@/types'
import UiModal from '@/components/ui/UiModal.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  open: boolean
  codes: RedeemCode[]
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const { copyToClipboard: clipboardCopy } = useClipboard()

const copiedAll = ref(false)

const generatedCodesText = computed(() => props.codes.map((code) => code.code).join('\n'))

const textareaHeight = computed(() => {
  const lineCount = props.codes.length
  const lineHeight = 24 // approximate line height in px
  const padding = 24 // top + bottom padding
  const minHeight = 60
  const maxHeight = 240
  const calculatedHeight = Math.min(
    Math.max(lineCount * lineHeight + padding, minHeight),
    maxHeight
  )
  return `${calculatedHeight}px`
})

const handleClose = () => {
  copiedAll.value = false
  emit('close')
}

const copyGeneratedCodes = async () => {
  const success = await clipboardCopy(generatedCodesText.value, t('admin.redeem.copied'))
  if (success) {
    copiedAll.value = true
    setTimeout(() => {
      copiedAll.value = false
    }, 2000)
  }
}

const downloadGeneratedCodes = () => {
  const blob = new Blob([generatedCodesText.value], { type: 'text/plain' })
  const url = window.URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `redeem-codes-${new Date().toISOString().split('T')[0]}.txt`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  window.URL.revokeObjectURL(url)
}
</script>

<style scoped>
.generated-codes {
  width: 100%;
  resize: none;
}
</style>
