import { computed, reactive, ref, type ComputedRef } from 'vue'
import { listBatchImageJobs, type BatchImageJob, type BatchImageJobsListOptions } from '@/api/batchImage'
import type { ApiKey } from '@/types'
import type { Column } from '@/components/common/types'
import type { SelectOption } from '@/components/common/Select.vue'
import { getPersistedPageSize, setPersistedPageSize } from '@/composables/usePersistedPageSize'
import type { useAppStore } from '@/stores/app'
import { TERMINAL_STATUSES } from '../constants'
import type { BatchImageTextKey } from '../errorMessages'
import { defaultTaskName } from '../format'
import type { BatchImageJobRow } from '../types'

/**
 * Job list state: filters, pagination, selection, parent/child grouping and
 * the list-level table config for the /batch-image page. Extracted from
 * useBatchImageGuide.ts (glass-ui-redesign task 6.4); behaviour and copy are
 * unchanged, only relocated.
 */
export interface UseBatchImageJobsListFilters {
  taskName: string
  apiKeyId: string
  status: string
  downloaded: string
}

export interface UseBatchImageJobsListOptions {
  filters: UseBatchImageJobsListFilters
  filteredApiKeys: ComputedRef<ApiKey[]>
  selectedApiKey: ComputedRef<ApiKey | null>
  appStore: ReturnType<typeof useAppStore>
  t: (key: string) => string
  batchImageText: (key: BatchImageTextKey) => string
  batchImageErrorMessage: (error: any, fallback: string) => string
}

export function useBatchImageJobsList(options: UseBatchImageJobsListOptions) {
  const { filters, filteredApiKeys, selectedApiKey, appStore, t, batchImageText, batchImageErrorMessage } = options

  const columns = computed<Column[]>(() => [
    { key: 'select', label: '', sortable: false, class: 'w-12 text-center' },
    { key: 'id', label: t('batchImage.columns.taskName'), sortable: false, class: 'w-[200px] max-w-[200px]' },
    { key: 'model', label: t('batchImage.columns.model'), sortable: false, class: 'w-[150px] max-w-[150px] text-center' },
    { key: 'api_key_name', label: t('batchImage.columns.apiKey'), sortable: false, class: 'w-32 max-w-32 text-center' },
    { key: 'status', label: t('common.status'), sortable: false, class: 'w-28 text-center' },
    { key: 'counts', label: t('batchImage.columns.result'), sortable: false, class: 'w-28 text-center' },
    { key: 'cost', label: t('batchImage.columns.cost'), sortable: false, class: 'w-28 text-center' },
    { key: 'downloaded', label: t('batchImage.columns.downloadStatus'), sortable: false, class: 'w-36 text-center' },
    { key: 'actions', label: t('common.actions'), sortable: false, class: 'w-32 text-center' },
  ])

  const statusFilterOptions = computed<SelectOption[]>(() => [
    { value: '', label: t('batchImage.filters.allStatuses') },
    { value: 'queued', label: t('batchImage.status.queued') },
    { value: 'running', label: t('batchImage.status.running') },
    { value: 'processing_results', label: t('batchImage.status.processingResults') },
    { value: 'settling', label: t('batchImage.status.settling') },
    { value: 'completed', label: t('batchImage.status.completed') },
    { value: 'failed', label: t('batchImage.status.failed') },
    { value: 'cancelled', label: t('batchImage.status.cancelled') },
    { value: 'output_deleted', label: t('batchImage.status.outputDeleted') },
  ])

  const downloadFilterOptions = computed<SelectOption[]>(() => [
    { value: '', label: t('batchImage.filters.allDownloadStates') },
    { value: 'true', label: t('batchImage.filters.downloaded') },
    { value: 'false', label: t('batchImage.filters.notDownloaded') },
  ])

  const pagination = reactive({
    page: 1,
    page_size: Math.min(getPersistedPageSize(20), 100),
    has_more: false,
  })

  const loadingJobs = ref(false)
  const batchJobs = ref<BatchImageJobRow[]>([])
  const selectedJobIds = ref(new Set<string>())
  const expandedParentIds = ref(new Set<string>())
  const openMoreJobId = ref('')
  const moreMenuStyle = ref<Record<string, string>>({})

  const selectedRows = computed(() =>
    batchJobs.value.filter(job => selectedJobIds.value.has(job.id)),
  )

  const childrenByParent = computed(() => {
    const groups = new Map<string, BatchImageJobRow[]>()
    for (const job of batchJobs.value) {
      if (!job.parent_batch_id) continue
      const rows = groups.get(job.parent_batch_id) || []
      rows.push(job)
      groups.set(job.parent_batch_id, rows)
    }
    for (const rows of groups.values()) {
      rows.sort((a, b) => a.created_at - b.created_at)
    }
    return groups
  })

  const visibleBatchJobs = computed(() => {
    const rows: BatchImageJobRow[] = []
    for (const job of batchJobs.value.filter(item => !item.parent_batch_id)) {
      rows.push(job)
      if (expandedParentIds.value.has(job.id)) {
        rows.push(...(childrenByParent.value.get(job.id) || []).map(child => ({ ...child, is_child: true })))
      }
    }
    return rows
  })

  const allVisibleSelected = computed(() =>
    visibleBatchJobs.value.length > 0 && visibleBatchJobs.value.every(job => selectedJobIds.value.has(job.id)),
  )

  const someVisibleSelected = computed(() =>
    visibleBatchJobs.value.some(job => selectedJobIds.value.has(job.id)) && !allVisibleSelected.value,
  )

  function listOptions(): BatchImageJobsListOptions {
    const options: BatchImageJobsListOptions = {
      limit: pagination.page_size,
      cursor: String((pagination.page - 1) * pagination.page_size),
    }
    if (filters.taskName.trim()) options.taskName = filters.taskName.trim()
    if (filters.status) options.status = filters.status
    if (filters.downloaded) options.downloaded = filters.downloaded
    return options
  }

  function toJobRow(job: BatchImageJob, key: ApiKey | null = selectedApiKey.value): BatchImageJobRow {
    return {
      id: job.id,
      task_name: job.task_name || defaultTaskName(job.created_at),
      parent_batch_id: job.parent_batch_id || null,
      status: job.status,
      model: job.model,
      provider: job.provider,
      item_count: job.item_count,
      success_count: job.success_count,
      fail_count: job.fail_count,
      estimated_cost: job.estimated_cost,
      hold_amount: job.hold_amount,
      actual_cost: job.actual_cost,
      created_at: job.created_at,
      downloaded_at: job.downloaded_at,
      api_key_id: key?.id || 0,
      api_key_name: key?.name || '',
      child_count: 0,
    }
  }

  function applyChildCounts(rows: BatchImageJobRow[]) {
    const counts = new Map<string, number>()
    for (const row of rows) {
      if (!row.parent_batch_id) continue
      counts.set(row.parent_batch_id, (counts.get(row.parent_batch_id) || 0) + 1)
    }
    return rows.map(row => ({ ...row, child_count: counts.get(row.id) || 0 }))
  }

  function displayJob<T extends Pick<BatchImageJob, 'id' | 'parent_batch_id' | 'status' | 'item_count' | 'success_count' | 'fail_count' | 'estimated_cost' | 'hold_amount' | 'actual_cost'>>(job: T): T {
    if (job.parent_batch_id) return job
    const children = childrenByParent.value.get(job.id) || []
    if (!children.length) return job

    const childSuccess = children.reduce((sum, child) => sum + child.success_count, 0)
    const childEstimated = children.reduce((sum, child) => sum + child.estimated_cost, 0)
    const childHold = children.reduce((sum, child) => sum + child.hold_amount, 0)
    const childActual = children.reduce((sum, child) => sum + (child.actual_cost || 0), 0)
    const childActualReady = children.every(child => child.actual_cost !== null)
    const successCount = Math.min(job.item_count, job.success_count + childSuccess)
    const failCount = Math.max(0, job.item_count - successCount)
    const actualCost = job.actual_cost === null
      ? (childActualReady ? childActual : null)
      : job.actual_cost + childActual

    return {
      ...job,
      success_count: successCount,
      fail_count: failCount,
      status: failCount === 0 && TERMINAL_STATUSES.has(job.status) ? 'completed' : job.status,
      estimated_cost: job.estimated_cost + childEstimated,
      hold_amount: job.hold_amount + childHold,
      actual_cost: actualCost,
    }
  }

  function hasChildJobs(batchId: string) {
    return (childrenByParent.value.get(batchId) || []).length > 0
  }

  function toggleChildRows(batchId: string) {
    const next = new Set(expandedParentIds.value)
    if (next.has(batchId)) next.delete(batchId)
    else next.add(batchId)
    expandedParentIds.value = next
  }

  function closeMoreMenu() {
    openMoreJobId.value = ''
  }

  function toggleMoreMenu(job: BatchImageJobRow, event: MouseEvent) {
    if (openMoreJobId.value === job.id) {
      closeMoreMenu()
      return
    }
    const trigger = event.currentTarget as HTMLElement | null
    const rect = trigger?.getBoundingClientRect()
    if (!rect) return
    const menuWidth = 176
    const margin = 8
    const left = Math.max(margin, Math.min(rect.right - menuWidth, window.innerWidth - menuWidth - margin))
    const top = Math.min(rect.bottom + margin, window.innerHeight - 96)
    moreMenuStyle.value = {
      left: `${left}px`,
      top: `${Math.max(margin, top)}px`,
    }
    openMoreJobId.value = job.id
  }

  async function loadBatchJobs() {
    const keys = filteredApiKeys.value
    if (!keys.length) {
      batchJobs.value = []
      pagination.has_more = false
      return
    }
    loadingJobs.value = true
    closeMoreMenu()
    try {
      const opts = listOptions()
      const results = await Promise.all(keys.map(async (key) => {
        const result = await listBatchImageJobs(key.key, opts)
        return {
          hasMore: Boolean(result.has_more),
          rows: (result.data || []).map(job => toJobRow(job, key)),
        }
      }))
      batchJobs.value = applyChildCounts(results
        .flatMap(result => result.rows)
        .sort((a, b) => b.created_at - a.created_at)
        .slice(0, pagination.page_size))
      pagination.has_more = results.some(result => result.hasMore)
      selectedJobIds.value = new Set([...selectedJobIds.value].filter(id => visibleBatchJobs.value.some(job => job.id === id)))
    } catch (error: any) {
      appStore.showError(batchImageErrorMessage(error, batchImageText('loadJobsFailed')))
    } finally {
      loadingJobs.value = false
    }
  }

  function upsertJob(job: BatchImageJob) {
    const next = toJobRow(job)
    const index = batchJobs.value.findIndex(item => item.id === job.id)
    if (index >= 0) {
      const rows = [...batchJobs.value]
      rows[index] = { ...next, is_child: rows[index].is_child }
      batchJobs.value = applyChildCounts(rows)
      return
    }
    batchJobs.value = applyChildCounts([next, ...batchJobs.value].slice(0, pagination.page_size))
  }

  function applyFilters() {
    pagination.page = 1
    selectedJobIds.value = new Set()
    void loadBatchJobs()
  }

  function resetFilters() {
    filters.taskName = ''
    filters.apiKeyId = ''
    filters.status = ''
    filters.downloaded = ''
    applyFilters()
  }

  function handlePageChange(page: number) {
    if (page < 1 || page === pagination.page) return
    pagination.page = page
    selectedJobIds.value = new Set()
    void loadBatchJobs()
  }

  function handlePageSizeChange(value: string | number | boolean | null) {
    if (value === null || typeof value === 'boolean') return
    const nextSize = Math.min(Math.max(Number(value) || 20, 1), 100)
    pagination.page_size = nextSize
    pagination.page = 1
    setPersistedPageSize(nextSize)
    selectedJobIds.value = new Set()
    void loadBatchJobs()
  }

  function toggleJobSelection(batchId: string, checked: boolean) {
    const next = new Set(selectedJobIds.value)
    if (checked) next.add(batchId)
    else next.delete(batchId)
    selectedJobIds.value = next
  }

  function toggleAllVisible(checked: boolean) {
    const next = new Set(selectedJobIds.value)
    for (const job of visibleBatchJobs.value) {
      if (checked) next.add(job.id)
      else next.delete(job.id)
    }
    selectedJobIds.value = next
  }

  function canDeleteRecord(job: Pick<BatchImageJob, 'status'>) {
    return TERMINAL_STATUSES.has(job.status)
  }

  function markJobDownloaded(batchId: string) {
    const downloadedAt = Math.floor(Date.now() / 1000)
    batchJobs.value = batchJobs.value.map(job => job.id === batchId ? { ...job, downloaded_at: job.downloaded_at || downloadedAt } : job)
  }

  function removeJobFromList(batchId: string) {
    batchJobs.value = batchJobs.value.filter(job => job.id !== batchId)
    toggleJobSelection(batchId, false)
  }

  return {
    columns,
    statusFilterOptions,
    downloadFilterOptions,
    filters,
    pagination,
    loadingJobs,
    batchJobs,
    selectedJobIds,
    expandedParentIds,
    openMoreJobId,
    moreMenuStyle,
    selectedRows,
    childrenByParent,
    visibleBatchJobs,
    allVisibleSelected,
    someVisibleSelected,
    toJobRow,
    displayJob,
    hasChildJobs,
    toggleChildRows,
    closeMoreMenu,
    toggleMoreMenu,
    loadBatchJobs,
    upsertJob,
    applyFilters,
    resetFilters,
    handlePageChange,
    handlePageSizeChange,
    toggleJobSelection,
    toggleAllVisible,
    canDeleteRecord,
    markJobDownloaded,
    removeJobFromList,
  }
}
