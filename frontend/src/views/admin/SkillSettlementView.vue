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
              <label class="input-label mb-1.5 block">结算状态</label>
              <Select :model-value="filters.settlement_status" :options="settlementStatusOptions" @update:model-value="updateSettlementStatusFilter" />
            </div>

          </div>
          <p class="text-xs text-gray-500 dark:text-gray-400">
            结算周期以后端返回的真实 `period_label` 为准，当前页面不再提供无后端支持的前端假筛选。
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
              <button class="btn btn-secondary btn-sm" @click="selectSettlement(row)">查看</button>
            </template>

            <template #empty>
              <EmptyState
                title="暂无技能结算记录"
                description="技能中心的收入拆分、冻结与结算状态会在这里汇总。"
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
                  <span v-if="selectedSettlement.author_name"> · 开发者 {{ selectedSettlement.author_name }}</span>
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
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">结算周期</p>
                <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">{{ selectedSettlement.period_label }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">应结金额</p>
                <p class="mt-2 text-lg font-semibold text-gray-900 dark:text-white">
                  {{ formatCurrency(selectedSettlement.payout_amount, selectedSettlement.currency) }}
                </p>
              </div>
            </div>

            <div class="mt-5 rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
              <div class="grid gap-4 md:grid-cols-2">
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">总流水</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.gross_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">平台抽成</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.platform_fee_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">冻结金额</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">
                    {{ formatCurrency(selectedSettlement.frozen_amount, selectedSettlement.currency) }}
                  </p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">最近更新</p>
                  <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ formatTime(selectedSettlement.updated_at) }}</p>
                </div>
              </div>
            </div>

            <div class="mt-5">
              <p class="text-sm font-medium text-gray-900 dark:text-white">结算备注</p>
              <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">
                {{ selectedSettlement.note || '当前结算记录还没有附带备注。' }}
              </p>
            </div>
          </template>

          <template v-else>
            <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
              选择一条结算记录后，这里会展示收入拆分、冻结金额和备注。
            </div>
          </template>
        </div>

        <SkillAdminTimelineCard
          title="结算流程"
          description="展示技能收入从汇总、审核到实际结算的状态流转。"
          empty-text="还没有选中结算记录。"
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

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
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
  { value: 'pending', label: '待结算' },
  { value: 'ready', label: '待打款' },
  { value: 'settled', label: '已结算' },
  { value: 'frozen', label: '冻结中' },
  { value: 'rejected', label: '已驳回' }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: '技能' },
  { key: 'author_name', label: '开发者' },
  { key: 'period_label', label: '结算周期' },
  { key: 'gross_amount', label: '总流水' },
  { key: 'platform_fee_amount', label: '平台抽成' },
  { key: 'payout_amount', label: '应结金额' },
  { key: 'frozen_amount', label: '冻结金额' },
  { key: 'settlement_status', label: '结算状态' },
  { key: 'updated_at', label: '最近更新' },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'pending',
    label: '待结算金额',
    value: formatCurrency(summary.pending_amount, summary.currency),
    hint: `待处理技能 ${summary.pending_skill_count}`,
    icon: 'calculator',
    tone: 'warning' as const
  },
  {
    key: 'settled',
    label: '已结算金额',
    value: formatCurrency(summary.settled_amount, summary.currency),
    hint: '已进入打款完成状态的金额汇总',
    icon: 'checkCircle',
    tone: 'success' as const
  },
  {
    key: 'frozen',
    label: '冻结金额',
    value: formatCurrency(summary.frozen_amount, summary.currency),
    hint: '存在争议或待复核的结算金额',
    icon: 'lock',
    tone: 'slate' as const
  },
  {
    key: 'currency',
    label: '结算币种',
    value: summary.currency,
    hint: '当前页面金额展示所使用的币种',
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
      title: '收入汇总',
      description: `周期 ${item.period_label} 的技能收入已完成归集。`,
      time: formatTime(item.created_at),
      status: 'done' as const
    },
    {
      key: 'audit',
      title: status === 'rejected' ? '结算驳回' : status === 'frozen' ? '风险冻结' : '结算审核',
      description: status === 'rejected'
        ? item.note || '管理员已驳回当前结算记录。'
        : status === 'frozen'
          ? item.note || '当前金额因风控或争议进入冻结状态。'
          : '平台对收入、抽成和冻结额进行复核。',
      time: formatTime(item.updated_at),
      status: status === 'rejected' || status === 'frozen' ? 'danger' as const : status === 'pending' ? 'current' as const : 'done' as const
    },
    {
      key: 'ready',
      title: status === 'ready' || status === 'settled' ? '待打款' : '等待出账',
      description: status === 'ready' || status === 'settled'
        ? '当前记录已通过审核，等待财务或系统执行结算。'
        : '审核通过后会进入待打款状态。',
      time: status === 'ready' || status === 'settled' ? formatTime(item.updated_at) : null,
      status: status === 'ready' ? 'current' as const : status === 'settled' ? 'done' as const : 'todo' as const
    },
    {
      key: 'settled',
      title: status === 'settled' ? '已结算' : '完成打款',
      description: status === 'settled'
        ? '结算已完成，金额应已同步到收款侧。'
        : '待记录进入 settled 后视为完成。',
      time: status === 'settled' ? formatTime(item.updated_at) : null,
      status: status === 'settled' ? 'done' as const : 'todo' as const
    }
  ]
})

function settlementStatusLabel(status: SkillSettlementStatus): string {
  return {
    pending: '待结算',
    ready: '待打款',
    settled: '已结算',
    frozen: '冻结中',
    rejected: '已驳回'
  }[status]
}

function formatCurrency(value: number, currency = 'CNY'): string {
  return new Intl.NumberFormat('zh-CN', {
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
