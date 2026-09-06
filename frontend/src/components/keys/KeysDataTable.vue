<template>
  <DataTable
    class="keys-table"
    :columns="columns"
    :data="apiKeys"
    :loading="loading"
    :server-side-sort="true"
    default-sort-key="created_at"
    default-sort-order="desc"
    @sort="(key, order) => $emit('sort', key, order)"
  >
    <template #cell-name="{ value, row }">
      <div class="keys-cell-name">
        <span class="keys-name-line">
          <span class="keys-name-text">{{ value }}</span>
          <Icon
            v-if="hasIpRestriction(row)"
            name="shield"
            size="xs"
            class="keys-name-shield"
            :title="t('keys.ipRestrictionEnabled')"
          />
        </span>
        <span class="keys-name-id">#{{ row.id }}</span>
      </div>
    </template>

    <template #cell-id="{ value }">
      <span class="keys-name-id">#{{ value }}</span>
    </template>

    <template #cell-key="{ value, row }">
      <div class="keys-cell-key">
        <code class="code keys-key-code">{{ isKeyRevealed(row.id) ? value : maskApiKey(value) }}</code>
        <button
          type="button"
          class="icon-btn keys-icon-btn-xs"
          :title="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
          :aria-label="isKeyRevealed(row.id) ? t('keys.hideKey') : t('keys.showKey')"
          @click.stop="$emit('toggle-reveal', row.id)"
        >
          <Icon :name="isKeyRevealed(row.id) ? 'eyeOff' : 'eye'" size="xs" />
        </button>
        <button
          type="button"
          class="icon-btn keys-icon-btn-xs"
          :class="{ 'is-copied': copiedKeyId === row.id }"
          :title="copiedKeyId === row.id ? t('keys.copied') : t('keys.copyToClipboard')"
          :aria-label="t('keys.copyToClipboard')"
          @click.stop="$emit('copy', value, row.id)"
        >
          <Icon :name="copiedKeyId === row.id ? 'check' : 'clipboard'" size="xs" />
        </button>
      </div>
    </template>

    <template #cell-group="{ row }">
      <div class="group/dropdown relative">
        <button
          type="button"
          :ref="(el) => setGroupButtonRef(row.id, el)"
          class="keys-group-btn"
          :title="groupCellTooltip(row)"
          @click="$emit('open-group-selector', row)"
        >
          <span v-if="row.group" class="tag tag-accent keys-group-pill">
            <span class="truncate">{{ row.group.name }}</span>
            <span v-if="groupCellSuffix(row)" class="keys-group-suffix">{{ groupCellSuffix(row) }}</span>
          </span>
          <span v-else class="tag">{{ t('keys.noGroup') }}</span>
          <Icon name="chevronDown" size="xs" class="keys-group-caret" />
        </button>
      </div>
    </template>

    <template #cell-current_concurrency="{ value }">
      <span class="keys-concurrency">{{ value ?? 0 }}</span>
    </template>

    <template #cell-usage="{ row }">
      <div class="keys-usage">
        <span class="keys-usage-today" :title="`$${(usageStats[row.id]?.today_actual_cost ?? 0).toFixed(4)}`">
          {{ formatCost(usageStats[row.id]?.today_actual_cost) }}
        </span>
        <span class="keys-usage-total" :title="`$${(usageStats[row.id]?.total_actual_cost ?? 0).toFixed(4)}`">
          {{ formatCost(usageStats[row.id]?.total_actual_cost) }}
        </span>
        <span
          v-if="row.quota > 0"
          class="progress progress-thin keys-usage-quota"
          :title="`${t('keys.quota')} ${formatCost(row.quota_used)} / ${formatCost(row.quota)}`"
        >
          <span
            class="progress-bar"
            :class="quotaBarClass(row)"
            :style="{ width: quotaPercent(row) + '%' }"
          />
        </span>
      </div>
    </template>

    <template #cell-rate_limit="{ row }">
      <span
        v-if="rateLimitSummary(row)"
        class="keys-rate-limit"
        :class="rateLimitToneClass(row)"
        :title="rateLimitDetail(row)"
      >
        {{ rateLimitSummary(row) }}
      </span>
      <span v-else class="keys-cell-empty">—</span>
    </template>

    <template #cell-expires_at="{ value }">
      <span class="keys-expiry" :class="expiryToneClass(value, now)">
        {{ value ? formatDate(value) : t('keys.noExpiration') }}
      </span>
    </template>

    <template #cell-last_used_at="{ value }">
      <span class="keys-muted-cell">{{ value ? formatDate(value) : '—' }}</span>
    </template>

    <template #cell-last_used_ip="{ value }">
      <span class="keys-muted-cell keys-mono-cell">{{ value || '—' }}</span>
    </template>

    <template #cell-created_at="{ value }">
      <span class="keys-muted-cell">{{ formatDate(value) }}</span>
    </template>

    <template #cell-status="{ value }">
      <StatusBadge :tone="statusTone(value)" :label="t('keys.status.' + value)" dot />
    </template>

    <template #cell-actions="{ row }">
      <div class="keys-actions">
        <button type="button" class="keys-use-btn" @click.stop="$emit('use', row)">
          {{ t('keys.use') }}
        </button>
        <button
          type="button"
          class="icon-btn keys-more-btn"
          :title="t('keys.moreActions')"
          :aria-label="t('keys.moreActions')"
          @click.stop="$emit('more', row, $event)"
        >
          <Icon name="more" size="sm" :stroke-width="2.4" />
        </button>
      </div>
    </template>

    <template #empty>
      <EmptyState
        :title="t('keys.noKeysYet')"
        :description="t('keys.createFirstKey')"
        :action-text="t('keys.createKey')"
        @action="$emit('empty-action')"
      />
    </template>
  </DataTable>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ComponentPublicInstance } from 'vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { ApiKey } from '@/types'
import type { Column } from '@/components/common/types'
import type { BatchApiKeyUsageStats } from '@/api/usage'
import { formatDate } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import {
  formatCost,
  quotaPercent,
  quotaBarClass,
  statusTone,
  expiryToneClass,
  hasIpRestriction,
  rateLimitSummary,
  rateLimitToneClass
} from './keyUtils'

defineProps<{
  columns: Column[]
  apiKeys: ApiKey[]
  loading: boolean
  usageStats: Record<string, BatchApiKeyUsageStats>
  now: Date
  copiedKeyId: number | null
  isKeyRevealed: (id: number) => boolean
  setGroupButtonRef: (id: number, el: Element | ComponentPublicInstance | null) => void
  groupCellSuffix: (row: ApiKey) => string
  groupCellTooltip: (row: ApiKey) => string
  rateLimitDetail: (row: ApiKey) => string
}>()

defineEmits<{
  sort: [key: string, order: 'asc' | 'desc']
  'toggle-reveal': [id: number]
  copy: [value: string, keyId: number]
  'open-group-selector': [row: ApiKey]
  use: [row: ApiKey]
  more: [row: ApiKey, event: MouseEvent]
  'empty-action': []
}>()

const { t } = useI18n()
</script>

<style scoped>
/* ---------- Table card ---------- */
.table-wrapper.keys-table :deep(thead) {
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
  backdrop-filter: none;
}

.table-wrapper.keys-table :deep(tbody) {
  background: transparent;
}

.table-wrapper.keys-table :deep(table) {
  table-layout: fixed;
}

.table-wrapper.keys-table :deep(th) {
  height: 43px;
  padding: 0 8px;
  font-size: var(--fs-11);
  font-weight: var(--fw-semibold);
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}

.table-wrapper.keys-table :deep(td) {
  height: 59px;
  padding: 0 8px;
  font-size: var(--fs-13);
  color: var(--foreground);
  border-bottom: 1px solid var(--border);
  vertical-align: middle;
}

.table-wrapper.keys-table :deep(th:first-child),
.table-wrapper.keys-table :deep(td:first-child) {
  padding-left: 16px;
}

.table-wrapper.keys-table :deep(th:last-child),
.table-wrapper.keys-table :deep(td:last-child) {
  padding-right: 16px;
}

/* Fixed column widths (ratios match the 05 reference: name/key flex a bit
   wider, the rest are content-driven fixed widths). table-layout:fixed with
   table {width:100%} scales these proportionally to the card's inner width. */
.table-wrapper.keys-table :deep(th:nth-child(1)),
.table-wrapper.keys-table :deep(td:nth-child(1)) {
  width: 150px;
}

.table-wrapper.keys-table :deep(th:nth-child(2)),
.table-wrapper.keys-table :deep(td:nth-child(2)) {
  width: 250px;
}

.table-wrapper.keys-table :deep(th:nth-child(3)),
.table-wrapper.keys-table :deep(td:nth-child(3)) {
  width: 96px;
}

.table-wrapper.keys-table :deep(th:nth-child(4)),
.table-wrapper.keys-table :deep(td:nth-child(4)) {
  width: 56px;
}

.table-wrapper.keys-table :deep(th:nth-child(5)),
.table-wrapper.keys-table :deep(td:nth-child(5)) {
  width: 100px;
}

.table-wrapper.keys-table :deep(th:nth-child(6)),
.table-wrapper.keys-table :deep(td:nth-child(6)) {
  width: 72px;
}

.table-wrapper.keys-table :deep(th:nth-child(7)),
.table-wrapper.keys-table :deep(td:nth-child(7)) {
  width: 88px;
}

.table-wrapper.keys-table :deep(th:nth-child(8)),
.table-wrapper.keys-table :deep(td:nth-child(8)) {
  width: 80px;
}

.table-wrapper.keys-table :deep(th:nth-child(9)),
.table-wrapper.keys-table :deep(td:nth-child(9)) {
  width: 76px;
}

.table-wrapper.keys-table :deep(th:nth-child(10)),
.table-wrapper.keys-table :deep(td:nth-child(10)) {
  width: 80px;
}

/* ---------- Cells ---------- */
.keys-cell-name {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 2px;
  min-width: 0;
  line-height: 1.2;
}

.keys-name-line {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-width: 0;
}

.keys-name-text {
  font-weight: var(--fw-semibold);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-name-shield {
  flex: none;
  color: var(--accent);
}

.keys-name-id {
  font-family: var(--font-mono);
  font-size: var(--fs-11-5);
  color: var(--muted);
}

.keys-cell-key {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}

.keys-key-code {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-icon-btn-xs {
  width: 26px;
  height: 26px;
  border-radius: var(--radius-7);
}

.keys-icon-btn-xs.is-copied {
  color: var(--success-text);
}

.keys-group-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  max-width: 100%;
  margin: -3px -6px;
  padding: 3px 6px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  cursor: pointer;
  transition: background 0.15s ease;
}

.keys-group-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
}

.keys-group-pill {
  max-width: 160px;
}

.keys-group-suffix {
  flex: none;
  opacity: 0.65;
  font-weight: var(--fw-medium);
}

.keys-group-caret {
  flex: none;
  color: var(--muted);
  opacity: 0.6;
}

.keys-group-btn:hover .keys-group-caret {
  opacity: 1;
}

.keys-concurrency {
  font-family: var(--font-mono);
  font-size: var(--fs-12-5);
  font-variant-numeric: tabular-nums;
}

.keys-usage {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 2px;
  font-variant-numeric: tabular-nums;
  line-height: 1.2;
}

.keys-usage-today {
  font-weight: var(--fw-semibold);
}

.keys-usage-total {
  font-size: var(--fs-11-5);
  color: var(--muted);
}

.keys-usage-quota {
  display: block;
  width: 72px;
  margin-top: 2px;
}

.keys-rate-limit,
.keys-expiry,
.keys-muted-cell {
  font-size: var(--fs-12-5);
  color: var(--muted);
}

.keys-mono-cell {
  font-family: var(--font-mono);
  font-size: var(--fs-12);
}

.keys-cell-empty {
  color: var(--muted);
}

.keys-tone-danger {
  color: var(--danger-text);
}

.keys-tone-warning {
  color: var(--warning-text);
}

.keys-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  gap: 2px;
}

.keys-use-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  height: 28px;
  padding: 0 9px;
  border: 0;
  border-radius: var(--radius-sm);
  background: transparent;
  color: var(--accent);
  font-size: var(--fs-12);
  font-weight: var(--fw-semibold);
  cursor: pointer;
  transition: background 0.15s ease;
}

.keys-use-btn:hover {
  background: color-mix(in oklch, var(--accent) 10%, transparent);
}

.keys-more-btn {
  width: 28px;
  height: 28px;
}
</style>
