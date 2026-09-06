import type { BatchImageItem, BatchImageJob, BatchImageReferenceImage } from '@/api/batchImage'

/**
 * Shared types for the /batch-image page.
 * Extracted from BatchImageGuideView.vue (glass-ui-redesign task 12.12) so the
 * view file, its composable and its section components can share one definition.
 */

export type BatchImageJobRow = Pick<
  BatchImageJob,
  | 'id'
  | 'task_name'
  | 'parent_batch_id'
  | 'status'
  | 'model'
  | 'provider'
  | 'item_count'
  | 'success_count'
  | 'fail_count'
  | 'estimated_cost'
  | 'hold_amount'
  | 'actual_cost'
  | 'created_at'
  | 'downloaded_at'
> & {
  api_key_id: number
  api_key_name: string
  child_count: number
  is_child?: boolean
}

export type BatchImageDetailItem = BatchImageItem & {
  batch_id: string
  source_task_name: string
}

export type PromptRow = {
  localId: string
  custom_id: string
  prompt: string
  output_count: number
  reference_images: BatchImageReferenceImage[]
}

export type ReferenceImageDraft = BatchImageReferenceImage & {
  name: string
  size: number
}

export type PreviewCacheRecord = {
  key: string
  blob: Blob
  size: number
  createdAt: number
  lastAccessedAt: number
}

export type PreviewImageSource = ImageBitmap | HTMLImageElement
