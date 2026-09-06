import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useClipboard } from '@/composables/useClipboard'
import { useAppStore } from '@/stores/app'
import type { BatchImageJob } from '@/api/batchImage'
import {
  BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM,
  BATCH_IMAGE_MAX_OUTPUTS_PER_JOB,
  batchPageSizeOptions,
  outputCountOptions,
} from './constants'
import { buildAgentInstruction } from './agentInstruction'
import { createPreviewCache } from './previewCache'
import { useBatchImageErrorMessages } from './errorMessages'
import { defaultTaskName, formatDate, statusBadgeClass } from './format'
import { useBatchImagePromptPopover } from './guide/useBatchImagePromptPopover'
import { useBatchImageKeys } from './guide/useBatchImageKeys'
import { useBatchImageModels } from './guide/useBatchImageModels'
import { useBatchImagePromptForm } from './guide/useBatchImagePromptForm'
import { useBatchImageJobsList } from './guide/useBatchImageJobsList'
import { useBatchImageItemPreviews } from './guide/useBatchImageItemPreviews'
import { useBatchImageJobDetail } from './guide/useBatchImageJobDetail'
import { useBatchImageSubmit } from './guide/useBatchImageSubmit'
import { useBatchImageLabels } from './guide/useBatchImageLabels'

/**
 * Extracted state/logic for the user-facing /batch-image page.
 * Moved out of BatchImageGuideView.vue (glass-ui-redesign task 12.12) so the
 * view file can stay near the ≤400-line budget; behaviour is unchanged.
 *
 * Split further under ./guide/*.ts (glass-ui-redesign task 6.4) so this file
 * stays under the line budget; this function is now only a thin composition
 * root that wires the extracted composables together. Behaviour, i18n keys
 * and the returned object's shape are unchanged.
 */
export function useBatchImageGuide() {
  const appStore = useAppStore()
  const { copyToClipboard } = useClipboard()
  const { t, locale } = useI18n()
  const previewCache = createPreviewCache()

  function isZhLocale() {
    return String(locale.value || '').toLowerCase().startsWith('zh')
  }

  const { batchImageText, batchImageErrorMessage } = useBatchImageErrorMessages(
    (key, params) => t(key, params as any),
    isZhLocale,
  )

  const promptPopoverApi = useBatchImagePromptPopover({ copyToClipboard, t })

  const form = reactive({
    apiKeyId: 0,
    taskName: '',
    model: '',
    responseMimeType: 'image/png',
  })

  const filters = reactive({
    taskName: '',
    apiKeyId: '',
    status: '',
    downloaded: '',
  })

  const currentJob = ref<BatchImageJob | null>(null)
  const selectedBatchId = ref('')
  const selectedBatchApiKeyId = ref(0)
  const showGuideModal = ref(false)

  let previewCacheCleanupTimer: ReturnType<typeof setInterval> | null = null
  let clearAvailableModelsImpl: () => void = () => {}

  const keys = useBatchImageKeys({
    form,
    filters,
    selectedBatchApiKeyId,
    clearAvailableModels: () => clearAvailableModelsImpl(),
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  })

  const models = useBatchImageModels({
    form,
    selectedApiKey: keys.selectedApiKey,
    batchImageText,
    batchImageErrorMessage,
  })
  clearAvailableModelsImpl = () => {
    models.availableBatchImageModels.value = []
  }

  const promptForm = useBatchImagePromptForm({ form, appStore, t })

  const jobsList = useBatchImageJobsList({
    filters,
    filteredApiKeys: keys.filteredApiKeys,
    selectedApiKey: keys.selectedApiKey,
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  })

  const itemPreviews = useBatchImageItemPreviews({
    currentJob,
    selectedBatchId,
    batchJobs: jobsList.batchJobs,
    childrenByParent: jobsList.childrenByParent,
    toJobRow: jobsList.toJobRow,
    keyForSelectedBatch: keys.keyForSelectedBatch,
    requireApiKey: keys.requireApiKey,
    selectedApiKey: keys.selectedApiKey,
    previewCache,
    closePromptPopover: promptPopoverApi.closePromptPopover,
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  })

  const jobDetail = useBatchImageJobDetail({
    form,
    currentJob,
    selectedBatchId,
    selectedBatchApiKeyId,
    batchJobs: jobsList.batchJobs,
    geminiApiKeys: keys.geminiApiKeys,
    expandedParentIds: jobsList.expandedParentIds,
    selectedRows: jobsList.selectedRows,
    displayJob: jobsList.displayJob,
    upsertJob: jobsList.upsertJob,
    markJobDownloaded: jobsList.markJobDownloaded,
    removeJobFromList: jobsList.removeJobFromList,
    canDeleteRecord: jobsList.canDeleteRecord,
    closeMoreMenu: jobsList.closeMoreMenu,
    apiKeyForJob: keys.apiKeyForJob,
    applyJobApiKey: keys.applyJobApiKey,
    keyForSelectedBatch: keys.keyForSelectedBatch,
    requireApiKey: keys.requireApiKey,
    items: itemPreviews.items,
    loadItems: itemPreviews.loadItems,
    clearItemPreviews: itemPreviews.clearItemPreviews,
    closePromptPopover: promptPopoverApi.closePromptPopover,
    appStore,
    t,
    batchImageText,
    batchImageErrorMessage,
  })

  const submit = useBatchImageSubmit({
    form,
    apiKeys: keys.apiKeys,
    loadApiKeys: keys.loadApiKeys,
    requireApiKey: keys.requireApiKey,
    availableBatchImageModels: models.availableBatchImageModels,
    promptRows: promptForm.promptRows,
    promptDraft: promptForm.promptDraft,
    addPromptRow: promptForm.addPromptRow,
    resetCreateDraft: promptForm.resetCreateDraft,
    parsedItems: promptForm.parsedItems,
    estimatedOutputCount: promptForm.estimatedOutputCount,
    selectedModelReferenceLimit: promptForm.selectedModelReferenceLimit,
    currentJob,
    selectedBatchId,
    selectedBatchApiKeyId,
    items: itemPreviews.items,
    upsertJob: jobsList.upsertJob,
    loadItems: itemPreviews.loadItems,
    startPolling: jobDetail.startPolling,
    appStore,
    batchImageText,
    batchImageErrorMessage,
  })

  const labels = useBatchImageLabels({
    t,
    isRecoveredOriginalFailure: itemPreviews.isRecoveredOriginalFailure,
    itemPreviewUrls: itemPreviews.itemPreviewUrls,
    itemPreviewKey: itemPreviews.itemPreviewKey,
  })

  const endpointBase = computed(() => {
    const configured = appStore.apiBaseUrl?.trim()
    if (configured) return configured.replace(/\/+$/, '')
    if (typeof window !== 'undefined') return window.location.origin.replace(/\/+$/, '')
    return '<你的 Sub2API API 端点>'
  })

  const agentInstruction = computed(() => buildAgentInstruction(endpointBase.value))

  function copyInstruction() {
    void copyToClipboard(agentInstruction.value, batchImageText('copiedInstruction'))
  }

  async function refreshPage() {
    await keys.loadApiKeys()
    await jobsList.loadBatchJobs()
  }

  onMounted(() => {
    void appStore.fetchPublicSettings()
    void refreshPage()
    void previewCache.cleanupPreviewCache()
    previewCacheCleanupTimer = setInterval(() => {
      void previewCache.cleanupPreviewCache()
    }, 60 * 60 * 1000)
    document.addEventListener('click', jobsList.closeMoreMenu)
    window.addEventListener('resize', jobsList.closeMoreMenu)
    window.addEventListener('scroll', jobsList.closeMoreMenu, true)
    window.addEventListener('resize', promptPopoverApi.closePromptPopover)
    window.addEventListener('scroll', promptPopoverApi.closePromptPopover, true)
  })

  onBeforeUnmount(() => {
    jobDetail.stopPolling()
    if (previewCacheCleanupTimer) {
      clearInterval(previewCacheCleanupTimer)
      previewCacheCleanupTimer = null
    }
    itemPreviews.clearItemPreviews()
    document.removeEventListener('click', jobsList.closeMoreMenu)
    window.removeEventListener('resize', jobsList.closeMoreMenu)
    window.removeEventListener('scroll', jobsList.closeMoreMenu, true)
    window.removeEventListener('resize', promptPopoverApi.closePromptPopover)
    window.removeEventListener('scroll', promptPopoverApi.closePromptPopover, true)
  })

  return {
    // constants passed through for the template
    BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM,
    BATCH_IMAGE_MAX_OUTPUTS_PER_JOB,
    outputCountOptions,
    batchPageSizeOptions,
    // i18n helpers
    t,
    batchImageText,
    // columns / filter options
    columns: jobsList.columns,
    statusFilterOptions: jobsList.statusFilterOptions,
    downloadFilterOptions: jobsList.downloadFilterOptions,
    apiKeyFilterOptions: keys.apiKeyFilterOptions,
    // filters & pagination
    filters,
    pagination: jobsList.pagination,
    applyFilters: jobsList.applyFilters,
    resetFilters: jobsList.resetFilters,
    handlePageChange: jobsList.handlePageChange,
    handlePageSizeChange: jobsList.handlePageSizeChange,
    refreshPage,
    // keys / models
    apiKeys: keys.apiKeys,
    geminiApiKeys: keys.geminiApiKeys,
    selectedApiKey: keys.selectedApiKey,
    loadingKeys: keys.loadingKeys,
    loadingModels: models.loadingModels,
    availableBatchImageModels: models.availableBatchImageModels,
    modelLoadError: models.modelLoadError,
    selectedModelReferenceLimit: promptForm.selectedModelReferenceLimit,
    // job list
    loadingJobs: jobsList.loadingJobs,
    batchJobs: jobsList.batchJobs,
    visibleBatchJobs: jobsList.visibleBatchJobs,
    selectedJobIds: jobsList.selectedJobIds,
    selectedRows: jobsList.selectedRows,
    selectedDownloadableRows: jobDetail.selectedDownloadableRows,
    allVisibleSelected: jobsList.allVisibleSelected,
    someVisibleSelected: jobsList.someVisibleSelected,
    toggleAllVisible: jobsList.toggleAllVisible,
    toggleJobSelection: jobsList.toggleJobSelection,
    expandedParentIds: jobsList.expandedParentIds,
    toggleChildRows: jobsList.toggleChildRows,
    hasChildJobs: jobsList.hasChildJobs,
    displayJob: jobsList.displayJob,
    bulkDownloading: jobDetail.bulkDownloading,
    bulkDeleting: jobDetail.bulkDeleting,
    downloadSelectedJobs: jobDetail.downloadSelectedJobs,
    deleteSelectedJobs: jobDetail.deleteSelectedJobs,
    downloading: jobDetail.downloading,
    downloadingBatchId: jobDetail.downloadingBatchId,
    isDownloadingJob: jobDetail.isDownloadingJob,
    downloadJob: jobDetail.downloadJob,
    retryingBatchId: jobDetail.retryingBatchId,
    retryFailedJob: jobDetail.retryFailedJob,
    deletingBatchId: jobDetail.deletingBatchId,
    deleteJob: jobDetail.deleteJob,
    cancelling: jobDetail.cancelling,
    canCancel: jobDetail.canCancel,
    canDownload: jobDetail.canDownload,
    canRetry: jobDetail.canRetry,
    canDeleteRecord: jobsList.canDeleteRecord,
    selectJob: jobDetail.selectJob,
    openMoreJobId: jobsList.openMoreJobId,
    moreMenuStyle: jobsList.moreMenuStyle,
    toggleMoreMenu: jobsList.toggleMoreMenu,
    closeMoreMenu: jobsList.closeMoreMenu,
    // prompt popover
    promptPopover: promptPopoverApi.promptPopover,
    cancelPromptPopoverClose: promptPopoverApi.cancelPromptPopoverClose,
    schedulePromptPopoverClose: promptPopoverApi.schedulePromptPopoverClose,
    schedulePromptPopoverOpen: promptPopoverApi.schedulePromptPopoverOpen,
    showPromptPopover: promptPopoverApi.showPromptPopover,
    copyPromptPopover: promptPopoverApi.copyPromptPopover,
    // create modal
    showCreateModal: submit.showCreateModal,
    openCreateModal: submit.openCreateModal,
    closeCreateModal: submit.closeCreateModal,
    form,
    promptRows: promptForm.promptRows,
    promptDraft: promptForm.promptDraft,
    customIdDraft: promptForm.customIdDraft,
    outputCountDraft: promptForm.outputCountDraft,
    referenceImageDrafts: promptForm.referenceImageDrafts,
    estimatedOutputCount: promptForm.estimatedOutputCount,
    parsedItems: promptForm.parsedItems,
    addPromptRow: promptForm.addPromptRow,
    removePromptRow: promptForm.removePromptRow,
    removeReferenceImageDraft: promptForm.removeReferenceImageDraft,
    handleReferenceImageFiles: promptForm.handleReferenceImageFiles,
    submitting: submit.submitting,
    submitJob: submit.submitJob,
    // guide modal
    showGuideModal,
    agentInstruction,
    copyInstruction,
    // detail modal
    currentJob,
    currentDisplayJob: jobDetail.currentDisplayJob,
    selectedBatchId,
    closeDetail: jobDetail.closeDetail,
    refreshing: jobDetail.refreshing,
    loadingItems: itemPreviews.loadingItems,
    refreshDetail: jobDetail.refreshDetail,
    items: itemPreviews.items,
    itemPreviewKey: itemPreviews.itemPreviewKey,
    detailItemRowClass: itemPreviews.detailItemRowClass,
    isRecoveredOriginalFailure: itemPreviews.isRecoveredOriginalFailure,
    itemDisplayStatusBadgeClass: labels.itemDisplayStatusBadgeClass,
    itemDisplayStatusLabel: labels.itemDisplayStatusLabel,
    itemResultClass: labels.itemResultClass,
    itemResultLabel: labels.itemResultLabel,
    itemPreviewUrls: itemPreviews.itemPreviewUrls,
    previewErrorIds: itemPreviews.previewErrorIds,
    previewLoadingIds: itemPreviews.previewLoadingIds,
    canLoadItemPreview: itemPreviews.canLoadItemPreview,
    loadItemPreview: itemPreviews.loadItemPreview,
    openImagePreview: itemPreviews.openImagePreview,
    handlePreviewError: itemPreviews.handlePreviewError,
    cancelSelected: jobDetail.cancelSelected,
    retrySelected: jobDetail.retrySelected,
    downloadSelected: jobDetail.downloadSelected,
    // image preview modal
    previewImageItem: itemPreviews.previewImageItem,
    previewImageUrl: itemPreviews.previewImageUrl,
    closeImagePreview: itemPreviews.closeImagePreview,
    // shared formatting
    formatDate,
    defaultTaskName,
    statusLabel: labels.statusLabel,
    statusBadgeClass,
    costLabel: labels.costLabel,
  }
}
