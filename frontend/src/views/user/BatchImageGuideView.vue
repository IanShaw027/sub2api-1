<template>
  <AppLayout>
    <PageHeader :title="t('batchImageGuide.title')" :description="t('batchImageGuide.description')" />
    <TablePageLayout>
      <template #filters>
        <BatchImageFiltersBar
          v-model:filters="filters"
          :api-key-filter-options="apiKeyFilterOptions"
          :status-filter-options="statusFilterOptions"
          :download-filter-options="downloadFilterOptions"
          :loading-jobs="loadingJobs"
          :loading-keys="loadingKeys"
          :selected-count="selectedJobIds.size"
          :selected-downloadable-count="selectedDownloadableRows.length"
          :bulk-downloading="bulkDownloading"
          :bulk-deleting="bulkDeleting"
          @apply="applyFilters"
          @reset="resetFilters"
          @refresh="refreshPage"
          @open-guide="showGuideModal = true"
          @open-create="openCreateModal"
          @download-selected="downloadSelectedJobs"
          @delete-selected="deleteSelectedJobs"
        />
      </template>

      <template #table>
        <BatchImageJobsTable
          :columns="columns"
          :rows="visibleBatchJobs"
          :loading="loadingKeys || loadingJobs"
          :all-selected="allVisibleSelected"
          :some-selected="someVisibleSelected"
          :selected-ids="selectedJobIds"
          :expanded-parent-ids="expandedParentIds"
          :open-more-job-id="openMoreJobId"
          :downloading="downloading"
          :default-task-name="defaultTaskName"
          :format-date="formatDate"
          :status-badge-class="statusBadgeClass"
          :status-label="statusLabel"
          :cost-label="costLabel"
          :display-job="displayJob"
          :can-download="canDownload"
          :can-retry="canRetry"
          :can-delete-record="canDeleteRecord"
          :is-downloading-job="isDownloadingJob"
          @toggle-all="toggleAllVisible"
          @toggle-row="toggleJobSelection"
          @toggle-child="toggleChildRows"
          @select-job="selectJob"
          @download-job="downloadJob"
          @toggle-more-menu="toggleMoreMenu"
        />
      </template>

      <template #pagination>
        <BatchImagePaginationBar
          :visible-count="visibleBatchJobs.length"
          :page="pagination.page"
          :page-size="pagination.page_size"
          :has-more="pagination.has_more"
          :loading="loadingJobs"
          :page-size-options="batchPageSizeOptions"
          @change-page="handlePageChange"
          @change-page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <BatchImageMoreMenu
      :job-id="openMoreJobId"
      :jobs="batchJobs"
      :style="moreMenuStyle"
      :retrying-batch-id="retryingBatchId"
      :deleting-batch-id="deletingBatchId"
      :can-retry="canRetry"
      :can-delete-record="canDeleteRecord"
      @retry="retryFailedJob"
      @delete="deleteJob"
    />

    <BatchImagePromptPopover
      :visible="promptPopover.visible"
      :text="promptPopover.text"
      :style="promptPopover.style"
      @cancel-close="cancelPromptPopoverClose"
      @schedule-close="schedulePromptPopoverClose"
      @copy="copyPromptPopover"
    />

    <BatchImageDetailModal
      :current-job="currentJob"
      :current-display-job="currentDisplayJob"
      :items="items"
      :loading-items="loadingItems"
      :refreshing="refreshing"
      :cancelling="cancelling"
      :downloading="downloading"
      :retrying-batch-id="retryingBatchId"
      :item-preview-urls="itemPreviewUrls"
      :preview-error-ids="previewErrorIds"
      :preview-loading-ids="previewLoadingIds"
      :status-badge-class="statusBadgeClass"
      :status-label="statusLabel"
      :cost-label="costLabel"
      :format-date="formatDate"
      :has-child-jobs="hasChildJobs"
      :can-cancel="canCancel"
      :can-download="canDownload"
      :can-retry="canRetry"
      :is-downloading-job="isDownloadingJob"
      :item-preview-key="itemPreviewKey"
      :detail-item-row-class="detailItemRowClass"
      :is-recovered-original-failure="isRecoveredOriginalFailure"
      :item-display-status-badge-class="itemDisplayStatusBadgeClass"
      :item-display-status-label="itemDisplayStatusLabel"
      :item-result-class="itemResultClass"
      :item-result-label="itemResultLabel"
      :can-load-item-preview="canLoadItemPreview"
      @close="closeDetail"
      @refresh="refreshDetail"
      @cancel="cancelSelected"
      @retry="retrySelected"
      @download="downloadSelected"
      @open-preview="openImagePreview"
      @load-preview="loadItemPreview"
      @preview-error="handlePreviewError"
      @prompt-hover="schedulePromptPopoverOpen"
      @prompt-leave="schedulePromptPopoverClose"
      @prompt-show="showPromptPopover"
    />

    <BatchImageImagePreviewModal
      :item="previewImageItem"
      :image-url="previewImageUrl"
      @close="closeImagePreview"
    />

    <BatchImageCreateModal
      v-model:form="form"
      v-model:prompt-draft="promptDraft"
      v-model:custom-id-draft="customIdDraft"
      v-model:output-count-draft="outputCountDraft"
      :show="showCreateModal"
      :gemini-api-keys="geminiApiKeys"
      :loading-keys="loadingKeys"
      :available-batch-image-models="availableBatchImageModels"
      :loading-models="loadingModels"
      :model-load-error="modelLoadError"
      :selected-api-key="selectedApiKey"
      :selected-model-reference-limit="selectedModelReferenceLimit"
      :estimated-output-count="estimatedOutputCount"
      :prompt-rows="promptRows"
      :reference-image-drafts="referenceImageDrafts"
      :submitting="submitting"
      :output-count-options="outputCountOptions"
      :max-outputs-per-item="BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM"
      :max-outputs-per-job="BATCH_IMAGE_MAX_OUTPUTS_PER_JOB"
      :parsed-items-count="parsedItems.length"
      :batch-image-text="batchImageText"
      @close="closeCreateModal"
      @submit="submitJob"
      @add-prompt="addPromptRow"
      @remove-prompt="removePromptRow"
      @remove-reference="removeReferenceImageDraft"
      @upload-reference="handleReferenceImageFiles"
    />

    <BatchImageGuideModal
      :show="showGuideModal"
      :instruction="agentInstruction"
      @close="showGuideModal = false"
      @copy="copyInstruction"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import BatchImageFiltersBar from '@/components/user/batch-image/BatchImageFiltersBar.vue'
import BatchImageJobsTable from '@/components/user/batch-image/BatchImageJobsTable.vue'
import BatchImagePaginationBar from '@/components/user/batch-image/BatchImagePaginationBar.vue'
import BatchImageMoreMenu from '@/components/user/batch-image/BatchImageMoreMenu.vue'
import BatchImagePromptPopover from '@/components/user/batch-image/BatchImagePromptPopover.vue'
import BatchImageDetailModal from '@/components/user/batch-image/BatchImageDetailModal.vue'
import BatchImageImagePreviewModal from '@/components/user/batch-image/BatchImageImagePreviewModal.vue'
import BatchImageCreateModal from '@/components/user/batch-image/BatchImageCreateModal.vue'
import BatchImageGuideModal from '@/components/user/batch-image/BatchImageGuideModal.vue'
import { useBatchImageGuide } from './batchImage/useBatchImageGuide'

const {
  t,
  batchImageText,
  BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM,
  BATCH_IMAGE_MAX_OUTPUTS_PER_JOB,
  outputCountOptions,
  batchPageSizeOptions,
  columns,
  statusFilterOptions,
  downloadFilterOptions,
  apiKeyFilterOptions,
  filters,
  pagination,
  applyFilters,
  resetFilters,
  handlePageChange,
  handlePageSizeChange,
  refreshPage,
  geminiApiKeys,
  selectedApiKey,
  loadingKeys,
  loadingModels,
  availableBatchImageModels,
  modelLoadError,
  selectedModelReferenceLimit,
  loadingJobs,
  batchJobs,
  visibleBatchJobs,
  selectedJobIds,
  selectedDownloadableRows,
  allVisibleSelected,
  someVisibleSelected,
  toggleAllVisible,
  toggleJobSelection,
  expandedParentIds,
  toggleChildRows,
  hasChildJobs,
  displayJob,
  bulkDownloading,
  bulkDeleting,
  downloadSelectedJobs,
  deleteSelectedJobs,
  downloading,
  isDownloadingJob,
  downloadJob,
  retryingBatchId,
  retryFailedJob,
  deletingBatchId,
  deleteJob,
  canCancel,
  canDownload,
  canRetry,
  canDeleteRecord,
  selectJob,
  openMoreJobId,
  moreMenuStyle,
  toggleMoreMenu,
  promptPopover,
  cancelPromptPopoverClose,
  schedulePromptPopoverClose,
  schedulePromptPopoverOpen,
  showPromptPopover,
  copyPromptPopover,
  showCreateModal,
  openCreateModal,
  closeCreateModal,
  form,
  promptRows,
  promptDraft,
  customIdDraft,
  outputCountDraft,
  referenceImageDrafts,
  estimatedOutputCount,
  parsedItems,
  addPromptRow,
  removePromptRow,
  removeReferenceImageDraft,
  handleReferenceImageFiles,
  submitting,
  submitJob,
  showGuideModal,
  agentInstruction,
  copyInstruction,
  currentJob,
  currentDisplayJob,
  closeDetail,
  refreshing,
  loadingItems,
  refreshDetail,
  items,
  itemPreviewKey,
  detailItemRowClass,
  isRecoveredOriginalFailure,
  itemDisplayStatusBadgeClass,
  itemDisplayStatusLabel,
  itemResultClass,
  itemResultLabel,
  itemPreviewUrls,
  previewErrorIds,
  previewLoadingIds,
  canLoadItemPreview,
  loadItemPreview,
  openImagePreview,
  handlePreviewError,
  cancelSelected,
  retrySelected,
  downloadSelected,
  previewImageItem,
  previewImageUrl,
  closeImagePreview,
  formatDate,
  defaultTaskName,
  statusLabel,
  statusBadgeClass,
  costLabel,
  cancelling,
} = useBatchImageGuide()
</script>
