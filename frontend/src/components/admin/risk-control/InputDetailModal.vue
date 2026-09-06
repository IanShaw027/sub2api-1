<template>
  <UiModal
    :open="row !== null"
    :title="t('admin.riskControl.inputDetailTitle')"
    width="lg"
    @close="emit('close')"
  >
    <div v-if="row" class="space-y-5">
      <div class="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
        <div class="rounded-lg border border-line bg-surface-2 p-4">
          <p class="text-xs font-medium text-muted">{{ t('admin.riskControl.table.time') }}</p>
          <p class="mt-1 truncate text-sm font-semibold text-foreground">{{ formatDateTime(row.created_at) }}</p>
        </div>
        <div class="rounded-lg border border-line bg-surface-2 p-4">
          <p class="text-xs font-medium text-muted">{{ t('admin.riskControl.table.user') }}</p>
          <p class="mt-1 truncate text-sm font-semibold text-foreground">{{ row.user_email || '-' }}</p>
        </div>
        <div class="rounded-lg border border-line bg-surface-2 p-4">
          <p class="text-xs font-medium text-muted">{{ t('admin.riskControl.table.result') }}</p>
          <span class="mt-1 inline-flex rounded-md px-2 py-1 text-xs font-medium" :class="resultBadgeClass(row)">
            {{ resultLabel(row) }}
          </span>
        </div>
        <div class="rounded-lg border border-line bg-surface-2 p-4">
          <p class="text-xs font-medium text-muted">{{ t('admin.riskControl.table.highest') }}</p>
          <p class="mt-1 truncate text-sm font-semibold text-foreground">
            {{ row.highest_category || '-' }} / {{ percent(row.highest_score) }}
          </p>
        </div>
        <div v-if="row.matched_keyword" class="rounded-lg border border-danger-100 bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] p-4  ">
          <p class="text-xs font-medium text-danger-500 ">{{ t('admin.riskControl.matchedKeyword') }}</p>
          <p class="mt-1 truncate text-sm font-semibold text-danger-text " :title="row.matched_keyword">{{ row.matched_keyword }}</p>
        </div>
      </div>

      <div class="rounded-xl border border-line bg-surface p-4 shadow-sm">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div>
            <p class="text-sm font-semibold text-foreground">{{ t('admin.riskControl.inputDetailContent') }}</p>
            <p class="mt-1 text-xs text-muted">
              {{ row.endpoint || '-' }} · {{ row.provider || '-' }} / {{ row.model || '-' }}
            </p>
          </div>
          <span v-if="row.group_name" class="inline-flex rounded-md bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-2.5 py-1 text-xs font-medium text-accent  ">
            {{ row.group_name }}
          </span>
        </div>
        <pre class="code-block mt-4 max-h-[420px] overflow-auto whitespace-pre-wrap break-words">{{ inputDetailText }}</pre>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end">
        <button type="button" class="btn-glass-secondary" @click="emit('close')">{{ t('common.close') }}</button>
      </div>
    </template>
  </UiModal>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import UiModal from '@/components/ui/UiModal.vue'
import type { ContentModerationLog } from '@/api/admin/riskControl'
import { formatDateTime, percent, resultBadgeClass } from './riskControlUtils'

const { t } = useI18n()

const props = defineProps<{
  row: ContentModerationLog | null
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

function resultLabel(row: ContentModerationLog): string {
  if (row.action === 'cyber_policy') return t('admin.riskControl.action.cyberPolicy')
  if (row.action === 'keyword_block') return t('admin.riskControl.action.keywordBlock')
  if (row.action === 'block') return t('admin.riskControl.action.block')
  if (row.action === 'error' || row.error) return t('admin.riskControl.action.error')
  if (row.flagged) return t('admin.riskControl.result.hit')
  return t('admin.riskControl.result.pass')
}

const inputDetailText = computed(() => {
  if (!props.row) return '-'
  return props.row.input_excerpt || props.row.error || '-'
})
</script>
