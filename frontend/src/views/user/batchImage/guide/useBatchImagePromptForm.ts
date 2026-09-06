import { computed, ref, watch } from 'vue'
import type { BatchImageSubmitItem } from '@/api/batchImage'
import type { useAppStore } from '@/stores/app'
import { BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM } from '../constants'
import type { PromptRow, ReferenceImageDraft } from '../types'

/**
 * Prompt-row / reference-image draft state for the "create job" flow on the
 * /batch-image page. Extracted from useBatchImageGuide.ts (glass-ui-redesign
 * task 6.4); behaviour and copy are unchanged, only relocated.
 */
export interface UseBatchImagePromptFormOptions {
  form: { model: string }
  appStore: ReturnType<typeof useAppStore>
  t: (key: string, params?: Record<string, unknown>) => string
}

export function useBatchImagePromptForm(options: UseBatchImagePromptFormOptions) {
  const { form, appStore, t } = options

  const promptRows = ref<PromptRow[]>([])
  const promptDraft = ref('')
  const customIdDraft = ref('')
  const outputCountDraft = ref(1)
  const referenceImageDrafts = ref<ReferenceImageDraft[]>([])

  const selectedModelReferenceLimit = computed(() => referenceImageLimitForModel(form.model))

  const estimatedOutputCount = computed(() =>
    promptRows.value.reduce((sum, row) => sum + normalizeOutputCount(row.output_count), 0),
  )

  const parsedItems = computed<BatchImageSubmitItem[]>(() => {
    const used = new Set<string>()
    return promptRows.value
      .map((row, index) => {
        const customID = uniqueCustomID(row.custom_id || `img_${String(index + 1).padStart(3, '0')}`, used, index)
        const item: BatchImageSubmitItem = { custom_id: customID, prompt: row.prompt.trim() }
        const outputCount = normalizeOutputCount(row.output_count)
        if (outputCount > 1) {
          item.output_count = outputCount
        }
        if (row.reference_images.length) {
          item.reference_images = row.reference_images
        }
        return item
      })
      .filter(item => item.prompt)
  })

  function referenceImageLimitForModel(model: string) {
    const normalized = String(model || '').toLowerCase()
    if (normalized.includes('pro-image')) return 14
    if (normalized.includes('flash-image')) return 3
    return 0
  }

  function uniqueCustomID(raw: string, used: Set<string>, index: number): string {
    const base = raw.replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || `img_${String(index + 1).padStart(3, '0')}`
    let candidate = base
    let suffix = 2
    while (used.has(candidate)) {
      candidate = `${base}_${suffix}`
      suffix += 1
    }
    used.add(candidate)
    return candidate
  }

  function normalizeOutputCount(value: unknown): number {
    const parsed = Math.floor(Number(value || 1))
    if (!Number.isFinite(parsed)) return 1
    return Math.min(BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM, Math.max(1, parsed))
  }

  function addPromptRow() {
    const prompt = promptDraft.value.trim()
    if (!prompt) return
    const outputCount = normalizeOutputCount(outputCountDraft.value)
    const used = new Set(promptRows.value.map(row => row.custom_id))
    const customID = uniqueCustomID(customIdDraft.value || `img_${String(promptRows.value.length + 1).padStart(3, '0')}`, used, promptRows.value.length)
    promptRows.value = [
      ...promptRows.value,
      {
        localId: `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`,
        custom_id: customID,
        prompt,
        output_count: outputCount,
        reference_images: referenceImageDrafts.value.map(({ name: _name, size: _size, ...ref }) => ref),
      },
    ]
    promptDraft.value = ''
    customIdDraft.value = ''
    outputCountDraft.value = 1
    referenceImageDrafts.value = []
  }

  function removePromptRow(index: number) {
    promptRows.value = promptRows.value.filter((_, currentIndex) => currentIndex !== index)
  }

  function removeReferenceImageDraft(index: number) {
    referenceImageDrafts.value = referenceImageDrafts.value.filter((_, currentIndex) => currentIndex !== index)
  }

  async function handleReferenceImageFiles(event: Event) {
    const input = event.target as HTMLInputElement
    const files = Array.from(input.files || [])
    input.value = ''
    if (files.length === 0) return
    const limit = selectedModelReferenceLimit.value
    if (limit <= 0) {
      appStore.showError(t('batchImage.create.modelNoReferenceImages'))
      return
    }
    const slots = Math.max(0, limit - referenceImageDrafts.value.length)
    if (slots <= 0) {
      appStore.showError(t('batchImage.create.refLimitReached', { limit }))
      return
    }
    const accepted = files.slice(0, slots)
    if (accepted.length < files.length) {
      appStore.showError(t('batchImage.create.refLimitExceededIgnored', { limit }))
    }
    const next: ReferenceImageDraft[] = []
    for (const file of accepted) {
      if (!['image/png', 'image/jpeg', 'image/webp'].includes(file.type)) {
        appStore.showError(t('batchImage.create.refFormatUnsupported'))
        continue
      }
      if (file.size > 10 * 1024 * 1024) {
        appStore.showError(t('batchImage.create.refFileTooLarge', { name: file.name }))
        continue
      }
      const data = await readFileAsBase64(file)
      next.push({
        id: file.name,
        type: 'reference',
        mime_type: file.type,
        data,
        name: file.name,
        size: file.size,
      })
    }
    referenceImageDrafts.value = [...referenceImageDrafts.value, ...next]
  }

  function readFileAsBase64(file: File): Promise<string> {
    return new Promise((resolve, reject) => {
      const reader = new FileReader()
      reader.onerror = () => reject(reader.error || new Error('Failed to read file'))
      reader.onload = () => {
        const result = String(reader.result || '')
        resolve(result.includes(',') ? result.slice(result.indexOf(',') + 1) : result)
      }
      reader.readAsDataURL(file)
    })
  }

  function resetCreateDraft() {
    promptRows.value = []
    promptDraft.value = ''
    customIdDraft.value = ''
    outputCountDraft.value = 1
    referenceImageDrafts.value = []
  }

  watch(
    () => form.model,
    () => {
      const limit = selectedModelReferenceLimit.value
      if (limit <= 0) {
        referenceImageDrafts.value = []
        return
      }
      if (referenceImageDrafts.value.length > limit) {
        referenceImageDrafts.value = referenceImageDrafts.value.slice(0, limit)
      }
    },
  )

  return {
    promptRows,
    promptDraft,
    customIdDraft,
    outputCountDraft,
    referenceImageDrafts,
    selectedModelReferenceLimit,
    estimatedOutputCount,
    parsedItems,
    addPromptRow,
    removePromptRow,
    removeReferenceImageDraft,
    handleReferenceImageFiles,
    resetCreateDraft,
  }
}
