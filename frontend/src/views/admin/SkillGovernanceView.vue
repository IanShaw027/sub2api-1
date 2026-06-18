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
              <label class="input-label mb-1.5 block">{{ t('skills.admin.governance.status') }}</label>
              <Select :model-value="filters.governance_status" :options="governanceStatusOptions" @update:model-value="updateGovernanceStatusFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.governance.latestReview') }}</label>
              <Select :model-value="filters.review_status" :options="reviewStatusOptions" @update:model-value="updateReviewStatusFilter" />
            </div>

            <div>
              <label class="input-label mb-1.5 block">{{ t('skills.admin.governance.visibility') }}</label>
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
                <button class="btn btn-secondary btn-sm" @click="selectSkill(row)">{{ t('common.view') }}</button>
                <button
                  class="btn btn-secondary btn-sm"
                  :disabled="row.governance_status === 'force_private' || (actionLoading && actionTarget?.id === row.id)"
                  @click="openAction('force-private', row)"
                >
                  {{ t('skills.admin.governance.forcePrivateAction') }}
                </button>
                <button
                  class="btn btn-danger btn-sm"
                  :disabled="row.governance_status === 'disabled' || (actionLoading && actionTarget?.id === row.id)"
                  @click="openAction('disable', row)"
                >
                  {{ t('skills.admin.governance.disableShort') }}
                </button>
              </div>
            </template>

            <template #empty>
              <EmptyState
                :title="t('skills.admin.governance.emptyTitle')"
                :description="t('skills.admin.governance.emptyDesc')"
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
                  <span v-if="selectedSkill.author_name"> · {{ t('skills.admin.governance.authorLabel') }} {{ selectedSkill.author_name }}</span>
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
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.latestReview') }}</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ reviewStatusLabel(selectedSkill.latest_review_status) }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.metrics.requests24h') }}</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ selectedSkill.requests_24h.toLocaleString() }}</p>
              </div>
              <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.metrics.revenue30d') }}</p>
                <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(selectedSkill.revenue_30d) }}</p>
              </div>
            </div>

            <div class="mt-5 space-y-4">
              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.governance.noteTitle') }}</p>
                <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">
                  {{ selectedSkill.review_note || t('skills.admin.governance.noteEmpty') }}
                </p>
              </div>

              <div>
                <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.governance.tagsTitle') }}</p>
                <div v-if="selectedSkill.tags.length" class="mt-2 flex flex-wrap gap-2">
                  <span
                    v-for="tag in selectedSkill.tags"
                    :key="tag"
                    class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
                  >
                    {{ tag }}
                  </span>
                </div>
                <p v-else class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.tagsEmpty') }}</p>
              </div>

              <div class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
                <div class="grid gap-4 md:grid-cols-2">
                  <div>
                    <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.publishedVersionTitle') }}</p>
                    <p class="mt-2 text-sm font-medium text-gray-900 dark:text-white">{{ selectedSkill.latest_published_version || '-' }}</p>
                  </div>
                  <div>
                    <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.governance.successRateTitle') }}</p>
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
                {{ t('skills.admin.governance.forcePrivateAction') }}
              </button>
              <button
                type="button"
                class="btn btn-danger btn-sm"
                :disabled="selectedSkill.governance_status === 'disabled'"
                @click="openAction('disable', selectedSkill)"
              >
                {{ t('skills.admin.governance.disableAction') }}
              </button>
            </div>
          </template>

          <template v-else>
            <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
              {{ t('skills.admin.governance.detailEmpty') }}
            </div>
          </template>
        </div>

        <SkillAdminTimelineCard
          :title="t('skills.admin.governance.timelineTitle')"
          :description="t('skills.admin.governance.timelineDescription')"
          :empty-text="t('skills.admin.governance.timelineEmpty')"
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

const { t, locale } = useI18n()
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
  { value: 'online', label: t('skills.admin.governance.labels.online') },
  { value: 'force_private', label: t('skills.admin.governance.labels.forcePrivate') },
  { value: 'disabled', label: t('skills.admin.governance.labels.disabled') },
  { value: 'draft', label: t('skills.admin.governance.labels.draft') }
]

const reviewStatusOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'pending', label: t('skills.admin.review.labels.pending') },
  { value: 'approved', label: t('skills.admin.review.labels.approved') },
  { value: 'rejected', label: t('skills.admin.review.labels.rejected') }
]

const visibilityOptions = [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'public', label: t('skills.admin.review.labels.public') },
  { value: 'private', label: t('skills.admin.review.labels.private') },
  { value: 'force_private', label: t('skills.admin.review.labels.forcePrivate') }
]

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: t('skills.admin.governance.columns.skillVersion') },
  { key: 'author_name', label: t('skills.admin.governance.columns.author') },
  { key: 'latest_review_status', label: t('skills.admin.governance.columns.latestReview') },
  { key: 'governance_status', label: t('skills.admin.governance.columns.governanceStatus') },
  { key: 'visibility', label: t('skills.admin.governance.columns.visibility') },
  { key: 'requests_24h', label: t('skills.admin.governance.columns.requests24h') },
  { key: 'success_rate', label: t('skills.admin.governance.columns.successRate') },
  { key: 'revenue_30d', label: t('skills.admin.governance.columns.revenue30d') },
  { key: 'updated_at', label: t('skills.admin.governance.columns.updatedAt') },
  { key: 'actions', label: t('common.actions', '操作') }
])

const metricCards = computed(() => [
  {
    key: 'online',
    label: t('skills.admin.governance.metrics.onlineSkills'),
    value: summary.online_count,
    hint: t('skills.admin.governance.metrics.onlineHint'),
    icon: 'sparkles',
    tone: 'success' as const
  },
  {
    key: 'forcePrivate',
    label: t('skills.admin.governance.metrics.forcePrivate'),
    value: summary.force_private_count,
    hint: t('skills.admin.governance.metrics.forcePrivateHint'),
    icon: 'lock',
    tone: 'warning' as const
  },
  {
    key: 'disabled',
    label: t('skills.admin.governance.metrics.disabledSkills'),
    value: summary.disabled_count,
    hint: t('skills.admin.governance.metrics.disabledHint'),
    icon: 'ban',
    tone: 'danger' as const
  },
  {
    key: 'pendingVersions',
    label: t('skills.admin.governance.metrics.pendingVersions'),
    value: summary.pending_versions_count,
    hint: t('skills.admin.governance.metrics.pendingHint'),
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
      title: t('skills.admin.governance.timeline.createdTitle'),
      description: t('skills.admin.governance.timeline.createdDesc'),
      time: formatTime(item.created_at),
      status: 'done' as const
    },
    {
      key: 'review',
      title: t('skills.admin.governance.timeline.reviewTitle', {
        status: reviewStatusLabel(item.latest_review_status)
      }),
      description: item.latest_review_status === 'pending'
        ? t('skills.admin.governance.timeline.reviewPendingDesc')
        : t('skills.admin.governance.timeline.reviewDesc'),
      time: formatTime(item.updated_at),
      status: item.latest_review_status === 'pending' ? 'current' as const : 'done' as const
    },
    {
      key: 'visibility',
      title: t('skills.admin.governance.timeline.visibilityTitle', {
        status: visibilityLabel(item.visibility)
      }),
      description: item.visibility === 'force_private'
        ? t('skills.admin.governance.timeline.visibilityForcePrivateDesc')
        : item.visibility === 'public'
          ? t('skills.admin.governance.timeline.visibilityPublicDesc')
          : t('skills.admin.governance.timeline.visibilityPrivateDesc'),
      time: formatTime(item.updated_at),
      status: isForcePrivate ? 'danger' as const : 'done' as const
    },
    {
      key: 'governance',
      title: t('skills.admin.governance.timeline.governanceTitle', {
        status: governanceStatusLabel(item.governance_status)
      }),
      description: isDisabled
        ? t('skills.admin.governance.timeline.governanceDisabledDesc')
        : isForcePrivate
          ? t('skills.admin.governance.timeline.governanceForcePrivateDesc')
          : t('skills.admin.governance.timeline.governanceDesc'),
      time: formatTime(item.updated_at),
      status: isDisabled || isForcePrivate ? 'danger' as const : 'todo' as const
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

function governanceStatusLabel(status: SkillGovernanceStatus): string {
  return {
    online: t('skills.admin.governance.labels.online'),
    disabled: t('skills.admin.governance.labels.disabled'),
    force_private: t('skills.admin.governance.labels.forcePrivate'),
    draft: t('skills.admin.governance.labels.draft')
  }[status]
}

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: t('skills.admin.review.labels.public'),
    private: t('skills.admin.review.labels.private'),
    force_private: t('skills.admin.review.labels.forcePrivate')
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
  return new Intl.NumberFormat(locale.value.startsWith('zh') ? 'zh-CN' : 'en-US', {
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
  filters.review_status = next === 'approved' || next === 'rejected' || next === 'pending'
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
      appStore.showSuccess(receipt.message || t('skills.admin.governance.toastDisabled'))
    } else if (dialogAction.value === 'force-private') {
      const receipt = await adminSkillsAPI.forceSkillPrivate(actionTarget.value.skill_id, {
        note: payload.note,
        reason: payload.note
      })
      appStore.showSuccess(receipt.message || t('skills.admin.governance.toastForcePrivate'))
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
