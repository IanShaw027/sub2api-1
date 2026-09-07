<template>
  <div class="distribution">
    <p v-if="view === 'users'" class="distribution-caption">{{ t('admin.dashboard.rankingLimit', { count: 12 }) }}</p>
    <div v-for="row in rows" :key="row.key" class="distribution-item">
      <div class="distribution-heading">
        <button type="button" class="distribution-toggle" :aria-expanded="!!expanded[row.key]"
          :title="t(expanded[row.key] ? 'admin.dashboard.collapseDetails' : 'admin.dashboard.expandDetails')"
          :aria-label="`${t(expanded[row.key] ? 'admin.dashboard.collapseDetails' : 'admin.dashboard.expandDetails')}: ${row.name}`"
          @click="toggle(row)">
          <Icon :name="expanded[row.key] ? 'chevronDown' : 'chevronRight'" size="sm" />
          <span class="distribution-name" :title="row.name">{{ row.name }}</span>
        </button>
        <span class="distribution-cost" :title="t('admin.dashboard.actual')">${{ row.cost }} · {{ row.pct }}%</span>
        <button v-if="row.userId" type="button" class="btn-icon" :title="t('admin.dashboard.viewUsage')"
          :aria-label="t('admin.dashboard.viewUsage')" @click="$emit('userRowClick', row.userId)">
          <Icon name="arrowRight" size="sm" />
        </button>
      </div>
      <dl class="distribution-metrics">
        <div><dt>{{ t('admin.dashboard.requestsShort') }}</dt><dd>{{ formatNumber(row.requests) }}</dd></div>
        <div><dt>Tokens</dt><dd>{{ formatTokens(row.tokens) }}</dd></div>
        <div v-if="row.standardCost != null"><dt>{{ t('admin.dashboard.standard') }}</dt><dd>${{ formatCost(row.standardCost) }}</dd></div>
        <div v-if="row.accountCost != null"><dt>{{ t('admin.dashboard.accountCost') }}</dt><dd>${{ formatCost(row.accountCost) }}</dd></div>
      </dl>
      <div class="distribution-track"><span :style="{ width: `${Math.min(100, Math.max(0, row.pct))}%` }" /></div>
      <div v-if="expanded[row.key]" class="distribution-details">
        <LoadingSpinner v-if="details[row.key]?.loading" size="sm" />
        <div v-else-if="details[row.key]?.error" role="alert" class="distribution-error">
          {{ t('admin.dashboard.failedToLoad') }}
          <button type="button" class="btn btn-secondary btn-sm" @click="load(row)">{{ t('common.retry') }}</button>
        </div>
        <DataTable v-else :columns="columns" :data="details[row.key]?.rows || []" row-key="key">
          <template #cell-name="{ row: detail }">
            <button v-if="detail.userId" type="button" class="distribution-user" :title="t('admin.dashboard.viewUsage')"
              @click="$emit('userRowClick', detail.userId)">{{ detail.name }}</button>
            <span v-else>{{ detail.name }}</span>
          </template>
        </DataTable>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import DataTable from '@/components/common/DataTable.vue'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import type { Column } from '@/components/common/types'
import { getModelStats, getUserBreakdown } from '@/api/admin/dashboard'
import { formatCost, formatNumber, formatTokens } from './useDashboardFormat'
import type { DistributionRow } from './types'

const props = defineProps<{ rows: DistributionRow[]; view: 'models' | 'users'; startDate: string; endDate: string }>()
defineEmits<{ userRowClick: [userId: number] }>()
const { t } = useI18n()
type DetailRow = { key: string; name: string; userId?: number; requests: string; tokens: string; input: string; output: string; cache: string; actual: string; standard: string; account: string }
type Detail = { loading: boolean; error: boolean; rows: DetailRow[] }
const expanded = ref<Record<string, boolean>>({})
const details = ref<Record<string, Detail>>({})
let generation = 0
watch(() => [props.startDate, props.endDate, props.rows], () => {
  generation++
  expanded.value = {}
  details.value = {}
})
const columns = computed<Column[]>(() => [
  { key: 'name', label: t(props.view === 'models' ? 'admin.dashboard.viewSpendingRanking' : 'admin.dashboard.viewModelDistribution'), maxWidth: 220 },
  { key: 'requests', label: t('admin.dashboard.requestsShort'), maxWidth: 100 },
  { key: 'tokens', label: 'Tokens', maxWidth: 100 },
  { key: 'input', label: t('admin.dashboard.input'), maxWidth: 100 },
  { key: 'output', label: t('admin.dashboard.output'), maxWidth: 100 },
  { key: 'cache', label: t('admin.dashboard.cache'), maxWidth: 100 },
  { key: 'actual', label: t('admin.dashboard.actual'), maxWidth: 110 },
  { key: 'standard', label: t('admin.dashboard.standard'), maxWidth: 110 },
  { key: 'account', label: t('admin.dashboard.accountCost'), maxWidth: 110 }
])
async function load(row: DistributionRow) {
  const current = generation
  details.value[row.key] = { loading: true, error: false, rows: [] }
  const range = { start_date: props.startDate, end_date: props.endDate }
  try {
    const rows: DetailRow[] = row.userId
      ? (await getModelStats({ ...range, user_id: row.userId })).models.map(item => ({
        key: item.model, name: item.model, requests: formatNumber(item.requests), tokens: formatTokens(item.total_tokens),
        input: formatTokens(item.input_tokens), output: formatTokens(item.output_tokens),
        cache: formatTokens(item.cache_creation_tokens + item.cache_read_tokens),
        actual: `$${formatCost(item.actual_cost)}`, standard: `$${formatCost(item.cost)}`,
        account: item.account_cost == null ? '-' : `$${formatCost(item.account_cost)}`
      }))
      : (await getUserBreakdown({ ...range, model: row.model, limit: 100 })).users.map(item => ({
        key: String(item.user_id), name: item.email || `#${item.user_id}`, userId: item.user_id,
        requests: formatNumber(item.requests), tokens: formatTokens(item.total_tokens),
        input: formatTokens(item.input_tokens), output: formatTokens(item.output_tokens), cache: formatTokens(item.cache_tokens),
        actual: `$${formatCost(item.actual_cost)}`, standard: `$${formatCost(item.cost)}`, account: `$${formatCost(item.account_cost)}`
      }))
    if (current === generation) details.value[row.key] = { loading: false, error: false, rows }
  } catch {
    if (current === generation) details.value[row.key] = { loading: false, error: true, rows: [] }
  }
}
function toggle(row: DistributionRow) {
  expanded.value[row.key] = !expanded.value[row.key]
  if (expanded.value[row.key] && !details.value[row.key]) void load(row)
}
</script>

<style scoped>
.distribution { min-width: 0; max-height: 320px; overflow: auto; }
.distribution-caption { font-size: 12px; color: var(--muted); margin: 0 0 8px; }
.distribution-item { padding: 12px 0; border-bottom: 1px solid var(--border); }
.distribution-item:last-child { border-bottom: 0; }
.distribution-heading { display: flex; align-items: center; gap: 8px; min-width: 0; }
.distribution-toggle { display: flex; align-items: center; gap: 6px; flex: 1; min-width: 0; text-align: left; }
.distribution-toggle > svg { flex: none; }
.distribution-name { overflow-wrap: anywhere; font-size: 12px; }
.distribution-cost { flex: none; font-size: 12px; font-variant-numeric: tabular-nums; }
.distribution-metrics { display: flex; gap: 6px 14px; flex-wrap: wrap; font-size: 11px; margin: 6px 0; }
.distribution-metrics > div { display: flex; gap: 4px; }
.distribution-metrics dt { color: var(--muted); }
.distribution-metrics dd { margin: 0; font-variant-numeric: tabular-nums; }
.distribution-track { height: 4px; background: var(--surface-tertiary); }
.distribution-track > span { display: block; height: 100%; background: var(--accent); }
.distribution-details { margin-top: 12px; min-width: 0; }
.distribution-error { display: flex; align-items: center; gap: 8px; flex-wrap: wrap; }
.distribution-user { color: var(--info-text); text-align: left; overflow-wrap: anywhere; }
@media (max-width: 767px) { .distribution { padding: 0 14px; } }
</style>
