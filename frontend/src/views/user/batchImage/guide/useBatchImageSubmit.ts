import { ref, type ComputedRef, type Ref } from 'vue'
import { submitBatchImageJob, type BatchImageJob, type BatchImageSubmitItem } from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { useAppStore } from '@/stores/app'
import { BATCH_IMAGE_MAX_OUTPUTS_PER_JOB } from '../constants'
import type { BatchImageTextKey } from '../errorMessages'
import { defaultTaskName } from '../format'
import type { BatchImageDetailItem, PromptRow } from '../types'

/**
 * "Create job" modal + submit-validation flow for the /batch-image page.
 * Extracted from useBatchImageGuide.ts (glass-ui-redesign task 6.4);
 * behaviour and copy are unchanged, only relocated.
 */
export interface UseBatchImageSubmitOptions {
  form: { model: string; taskName: string; responseMimeType: string }
  apiKeys: Ref<ApiKey[]>
  loadApiKeys: () => Promise<void>
  requireApiKey: () => ApiKey | null
  availableBatchImageModels: Ref<Array<{ value: string; label: string }>>
  promptRows: Ref<PromptRow[]>
  promptDraft: Ref<string>
  addPromptRow: () => void
  resetCreateDraft: () => void
  parsedItems: ComputedRef<BatchImageSubmitItem[]>
  estimatedOutputCount: ComputedRef<number>
  selectedModelReferenceLimit: ComputedRef<number>
  currentJob: Ref<BatchImageJob | null>
  selectedBatchId: Ref<string>
  selectedBatchApiKeyId: Ref<number>
  items: Ref<BatchImageDetailItem[]>
  upsertJob: (job: BatchImageJob) => void
  loadItems: () => Promise<void>
  startPolling: () => void
  appStore: ReturnType<typeof useAppStore>
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageSubmit(options: UseBatchImageSubmitOptions) {
  const {
    form,
    apiKeys,
    loadApiKeys,
    requireApiKey,
    availableBatchImageModels,
    promptRows,
    promptDraft,
    addPromptRow,
    resetCreateDraft,
    parsedItems,
    estimatedOutputCount,
    selectedModelReferenceLimit,
    currentJob,
    selectedBatchId,
    selectedBatchApiKeyId,
    items,
    upsertJob,
    loadItems,
    startPolling,
    appStore,
    batchImageText,
    batchImageErrorMessage,
  } = options

  const showCreateModal = ref(false)
  const submitting = ref(false)

  function openCreateModal() {
    showCreateModal.value = true
    if (!apiKeys.value.length) {
      void loadApiKeys()
    }
  }

  function closeCreateModal() {
    if (submitting.value) return
    showCreateModal.value = false
    resetCreateDraft()
  }

  function validateForm(): boolean {
    if (!requireApiKey()) return false
    if (!form.model) {
      appStore.showError(availableBatchImageModels.value.length === 0 ? batchImageText('noModelsForKey') : batchImageText('selectModel'))
      return false
    }
    if (parsedItems.value.length === 0) {
      appStore.showError(batchImageText('promptRequired'))
      return false
    }
    if (estimatedOutputCount.value > BATCH_IMAGE_MAX_OUTPUTS_PER_JOB) {
      appStore.showError(batchImageText('tooManyOutputImages'))
      return false
    }
    const refLimit = selectedModelReferenceLimit.value
    if (promptRows.value.some(row => row.reference_images.length > refLimit)) {
      appStore.showError(batchImageText('tooManyReferenceImages'))
      return false
    }
    return true
  }

  async function submitJob() {
    if (submitting.value) return
    if (promptDraft.value.trim()) addPromptRow()
    if (!validateForm()) return
    const key = requireApiKey()
    if (!key) return
    submitting.value = true
    try {
      const job = await submitBatchImageJob(
        key.key,
        {
          model: form.model,
          task_name: form.taskName.trim() || defaultTaskName(),
          image_size: '1K',
          response_mime_type: form.responseMimeType,
          items: parsedItems.value,
        },
        `sub2api-ui-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
      )
      currentJob.value = job
      selectedBatchId.value = job.id
      selectedBatchApiKeyId.value = key.id
      items.value = []
      upsertJob(job)
      showCreateModal.value = false
      resetCreateDraft()
      appStore.showSuccess(batchImageText('submitted'))
      void loadItems()
      startPolling()
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('submitFailed')))
    } finally {
      submitting.value = false
    }
  }

  return {
    showCreateModal,
    submitting,
    openCreateModal,
    closeCreateModal,
    submitJob,
  }
}
