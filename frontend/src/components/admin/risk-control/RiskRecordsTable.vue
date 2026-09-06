<template>
  <GlassCard>
    <div class="flex flex-col gap-4 border-b border-line px-6 py-4">
      <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h2 class="text-base font-semibold text-foreground">{{ t('admin.riskControl.records') }}</h2>
          <p class="mt-1 text-sm text-muted">{{ t('admin.riskControl.recordsHint') }}</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <span
            class="inline-flex h-9 max-w-[260px] flex-shrink-0 items-center gap-1.5 truncate rounded-full border border-line bg-surface-2 px-3 text-xs text-muted"
            :title="modelFilterTooltip"
          >
            <Icon name="filter" size="xs" class="flex-shrink-0" />
            <span class="truncate">{{ modelFilterSummary }}</span>
          </span>
          <button type="button" class="btn-glass-secondary inline-flex items-center gap-2" :disabled="logsLoading" @click="emit('refresh')">
            <Icon name="refresh" size="sm" :class="logsLoading ? 'animate-spin' : ''" />
            {{ t('admin.riskControl.refresh') }}
          </button>
        </div>
      </div>

      <div class="flex flex-wrap items-center gap-2">
        <Select v-model="filters.result" class="w-[128px]" :options="resultOptions" @change="emit('reload-first-page')" />
        <Select v-model="filters.group_id" class="w-[160px]" :options="groupFilterOptions" @change="emit('reload-first-page')" />
        <Select v-model="filters.endpoint" class="w-[180px]" :options="endpointOptions" @change="emit('reload-first-page')" />
        <input v-model.trim="filters.search" type="search" class="input w-[200px]" :placeholder="t('admin.riskControl.filters.search')" @keyup.enter="emit('reload-first-page')" />
        <DateRangePicker v-model:start-date="filters.from" v-model:end-date="filters.to" @change="emit('reload-first-page')" />
      </div>
    </div>

    <p class="mb-2 flex items-center gap-1 text-xs text-muted md:hidden">
      <Icon name="chevronRight" size="xs" class="flex-shrink-0" />
      {{ t('admin.riskControl.table.scrollHint') }}
    </p>

    <div class="overflow-x-auto">
      <table class="w-full table-fixed divide-y divide-line">
        <thead class="bg-surface-2">
          <tr>
            <th class="w-[92px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.time') }}</th>
            <th class="w-[84px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.group') }}</th>
            <th class="w-[150px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.user') }}</th>
            <th class="w-[104px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.apiKey') }}</th>
            <th class="w-[152px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.endpoint') }}</th>
            <th class="w-[80px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.result') }}</th>
            <th class="w-[104px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.highest') }}</th>
            <th class="w-[144px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.actionMeta') }}</th>
            <th class="w-[92px] px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.latency') }}</th>
            <th class="px-3 py-2.5 h-[42px] text-left text-xs font-medium uppercase tracking-wider text-muted">{{ t('admin.riskControl.table.input') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-line bg-surface">
          <tr v-if="logsLoading">
            <td colspan="10" class="px-5 py-12 text-center text-sm text-muted">{{ t('common.loading') }}</td>
          </tr>
          <tr v-else-if="logs.length === 0">
            <td colspan="10" class="px-5 py-12 text-center text-sm text-muted">{{ t('admin.riskControl.emptyLogs') }}</td>
          </tr>
          <template v-else>
            <tr v-for="row in logs" :key="row.id" class="hover:bg-surface-2/60">
              <td class="px-3 py-3.5 align-top">
                <span class="cell-time text-[12.5px]" :title="formatDateTime(row.created_at)">{{ formatRelativeTime(row.created_at) }}</span>
              </td>
              <td class="px-3 py-3.5 align-top">
                <span class="block truncate text-sm text-foreground" :title="row.group_name || '-'">{{ row.group_name || '-' }}</span>
              </td>
              <td class="px-3 py-3.5 align-top">
                <div class="cell-stack" :title="row.user_email || '-'">
                  <span class="cell-title text-[13px] font-medium">{{ row.user_email || '-' }}</span>
                  <span v-if="row.user_id" class="cell-meta text-[11.5px]">UID {{ row.user_id }}</span>
                </div>
              </td>
              <td class="px-3 py-3.5 align-top">
                <span class="block truncate text-sm text-foreground" :title="row.api_key_name || '-'">{{ row.api_key_name || '-' }}</span>
              </td>
              <td class="px-3 py-3.5 align-top">
                <div class="cell-stack" :title="`${row.provider || '-'} / ${row.model || '-'}`">
                  <span class="cell-title text-[13px] font-medium">{{ row.endpoint || '-' }}</span>
                  <span class="cell-meta text-[11.5px]">{{ row.provider || '-' }} / {{ row.model || '-' }}</span>
                </div>
              </td>
              <td class="px-3 py-3.5 align-top">
                <span class="inline-flex rounded-md px-2 py-1 text-xs font-medium" :class="resultBadgeClass(row)">
                  {{ resultLabel(row) }}
                </span>
              </td>
              <td class="px-3 py-3.5 align-top">
                <div
                  class="cell-stack"
                  :title="row.matched_keyword ? `${t('admin.riskControl.matchedKeyword')}: ${row.matched_keyword}` : undefined"
                >
                  <span class="cell-title text-[13px] font-medium">{{ row.highest_category || '-' }}</span>
                  <span class="cell-meta text-[11.5px]">
                    {{ percent(row.highest_score) }}
                    <template v-if="row.matched_keyword"> · {{ t('admin.riskControl.matchedKeyword') }}</template>
                  </span>
                </div>
              </td>
              <td class="px-3 py-3.5 align-top">
                <div class="flex items-start justify-between gap-1.5">
                  <div class="cell-stack min-w-0">
                    <span class="cell-title text-[13px] font-medium">{{ violationCountText(row) }}</span>
                    <span class="cell-meta text-[11.5px]">
                      {{ row.email_sent ? t('admin.riskControl.emailSent') : t('admin.riskControl.emailNotSent') }}
                      <template v-if="row.auto_banned"> · {{ t('admin.riskControl.autoBanned') }}</template>
                    </span>
                  </div>
                  <button
                    v-if="canUnbanRow(row)"
                    type="button"
                    class="icon-btn flex-shrink-0"
                    :disabled="unbanningUserID === row.user_id"
                    :title="t('admin.riskControl.unbanUser')"
                    :aria-label="t('admin.riskControl.unbanUser')"
                    @click="emit('unban', row)"
                  >
                    <Icon name="checkCircle" size="xs" :class="unbanningUserID === row.user_id ? 'animate-spin' : ''" />
                  </button>
                </div>
              </td>
              <td class="px-3 py-3.5 align-top">
                <div class="cell-stack">
                  <span class="cell-title text-[13px] font-medium">{{ latencyText(row.upstream_latency_ms) }}</span>
                  <span v-if="row.queue_delay_ms !== null && row.queue_delay_ms !== undefined" class="cell-meta text-[11.5px]">
                    {{ t('admin.riskControl.queueDelay', { ms: row.queue_delay_ms }) }}
                  </span>
                </div>
              </td>
              <td class="px-3 py-3.5 align-top text-sm text-foreground">
                <button
                  type="button"
                  class="group flex w-full min-w-0 items-center gap-2 rounded-lg px-2 py-1 text-left transition-colors hover:bg-surface-2"
                  :title="inputSummaryText(row)"
                  @click="emit('open-input-detail', row)"
                >
                  <span class="min-w-0 flex-1 truncate">{{ inputSummaryText(row) }}</span>
                  <Icon name="eye" size="xs" class="flex-shrink-0 text-muted transition-colors group-hover:text-accent" />
                </button>
              </td>
            </tr>
          </template>
        </tbody>
      </table>
    </div>

    <Pagination
      v-if="pagination.total > 0"
      :page="pagination.page"
      :total="pagination.total"
      :page-size="pagination.page_size"
      @update:page="emit('update:page', $event)"
      @update:pageSize="emit('update:pageSize', $event)"
    />
  </GlassCard>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import GlassCard from '@/components/ui/GlassCard.vue'
import Icon from '@/components/icons/Icon.vue'
import Select from '@/components/common/Select.vue'
import Pagination from '@/components/common/Pagination.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import type { ContentModerationLog } from '@/api/admin/riskControl'
import type { SelectOption } from '@/types'
import { formatRelativeTime } from '@/utils/format'
import {
  canUnbanRow,
  formatDateTime,
  inputSummaryText,
  latencyText,
  percent,
  resultBadgeClass,
} from './riskControlUtils'

const { t } = useI18n()

defineProps<{
  logs: ContentModerationLog[]
  logsLoading: boolean
  pagination: { page: number; total: number; page_size: number }
  resultOptions: SelectOption[]
  groupFilterOptions: SelectOption[]
  endpointOptions: SelectOption[]
  modelFilterSummary: string
  modelFilterTooltip: string
  unbanningUserID: number | null
}>()

function resultLabel(row: ContentModerationLog): string {
  if (row.action === 'cyber_policy') return t('admin.riskControl.action.cyberPolicy')
  if (row.action === 'keyword_block') return t('admin.riskControl.action.keywordBlock')
  if (row.action === 'block') return t('admin.riskControl.action.block')
  if (row.action === 'error' || row.error) return t('admin.riskControl.action.error')
  if (row.flagged) return t('admin.riskControl.result.hit')
  return t('admin.riskControl.result.pass')
}

function violationCountText(row: ContentModerationLog): string {
  if (!row.flagged) return '-'
  if (row.violation_count === 0) return t('admin.riskControl.violationNotCounted')
  return t('admin.riskControl.violationCount', { count: row.violation_count || 1 })
}

const filters = defineModel<{
  result: string
  group_id: number
  endpoint: string
  search: string
  from: string
  to: string
}>('filters', { required: true })

const emit = defineEmits<{
  (e: 'refresh'): void
  (e: 'reload-first-page'): void
  (e: 'unban', row: ContentModerationLog): void
  (e: 'open-input-detail', row: ContentModerationLog): void
  (e: 'update:page', page: number): void
  (e: 'update:pageSize', pageSize: number): void
}>()
</script>

<style scoped>
.cell-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.cell-title {
  line-height: 1.25;
  color: var(--foreground);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-meta {
  line-height: 1.3;
  color: var(--muted);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.cell-time {
  color: var(--muted);
  font-variant-numeric: tabular-nums;
}
</style>
