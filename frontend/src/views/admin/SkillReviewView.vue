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
                  <span v-if="row.latest_published_version"> · {{ t('skills.admin.review.previousPublishedVersion') }} {{ row.latest_published_version }}</span>
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
                <button class="btn btn-secondary btn-sm" @click="selectReview(row)">{{ t('common.view') }}</button>
                <button
                  class="btn btn-primary btn-sm"
                  :disabled="actionLoading && actionTarget?.id === row.id"
                  @click="openAction('approve', row)"
                >
                  {{ t('skills.admin.review.approveShort') }}
                </button>
                <button
                  class="btn btn-danger btn-sm"
                  :disabled="actionLoading && actionTarget?.id === row.id"
                  @click="openAction('reject', row)"
                >
                  {{ t('skills.admin.review.rejectShort') }}
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
          :title="t('skills.admin.review.timelineTitle')"
          :description="t('skills.admin.review.timelineDescription')"
          :empty-text="t('skills.admin.review.timelineEmpty')"
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
  { value: 'pending', label: t('skills.admin.review.labels.pending') },
  { value: 'approved', label: t('skills.admin.review.labels.approved') },
  { value: 'rejected', label: t('skills.admin.review.labels.rejected') }
]

const riskOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'low', label: t('skills.admin.review.labels.low') },
  { value: 'medium', label: t('skills.admin.review.labels.medium') },
  { value: 'high', label: t('skills.admin.review.labels.high') }
]

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: t('skills.admin.review.labels.public') },
  { value: 'private', label: t('skills.admin.review.labels.private') },
  { value: 'force_private', label: t('skills.admin.review.labels.forcePrivate') }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: t('skills.admin.review.columns.skillVersion') },
  { key: 'author_name', label: t('skills.admin.review.columns.author') },
  { key: 'review_status', label: t('skills.admin.review.columns.status') },
  { key: 'visibility', label: t('skills.admin.review.columns.visibility') },
  { key: 'risk_level', label: t('skills.admin.review.columns.riskLevel') },
  { key: 'submitted_at', label: t('skills.admin.review.columns.submittedAt') },
  { key: 'reviewed_at', label: t('skills.admin.review.columns.reviewedAt') },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'pending',
    label: t('skills.admin.review.metrics.pendingVersions'),
    value: summary.pending_count,
    hint: t('skills.admin.review.metrics.pendingHint'),
    icon: 'clipboard',
    tone: 'warning' as const
  },
  {
    key: 'approved',
    label: t('skills.admin.review.metrics.approvedVersions'),
    value: summary.approved_count,
    hint: t('skills.admin.review.metrics.approvedHint'),
    icon: 'checkCircle',
    tone: 'success' as const
  },
  {
    key: 'rejected',
    label: t('skills.admin.review.metrics.rejectedVersions'),
    value: summary.rejected_count,
    hint: t('skills.admin.review.metrics.rejectedHint'),
    icon: 'xCircle',
    tone: 'danger' as const
  },
  {
    key: 'highRisk',
    label: t('skills.admin.review.metrics.highRiskSubmissions'),
    value: summary.high_risk_count,
    hint: t('skills.admin.review.metrics.highRiskHint'),
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
      title: t('skills.admin.review.timeline.submittedTitle'),
      description: t('skills.admin.review.timeline.submittedDesc'),
      time: formatTime(item.submitted_at),
      status: 'done' as const
    },
    {
      key: 'queued',
      title: t('skills.admin.review.timeline.queuedTitle'),
      description: item.risk_level === 'high'
        ? t('skills.admin.review.timeline.queuedHighRiskDesc')
        : t('skills.admin.review.timeline.queuedDesc'),
      time: formatTime(item.updated_at),
      status: hasDecision ? 'done' as const : 'current' as const
    },
    {
      key: 'decision',
      title: isRejected
        ? t('skills.admin.review.timeline.decisionRejectedTitle')
        : hasDecision
          ? t('skills.admin.review.timeline.decisionApprovedTitle')
          : t('skills.admin.review.timeline.decisionPendingTitle'),
      description: isRejected
        ? item.rejection_reason || t('skills.admin.review.timeline.decisionRejectedDesc')
        : hasDecision
          ? item.review_note || t('skills.admin.review.timeline.decisionApprovedDesc')
          : t('skills.admin.review.timeline.decisionPendingDesc'),
      time: item.reviewed_at ? formatTime(item.reviewed_at) : null,
      status: isRejected ? 'danger' as const : hasDecision ? 'done' as const : 'todo' as const
    },
    {
      key: 'publish',
      title: t('skills.admin.review.timeline.publishTitle'),
      description: item.review_status === 'approved'
        ? t('skills.admin.review.timeline.publishApprovedDesc')
        : t('skills.admin.review.timeline.publishPendingDesc'),
      time: item.review_status === 'approved' ? formatTime(item.reviewed_at || item.updated_at) : null,
      status: item.review_status === 'approved' ? 'done' as const : item.review_status === 'rejected' ? 'todo' as const : 'todo' as const
    }
  ]
})

function reviewStatusLabel(status: SkillReviewStatus): string {
  return {
    pending: t('skills.admin.review.labels.pending'),
    approved: t('skills.admin.review.labels.approved'),
    rejected: t('skills.admin.review.labels.rejected')
  }[status]
}

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: t('skills.admin.review.labels.public'),
    private: t('skills.admin.review.labels.private'),
    force_private: t('skills.admin.review.labels.forcePrivate')
  }[status]
}

function riskLabel(level: SkillRiskLevel): string {
  return {
    low: t('skills.admin.review.labels.low'),
    medium: t('skills.admin.review.labels.medium'),
    high: t('skills.admin.review.labels.high')
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
  filters.review_status = next === 'approved' || next === 'rejected' || next === 'pending'
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
      appStore.showSuccess(receipt.message || t('skills.admin.review.toastApproved'))
    } else if (dialogAction.value === 'reject') {
      const receipt = await adminSkillsAPI.rejectReview(actionTarget.value.id, {
        note: payload.note,
        reason: payload.note
      })
      appStore.showSuccess(receipt.message || t('skills.admin.review.toastRejected'))
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
