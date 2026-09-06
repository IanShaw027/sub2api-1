<template>
  <DataTable
    :columns="columns"
    :data="rows"
    :loading="loading"
    :expandable-actions="false"
    row-key="id"
  >
    <template #header-select>
      <input
        type="checkbox"
        class="h-4 w-4 rounded border-line text-accent focus:ring-[var(--accent)]"
        :checked="allSelected"
        :indeterminate="someSelected"
        @change="$emit('toggle-all', ($event.target as HTMLInputElement).checked)"
      />
    </template>

    <template #cell-select="{ row }">
      <input
        type="checkbox"
        class="h-4 w-4 rounded border-line text-accent focus:ring-[var(--accent)]"
        :checked="selectedIds.has(row.id)"
        @change="$emit('toggle-row', row.id, ($event.target as HTMLInputElement).checked)"
        @click.stop
      />
    </template>

    <template #cell-id="{ row }">
      <div class="flex w-[220px] items-start gap-1" :class="row.is_child ? 'pl-6' : ''">
        <button
          v-if="row.child_count > 0 && !row.is_child"
          type="button"
          class="mt-1 flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-md text-muted transition-colors hover:bg-surface-2 hover:text-foreground focus:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklch,var(--accent)_30%,transparent)]"
          :title="expandedParentIds.has(row.id) ? t('batchImage.list.collapseChildren') : t('batchImage.list.expandChildren', { n: row.child_count }, row.child_count)"
          @click.stop="$emit('toggle-child', row.id)"
        >
          <Icon :name="expandedParentIds.has(row.id) ? 'chevronDown' : 'chevronRight'" size="xs" />
        </button>
        <span v-else class="w-6 flex-shrink-0" />
        <button type="button" class="min-w-0 flex-1 rounded-lg text-left transition-colors hover:bg-surface-2 focus:outline-none focus-visible:ring-2 focus-visible:ring-[color-mix(in_oklch,var(--accent)_30%,transparent)]" @click="$emit('select-job', row.id)">
          <span
            class="flex min-w-0 items-center gap-2 text-sm font-medium"
            :class="row.task_name ? 'text-foreground' : 'text-muted'"
          >
            <span class="min-w-0 truncate">{{ row.task_name || defaultTaskName(row.created_at) }}</span>
            <span v-if="row.child_count > 0 && !row.is_child" class="flex-shrink-0 rounded-full bg-surface-2 px-2 py-0.5 text-xs font-normal text-muted ">
              {{ t('batchImage.list.childCount', { n: row.child_count }, row.child_count) }}
            </span>
            <span v-if="row.is_child" class="flex-shrink-0 rounded-full bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] px-2 py-0.5 text-xs font-normal text-warning-text ">
              {{ t('batchImage.list.childBadge') }}
            </span>
          </span>
          <span class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs leading-4 text-muted">
            <span>{{ formatDate(row.created_at) }}</span>
          </span>
        </button>
      </div>
    </template>

    <template #cell-model="{ row }">
      <div class="mx-auto max-w-[180px] text-center">
        <p class="truncate text-sm text-foreground" :title="row.model">{{ row.model }}</p>
      </div>
    </template>

    <template #cell-api_key_name="{ value }">
      <span class="block truncate text-center text-sm text-foreground">
        {{ value || t('batchImage.list.keyNotRecorded') }}
      </span>
    </template>

    <template #cell-status="{ row }">
      <div class="flex justify-center">
        <span :class="statusBadgeClass(displayJob(row))" class="badge">
          {{ statusLabel(displayJob(row)) }}
        </span>
      </div>
    </template>

    <template #cell-counts="{ row }">
      <div class="flex items-center justify-center gap-2 text-sm tabular-nums">
        <span class="text-success-text ">{{ displayJob(row).success_count }}</span>
        <span class="text-muted">/</span>
        <span :class="displayJob(row).fail_count > 0 ? 'text-danger-text ' : 'text-muted'">{{ displayJob(row).fail_count }}</span>
        <span class="text-xs text-muted">{{ t('batchImage.list.totalCount', { n: displayJob(row).item_count }) }}</span>
      </div>
    </template>

    <template #cell-cost="{ row }">
      <span class="block text-center text-sm text-foreground">
        {{ costLabel(displayJob(row)) }}
      </span>
    </template>

    <template #cell-downloaded="{ row }">
      <span class="block text-center text-sm" :class="row.downloaded_at ? 'text-success-text ' : 'text-muted'">
        {{ row.downloaded_at ? formatDate(row.downloaded_at) : t('batchImage.list.notDownloaded') }}
      </span>
    </template>

    <template #cell-actions="{ row }">
      <div class="flex items-center justify-center gap-1">
        <button
          type="button"
          class="icon-btn"
          :title="t('batchImage.actions.viewDetail')"
          :aria-label="t('batchImage.actions.viewDetail')"
          @click="$emit('select-job', row.id)"
        >
          <Icon name="eye" size="sm" />
        </button>
        <button
          type="button"
          class="icon-btn"
          :disabled="!canDownload(row) || downloading"
          :title="t('batchImage.actions.downloadZip')"
          :aria-label="t('batchImage.actions.downloadZip')"
          @click="$emit('download-job', row)"
        >
          <Icon
            :name="isDownloadingJob(row.id) ? 'refresh' : 'download'"
            size="sm"
            :class="isDownloadingJob(row.id) ? 'animate-spin' : ''"
          />
        </button>
        <button
          v-if="canRetry(row) || canDeleteRecord(row)"
          type="button"
          class="icon-btn"
          :class="{ 'bg-surface-2 text-foreground': openMoreJobId === row.id }"
          :title="t('batchImage.actions.moreActions')"
          :aria-label="t('batchImage.actions.moreActions')"
          @click.stop="$emit('toggle-more-menu', row, $event)"
        >
          <Icon name="more" size="sm" />
        </button>
      </div>
    </template>

    <template #empty>
      <div class="flex min-h-[260px] flex-col items-center justify-center py-6 md:min-h-[300px]">
        <Icon name="sparkles" size="xl" class="mb-4 h-12 w-12 text-muted" />
        <p class="text-lg font-medium text-foreground ">{{ t('batchImage.list.empty') }}</p>
        <p class="mt-1 text-sm text-muted">
          {{ t('batchImage.list.emptyHint') }}
        </p>
      </div>
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Icon from '@/components/icons/Icon.vue'
import type { Column } from '@/components/common/types'
import type { BatchImageJob } from '@/api/batchImage'
import type { BatchImageJobRow } from '@/views/user/batchImage/types'

defineProps<{
  columns: Column[]
  rows: BatchImageJobRow[]
  loading: boolean
  allSelected: boolean
  someSelected: boolean
  selectedIds: Set<string>
  expandedParentIds: Set<string>
  openMoreJobId: string
  downloading: boolean
  defaultTaskName: (timestamp?: number) => string
  formatDate: (timestamp: number) => string
  statusBadgeClass: (job: Pick<BatchImageJob, 'status' | 'success_count' | 'fail_count'>) => string
  statusLabel: (job: Pick<BatchImageJob, 'status' | 'success_count' | 'fail_count'>) => string
  costLabel: (job: Pick<BatchImageJob, 'status' | 'hold_amount' | 'actual_cost'>) => string
  displayJob: (job: BatchImageJobRow) => BatchImageJobRow
  canDownload: (job: Pick<BatchImageJob, 'status' | 'success_count'>) => boolean
  canRetry: (job: BatchImageJobRow) => boolean
  canDeleteRecord: (job: Pick<BatchImageJob, 'status'>) => boolean
  isDownloadingJob: (batchId: string) => boolean
}>()

defineEmits<{
  'toggle-all': [checked: boolean]
  'toggle-row': [id: string, checked: boolean]
  'toggle-child': [id: string]
  'select-job': [id: string]
  'download-job': [job: BatchImageJobRow]
  'toggle-more-menu': [job: BatchImageJobRow, event: MouseEvent]
}>()

const { t } = useI18n()
</script>
