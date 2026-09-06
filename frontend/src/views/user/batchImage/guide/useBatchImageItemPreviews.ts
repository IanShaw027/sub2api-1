import { computed, reactive, ref, type ComputedRef, type Ref } from 'vue'
import { getBatchImageItemContent, listBatchImageItems, type BatchImageItem } from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { useAppStore } from '@/stores/app'
import type { BatchImageTextKey } from '../errorMessages'
import type { PreviewCache } from '../previewCache'
import type { BatchImageDetailItem, BatchImageJobRow } from '../types'

/**
 * Detail-item list + image-preview cache/loading for the /batch-image detail
 * modal. Extracted from useBatchImageGuide.ts (glass-ui-redesign task 6.4);
 * behaviour and copy are unchanged, only relocated.
 */
export interface UseBatchImageItemPreviewsOptions {
  currentJob: Ref<import('@/api/batchImage').BatchImageJob | null>
  selectedBatchId: Ref<string>
  batchJobs: Ref<BatchImageJobRow[]>
  childrenByParent: ComputedRef<Map<string, BatchImageJobRow[]>>
  toJobRow: (job: import('@/api/batchImage').BatchImageJob, key?: ApiKey | null) => BatchImageJobRow
  keyForSelectedBatch: () => ApiKey | null
  requireApiKey: () => ApiKey | null
  selectedApiKey: ComputedRef<ApiKey | null>
  previewCache: PreviewCache
  closePromptPopover: () => void
  appStore: ReturnType<typeof useAppStore>
  t: (key: string, params?: Record<string, unknown>) => string
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageItemPreviews(options: UseBatchImageItemPreviewsOptions) {
  const {
    currentJob,
    selectedBatchId,
    batchJobs,
    childrenByParent,
    toJobRow,
    keyForSelectedBatch,
    requireApiKey,
    selectedApiKey,
    previewCache,
    closePromptPopover,
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  } = options

  const items = ref<BatchImageDetailItem[]>([])
  const loadingItems = ref(false)
  const itemPreviewUrls = reactive<Record<string, string>>({})
  const previewLoadingIds = ref(new Set<string>())
  const previewErrorIds = ref(new Set<string>())
  const previewImageItem = ref<BatchImageItem | null>(null)

  const previewImageUrl = computed(() => {
    const item = previewImageItem.value
    if (!item) return ''
    return itemPreviewUrls[itemPreviewKey(item)] || ''
  })

  const recoveredOriginalCustomIds = computed(() => {
    const rootBatchId = detailRootBatchId()
    if (!rootBatchId) return new Set<string>()
    const ids = new Set<string>()
    for (const item of items.value) {
      if (!isChildDetailItem(item) || !isSuccessfulImageItem(item)) continue
      const sourceCustomID = retrySourceCustomID(item.custom_id)
      if (sourceCustomID) ids.add(sourceCustomID)
    }
    return ids
  })

  function canLoadItemPreview(item: BatchImageItem) {
    return (item.status === 'succeeded' || item.status === 'success') && item.image_count > 0
  }

  function isSuccessfulImageItem(item: Pick<BatchImageItem, 'status' | 'image_count'>) {
    return (item.status === 'succeeded' || item.status === 'success') && item.image_count > 0
  }

  function detailRootBatchId() {
    return currentJob.value?.parent_batch_id || selectedBatchId.value || currentJob.value?.id || ''
  }

  function isChildDetailItem(item: Pick<BatchImageDetailItem, 'batch_id'>) {
    const rootBatchId = detailRootBatchId()
    return Boolean(rootBatchId && item.batch_id && item.batch_id !== rootBatchId)
  }

  function retrySourceCustomID(customID: string) {
    return String(customID || '').replace(/(?:_retry_[a-z0-9]+)+$/i, '')
  }

  function isRecoveredOriginalFailure(item: BatchImageDetailItem) {
    const rootBatchId = detailRootBatchId()
    return Boolean(
      rootBatchId
      && item.batch_id === rootBatchId
      && item.status === 'failed'
      && recoveredOriginalCustomIds.value.has(item.custom_id),
    )
  }

  function detailItemRowClass(item: BatchImageDetailItem) {
    if (isRecoveredOriginalFailure(item)) {
      return 'bg-surface-2 text-muted hover:bg-surface-2   '
    }
    return 'hover:bg-surface-2 '
  }

  function itemPreviewKey(item: Pick<BatchImageItem, 'batch_id' | 'custom_id'>) {
    return previewCache.previewCacheKey(item.batch_id || selectedBatchId.value || currentJob.value?.id || '', item.custom_id, 0)
  }

  async function hydrateCachedItemPreviews(detailItems: BatchImageDetailItem[]) {
    const previewableItems = detailItems.filter(item => canLoadItemPreview(item))
    if (!previewableItems.length || !previewCache.previewCacheSupported()) return

    await Promise.all(previewableItems.map(async (item) => {
      const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
      const previewKey = itemPreviewKey(item)
      if (!batchId || itemPreviewUrls[previewKey] || previewErrorIds.value.has(previewKey)) return
      const cached = await previewCache.getCachedPreviewBlob(previewCache.previewCacheKey(batchId, item.custom_id, 0)).catch(() => null)
      if (!cached || itemPreviewUrls[previewKey]) return
      itemPreviewUrls[previewKey] = URL.createObjectURL(cached)
    }))
  }

  function detailJobsForBatch(batchId: string): BatchImageJobRow[] {
    const row = batchJobs.value.find(job => job.id === batchId)
    const base = row || (currentJob.value && currentJob.value.id === batchId ? toJobRow(currentJob.value, keyForSelectedBatch() || selectedApiKey.value) : null)
    if (!base) return []
    if (base.parent_batch_id) return [base]
    return [base, ...(childrenByParent.value.get(base.id) || [])]
  }

  function detailSourceName(job: Pick<BatchImageJobRow, 'id' | 'task_name' | 'parent_batch_id'>, rootBatchId: string) {
    const name = job.task_name || job.id
    if (job.id === rootBatchId) return t('batchImage.detail.mainTask', { name })
    return t('batchImage.detail.childTask', { name })
  }

  async function loadItems() {
    const batchId = selectedBatchId.value || currentJob.value?.id || ''
    if (!batchId) return
    const key = keyForSelectedBatch() || requireApiKey()
    if (!key) return
    loadingItems.value = true
    try {
      clearItemPreviews()
      const jobs = detailJobsForBatch(batchId)
      const results = await Promise.all(jobs.map(async (job) => {
        const result = await listBatchImageItems(key.key, job.id)
        return (result.data || []).map(item => ({
          ...item,
          batch_id: job.id,
          source_task_name: detailSourceName(job, batchId),
        }))
      }))
      const detailItems = results.flat()
      items.value = detailItems
      void hydrateCachedItemPreviews(detailItems)
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('loadItemsFailed')))
    } finally {
      loadingItems.value = false
    }
  }

  async function loadItemPreview(item: BatchImageItem) {
    const batchId = item.batch_id || selectedBatchId.value || currentJob.value?.id || ''
    const previewKey = itemPreviewKey(item)
    if (!batchId || !canLoadItemPreview(item) || (itemPreviewUrls[previewKey] && !previewErrorIds.value.has(previewKey))) return
    const key = keyForSelectedBatch() || requireApiKey()
    if (!key) return
    const cacheKey = previewCache.previewCacheKey(batchId, item.custom_id, 0)
    previewLoadingIds.value = new Set([...previewLoadingIds.value, previewKey])
    try {
      previewErrorIds.value = new Set([...previewErrorIds.value].filter(id => id !== previewKey))
      if (itemPreviewUrls[previewKey]) {
        URL.revokeObjectURL(itemPreviewUrls[previewKey])
        delete itemPreviewUrls[previewKey]
      }
      const cached = await previewCache.getCachedPreviewBlob(cacheKey)
      if (cached) {
        itemPreviewUrls[previewKey] = URL.createObjectURL(cached)
        return
      }
      const blob = await getBatchImageItemContent(key.key, batchId, item.custom_id, 0)
      const thumbnail = await previewCache.createThumbnailBlob(blob).catch(() => blob)
      itemPreviewUrls[previewKey] = URL.createObjectURL(thumbnail)
      if (thumbnail !== blob || thumbnail.size <= 1024 * 1024) {
        void previewCache.putCachedPreviewBlob(cacheKey, thumbnail)
      }
    } catch (error: any) {
      previewErrorIds.value = new Set([...previewErrorIds.value, previewKey])
      appStore.showError(batchImageErrorMessage(error, batchImageText('loadPreviewFailed')))
    } finally {
      const next = new Set(previewLoadingIds.value)
      next.delete(previewKey)
      previewLoadingIds.value = next
    }
  }

  function openImagePreview(item: BatchImageItem) {
    const previewKey = itemPreviewKey(item)
    if (!itemPreviewUrls[previewKey] || previewErrorIds.value.has(previewKey)) return
    previewImageItem.value = item
  }

  function closeImagePreview() {
    previewImageItem.value = null
  }

  function handlePreviewError(customID: string) {
    if (itemPreviewUrls[customID]) {
      URL.revokeObjectURL(itemPreviewUrls[customID])
      delete itemPreviewUrls[customID]
    }
    previewErrorIds.value = new Set([...previewErrorIds.value, customID])
  }

  function clearItemPreviews() {
    closePromptPopover()
    for (const url of Object.values(itemPreviewUrls)) {
      if (url) URL.revokeObjectURL(url)
    }
    for (const key of Object.keys(itemPreviewUrls)) {
      delete itemPreviewUrls[key]
    }
    previewLoadingIds.value = new Set()
    previewErrorIds.value = new Set()
    previewImageItem.value = null
  }

  return {
    items,
    loadingItems,
    itemPreviewUrls,
    previewLoadingIds,
    previewErrorIds,
    previewImageItem,
    previewImageUrl,
    recoveredOriginalCustomIds,
    canLoadItemPreview,
    isSuccessfulImageItem,
    detailRootBatchId,
    isChildDetailItem,
    retrySourceCustomID,
    isRecoveredOriginalFailure,
    detailItemRowClass,
    itemPreviewKey,
    hydrateCachedItemPreviews,
    detailJobsForBatch,
    detailSourceName,
    loadItems,
    loadItemPreview,
    openImagePreview,
    closeImagePreview,
    handlePreviewError,
    clearItemPreviews,
  }
}
