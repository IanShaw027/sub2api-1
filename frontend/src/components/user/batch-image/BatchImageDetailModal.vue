<template>
  <BaseDialog :show="!!currentJob" :title="t('batchImage.detail.title')" width="extra-wide" @close="$emit('close')">
    <div v-if="currentJob" class="space-y-4">
      <div class="rounded-lg border border-line bg-surface-2 px-4 py-3 ">
        <div class="grid gap-x-6 gap-y-3 sm:grid-cols-2 lg:grid-cols-4">
          <div class="min-w-0 text-center">
            <p class="text-xs text-muted">{{ t('common.status') }}</p>
            <div class="mt-1 flex justify-center">
              <span :class="statusBadgeClass(currentDisplayJob || currentJob)" class="badge whitespace-nowrap">
                {{ statusLabel(currentDisplayJob || currentJob) }}
              </span>
            </div>
          </div>
          <div class="min-w-0 text-center">
            <p class="text-xs text-muted">{{ hasChildJobs(currentJob.id) ? t('batchImage.detail.aggregatedResult') : t('batchImage.detail.result') }}</p>
            <p class="mt-1 flex items-center justify-center gap-2 font-medium tabular-nums">
              <span class="text-success-text ">{{ (currentDisplayJob || currentJob).success_count }}</span>
              <span class="text-muted">/</span>
              <span :class="(currentDisplayJob || currentJob).fail_count > 0 ? 'text-danger-text ' : 'text-muted'">{{ (currentDisplayJob || currentJob).fail_count }}</span>
            </p>
          </div>
          <div class="min-w-0 text-center">
            <p class="text-xs text-muted">{{ t('batchImage.detail.cost') }}</p>
            <p class="mt-1 truncate font-medium text-foreground">{{ costLabel(currentDisplayJob || currentJob) }}</p>
          </div>
          <div class="min-w-0 text-center">
            <p class="text-xs text-muted">{{ t('batchImage.detail.downloadStatus') }}</p>
            <p class="mt-1 truncate font-medium text-foreground">
              {{ currentJob.downloaded_at ? formatDate(currentJob.downloaded_at) : t('batchImage.list.notDownloaded') }}
            </p>
          </div>
        </div>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3">
        <h3 class="text-sm font-semibold text-foreground">{{ t('batchImage.detail.items') }}</h3>
        <button type="button" class="btn-glass-secondary btn-sm" :disabled="refreshing || loadingItems" @click="$emit('refresh')">
          <Icon name="refresh" size="sm" class="mr-1.5" :class="refreshing || loadingItems ? 'animate-spin' : ''" />
          {{ t('common.refresh') }}
        </button>
      </div>

      <div v-if="items.length" class="overflow-x-auto rounded-lg border border-line bg-surface ">
        <table class="w-full min-w-[860px] table-fixed divide-y divide-[var(--border)] text-sm ">
          <colgroup>
            <col class="w-[18%]" />
            <col class="w-[34%]" />
            <col class="w-[12%]" />
            <col class="w-[10%]" />
            <col class="w-[26%]" />
          </colgroup>
          <thead class="bg-surface-2/80">
            <tr>
              <th class="px-3 py-3 text-center text-sm font-medium text-muted">Custom ID</th>
              <th class="px-3 py-3 text-left text-sm font-medium text-muted">Prompt</th>
              <th class="px-3 py-3 text-center text-sm font-medium text-muted">{{ t('common.status') }}</th>
              <th class="px-3 py-3 text-center text-sm font-medium text-muted">{{ t('batchImage.detail.preview') }}</th>
              <th class="px-3 py-3 text-center text-sm font-medium text-muted">{{ t('batchImage.detail.result') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-[var(--border)]">
            <tr
              v-for="item in items"
              :key="itemPreviewKey(item)"
              class="align-middle"
              :class="detailItemRowClass(item)"
            >
              <td class="px-3 py-2.5 text-center">
                <span
                  class="block min-w-0 truncate font-mono text-sm"
                  :class="isRecoveredOriginalFailure(item) ? 'text-muted' : 'text-foreground'"
                  :title="item.custom_id"
                >
                  {{ item.custom_id }}
                </span>
              </td>
              <td class="px-3 py-2.5 text-left" :class="isRecoveredOriginalFailure(item) ? 'text-muted' : 'text-foreground'">
                <div
                  class="batch-prompt-trigger cursor-default truncate rounded px-1 text-sm leading-6 focus:outline-none"
                  tabindex="0"
                  @pointerenter="$emit('prompt-hover', $event, item.prompt_preview || '-')"
                  @pointerleave="$emit('prompt-leave')"
                  @mouseenter="$emit('prompt-hover', $event, item.prompt_preview || '-')"
                  @mouseleave="$emit('prompt-leave')"
                  @click="$emit('prompt-show', $event, item.prompt_preview || '-')"
                  @focus="$emit('prompt-show', $event, item.prompt_preview || '-')"
                  @focusin="$emit('prompt-show', $event, item.prompt_preview || '-')"
                  @blur="$emit('prompt-leave')"
                >
                  {{ item.prompt_preview || '-' }}
                </div>
              </td>
              <td class="px-3 py-2.5 text-center">
                <span :class="itemDisplayStatusBadgeClass(item)" class="badge max-w-full truncate whitespace-nowrap" :title="itemDisplayStatusLabel(item)">
                  {{ itemDisplayStatusLabel(item) }}
                </span>
              </td>
              <td class="px-3 py-2.5 text-center">
                <div class="mx-auto h-12 w-12 overflow-hidden rounded-md border border-line bg-surface-2 ">
                  <button
                    v-if="itemPreviewUrls[itemPreviewKey(item)] && !previewErrorIds.has(itemPreviewKey(item))"
                    type="button"
                    class="block h-full w-full overflow-hidden"
                    :title="t('batchImage.detail.previewZoom', { id: item.custom_id })"
                    @click="$emit('open-preview', item)"
                  >
                    <img
                      :src="itemPreviewUrls[itemPreviewKey(item)]"
                      class="h-full w-full object-cover"
                      alt=""
                      @error="$emit('preview-error', itemPreviewKey(item))"
                    />
                  </button>
                  <button
                    v-else-if="canLoadItemPreview(item)"
                    type="button"
                    class="flex h-full w-full items-center justify-center text-muted transition-colors hover:bg-surface-2 hover:text-accent disabled:cursor-wait disabled:opacity-70"
                    :disabled="previewLoadingIds.has(itemPreviewKey(item))"
                    :title="previewErrorIds.has(itemPreviewKey(item)) ? t('batchImage.detail.previewReload') : t('batchImage.detail.previewLoad')"
                    @click="$emit('load-preview', item)"
                  >
                    <Icon :name="previewLoadingIds.has(itemPreviewKey(item)) ? 'refresh' : 'eye'" size="sm" :class="previewLoadingIds.has(itemPreviewKey(item)) ? 'animate-spin' : ''" />
                  </button>
                  <div v-else class="flex h-full w-full items-center justify-center text-muted" :title="item.image_count > 0 ? t('batchImage.detail.previewUnavailable') : t('batchImage.detail.noImage')">
                    <Icon name="document" size="sm" />
                  </div>
                </div>
              </td>
              <td class="px-3 py-2.5 text-center">
                <span
                  class="inline-flex max-w-full items-center justify-center truncate rounded-md px-2.5 py-1 text-xs font-medium leading-5 ring-1 ring-inset"
                  :class="itemResultClass(item)"
                  :title="itemResultLabel(item)"
                >
                  {{ itemResultLabel(item) }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else class="rounded-lg border border-dashed border-line py-10 text-center ">
        <Icon name="refresh" size="lg" class="mx-auto mb-3 text-muted" :class="loadingItems ? 'animate-spin' : ''" />
        <p class="text-sm font-medium text-foreground">
          {{ loadingItems ? t('batchImage.detail.loadingItems') : t('batchImage.detail.noItems') }}
        </p>
        <p v-if="!loadingItems" class="mt-1 text-sm text-muted">
          {{ t('batchImage.detail.noItemsHint') }}
        </p>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn-glass-secondary" :disabled="!currentJob || !canCancel(currentJob) || cancelling" @click="$emit('cancel')">
          <Icon v-if="cancelling" name="refresh" size="sm" class="mr-2 animate-spin" />
          {{ t('batchImage.actions.cancelJob') }}
        </button>
        <button
          v-if="currentJob && currentDisplayJob && canRetry(currentDisplayJob)"
          type="button"
          class="btn-glass-secondary inline-flex min-w-[116px] items-center justify-center"
          :disabled="retryingBatchId === currentJob.id"
          @click="$emit('retry')"
        >
          <Icon name="refresh" size="sm" class="mr-2" :class="currentJob && retryingBatchId === currentJob.id ? 'animate-spin' : ''" />
          {{ t('batchImage.actions.retryFailedItems') }}
        </button>
        <button
          type="button"
          class="btn-glass-primary inline-flex min-w-[112px] items-center justify-center"
          :disabled="!currentJob || !canDownload(currentJob) || downloading"
          @click="$emit('download')"
        >
          <Icon
            :name="currentJob && isDownloadingJob(currentJob.id) ? 'refresh' : 'download'"
            size="sm"
            class="mr-2"
            :class="currentJob && isDownloadingJob(currentJob.id) ? 'animate-spin' : ''"
          />
          {{ t('batchImage.actions.downloadZip') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import type { BatchImageItem, BatchImageJob } from '@/api/batchImage'
import type { BatchImageDetailItem } from '@/views/user/batchImage/types'

defineProps<{
  currentJob: BatchImageJob | null
  currentDisplayJob: BatchImageJob | null
  items: BatchImageDetailItem[]
  loadingItems: boolean
  refreshing: boolean
  cancelling: boolean
  downloading: boolean
  retryingBatchId: string
  itemPreviewUrls: Record<string, string>
  previewErrorIds: Set<string>
  previewLoadingIds: Set<string>
  statusBadgeClass: (job: any) => string
  statusLabel: (job: any) => string
  costLabel: (job: any) => string
  formatDate: (timestamp: number) => string
  hasChildJobs: (batchId: string) => boolean
  canCancel: (job: any) => boolean
  canDownload: (job: any) => boolean
  canRetry: (job: any) => boolean
  isDownloadingJob: (batchId: string) => boolean
  itemPreviewKey: (item: BatchImageItem) => string
  detailItemRowClass: (item: BatchImageDetailItem) => string
  isRecoveredOriginalFailure: (item: BatchImageDetailItem) => boolean
  itemDisplayStatusBadgeClass: (item: BatchImageDetailItem) => string
  itemDisplayStatusLabel: (item: BatchImageDetailItem) => string
  itemResultClass: (item: BatchImageDetailItem) => string
  itemResultLabel: (item: BatchImageDetailItem) => string
  canLoadItemPreview: (item: BatchImageItem) => boolean
}>()

defineEmits<{
  close: []
  refresh: []
  cancel: []
  retry: []
  download: []
  'open-preview': [item: BatchImageItem]
  'load-preview': [item: BatchImageItem]
  'preview-error': [key: string]
  'prompt-hover': [event: MouseEvent | PointerEvent, text: string]
  'prompt-leave': []
  'prompt-show': [event: MouseEvent | FocusEvent, text: string]
}>()

const { t } = useI18n()
</script>

<style scoped>
.batch-prompt-trigger:focus {
  outline: none;
  box-shadow: none;
}
</style>
