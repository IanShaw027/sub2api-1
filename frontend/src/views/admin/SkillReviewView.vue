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
              :placeholder="t('skills.admin.review.searchPlaceholder', '搜技能名、版本、作者')"
            />

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.review.status', '审核状态') }}</label>
              <Select :model-value="filters.review_status" :options="reviewStatusOptions" @update:model-value="updateReviewStatusFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.review.riskLevel', '风险等级') }}</label>
              <Select :model-value="filters.risk_level" :options="riskOptions" @update:model-value="updateRiskFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.review.visibility', '可见性') }}</label>
              <Select :model-value="filters.visibility" :options="visibilityOptions" @update:model-value="updateVisibilityFilter" />
            </div>
          </div>
        </template>

        <template #actions>
          <div class="flex justify-end gap-3">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
            <button class="btn btn-secondary" :disabled="loading" @click="loadReviews">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="reviews" :loading="loading">
            <template #cell-skill_name="{ row }">
              <div class="min-w-[260px]">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="text-left font-medium text-gray-900 hover:text-primary-600 dark:text-white dark:hover:text-primary-300"
                    @click="selectReview(row)"
                  >
                    {{ row.skill_name }}
                  </button>
                  <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
                    {{ row.version_name }}
                  </span>
                </div>
                <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
                  {{ row.skill_slug }}
                  <span v-if="row.latest_published_version"> · 上个公开版本 {{ row.latest_published_version }}</span>
                </div>
              </div>
            </template>

            <template #cell-author_name="{ value }">
              <span class="text-sm text-gray-700 dark:text-gray-300">{{ value || '-' }}</span>
            </template>

            <template #cell-review_status="{ value }">
              <SkillAdminStatusBadge :status="value" :label="reviewStatusLabel(value)" mode="review" />
            </template>

            <template #cell-visibility="{ value }">
              <SkillAdminStatusBadge :status="value" :label="visibilityLabel(value)" mode="visibility" />
            </template>

            <template #cell-risk_level="{ value }">
              <span :class="riskBadgeClass(value)" class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold">
                {{ riskLabel(value) }}
              </span>
            </template>

            <template #cell-submitted_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatTime(value) }}</span>
            </template>

            <template #cell-reviewed_at="{ value }">
              <span class="text-sm text-gray-500 dark:text-gray-400">{{ formatTime(value) }}</span>
            </template>

            <template #cell-actions="{ row }">
              <div class="flex items-center gap-2">
                <button class="btn btn-secondary btn-sm" @click="selectReview(row)">查看</button>
                <button
                  class="btn btn-primary btn-sm"
                  :disabled="actionLoading && actionTarget?.id === row.id"
                  @click="openAction('approve', row)"
                >
                  通过
                </button>
                <button
                  class="btn btn-danger btn-sm"
                  :disabled="actionLoading && actionTarget?.id === row.id"
                  @click="openAction('reject', row)"
                >
                  拒绝
                </button>
              </div>
            </template>

            <template #empty>
              <EmptyState
                :title="t('skills.admin.review.emptyTitle', '暂无待审技能版本')"
                :description="t('skills.admin.review.emptyDesc', '技能版本提交后，会在这里进入审核队列。')"
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
        <SkillReviewDetailCard
          :item="selectedReview"
          @approve="selectedReview && openAction('approve', selectedReview)"
          @reject="selectedReview && openAction('reject', selectedReview)"
        />

        <SkillAdminTimelineCard
          title="审核流转"
          description="展示当前技能版本从提交、排队到审核结论的主要节点。"
          empty-text="还没有选中技能版本。"
          :steps="reviewTimelineSteps"
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
  type SkillReviewItem,
  type SkillReviewStatus,
  type SkillRiskLevel,
  type SkillVisibility,
  type SkillReviewSummary
} from '@/api/admin/skills'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import SkillActionDialog from '@/components/skills/admin/SkillActionDialog.vue'
import SkillAdminMetricGrid from '@/components/skills/admin/SkillAdminMetricGrid.vue'
import SkillAdminStatusBadge from '@/components/skills/admin/SkillAdminStatusBadge.vue'
import SkillAdminTimelineCard from '@/components/skills/admin/SkillAdminTimelineCard.vue'
import SkillReviewDetailCard from '@/components/skills/admin/SkillReviewDetailCard.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const actionLoading = ref(false)
const reviews = ref<SkillReviewItem[]>([])
const selectedReview = ref<SkillReviewItem | null>(null)
const actionTarget = ref<SkillReviewItem | null>(null)
const dialogAction = ref<SkillAdminAction | null>(null)

const pagination = reactive<BasePaginationResponse<SkillReviewItem>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const summary = reactive<SkillReviewSummary>({
  pending_count: 0,
  approved_count: 0,
  rejected_count: 0,
  changes_requested_count: 0,
  high_risk_count: 0
})

const filters = reactive({
  search: '',
  review_status: 'all' as SkillReviewStatus | 'all',
  risk_level: 'all' as SkillRiskLevel | 'all',
  visibility: 'all' as SkillVisibility | 'all'
})

const reviewStatusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'pending', label: '待审核' },
  { value: 'approved', label: '已通过' },
  { value: 'rejected', label: '已拒绝' },
  { value: 'changes_requested', label: '待修改' }
]

const riskOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'low', label: '低风险' },
  { value: 'medium', label: '中风险' },
  { value: 'high', label: '高风险' }
]

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: '公开' },
  { value: 'private', label: '私有' },
  { value: 'force_private', label: '强制私有' }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: '技能 / 版本' },
  { key: 'author_name', label: '提交者' },
  { key: 'review_status', label: '审核状态' },
  { key: 'visibility', label: '可见性' },
  { key: 'risk_level', label: '风险等级' },
  { key: 'submitted_at', label: '提交时间' },
  { key: 'reviewed_at', label: '处理时间' },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'pending',
    label: '待审核版本',
    value: summary.pending_count,
    hint: '当前审核队列中等待管理员处理的版本数',
    icon: 'clipboard',
    tone: 'warning' as const
  },
  {
    key: 'approved',
    label: '已通过版本',
    value: summary.approved_count,
    hint: '已进入后续发布或公开流程的版本',
    icon: 'checkCircle',
    tone: 'success' as const
  },
  {
    key: 'rejected',
    label: '已拒绝版本',
    value: summary.rejected_count,
    hint: '被驳回并需要提交方整改的版本',
    icon: 'xCircle',
    tone: 'danger' as const
  },
  {
    key: 'highRisk',
    label: '高风险提交',
    value: summary.high_risk_count,
    hint: '建议优先人工复核的版本',
    icon: 'exclamationTriangle',
    tone: 'slate' as const
  }
])

const dialogSubject = computed(() => {
  if (!actionTarget.value) return ''
  return `${actionTarget.value.skill_name} · ${actionTarget.value.version_name}`
})

const reviewTimelineSteps = computed(() => {
  if (!selectedReview.value) return []

  const item = selectedReview.value
  const isRejected = item.review_status === 'rejected'
  const hasDecision = item.review_status !== 'pending'

  return [
    {
      key: 'submitted',
      title: '版本提交',
      description: '提交方发起新的技能版本审核。',
      time: formatTime(item.submitted_at),
      status: 'done' as const
    },
    {
      key: 'queued',
      title: '进入审核队列',
      description: item.risk_level === 'high' ? '高风险版本，建议优先处理。' : '等待审核员查看版本内容与治理规则。',
      time: formatTime(item.updated_at),
      status: hasDecision ? 'done' as const : 'current' as const
    },
    {
      key: 'decision',
      title: isRejected ? '审核拒绝' : hasDecision ? '审核通过' : '等待审核结论',
      description: isRejected
        ? item.rejection_reason || '管理员已拒绝该版本。'
        : hasDecision
          ? item.review_note || '管理员已通过该版本审核。'
          : '管理员尚未提交最终审核动作。',
      time: item.reviewed_at ? formatTime(item.reviewed_at) : null,
      status: isRejected ? 'danger' as const : hasDecision ? 'done' as const : 'todo' as const
    },
    {
      key: 'publish',
      title: '后续发布 / 上线',
      description: item.review_status === 'approved'
        ? '版本可进入主线程后续的发布、公开或灰度流程。'
        : '待审核通过后，才能进入对外发布链路。',
      time: item.review_status === 'approved' ? formatTime(item.reviewed_at || item.updated_at) : null,
      status: item.review_status === 'approved' ? 'done' as const : item.review_status === 'rejected' ? 'todo' as const : 'todo' as const
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

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: '公开',
    private: '私有',
    force_private: '强制私有'
  }[status]
}

function riskLabel(level: SkillRiskLevel): string {
  return {
    low: '低风险',
    medium: '中风险',
    high: '高风险'
  }[level]
}

function riskBadgeClass(level: SkillRiskLevel): string {
  return {
    low: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
    medium: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
    high: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  }[level]
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function selectReview(row: SkillReviewItem) {
  selectedReview.value = row
}

function updateReviewStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.review_status = next === 'approved' || next === 'rejected' || next === 'changes_requested' || next === 'pending'
    ? next
    : 'all'
}

function updateRiskFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.risk_level = next === 'low' || next === 'medium' || next === 'high' ? next : 'all'
}

function updateVisibilityFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.visibility = next === 'public' || next === 'private' || next === 'force_private' ? next : 'all'
}

async function loadReviews() {
  loading.value = true
  try {
    const response = await adminSkillsAPI.listReviews(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      review_status: filters.review_status,
      risk_level: filters.risk_level,
      visibility: filters.visibility
    })
    reviews.value = response.items
    Object.assign(pagination, response)
    Object.assign(summary, response.summary)

    if (selectedReview.value) {
      selectedReview.value = response.items.find((item) => item.id === selectedReview.value?.id) ?? response.items[0] ?? null
    } else {
      selectedReview.value = response.items[0] ?? null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '加载失败')))
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.search = ''
  filters.review_status = 'all'
  filters.risk_level = 'all'
  filters.visibility = 'all'
  pagination.page = 1
  void loadReviews()
}

function openAction(action: SkillAdminAction, row: SkillReviewItem) {
  dialogAction.value = action
  actionTarget.value = row
  selectedReview.value = row
}

function closeDialog() {
  dialogAction.value = null
  actionTarget.value = null
}

async function handleActionSubmit(payload: { note: string }) {
  if (!dialogAction.value || !actionTarget.value) return

  actionLoading.value = true
  try {
    if (dialogAction.value === 'approve') {
      const receipt = await adminSkillsAPI.approveReview(actionTarget.value.id, {
        note: payload.note,
        reason: payload.note || undefined
      })
      appStore.showSuccess(receipt.message || '技能版本已通过审核')
    } else if (dialogAction.value === 'reject') {
      const receipt = await adminSkillsAPI.rejectReview(actionTarget.value.id, {
        note: payload.note,
        reason: payload.note
      })
      appStore.showSuccess(receipt.message || '技能版本已拒绝')
    }
    closeDialog()
    await loadReviews()
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '操作失败')))
  } finally {
    actionLoading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadReviews()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadReviews()
}

onMounted(async () => {
  await loadReviews()
})
</script>
