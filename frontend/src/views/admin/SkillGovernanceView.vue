<template>
  <AppLayout>
    <div class="space-y-6">
      <SkillAdminMetricGrid :items="metricCards" />

      <TablePageLayout>
        <template #filters>
          <div class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <Input
              v-model="filters.search"
              :label="t('common.search', '搜索')"
              :placeholder="t('skills.admin.governance.searchPlaceholder', '搜技能名、作者、slug')"
            />

            <div>
              <label class="input-label mb-1.5 block">治理状态</label>
              <Select :model-value="filters.governance_status" :options="governanceStatusOptions" @update:model-value="updateGovernanceStatusFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">最近审核</label>
              <Select :model-value="filters.review_status" :options="reviewStatusOptions" @update:model-value="updateReviewStatusFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">可见性</label>
              <Select :model-value="filters.visibility" :options="visibilityOptions" @update:model-value="updateVisibilityFilter" />
            </div>
          </div>
        </template>

        <template #actions>
          <div class="flex justify-end gap-3">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
            <button class="btn btn-secondary" :disabled="loading" @click="loadGovernance">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="skills" :loading="loading">
            <template #cell-skill_name="{ row }">
              <div class="min-w-[260px]">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="text-left font-medium text-gray-900 hover:text-primary-600 dark:text-white dark:hover:text-primary-300"
                    @click="selectSkill(row)"
                  >
                    {{ row.skill_name }}
                  </button>
                  <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ row.current_version }}
                  </span>
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ row.skill_slug }}
                  <span v-if="row.category"> · {{ row.category }}</span>
                </div>
              </div>
            </template>

            <template #cell-author_name="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || '-' }}</span>
            </template>

            <template #cell-latest_review_status="{ value }">
              <SkillAdminStatusBadge :status="value" :label="reviewStatusLabel(value)" mode="review" />
            </template>

            <template #cell-governance_status="{ value }">
              <SkillAdminStatusBadge :status="value" :label="governanceStatusLabel(value)" mode="governance" />
            </template>

            <template #cell-visibility="{ value }">
              <SkillAdminStatusBadge :status="value" :label="visibilityLabel(value)" mode="visibility" />
            </template>

            <template #cell-requests_24h="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ Number(value).toLocaleString() }}</span>
            </template>

            <template #cell-success_rate="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatPercent(Number(value)) }}</span>
            </template>

            <template #cell-revenue_30d="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ formatCurrency(Number(value)) }}</span>
            </template>

            <template #cell-updated_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button class="btn btn-secondary btn-sm" @click="selectSkill(row)">查看</button>
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="row.governance_status === 'force_private' || (actionLoading && actionTarget?.id === row.id)"
                  @click="openAction('force-private', row)"
                >
                  强制私有
                </button>
                <button
                  class="btn btn-danger btn-sm"
                  :disabled="row.governance_status === 'disabled' || (actionLoading && actionTarget?.id === row.id)"
                  @click="openAction('disable', row)"
                >
                  下线
                </button>
              </div>
            </template>

            <template #empty>
              <EmptyState
                title="暂无技能治理记录"
                description="技能上线后，会在这里展示治理状态、可见性和处置动作。"
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

      <div class="grid gap-6 xl:grid-cols-[1.3fr_0.9fr]">
        <div class="card border border-gray-200 p-5 dark:border-dark-700">
          <template v-if="selectedSkill">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ selectedSkill.skill_name }}</h3>
                  <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ selectedSkill.current_version }}
                  </span>
                </div>
                <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
                  {{ selectedSkill.skill_slug }}
                  <span v-if="selectedSkill.author_name"> · 开发者 {{ selectedSkill.author_name }}</span>
                </p>
              </div>
              <div class="flex flex-wrap items-center gap-2">
                <SkillAdminStatusBadge
                  :status="selectedSkill.governance_status"
                  :label="governanceStatusLabel(selectedSkill.governance_status)"
                  mode="governance"
                />
                <SkillAdminStatusBadge
                  :status="selectedSkill.visibility"
                  :label="visibilityLabel(selectedSkill.visibility)"
                  mode="visibility"
                />
              </div>
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-3">
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">最近审核</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ reviewStatusLabel(selectedSkill.latest_review_status) }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">24h 调用</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ selectedSkill.requests_24h.toLocaleString() }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">30d 收入</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(selectedSkill.revenue_30d) }}</p>
              </div>
            </div>

            <div class="mt-5 space-y-4">
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">治理备注</p>
                <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">
                  {{ selectedSkill.review_note || '当前技能尚未记录额外治理备注。' }}
                </p>
              </div>

              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">标签</p>
                <div v-if="selectedSkill.tags.length" class="mt-2 flex flex-wrap gap-2">
                  <span
                    v-for="tag in selectedSkill.tags"
                    :key="tag"
                    class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                  >
                    {{ tag }}
                  </span>
                </div>
                <p v-else class="mt-2 text-sm text-gray-500 dark:text-gray-400">当前技能未配置标签。</p>
              </div>

              <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
                <div class="grid gap-4 md:grid-cols-2">
                  <div>
                    <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">公开版本</p>
                    <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ selectedSkill.latest_published_version || '-' }}</p>
                  </div>
                  <div>
                    <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">成功率</p>
                    <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ formatPercent(selectedSkill.success_rate) }}</p>
                  </div>
                </div>
              </div>
            </div>

            <div class="mt-6 flex flex-wrap justify-end gap-3">
              <button
                type="button"
                class="btn btn-secondary btn-sm"
                :disabled="selectedSkill.governance_status === 'force_private'"
                @click="openAction('force-private', selectedSkill)"
              >
                强制私有
              </button>
              <button
                type="button"
                class="btn btn-danger btn-sm"
                :disabled="selectedSkill.governance_status === 'disabled'"
                @click="openAction('disable', selectedSkill)"
              >
                下线技能
              </button>
            </div>
          </template>

          <template v-else>
            <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
              选择一条技能记录后，这里会展示治理概况、备注与处置入口。
            </div>
          </template>
        </div>

        <SkillAdminTimelineCard
          title="治理流转"
          description="从审核通过到强制私有/下线的关键节点会在这里集中展示。"
          empty-text="还没有选中技能。"
          :steps="governanceTimelineSteps"
        />
      </div>
    </div>

    <SkillActionDialog
      :show="dialogAction !== null"
      :action="dialogAction"
      :subject="dialogSubject"
      :loading="actionLoading"
      @close="closeDialog"
      @submit="handleActionSubmit"
    />
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
  type SkillAdminAction,
  type SkillGovernanceItem,
  type SkillGovernanceStatus,
  type SkillReviewStatus,
  type SkillVisibility,
  type SkillGovernanceSummary
} from '@/api/admin/skills'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import SkillActionDialog from '@/components/skills/admin/SkillActionDialog.vue'
import SkillAdminMetricGrid from '@/components/skills/admin/SkillAdminMetricGrid.vue'
import SkillAdminStatusBadge from '@/components/skills/admin/SkillAdminStatusBadge.vue'
import SkillAdminTimelineCard from '@/components/skills/admin/SkillAdminTimelineCard.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const skills = ref<SkillGovernanceItem[]>([])
const selectedSkill = ref<SkillGovernanceItem | null>(null)
const actionTarget = ref<SkillGovernanceItem | null>(null)
const dialogAction = ref<SkillAdminAction | null>(null)

const pagination = reactive<BasePaginationResponse<SkillGovernanceItem>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const summary = reactive<SkillGovernanceSummary>({
  total_count: 0,
  online_count: 0,
  force_private_count: 0,
  disabled_count: 0,
  pending_versions_count: 0
})

const filters = reactive({
  search: '',
  governance_status: 'all' as SkillGovernanceStatus | 'all',
  review_status: 'all' as SkillReviewStatus | 'all',
  visibility: 'all' as SkillVisibility | 'all'
})

const governanceStatusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'online', label: '在线' },
  { value: 'force_private', label: '强制私有' },
  { value: 'disabled', label: '已下线' },
  { value: 'draft', label: '草稿 / 审核中' }
]

const reviewStatusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'pending', label: '待审核' },
  { value: 'approved', label: '已通过' },
  { value: 'rejected', label: '已拒绝' },
  { value: 'changes_requested', label: '待修改' }
]

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: '公开' },
  { value: 'private', label: '私有' },
  { value: 'force_private', label: '强制私有' }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: '技能 / 版本' },
  { key: 'author_name', label: '开发者' },
  { key: 'latest_review_status', label: '最近审核' },
  { key: 'governance_status', label: '治理状态' },
  { key: 'visibility', label: '可见性' },
  { key: 'requests_24h', label: '24h 调用' },
  { key: 'success_rate', label: '成功率' },
  { key: 'revenue_30d', label: '30d 收入' },
  { key: 'updated_at', label: '最近更新' },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'online',
    label: '在线技能',
    value: summary.online_count,
    hint: '当前仍可正常对外提供能力的技能数',
    icon: 'sparkles',
    tone: 'success' as const
  },
  {
    key: 'forcePrivate',
    label: '强制私有',
    value: summary.force_private_count,
    hint: '已退出公开市场但仍保留私域使用的技能',
    icon: 'lock',
    tone: 'warning' as const
  },
  {
    key: 'disabled',
    label: '已下线技能',
    value: summary.disabled_count,
    hint: '被治理动作完全下线的技能',
    icon: 'ban',
    tone: 'danger' as const
  },
  {
    key: 'pendingVersions',
    label: '待处理版本',
    value: summary.pending_versions_count,
    hint: '这些技能仍有版本处在审核队列中',
    icon: 'clock',
    tone: 'slate' as const
  }
])

const dialogSubject = computed(() => {
  if (!actionTarget.value) return ''
  return `${actionTarget.value.skill_name} · ${actionTarget.value.current_version}`
})

const governanceTimelineSteps = computed(() => {
  if (!selectedSkill.value) return []

  const item = selectedSkill.value
  const isDisabled = item.governance_status === 'disabled'
  const isForcePrivate = item.governance_status === 'force_private'

  return [
    {
      key: 'created',
      title: '技能创建',
      description: '技能主体已建立并进入版本治理流程。',
      time: formatTime(item.created_at),
      status: 'done' as const
    },
    {
      key: 'review',
      title: `最近审核：${reviewStatusLabel(item.latest_review_status)}`,
      description: item.latest_review_status === 'pending'
        ? '存在尚未处理的版本，需要管理员继续审核。'
        : '最近一个版本审核结论已同步到治理面板。',
      time: formatTime(item.updated_at),
      status: item.latest_review_status === 'pending' ? 'current' as const : 'done' as const
    },
    {
      key: 'visibility',
      title: `当前可见性：${visibilityLabel(item.visibility)}`,
      description: item.visibility === 'force_private'
        ? '技能已被强制退出公开市场。'
        : item.visibility === 'public'
          ? '技能仍可公开对外展示。'
          : '技能当前仅私有可见。',
      time: formatTime(item.updated_at),
      status: isForcePrivate ? 'danger' as const : 'done' as const
    },
    {
      key: 'governance',
      title: `治理状态：${governanceStatusLabel(item.governance_status)}`,
      description: isDisabled
        ? '技能已被下线，外部调用应同步中止。'
        : isForcePrivate
          ? '技能继续保留，但对外展示已收敛到私域。'
          : '当前无进一步治理动作。',
      time: formatTime(item.updated_at),
      status: isDisabled || isForcePrivate ? 'danger' as const : 'todo' as const
    }
  ]
})

function reviewStatusLabel(status: SkillReviewStatus): string {
  return {
    pending: '待审核',
    approved: '已通过',
    rejected: '已拒绝',
    changes_requested: '待修改'
  }[status]
}

function governanceStatusLabel(status: SkillGovernanceStatus): string {
  return {
    online: '在线',
    disabled: '已下线',
    force_private: '强制私有',
    draft: '草稿 / 审核中'
  }[status]
}

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: '公开',
    private: '私有',
    force_private: '强制私有'
  }[status]
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2
  }).format(value)
}

function selectSkill(row: SkillGovernanceItem) {
  selectedSkill.value = row
}

function updateGovernanceStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.governance_status = next === 'online' || next === 'force_private' || next === 'disabled' || next === 'draft'
    ? next
    : 'all'
}

function updateReviewStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.review_status = next === 'approved' || next === 'rejected' || next === 'changes_requested' || next === 'pending'
    ? next
    : 'all'
}

function updateVisibilityFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.visibility = next === 'public' || next === 'private' || next === 'force_private' ? next : 'all'
}

async function loadGovernance() {
  loading.value = true
  try {
    const response = await adminSkillsAPI.listGovernanceSkills(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      governance_status: filters.governance_status,
      review_status: filters.review_status,
      visibility: filters.visibility
    })
    skills.value = response.items
    Object.assign(pagination, response)
    Object.assign(summary, response.summary)

    if (selectedSkill.value) {
      selectedSkill.value = response.items.find((item) => item.id === selectedSkill.value?.id) ?? response.items[0] ?? null
    } else {
      selectedSkill.value = response.items[0] ?? null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '加载失败')))
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.search = ''
  filters.governance_status = 'all'
  filters.review_status = 'all'
  filters.visibility = 'all'
  pagination.page = 1
  void loadGovernance()
}

function openAction(action: SkillAdminAction, row: SkillGovernanceItem) {
  dialogAction.value = action
  actionTarget.value = row
  selectedSkill.value = row
}

function closeDialog() {
  dialogAction.value = null
  actionTarget.value = null
}

async function handleActionSubmit(payload: { note: string }) {
  if (!dialogAction.value || !actionTarget.value) return

  actionLoading.value = true
  try {
    if (dialogAction.value === 'disable') {
      const receipt = await adminSkillsAPI.disableSkill(actionTarget.value.skill_id, {
        note: payload.note,
        reason: payload.note
      })
      appStore.showSuccess(receipt.message || '技能已下线')
    } else if (dialogAction.value === 'force-private') {
      const receipt = await adminSkillsAPI.forceSkillPrivate(actionTarget.value.skill_id, {
        note: payload.note,
        reason: payload.note
      })
      appStore.showSuccess(receipt.message || '技能已转为强制私有')
    }
    closeDialog()
    await loadGovernance()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '操作失败')))
  } finally {
    actionLoading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadGovernance()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadGovernance()
}

onMounted(async () => {
  await loadGovernance()
})
</script>
