<template>
  <Teleport to="body">
    <div
      v-if="jobId"
      class="fixed z-[9999] w-44 overflow-hidden rounded-xl bg-surface py-1 text-sm shadow-lg ring-1 ring-black/5 "
      :style="style"
      @click.stop
    >
      <template v-for="job in jobs" :key="job.id">
        <template v-if="job.id === jobId">
          <button
            v-if="canRetry(job)"
            type="button"
            class="flex w-full items-center gap-2 px-3 py-2 text-left text-foreground transition-colors hover:bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] hover:text-warning-text disabled:opacity-60"
            :disabled="retryingBatchId === job.id"
            @click="$emit('retry', job)"
          >
            <Icon name="refresh" size="sm" :class="retryingBatchId === job.id ? 'animate-spin' : ''" />
            {{ t('batchImage.actions.retryFailedItems') }}
          </button>
          <button
            v-if="canDeleteRecord(job)"
            type="button"
            class="flex w-full items-center gap-2 px-3 py-2 text-left text-danger-text transition-colors hover:bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] disabled:opacity-60"
            :disabled="deletingBatchId === job.id"
            @click="$emit('delete', job)"
          >
            <Icon :name="deletingBatchId === job.id ? 'refresh' : 'trash'" size="sm" :class="deletingBatchId === job.id ? 'animate-spin' : ''" />
            {{ t('batchImage.actions.deleteRecords') }}
          </button>
        </template>
      </template>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { BatchImageJobRow } from '@/views/user/batchImage/types'

defineProps<{
  jobId: string
  jobs: BatchImageJobRow[]
  style: Record<string, string>
  retryingBatchId: string
  deletingBatchId: string
  canRetry: (job: BatchImageJobRow) => boolean
  canDeleteRecord: (job: BatchImageJobRow) => boolean
}>()

defineEmits<{
  retry: [job: BatchImageJobRow]
  delete: [job: BatchImageJobRow]
}>()

const { t } = useI18n()
</script>
