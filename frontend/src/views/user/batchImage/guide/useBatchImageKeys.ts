import { computed, ref, type Ref } from 'vue'
import { keysAPI } from '@/api'
import type { ApiKey } from '@/types'
import type { BatchImageJob } from '@/api/batchImage'
import type { SelectOption } from '@/components/common/Select.vue'
import type { BatchImageJobRow } from '../types'
import type { useAppStore } from '@/stores/app'
import type { BatchImageTextKey } from '../errorMessages'

/**
 * API-key selection/resolution for the /batch-image page (Gemini keys only).
 * Extracted from useBatchImageGuide.ts (glass-ui-redesign task 6.4); behaviour
 * and copy are unchanged, only relocated.
 */
export interface UseBatchImageKeysOptions {
  form: { apiKeyId: number; model: string }
  filters: { apiKeyId: string }
  selectedBatchApiKeyId: Ref<number>
  clearAvailableModels: () => void
  appStore: ReturnType<typeof useAppStore>
  t: (key: string) => string
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageKeys(options: UseBatchImageKeysOptions) {
  const { form, filters, selectedBatchApiKeyId, clearAvailableModels, appStore, t, batchImageText, batchImageErrorMessage } = options

  const apiKeys = ref<ApiKey[]>([])
  const loadingKeys = ref(false)

  const geminiApiKeys = computed(() =>
    apiKeys.value.filter((key) =>
      key.status === 'active' &&
      key.group?.platform === 'gemini' &&
      key.group?.allow_batch_image_generation === true,
    ),
  )

  const selectedApiKey = computed(() =>
    geminiApiKeys.value.find((key) => key.id === Number(form.apiKeyId)) || null,
  )

  const filteredApiKeys = computed(() => {
    const selectedFilterID = Number(filters.apiKeyId || 0)
    if (!selectedFilterID) return geminiApiKeys.value
    return geminiApiKeys.value.filter(key => key.id === selectedFilterID)
  })

  const apiKeyFilterOptions = computed<SelectOption[]>(() => [
    { value: '', label: t('batchImage.filters.allApiKeys') },
    ...geminiApiKeys.value.map(key => ({
      value: String(key.id),
      label: key.name || `API Key #${key.id}`,
    })),
  ])

  async function loadApiKeys() {
    loadingKeys.value = true
    try {
      const response = await keysAPI.list(1, 100, { status: 'active', sort_by: 'created_at', sort_order: 'desc' })
      apiKeys.value = response.items || []
      if (!selectedApiKey.value && geminiApiKeys.value.length > 0) {
        form.apiKeyId = geminiApiKeys.value[0].id
      }
      if (filters.apiKeyId && !geminiApiKeys.value.some(key => String(key.id) === filters.apiKeyId)) {
        filters.apiKeyId = ''
      }
      if (!selectedApiKey.value) {
        clearAvailableModels()
        form.model = ''
      }
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('loadKeysFailed')))
    } finally {
      loadingKeys.value = false
    }
  }

  function requireApiKey(): ApiKey | null {
    if (!selectedApiKey.value) {
      appStore.showError(batchImageText('selectApiKey'))
      return null
    }
    return selectedApiKey.value
  }

  function keyForSelectedBatch(): ApiKey | null {
    if (selectedBatchApiKeyId.value) {
      const key = geminiApiKeys.value.find(item => item.id === selectedBatchApiKeyId.value)
      if (key) return key
    }
    return selectedApiKey.value
  }

  function applyJobApiKey(job: BatchImageJobRow | Pick<BatchImageJob, 'id'>) {
    if ('api_key_id' in job && job.api_key_id && geminiApiKeys.value.some(key => key.id === job.api_key_id)) {
      form.apiKeyId = job.api_key_id
    }
  }

  function apiKeyForJob(job: BatchImageJobRow | Pick<BatchImageJob, 'id'>): ApiKey | null {
    if ('api_key_id' in job && job.api_key_id) {
      return geminiApiKeys.value.find(key => key.id === job.api_key_id) || null
    }
    return selectedApiKey.value
  }

  return {
    apiKeys,
    loadingKeys,
    geminiApiKeys,
    selectedApiKey,
    filteredApiKeys,
    apiKeyFilterOptions,
    loadApiKeys,
    requireApiKey,
    keyForSelectedBatch,
    applyJobApiKey,
    apiKeyForJob,
  }
}
