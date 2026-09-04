<template>
 <section aria-labelledby="prompt-events-title" class="py-6">
 <div class="flex flex-wrap items-start justify-between gap-3">
 <div>
 <h2 id="prompt-events-title" class="text-base font-semibold text-foreground">{{ t('admin.promptAudit.events.title') }}</h2>
 <p class="mt-1 text-sm text-muted">{{ t('admin.promptAudit.events.description') }}</p>
 </div>
 <div class="flex flex-wrap gap-2">
 <button type="button" class="btn btn-secondary btn-sm" :disabled="selectedIds.length === 0" @click="$emit('batch-delete')">
 {{ t('admin.promptAudit.events.deleteSelected', { count: selectedIds.length }) }}
 </button>
 <button type="button" class="btn btn-danger btn-sm" data-test="filter-delete" @click="$emit('preview-delete')">
 {{ t('admin.promptAudit.events.deleteByFilter') }}
 </button>
 </div>
 </div>

 <form class="mt-5 flex flex-wrap items-center gap-2" @submit.prevent="applyFilters">
 <select v-model="localFilters.decision" class="input h-9 w-[128px]" :aria-label="t('admin.promptAudit.events.decision')" @change="filtersChanged">
 <option value="">{{ t('admin.promptAudit.events.decision') }}</option>
 <option value="pass">{{ t('admin.promptAudit.decisions.pass') }}</option>
 <option value="flag">{{ t('admin.promptAudit.decisions.flag') }}</option>
 <option value="critical">{{ t('admin.promptAudit.decisions.critical') }}</option>
 </select>
 <select v-model="localFilters.risk_level" class="input h-9 w-[128px]" :aria-label="t('admin.promptAudit.events.risk')" @change="filtersChanged">
 <option value="">{{ t('admin.promptAudit.events.risk') }}</option>
 <option value="low">{{ t('admin.promptAudit.riskLevels.low') }}</option>
 <option value="medium">{{ t('admin.promptAudit.riskLevels.medium') }}</option>
 <option value="high">{{ t('admin.promptAudit.riskLevels.high') }}</option>
 <option value="critical">{{ t('admin.promptAudit.riskLevels.critical') }}</option>
 </select>
 <input v-model="localFilters.endpoint" type="text" class="input h-9 w-[150px]" :placeholder="t('admin.promptAudit.events.endpoint')" :aria-label="t('admin.promptAudit.events.endpoint')" @change="filtersChanged" />
 <input v-model="localFilters.keyword" type="text" class="input h-9 w-[180px]" :placeholder="t('admin.promptAudit.events.keyword')" :aria-label="t('admin.promptAudit.events.keyword')" @change="filtersChanged" />
 <DateRangePicker v-model:start-date="filterStartDate" v-model:end-date="filterEndDate" @change="handleDateRangeChange" />
 <button type="submit" class="btn btn-primary btn-sm h-9">{{ t('common.search') }}</button>
 <button type="button" class="btn btn-ghost btn-sm h-9" data-test="toggle-advanced" @click="advancedOpen = !advancedOpen">
 {{ t('admin.promptAudit.events.advancedFilters') }}
 <Icon name="chevronDown" size="xs" :class="advancedOpen ? 'rotate-180' : ''" />
 </button>
 <button type="button" class="btn btn-ghost btn-sm h-9" @click="resetFilters">{{ t('common.reset') }}</button>
 </form>
 <div v-show="advancedOpen" data-test="advanced-filters" class="mt-3 grid gap-3 rounded-lg border border-line bg-surface-2 p-3 sm:grid-cols-2 lg:grid-cols-4">
 <FilterInput v-model="localFilters.group_id" :label="t('admin.promptAudit.events.groupId')" type="number" @change="filtersChanged" />
 <FilterInput v-model="localFilters.user_id" :label="t('admin.promptAudit.events.userId')" type="number" @change="filtersChanged" />
 <FilterInput v-model="localFilters.api_key_id" :label="t('admin.promptAudit.events.apiKeyId')" type="number" @change="filtersChanged" />
 <FilterInput v-model="localFilters.request_id" :label="t('admin.promptAudit.events.requestId')" @change="filtersChanged" />
 <FilterInput v-model="localFilters.prompt_hash" :label="t('admin.promptAudit.events.promptHash')" @change="filtersChanged" />
 </div>
 <div v-if="error" role="alert" class="notice notice-danger mt-4 text-sm">{{ error }}</div>
 <div class="mt-5 overflow-hidden rounded-xl border border-line pa-events-card">
 <DataTable
 :columns="cols"
 :data="events"
 :loading="loading"
 row-key="id"
 :estimate-row-height="61"
 >
 <template #header-select>
 <input type="checkbox" class="pa-checkbox" :checked="allSelected" :aria-label="t('admin.promptAudit.events.selectAll')" @change="toggleAll" />
 </template>
 <template #cell-select="{ row }">
 <input type="checkbox" class="pa-checkbox" :checked="selectedIds.includes(row.id)" :aria-label="t('admin.promptAudit.events.selectEvent', { id: row.id })" @change="toggleOne(row.id)" />
 </template>
 <template #cell-time="{ row }">
 <span class="cell-time" :title="formatDate(row.created_at)">{{ formatRelativeTime(row.created_at) }}</span>
 </template>
 <template #cell-identity="{ row }">
 <div class="cell-stack" :title="`${row.snapshot.username} · ${row.snapshot.user_email} · ${row.snapshot.api_key_name}`">
 <span class="cell-title">{{ row.snapshot.username || '—' }}</span>
 <span class="cell-meta">{{ row.snapshot.user_email }}{{ row.snapshot.api_key_name ? ` · ${row.snapshot.api_key_name}` : '' }}</span>
 </div>
 </template>
 <template #cell-group="{ row }">
 <span class="cell-meta">{{ row.snapshot.group_name || '—' }}</span>
 </template>
 <template #cell-route="{ row }">
 <div class="cell-stack">
 <span class="cell-title">{{ row.snapshot.endpoint }}</span>
 <span class="cell-meta">{{ row.snapshot.model }} · {{ row.snapshot.protocol }} · {{ row.snapshot.stage || 'http' }}</span>
 </div>
 </template>
 <template #cell-result="{ row }">
 <div class="cell-stack">
 <span class="tag" :class="decisionClass(row.decision)">{{ formatDecisionRisk(row.decision, row.risk_level) }}</span>
 <span class="cell-meta" :title="formatCategories(row.categories)">{{ formatCategories(row.categories) }}</span>
 </div>
 </template>
 <template #cell-preview="{ row }">
 <p class="cell-meta" :title="row.snapshot.redacted_preview || '—'">{{ row.snapshot.redacted_preview || '—' }}</p>
 </template>
 <template #cell-actions="{ row }">
 <button type="button" class="btn btn-ghost btn-sm" @click="$emit('view', row.id)">{{ t('common.view') }}</button>
 <button type="button" class="btn btn-ghost btn-sm text-danger-text" @click="$emit('delete', row.id)">{{ t('common.delete') }}</button>
 </template>
 <template #empty>
 <div class="empty-state data-table-empty">
 <Icon name="inbox" class="data-table-empty-icon" :stroke-width="1.6" aria-hidden="true" />
 <p class="data-table-empty-title">{{ t('admin.promptAudit.events.empty') }}</p>
 </div>
 </template>
 </DataTable>
 <Pagination class="pa-events-pagination" :total="total" :page="page" :page-size="pageSize" @update:page="$emit('page', $event)" @update:page-size="$emit('page-size', $event)" />
 </div>
 </section>
</template>

<script setup lang="ts">
import { computed, defineComponent, h, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import type { Column } from '@/components/common/types'
import type { PromptAuditEvent, PromptEventFilters } from '../types'
import { cloneData, emptyEventFilters, SCANNER_CATALOG } from '../viewModel'
import { formatRelativeTime } from '@/utils/format'

const props = defineProps<{
 events: PromptAuditEvent[]; total: number; page: number; pageSize: number
 filters: PromptEventFilters; selectedIds: number[]; loading: boolean; error: string
}>()
const emit = defineEmits<{
 (event: 'filters-change', value: PromptEventFilters): void
 (event: 'search', value: PromptEventFilters): void
 (event: 'selection', value: number[]): void
 (event: 'page', value: number): void
 (event: 'page-size', value: number): void
 (event: 'view', id: number): void
 (event: 'delete', id: number): void
 (event: 'batch-delete'): void
 (event: 'preview-delete'): void
}>()
const { t, locale } = useI18n()
const localFilters = reactive<PromptEventFilters>(cloneData(props.filters))
// DateRangePicker only exposes date-level granularity (YYYY-MM-DD) and is kept as separate
// local state so that the end-of-day time suffix normalized into localFilters.end_at (see
// handleDateRangeChange) never round-trips back into the picker's own display value.
const datePart = (value: string) => (value ? value.slice(0, 10) : '')
const filterStartDate = ref(datePart(props.filters.start_at))
const filterEndDate = ref(datePart(props.filters.end_at))
watch(() => props.filters, (value) => {
 Object.assign(localFilters, cloneData(value))
 filterStartDate.value = datePart(value.start_at)
 filterEndDate.value = datePart(value.end_at)
}, { deep: true })
const advancedOpen = ref(false)
const allSelected = computed(() => props.events.length > 0 && props.events.every((event) => props.selectedIds.includes(event.id)))
const cols = computed<Column[]>(() => [
 { key: 'select', label: '' },
 { key: 'time', label: t('admin.promptAudit.events.time'), class: 'w-[104px]' },
 { key: 'identity', label: t('admin.promptAudit.events.identity'), class: 'w-[170px]' },
 { key: 'group', label: t('admin.promptAudit.events.group'), class: 'w-[96px]' },
 { key: 'route', label: t('admin.promptAudit.events.route'), class: 'w-[160px]' },
 { key: 'result', label: t('admin.promptAudit.events.result'), class: 'w-[128px]' },
 { key: 'preview', label: t('admin.promptAudit.events.preview'), class: 'w-[220px]' },
 { key: 'actions', label: t('admin.promptAudit.common.actions'), class: 'w-[112px] text-right' },
])

const FilterInput = defineComponent({
 props: { modelValue: { type: String, required: true }, label: { type: String, required: true }, type: { type: String, default: 'text' } },
 emits: ['update:modelValue', 'change'],
 setup(componentProps, { emit: componentEmit }) {
 return () => h('label', { class: 'text-xs text-muted' }, [
 h('span', componentProps.label),
 h('input', {
 value: componentProps.modelValue, type: componentProps.type, class: 'input mt-1 w-full', 'aria-label': componentProps.label,
 onInput: (event: Event) => componentEmit('update:modelValue', (event.target as HTMLInputElement).value),
 onChange: () => componentEmit('change'),
 }),
 ])
 },
})

function filtersChanged() {
 emit('filters-change', cloneData(localFilters))
}
function applyFilters() {
 const value = cloneData(localFilters)
 emit('filters-change', value)
 emit('search', value)
}
function resetFilters() {
 Object.assign(localFilters, emptyEventFilters())
 filterStartDate.value = ''
 filterEndDate.value = ''
 applyFilters()
}
// Selecting a calendar end date should include that entire day, so append end-of-day time
// only on localFilters.start_at/end_at (consumed by eventQueryParams' toISO), never on the
// picker's own filterStartDate/filterEndDate refs.
function handleDateRangeChange() {
 localFilters.start_at = filterStartDate.value
 localFilters.end_at = filterEndDate.value ? `${filterEndDate.value}T23:59:59.999` : ''
 filtersChanged()
}
function toggleOne(id: number) {
 const selected = new Set(props.selectedIds)
 if (selected.has(id)) selected.delete(id)
 else selected.add(id)
 emit('selection', [...selected])
}
function toggleAll() {
 emit('selection', allSelected.value ? [] : props.events.map((event) => event.id))
}
function formatDate(value: string): string {
 return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'medium' }).format(new Date(value))
}
function decisionClass(decision: string): string {
 if (decision === 'critical') return 'tag-danger'
 if (decision === 'flag') return 'tag-warning'
 return 'tag-success'
}
const DECISIONS = new Set(['pass', 'flag', 'critical'])
const RISK_LEVELS = new Set(['low', 'medium', 'high', 'critical'])

function translateDecision(decision: string): string {
 return DECISIONS.has(decision) ? t(`admin.promptAudit.decisions.${decision}`) : decision
}
function translateRiskLevel(riskLevel: string): string {
 return RISK_LEVELS.has(riskLevel) ? t(`admin.promptAudit.riskLevels.${riskLevel}`) : riskLevel
}
function translateCategory(category: string): string {
 return SCANNER_CATALOG.some((scanner) => scanner.id === category)
 ? t(`admin.promptAudit.scanners.${category}`)
 : category
}
function formatDecisionRisk(decision: string, riskLevel: string): string {
 return `${translateDecision(decision)} · ${translateRiskLevel(riskLevel)}`
}
function formatCategories(categories: string[]): string {
 if (!categories.length) return '—'
 return categories.map(translateCategory).join(', ')
}
</script>

<style scoped>
/* Sub-section table wrapper: a plain border box (no nested GlassCard — EventWorkspace
 * already renders inside PromptAuditView's outer GlassCard) around DataTable/Pagination,
 * since this workspace is one panel of the DashboardPage-shaped /admin/prompt-audit route,
 * not a standalone full-viewport ListPage route (so no TablePageLayout shell either). */
.pa-events-card {
 overflow: hidden;
}

.pa-events-pagination {
 border-top: 1px solid var(--border);
}

.pa-checkbox {
 width: 16px;
 height: 16px;
 border-radius: 5px;
 border: 1.5px solid var(--border);
 accent-color: var(--accent);
 cursor: pointer;
}

/* Same converged two-line cell pattern as TicketsView/RiskControlView (11G): page-scoped,
 * layout-only tokens are not yet available for font-size/font-weight (see deviations.md). */
.cell-stack {
 display: flex;
 flex-direction: column;
 gap: 2px;
 min-width: 0;
}

.cell-title {
 font-size: 13px;
 font-weight: 500;
 line-height: 1.25;
 color: var(--foreground);
 white-space: nowrap;
 overflow: hidden;
 text-overflow: ellipsis;
}

.cell-meta {
 font-size: 11.5px;
 line-height: 1.3;
 color: var(--muted);
 white-space: nowrap;
 overflow: hidden;
 text-overflow: ellipsis;
}

.cell-time {
 font-size: 12.5px;
 color: var(--muted);
 font-variant-numeric: tabular-nums;
 white-space: nowrap;
}
</style>
