import { ref, watch, type ComputedRef } from 'vue'
import { listBatchImageModels } from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { BatchImageTextKey } from '../errorMessages'

/**
 * Available-model loading for the currently selected API key on the
 * /batch-image page. Extracted from useBatchImageGuide.ts (glass-ui-redesign
 * task 6.4); behaviour is unchanged, only relocated.
 */
export interface UseBatchImageModelsOptions {
  form: { apiKeyId: number; model: string }
  selectedApiKey: ComputedRef<ApiKey | null>
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageModels(options: UseBatchImageModelsOptions) {
  const { form, selectedApiKey, batchImageText, batchImageErrorMessage } = options

  const availableBatchImageModels = ref<Array<{ value: string; label: string }>>([])
  const modelLoadError = ref('')
  const loadingModels = ref(false)
  let modelRequestSeq = 0

  async function loadAvailableModels() {
    const key = selectedApiKey.value
    const requestID = ++modelRequestSeq
    modelLoadError.value = ''
    availableBatchImageModels.value = []
    form.model = ''
    if (!key) return

    loadingModels.value = true
    try {
      const result = await listBatchImageModels(key.key)
      if (requestID !== modelRequestSeq) return
      const seen = new Set<string>()
      availableBatchImageModels.value = (result.data || [])
        .map(model => String(model.id || '').trim())
        .filter((model) => {
          if (!model || seen.has(model)) return false
          seen.add(model)
          return true
        })
        .map(model => ({ value: model, label: model }))
      form.model = availableBatchImageModels.value[0]?.value || ''
    } catch (error: any) {
      if (requestID !== modelRequestSeq) return
      modelLoadError.value = batchImageErrorMessage(error, batchImageText('loadModelsFailed'))
    } finally {
      if (requestID === modelRequestSeq) {
        loadingModels.value = false
      }
    }
  }

  watch(
    () => form.apiKeyId,
    () => {
      void loadAvailableModels()
    },
  )

  return {
    availableBatchImageModels,
    modelLoadError,
    loadingModels,
    loadAvailableModels,
  }
}
