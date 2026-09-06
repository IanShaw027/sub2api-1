import type { BatchImageItem, BatchImageJob, BatchImageStatus } from '@/api/batchImage'
import { formatMoney, itemStatusBadgeClass, terminalZeroCost } from '../format'
import type { BatchImageDetailItem } from '../types'

/**
 * Status/result label + badge-class formatting for the /batch-image page.
 * Extracted from useBatchImageGuide.ts (glass-ui-redesign task 6.4);
 * behaviour and copy are unchanged, only relocated.
 */
export interface UseBatchImageLabelsOptions {
  t: (key: string, params?: Record<string, unknown>) => string
  isRecoveredOriginalFailure: (item: BatchImageDetailItem) => boolean
  itemPreviewUrls: Record<string, string>
  itemPreviewKey: (item: Pick<BatchImageItem, 'batch_id' | 'custom_id'>) => string
}

export function useBatchImageLabels(options: UseBatchImageLabelsOptions) {
  const { t, isRecoveredOriginalFailure, itemPreviewUrls, itemPreviewKey } = options

  function statusLabel(jobOrStatus: BatchImageStatus | Pick<BatchImageJob, 'status' | 'success_count' | 'fail_count'>) {
    const status = typeof jobOrStatus === 'string' ? jobOrStatus : jobOrStatus.status
    if (typeof jobOrStatus !== 'string' && status === 'completed' && jobOrStatus.fail_count > 0) {
      if (jobOrStatus.success_count > 0) return t('batchImage.status.partialSuccess')
      return t('batchImage.status.allFailed')
    }
    const statusKeys: Record<string, string> = {
      queued: 'queued',
      running: 'running',
      indexing: 'processingResults',
      processing_results: 'processingResults',
      settling: 'settling',
      completed: 'completed',
      failed: 'failed',
      cancelled: 'cancelled',
      output_deleted: 'outputDeleted',
    }
    const key = statusKeys[status]
    return key ? t(`batchImage.status.${key}`) : status
  }

  function itemStatusLabel(status: string) {
    const statusKeys: Record<string, string> = {
      pending: 'pending',
      succeeded: 'succeeded',
      success: 'succeeded',
      failed: 'failed',
      cancelled: 'cancelled',
    }
    const key = statusKeys[status]
    return key ? t(`batchImage.itemStatus.${key}`) : status
  }

  function itemDisplayStatusLabel(item: BatchImageDetailItem) {
    if (isRecoveredOriginalFailure(item)) return t('batchImage.itemStatus.recovered')
    return itemStatusLabel(item.status)
  }

  function itemDisplayStatusBadgeClass(item: BatchImageDetailItem) {
    if (isRecoveredOriginalFailure(item)) return 'badge-gray'
    return itemStatusBadgeClass(item.status)
  }

  function itemResultLabel(item: BatchImageDetailItem) {
    if (isRecoveredOriginalFailure(item)) return t('batchImage.itemResult.recoveredByRetry')
    if (item.error) return friendlyItemError(item.error)
    if (item.status === 'succeeded' || item.status === 'success') {
      return itemPreviewUrls[itemPreviewKey(item)] ? t('batchImage.itemResult.readyPreview') : t('batchImage.itemResult.readyDownload')
    }
    if (item.status === 'failed') return t('batchImage.itemResult.noUsableImage')
    if (item.status === 'cancelled') return t('batchImage.itemResult.cancelled')
    return t('batchImage.itemResult.waiting')
  }

  function itemResultClass(item: BatchImageDetailItem) {
    if (isRecoveredOriginalFailure(item)) return 'bg-surface-2 text-muted ring-line   '
    if (item.error || item.status === 'failed' || item.status === 'cancelled') return 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text ring-danger-text   '
    if (item.status === 'succeeded' || item.status === 'success') return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text ring-success-text   '
    return 'bg-surface-2 text-muted ring-line   '
  }

  function friendlyItemError(error: BatchImageItem['error']) {
    if (!error) return '-'
    if (error.code === 'EMPTY_IMAGE_OUTPUT') return t('batchImage.itemResult.emptyImageOutput')
    if (error.code === 'PROVIDER_ITEM_FAILED') return t('batchImage.itemResult.providerItemFailed')
    return error.message || error.code || '-'
  }

  function costLabel(job: Pick<BatchImageJob, 'status' | 'hold_amount' | 'actual_cost'>) {
    if (job.actual_cost !== null) return formatMoney(job.actual_cost)
    if (terminalZeroCost(job)) return formatMoney(0)
    return t('batchImage.detail.holdCost', { amount: formatMoney(job.hold_amount) })
  }

  return {
    statusLabel,
    itemStatusLabel,
    itemDisplayStatusLabel,
    itemDisplayStatusBadgeClass,
    itemResultLabel,
    itemResultClass,
    friendlyItemError,
    costLabel,
  }
}
