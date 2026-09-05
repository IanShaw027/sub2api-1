import type { SelectOption } from '@/components/common/Select.vue'

/**
 * Constants for the /batch-image page.
 * Extracted from BatchImageGuideView.vue (glass-ui-redesign task 12.12).
 */

export const TERMINAL_STATUSES = new Set(['completed', 'failed', 'cancelled', 'output_deleted'])

export const PREVIEW_CACHE_DB_NAME = 'sub2api-batch-image-preview-cache'
export const PREVIEW_CACHE_STORE_NAME = 'thumbnails'
export const PREVIEW_THUMBNAIL_MAX_EDGE = 360
export const PREVIEW_THUMBNAIL_QUALITY = 0.72
export const PREVIEW_CACHE_MAX_AGE_MS = 3 * 24 * 60 * 60 * 1000
export const PREVIEW_CACHE_MAX_ENTRIES = 120
export const PREVIEW_CACHE_MAX_BYTES = 48 * 1024 * 1024

export const BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM = 4
export const BATCH_IMAGE_MAX_OUTPUTS_PER_JOB = 200

export const outputCountOptions = Array.from({ length: BATCH_IMAGE_MAX_OUTPUTS_PER_ITEM }, (_, index) => index + 1)

export const batchPageSizeOptions: SelectOption[] = [20, 50, 100].map(size => ({ value: size, label: String(size) }))
