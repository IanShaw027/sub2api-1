import { computed, ref, type ComputedRef, type Ref } from 'vue'
import {
  cancelBatchImageJob,
  deleteBatchImageJobRecord,
  downloadBatchImageZip,
  getBatchImageJob,
  listBatchImageItems,
  saveBlob,
  submitBatchImageJob,
  type BatchImageJob,
} from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { useAppStore } from '@/stores/app'
import { TERMINAL_STATUSES } from '../constants'
import type { BatchImageTextKey } from '../errorMessages'
import { defaultTaskName } from '../format'
import type { BatchImageDetailItem, BatchImageJobRow } from '../types'

/**
 * Selected-job detail state: polling, cancel/download/retry/delete actions
 * for the /batch-image detail modal. Extracted from useBatchImageGuide.ts
 * (glass-ui-redesign task 6.4); behaviour and copy are unchanged, only
 * relocated.
 */
export interface UseBatchImageJobDetailOptions {
  form: { responseMimeType: string; apiKeyId: number }
  currentJob: Ref<BatchImageJob | null>
  selectedBatchId: Ref<string>
  selectedBatchApiKeyId: Ref<number>
  batchJobs: Ref<BatchImageJobRow[]>
  geminiApiKeys: ComputedRef<ApiKey[]>
  expandedParentIds: Ref<Set<string>>
  selectedRows: ComputedRef<BatchImageJobRow[]>
  displayJob: <T extends Pick<BatchImageJob, 'id' | 'parent_batch_id' | 'status' | 'item_count' | 'success_count' | 'fail_count' | 'estimated_cost' | 'hold_amount' | 'actual_cost'>>(job: T) => T
  upsertJob: (job: BatchImageJob) => void
  markJobDownloaded: (batchId: string) => void
  removeJobFromList: (batchId: string) => void
  canDeleteRecord: (job: Pick<BatchImageJob, 'status'>) => boolean
  closeMoreMenu: () => void
  apiKeyForJob: (job: BatchImageJobRow | Pick<BatchImageJob, 'id'>) => ApiKey | null
  applyJobApiKey: (job: BatchImageJobRow | Pick<BatchImageJob, 'id'>) => void
  keyForSelectedBatch: () => ApiKey | null
  requireApiKey: () => ApiKey | null
  items: Ref<BatchImageDetailItem[]>
  loadItems: () => Promise<void>
  clearItemPreviews: () => void
  closePromptPopover: () => void
  appStore: ReturnType<typeof useAppStore>
  t: (key: string, params?: Record<string, unknown>) => string
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageJobDetail(options: UseBatchImageJobDetailOptions) {
  const {
    form,
    currentJob,
    selectedBatchId,
    selectedBatchApiKeyId,
    batchJobs,
    geminiApiKeys,
    expandedParentIds,
    selectedRows,
    displayJob,
    upsertJob,
    markJobDownloaded: markJobDownloadedInList,
    removeJobFromList,
    canDeleteRecord,
    closeMoreMenu,
    apiKeyForJob,
    applyJobApiKey,
    keyForSelectedBatch,
    requireApiKey,
    items,
    loadItems,
    clearItemPreviews,
    closePromptPopover,
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  } = options

  const refreshing = ref(false)
  const cancelling = ref(false)
  const downloading = ref(false)
  const downloadingBatchId = ref('')
  const retryingBatchId = ref('')
  const bulkDownloading = ref(false)
  const bulkDeleting = ref(false)
  const deletingBatchId = ref('')

  let pollTimer: ReturnType<typeof setInterval> | null = null

  const currentDisplayJob = computed(() => {
    if (!currentJob.value) return null
    return displayJob(currentJob.value)
  })

  const selectedDownloadableRows = computed(() =>
    selectedRows.value.filter(job => canDownload(job)),
  )

  function canCancel(job: Pick<BatchImageJob, 'status'>) {
    return !TERMINAL_STATUSES.has(job.status)
  }

  function canDownload(job: Pick<BatchImageJob, 'status' | 'success_count'>) {
    return job.status === 'completed' && job.success_count > 0
  }

  function canRetry(job: Pick<BatchImageJob, 'status' | 'fail_count'>) {
    const display = 'id' in job ? displayJob(job as BatchImageJob) : job
    return TERMINAL_STATUSES.has(display.status) && display.fail_count > 0
  }

  function isDownloadingJob(batchId: string) {
    return downloading.value && downloadingBatchId.value === batchId
  }

  function markJobDownloaded(batchId: string) {
    markJobDownloadedInList(batchId)
    if (currentJob.value?.id === batchId && !currentJob.value.downloaded_at) {
      const updated = batchJobs.value.find(job => job.id === batchId)
      if (updated) currentJob.value = { ...currentJob.value, downloaded_at: updated.downloaded_at }
    }
  }

  function closeDetail() {
    closePromptPopover()
    currentJob.value = null
    selectedBatchId.value = ''
    selectedBatchApiKeyId.value = 0
    items.value = []
    clearItemPreviews()
  }

  function selectJob(batchId: string) {
    const row = batchJobs.value.find(job => job.id === batchId)
    if (row?.api_key_id && geminiApiKeys.value.some(key => key.id === row.api_key_id)) {
      form.apiKeyId = row.api_key_id
      selectedBatchApiKeyId.value = row.api_key_id
    } else {
      selectedBatchApiKeyId.value = 0
    }
    selectedBatchId.value = batchId
    currentJob.value = null
    items.value = []
    void refreshSelected()
    void loadItems()
  }

  function startPolling() {
    stopPolling()
    pollTimer = setInterval(() => {
      if (!currentJob.value || TERMINAL_STATUSES.has(currentJob.value.status)) {
        stopPolling()
        return
      }
      void refreshSelected()
    }, 8000)
  }

  function stopPolling() {
    if (pollTimer) {
      clearInterval(pollTimer)
      pollTimer = null
    }
  }

  async function refreshSelected() {
    if (!selectedBatchId.value) return
    const key = keyForSelectedBatch() || requireApiKey()
    if (!key) return
    refreshing.value = true
    try {
      const job = await getBatchImageJob(key.key, selectedBatchId.value)
      currentJob.value = job
      upsertJob(job)
      if (TERMINAL_STATUSES.has(job.status)) stopPolling()
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('refreshFailed')))
    } finally {
      refreshing.value = false
    }
  }

  async function refreshDetail() {
    await Promise.all([
      refreshSelected(),
      loadItems(),
    ])
  }

  async function cancelSelected() {
    if (!currentJob.value) return
    const key = keyForSelectedBatch() || requireApiKey()
    if (!key) return
    if (!window.confirm(batchImageText('cancelConfirm'))) return
    cancelling.value = true
    try {
      const job = await cancelBatchImageJob(key.key, currentJob.value.id)
      currentJob.value = job
      upsertJob(job)
      appStore.showSuccess(batchImageText('cancelled'))
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('cancelFailed')))
    } finally {
      cancelling.value = false
    }
  }

  async function downloadSelected() {
    if (!currentJob.value) return
    await downloadJob(currentJob.value)
  }

  async function retrySelected() {
    if (!currentJob.value) return
    await retryFailedJob(currentJob.value)
  }

  async function retryFailedJob(job: BatchImageJobRow | BatchImageJob) {
    if (!canRetry(job) || retryingBatchId.value) return
    closeMoreMenu()
    const key = apiKeyForJob(job) || keyForSelectedBatch() || requireApiKey()
    if (!key) return
    retryingBatchId.value = job.id
    try {
      const sourceItems = await ensureItemsForRetry(key.key, job.id)
      const failedItems = sourceItems
        .filter(item => item.status === 'failed')
        .map(item => ({ custom_id: retryCustomID(item.custom_id), prompt: String(item.prompt_preview || '').trim() }))
        .filter(item => item.prompt)
      if (failedItems.length === 0) {
        appStore.showError(batchImageText('retryMissingPrompts'))
        return
      }
      const retryJob = await submitBatchImageJob(
        key.key,
        {
          model: job.model,
          task_name: `${job.task_name || defaultTaskName()} ${t('batchImage.messages.retryTaskNameSuffix')}`,
          parent_batch_id: rootBatchIdForRetry(job),
          provider: job.provider,
          image_size: '1K',
          response_mime_type: form.responseMimeType,
          items: failedItems,
        },
        `sub2api-ui-retry-${job.id}-${Date.now()}-${Math.random().toString(36).slice(2, 10)}`,
      )
      currentJob.value = retryJob
      selectedBatchId.value = retryJob.id
      selectedBatchApiKeyId.value = key.id
      items.value = []
      upsertJob(retryJob)
      if (retryJob.parent_batch_id) {
        expandedParentIds.value = new Set([...expandedParentIds.value, retryJob.parent_batch_id])
      }
      appStore.showSuccess(batchImageText('retrySubmitted'))
      void loadItems()
      startPolling()
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('retryFailed')))
    } finally {
      retryingBatchId.value = ''
    }
  }

  async function ensureItemsForRetry(apiKey: string, batchId: string) {
    if (selectedBatchId.value === batchId && items.value.length > 0) {
      return items.value
    }
    const result = await listBatchImageItems(apiKey, batchId)
    return result.data || []
  }

  function retryCustomID(customID: string) {
    const base = String(customID || 'item').replace(/[^\w.-]+/g, '_').replace(/^_+|_+$/g, '') || 'item'
    return `${base}_retry_${Date.now().toString(36)}`
  }

  function rootBatchIdForRetry(job: BatchImageJobRow | BatchImageJob) {
    return job.parent_batch_id || job.id
  }

  async function downloadJob(job: (BatchImageJobRow | Pick<BatchImageJob, 'id'>)) {
    if (downloading.value) return
    closeMoreMenu()
    applyJobApiKey(job)
    const key = apiKeyForJob(job) || requireApiKey()
    if (!key) return
    downloading.value = true
    downloadingBatchId.value = job.id
    try {
      const blob = await downloadBatchImageZip(key.key, job.id)
      saveBlob(blob, `${job.id}.zip`)
      markJobDownloaded(job.id)
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('downloadFailed')))
    } finally {
      downloading.value = false
      downloadingBatchId.value = ''
    }
  }

  async function downloadSelectedJobs() {
    if (bulkDownloading.value || selectedDownloadableRows.value.length === 0) return
    bulkDownloading.value = true
    try {
      for (const row of selectedDownloadableRows.value) {
        const key = apiKeyForJob(row)
        if (!key) continue
        downloading.value = true
        downloadingBatchId.value = row.id
        const blob = await downloadBatchImageZip(key.key, row.id)
        saveBlob(blob, `${row.id}.zip`)
        markJobDownloaded(row.id)
      }
      appStore.showSuccess(batchImageText('batchDownloadStarted'))
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('downloadFailed')))
    } finally {
      bulkDownloading.value = false
      downloading.value = false
      downloadingBatchId.value = ''
    }
  }

  async function deleteJob(job: BatchImageJobRow) {
    if (!canDeleteRecord(job) || deletingBatchId.value) return
    closeMoreMenu()
    const key = apiKeyForJob(job)
    if (!key) return
    if (!window.confirm(batchImageText('deleteConfirm'))) return
    deletingBatchId.value = job.id
    try {
      await deleteBatchImageJobRecord(key.key, job.id)
      removeJobFromList(job.id)
      if (currentJob.value?.id === job.id) closeDetail()
      appStore.showSuccess(batchImageText('deleted'))
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('deleteFailed')))
    } finally {
      deletingBatchId.value = ''
    }
  }

  async function deleteSelectedJobs() {
    const rows = selectedRows.value.filter(job => canDeleteRecord(job))
    if (bulkDeleting.value || rows.length === 0) return
    if (!window.confirm(batchImageText('deleteSelectedConfirm'))) return
    bulkDeleting.value = true
    try {
      for (const row of rows) {
        const key = apiKeyForJob(row)
        if (!key) continue
        deletingBatchId.value = row.id
        await deleteBatchImageJobRecord(key.key, row.id)
        removeJobFromList(row.id)
        if (currentJob.value?.id === row.id) closeDetail()
      }
      appStore.showSuccess(batchImageText('deleted'))
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('deleteFailed')))
    } finally {
      bulkDeleting.value = false
      deletingBatchId.value = ''
    }
  }

  return {
    refreshing,
    cancelling,
    downloading,
    downloadingBatchId,
    retryingBatchId,
    bulkDownloading,
    bulkDeleting,
    deletingBatchId,
    currentDisplayJob,
    selectedDownloadableRows,
    canCancel,
    canDownload,
    canRetry,
    isDownloadingJob,
    closeDetail,
    selectJob,
    startPolling,
    stopPolling,
    refreshSelected,
    refreshDetail,
    cancelSelected,
    downloadSelected,
    retrySelected,
    retryFailedJob,
    downloadJob,
    downloadSelectedJobs,
    deleteJob,
    deleteSelectedJobs,
  }
}
