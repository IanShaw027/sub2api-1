<template>
  <AppLayout>
    <div class="space-y-6">
      <SkillAdminMetricGrid :items="metricCards" />

      <TablePageLayout>
        <template #filters>
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-2">
            <Input
              v-model="filters.search"
              :label="t('common.search', '搜索')"
              :placeholder="t('skills.admin.settlement.searchPlaceholder', '搜技能名、作者、结算周期')"
            />

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.settlement.status') }}</label>
              <Select :model-value="filters.settlement_status" :options="settlementStatusOptions" @update:model-value="updateSettlementStatusFilter" />
            </div>

          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('skills.admin.settlement.filterHint') }}
          </p>
        </template>

        <template #actions>
          <div class="flex justify-end gap-3">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
            <button class="btn btn-secondary" :disabled="loading" @click="loadSettlements">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="settlements" :loading="loading">
            <template #cell-skill_name="{ row }">
              <div class="min-w-[240px]">
                <div class="font-medium text-gray-900 dark:text-white">{{ row.skill_name }}</div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ row.skill_slug }}</div>
              </div>
            </template>

            <template #cell-author_name="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || '-' }}</span>
            </template>

            <template #cell-gross_amount="{ value, row }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatCurrency(Number(value), row.currency) }}</span>
            </template>

            <template #cell-platform_fee_amount="{ value, row }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatCurrency(Number(value), row.currency) }}</span>
            </template>

            <template #cell-payout_amount="{ value, row }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatCurrency(Number(value), row.currency) }}</span>
            </template>

            <template #cell-frozen_amount="{ value, row }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatCurrency(Number(value), row.currency) }}</span>
            </template>

            <template #cell-settlement_status="{ value }">
              <SkillAdminStatusBadge :status="value" :label="settlementStatusLabel(value)" mode="settlement" />
            </template>

            <template #cell-updated_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <button class="btn btn-secondary btn-sm" @click="selectSettlement(row)">{{ t('common.view') }}</button>
            </template>

            <template #empty>
              <EmptyState
                :title="t('skills.admin.settlement.emptyTitle')"
                :description="t('skills.admin.settlement.emptyDesc')"
              />
            </template>
          </DataTable>
        </template>

        <template #pagination>
          <Pagination
            v-if="pagination.total > pagination.page_size"
            :page="pagination.page"
            :total="pagination.total"
            :page-size="pagination.page_size"
            @update:page="handlePageChange"
            @update:pageSize="handlePageSizeChange"
          />
        </template>
      </TablePageLayout>

      <div class="grid gap-6 xl:grid-cols-[1.25fr_0.95fr]">
        <div class="card border border-gray-200 p-5 dark:border-dark-700">
          <template v-if="selectedSettlement">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ selectedSettlement.skill_name }}</h3>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  {{ selectedSettlement.skill_slug }}
                  <span v-if="selectedSettlement.author_name"> · {{ t('skills.admin.settlement.authorLabel') }} {{ selectedSettlement.author_name }}</span>
                </p>
              </div>
              <SkillAdminStatusBadge
                :status="selectedSettlement.settlement_status"
                :label="settlementStatusLabel(selectedSettlement.settlement_status)"
                mode="settlement"
              />
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.periodTitle') }}</p>
                <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ selectedSettlement.period_label }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.payoutTitle') }}</p>
                <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ formatCurrency(selectedSettlement.payout_amount, selectedSettlement.currency) }}
                </p>
              </div>
            </div>

            <div class="mt-5 rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="grid gap-4 md:grid-cols-2">
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.grossTitle') }}</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.gross_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.platformFeeTitle') }}</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.platform_fee_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.frozenTitle') }}</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.frozen_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.settlement.updatedTitle') }}</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ formatTime(selectedSettlement.updated_at) }}</p>
                </div>
              </div>
            </div>

            <div class="mt-5">
              <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.settlement.noteTitle') }}</p>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">
                {{ selectedSettlement.note || t('skills.admin.settlement.noteEmpty') }}
              </p>
            </div>

            <div v-if="selectedSettlement.settlement_status === 'rejected'" class="mt-6 flex justify-end">
              <button
                data-test="settlement-replay-button"
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="replayLoading"
                @click="handleReplaySettlement"
              >
                {{ replayLoading ? t('common.submitting', '提交中') : t('skills.admin.settlement.replayAction', '重试结算') }}
              </button>
            </div>
          </template>

          <template v-else>
            <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
              {{ t('skills.admin.settlement.detailEmpty') }}
            </div>
          </template>
        </div>

        <SkillAdminTimelineCard
          :title="t('skills.admin.settlement.timelineTitle')"
          :description="t('skills.admin.settlement.timelineDescription')"
          :empty-text="t('skills.admin.settlement.timelineEmpty')"
          :steps="settlementTimelineSteps"
        />
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Icon from '@/components/icons/Icon.vue'
import Input from '@/components/common/Input.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import type { Column } from '@/components/common/types'
import type { BasePaginationResponse } from '@/types'
import adminSkillsAPI, {
  type SkillSettlementItem,
  type SkillSettlementStatus,
  type SkillSettlementSummary
} from '@/api/admin/skills'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import SkillAdminMetricGrid from '@/components/skills/admin/SkillAdminMetricGrid.vue'
import SkillAdminStatusBadge from '@/components/skills/admin/SkillAdminStatusBadge.vue'
import SkillAdminTimelineCard from '@/components/skills/admin/SkillAdminTimelineCard.vue'

const { t, locale } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const replayLoading = ref(false)
const settlements = ref<SkillSettlementItem[]>([])
const selectedSettlement = ref<SkillSettlementItem | null>(null)

const pagination = reactive<BasePaginationResponse<SkillSettlementItem>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const summary = reactive<SkillSettlementSummary>({
  pending_amount: 0,
  settled_amount: 0,
  frozen_amount: 0,
  pending_skill_count: 0,
  currency: 'CNY'
})

const filters = reactive({
  search: '',
  settlement_status: 'all' as SkillSettlementStatus | 'all'
})

const settlementStatusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'pending', label: t('skills.admin.settlement.labels.pending') },
  { value: 'ready', label: t('skills.admin.settlement.labels.ready') },
  { value: 'settled', label: t('skills.admin.settlement.labels.settled') },
  { value: 'frozen', label: t('skills.admin.settlement.labels.frozen') },
  { value: 'rejected', label: t('skills.admin.settlement.labels.rejected') }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: t('skills.admin.settlement.columns.skill') },
  { key: 'author_name', label: t('skills.admin.settlement.columns.author') },
  { key: 'period_label', label: t('skills.admin.settlement.columns.period') },
  { key: 'gross_amount', label: t('skills.admin.settlement.columns.gross') },
  { key: 'platform_fee_amount', label: t('skills.admin.settlement.columns.platformFee') },
  { key: 'payout_amount', label: t('skills.admin.settlement.columns.payout') },
  { key: 'frozen_amount', label: t('skills.admin.settlement.columns.frozen') },
  { key: 'settlement_status', label: t('skills.admin.settlement.columns.status') },
  { key: 'updated_at', label: t('skills.admin.settlement.columns.updatedAt') },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'pending',
    label: t('skills.admin.settlement.metrics.pendingAmount'),
    value: formatCurrency(summary.pending_amount, summary.currency),
    hint: t('skills.admin.settlement.metrics.pendingHint', { count: summary.pending_skill_count }),
    icon: 'calculator',
    tone: 'warning' as const
  },
  {
    key: 'settled',
    label: t('skills.admin.settlement.metrics.settledAmount'),
    value: formatCurrency(summary.settled_amount, summary.currency),
    hint: t('skills.admin.settlement.metrics.settledHint'),
    icon: 'checkCircle',
    tone: 'success' as const
  },
  {
    key: 'frozen',
    label: t('skills.admin.settlement.metrics.frozenAmount'),
    value: formatCurrency(summary.frozen_amount, summary.currency),
    hint: t('skills.admin.settlement.metrics.frozenHint'),
    icon: 'lock',
    tone: 'slate' as const
  },
  {
    key: 'currency',
    label: t('skills.admin.settlement.metrics.currency'),
    value: summary.currency,
    hint: t('skills.admin.settlement.metrics.currencyHint'),
    icon: 'creditCard',
    tone: 'primary' as const
  }
])

const settlementTimelineSteps = computed(() => {
  if (!selectedSettlement.value) return []

  const item = selectedSettlement.value
  const status = item.settlement_status

  return [
    {
      key: 'accrual',
      title: t('skills.admin.settlement.timeline.accrualTitle'),
      description: t('skills.admin.settlement.timeline.accrualDesc', { period: item.period_label }),
      time: formatTime(item.created_at),
      status: 'done' as const
    },
    {
      key: 'audit',
      title: status === 'rejected'
        ? t('skills.admin.settlement.timeline.auditRejectedTitle')
        : status === 'frozen'
          ? t('skills.admin.settlement.timeline.auditFrozenTitle')
          : t('skills.admin.settlement.timeline.auditTitle'),
      description: status === 'rejected'
        ? item.note || t('skills.admin.settlement.timeline.auditRejectedDesc')
        : status === 'frozen'
          ? item.note || t('skills.admin.settlement.timeline.auditFrozenDesc')
          : t('skills.admin.settlement.timeline.auditDesc'),
      time: formatTime(item.updated_at),
      status: status === 'rejected' || status === 'frozen' ? 'danger' as const : status === 'pending' ? 'current' as const : 'done' as const
    },
    {
      key: 'ready',
      title: status === 'ready' || status === 'settled'
        ? t('skills.admin.settlement.timeline.readyTitle')
        : t('skills.admin.settlement.timeline.readyPendingTitle'),
      description: status === 'ready' || status === 'settled'
        ? t('skills.admin.settlement.timeline.readyDesc')
        : t('skills.admin.settlement.timeline.readyPendingDesc'),
      time: status === 'ready' || status === 'settled' ? formatTime(item.updated_at) : null,
      status: status === 'ready' ? 'current' as const : status === 'settled' ? 'done' as const : 'todo' as const
    },
    {
      key: 'settled',
      title: status === 'settled'
        ? t('skills.admin.settlement.timeline.settledTitle')
        : t('skills.admin.settlement.timeline.settledPendingTitle'),
      description: status === 'settled'
        ? t('skills.admin.settlement.timeline.settledDesc')
        : t('skills.admin.settlement.timeline.settledPendingDesc'),
      time: status === 'settled' ? formatTime(item.updated_at) : null,
      status: status === 'settled' ? 'done' as const : 'todo' as const
    }
  ]
})

function settlementStatusLabel(status: SkillSettlementStatus): string {
  return {
    pending: t('skills.admin.settlement.labels.pending'),
    ready: t('skills.admin.settlement.labels.ready'),
    settled: t('skills.admin.settlement.labels.settled'),
    frozen: t('skills.admin.settlement.labels.frozen'),
    rejected: t('skills.admin.settlement.labels.rejected')
  }[status]
}

function formatCurrency(value: number, currency = 'CNY'): string {
  return new Intl.NumberFormat(locale.value.startsWith('zh') ? 'zh-CN' : 'en-US', {
    style: 'currency',
    currency,
    minimumFractionDigits: 2
  }).format(value)
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function selectSettlement(row: SkillSettlementItem) {
  selectedSettlement.value = row
}

function updateSettlementStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.settlement_status = next === 'pending' || next === 'ready' || next === 'settled' || next === 'frozen' || next === 'rejected'
    ? next
    : 'all'
}

async function loadSettlements() {
  loading.value = true
  try {
    const response = await adminSkillsAPI.listSettlements(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      settlement_status: filters.settlement_status
    })
    settlements.value = response.items
    Object.assign(pagination, response)
    Object.assign(summary, response.summary)

    if (selectedSettlement.value) {
      selectedSettlement.value = response.items.find((item) => item.id === selectedSettlement.value?.id) ?? response.items[0] ?? null
    } else {
      selectedSettlement.value = response.items[0] ?? null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '加载失败')))
  } finally {
    loading.value = false
  }
}

async function handleReplaySettlement() {
  if (!selectedSettlement.value || selectedSettlement.value.settlement_status !== 'rejected') return

  replayLoading.value = true
  try {
    const receipt = await adminSkillsAPI.replaySettlement(selectedSettlement.value.id, {})
    appStore.showSuccess(receipt.message || t('skills.admin.settlement.replaySuccess', '已重新触发结算重试'))
    await loadSettlements()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('skills.admin.settlement.replayFailed', '重试结算失败')))
  } finally {
    replayLoading.value = false
  }
}

function resetFilters() {
  filters.search = ''
  filters.settlement_status = 'all'
  pagination.page = 1
  void loadSettlements()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadSettlements()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadSettlements()
}

onMounted(async () => {
  await loadSettlements()
})
</script>
