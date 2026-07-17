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
              :placeholder="t('skills.admin.runtime.searchPlaceholder', '搜技能名、slug、版本')"
            />

            <div>
              <label class="input-label mb-1.5 block">{{ runtimeText.healthStatus }}</label>
              <Select :model-value="filters.health_status" :options="healthStatusOptions" @update:model-value="updateHealthStatusFilter" />
            </div>

          </div>
          <p class="text-xs text-ink-soft dark:text-dark-400">
            {{ runtimeText.summaryNote }}
          </p>
        </template>

        <template #actions>
          <div class="flex justify-end gap-3">
            <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset', '重置') }}</button>
            <button class="btn btn-secondary" :disabled="loading" @click="loadRuntime">
              <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            </button>
          </div>
        </template>

        <template #table>
          <DataTable :columns="columns" :data="runtimeItems" :loading="loading">
            <template #cell-skill_name="{ row }">
              <div class="min-w-[240px]">
                <div class="flex flex-wrap items-center gap-2">
                  <button
                    type="button"
                    class="text-left font-medium text-ink hover:text-brand-600 dark:text-white dark:hover:text-brand-300"
                    @click="selectRuntime(row)"
                  >
                    {{ row.skill_name }}
                  </button>
                  <span class="rounded-full bg-line px-2 py-0.5 text-xs text-ink-soft dark:bg-dark-700 dark:text-dark-300">
                    {{ row.current_version }}
                  </span>
                </div>
                <div class="mt-1 text-xs text-ink-soft dark:text-dark-400">{{ row.skill_slug }}</div>
              </div>
            </template>

            <template #cell-health_status="{ value }">
              <SkillAdminStatusBadge :status="value" :label="healthStatusLabel(value)" mode="runtime" />
            </template>

            <template #cell-requests_24h="{ value }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ Number(value).toLocaleString() }}</span>
            </template>

            <template #cell-success_rate="{ value }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ formatPercent(Number(value)) }}</span>
            </template>

            <template #cell-p95_latency_ms="{ value }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ formatMilliseconds(Number(value)) }}</span>
            </template>

            <template #cell-error_rate="{ value }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ formatPercent(Number(value)) }}</span>
            </template>

            <template #cell-queue_depth="{ value }">
              <span class="text-sm text-ink-body dark:text-dark-300">{{ Number(value).toLocaleString() }}</span>
            </template>

            <template #cell-last_run_at="{ value }">
              <span class="text-sm text-ink-soft dark:text-dark-400">{{ formatTime(value) }}</span>
            </template>

            <template #cell-last_error="{ value }">
              <span class="block max-w-[260px] truncate text-sm text-ink-soft dark:text-dark-400">{{ value || '-' }}</span>
            </template>

            <template #cell-actions="{ row }">
              <button class="btn btn-secondary btn-sm" @click="selectRuntime(row)">{{ runtimeText.view }}</button>
            </template>

            <template #empty>
              <EmptyState
                :title="runtimeText.empty.title"
                :description="runtimeText.empty.description"
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
        <div class="card border border-line p-5 dark:border-dark-700">
          <template v-if="selectedRuntime">
            <div class="flex flex-wrap items-start justify-between gap-4">
              <div class="min-w-0">
                <div class="flex flex-wrap items-center gap-2">
                  <h3 class="text-base font-semibold text-ink dark:text-white">{{ selectedRuntime.skill_name }}</h3>
                  <span class="rounded-full bg-line px-2 py-0.5 text-xs text-ink-soft dark:bg-dark-700 dark:text-dark-300">
                    {{ selectedRuntime.current_version }}
                  </span>
                </div>
                <p class="mt-2 text-sm text-ink-soft dark:text-dark-400">{{ selectedRuntime.skill_slug }}</p>
              </div>
              <SkillAdminStatusBadge
                :status="selectedRuntime.health_status"
                :label="healthStatusLabel(selectedRuntime.health_status)"
                mode="runtime"
              />
            </div>

            <div class="mt-5 grid gap-4 md:grid-cols-2">
              <div class="rounded-card bg-page p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.successRate }}</p>
                <p class="mt-2 text-xl font-semibold text-ink dark:text-white">{{ formatPercent(selectedRuntime.success_rate) }}</p>
              </div>
              <div class="rounded-card bg-page p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.p95Latency }}</p>
                <p class="mt-2 text-xl font-semibold text-ink dark:text-white">{{ formatMilliseconds(selectedRuntime.p95_latency_ms) }}</p>
              </div>
              <div class="rounded-card bg-page p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.errorRate }}</p>
                <p class="mt-2 text-xl font-semibold text-ink dark:text-white">{{ formatPercent(selectedRuntime.error_rate) }}</p>
              </div>
              <div class="rounded-card bg-page p-4 dark:bg-dark-900/60">
                <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.queueDepth }}</p>
                <p class="mt-2 text-xl font-semibold text-ink dark:text-white">{{ selectedRuntime.queue_depth.toLocaleString() }}</p>
              </div>
            </div>

            <div class="mt-5 rounded-card border border-line p-4 dark:border-dark-700">
              <div class="grid gap-4 md:grid-cols-2">
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.lastRunAt }}</p>
                  <p class="mt-2 text-sm font-medium text-ink dark:text-white">{{ formatTime(selectedRuntime.last_run_at) }}</p>
                </div>
                <div>
                  <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.lastAlertAt }}</p>
                  <p class="mt-2 text-sm font-medium text-ink dark:text-white">{{ formatTime(selectedRuntime.last_alert_at) }}</p>
                </div>
              </div>
              <div class="mt-4">
                <p class="text-xs uppercase tracking-[0.14em] text-ink-faint dark:text-dark-400">{{ runtimeText.detail.lastError }}</p>
                <p class="mt-2 text-sm leading-6 text-ink-soft dark:text-dark-400">
                  {{ selectedRuntime.last_error || runtimeText.detail.noRecentError }}
                </p>
              </div>
            </div>
          </template>

          <template v-else>
            <div
              data-test="runtime-detail-empty"
              class="rounded-card border border-dashed border-line px-4 py-10 text-center text-sm text-ink-soft dark:border-dark-700 dark:text-dark-400"
            >
              {{ runtimeText.selectionPlaceholder }}
            </div>
          </template>
        </div>

        <div class="card border border-line p-5 dark:border-dark-700">
          <div class="mb-5 flex items-center justify-between gap-3">
            <div>
              <h3 class="text-base font-semibold text-ink dark:text-white">{{ runtimeText.events.title }}</h3>
              <p class="mt-1 text-sm text-ink-soft dark:text-dark-400">{{ runtimeText.events.description }}</p>
            </div>
            <Icon name="bell" size="md" class="text-ink-faint dark:text-dark-500" />
          </div>

          <div v-if="filteredEvents.length" data-test="runtime-events-list" class="space-y-3">
            <div
              v-for="event in filteredEvents"
              :key="event.id"
              class="rounded-card border border-line p-4 dark:border-dark-700"
            >
              <div class="flex flex-wrap items-center justify-between gap-3">
                <div class="flex items-center gap-2">
                  <SkillAdminStatusBadge :status="event.level" :label="eventLevelLabel(event.level)" mode="runtime" />
                  <p class="text-sm font-medium text-ink dark:text-white">{{ event.skill_name || runtimeText.events.global }}</p>
                </div>
                <span class="text-xs text-ink-soft dark:text-dark-400">{{ formatTime(event.created_at) }}</span>
              </div>
              <p class="mt-3 text-sm leading-6 text-ink-soft dark:text-dark-400">{{ event.message }}</p>
              <p v-if="event.metric_name" class="mt-2 text-xs text-ink-soft dark:text-dark-400">
                {{ event.metric_name }}<span v-if="event.metric_value !== null"> · {{ event.metric_value }}</span>
              </p>
            </div>
          </div>

          <div
            v-else
            data-test="runtime-events-empty"
            class="rounded-card border border-dashed border-line px-4 py-10 text-center text-sm text-ink-soft dark:border-dark-700 dark:text-dark-400"
          >
            {{ runtimeText.events.empty }}
          </div>
        </div>
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
  type SkillRuntimeEvent,
  type SkillRuntimeHealth,
  type SkillRuntimeItem,
  type SkillRuntimeSummary
} from '@/api/admin/skills'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import SkillAdminMetricGrid from '@/components/skills/admin/SkillAdminMetricGrid.vue'
import SkillAdminStatusBadge from '@/components/skills/admin/SkillAdminStatusBadge.vue'

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const runtimeItems = ref<SkillRuntimeItem[]>([])
const events = ref<SkillRuntimeEvent[]>([])
const selectedRuntime = ref<SkillRuntimeItem | null>(null)

const pagination = reactive<BasePaginationResponse<SkillRuntimeItem>>({
  items: [],
  total: 0,
  page: 1,
  page_size: 20,
  pages: 1
})

const summary = reactive<SkillRuntimeSummary>({
  total_skills: 0,
  active_skills: 0,
  requests_24h: 0,
  success_rate: 0,
  p95_latency_ms: 0,
  warning_count: 0,
  critical_count: 0
})

const filters = reactive({
  search: '',
  health_status: 'all' as SkillRuntimeHealth | 'all'
})

const runtimeText = computed(() => ({
  subtitle: t('skills.admin.runtime.subtitle', '技能运行监控与告警'),
  healthStatus: t('skills.admin.runtime.healthStatus', '健康状态'),
  summaryNote: t(
    'skills.admin.runtime.summaryNote',
    '当前运行指标由后端按真实运行数据汇总；页面不再伪造前端统计窗口切换。'
  ),
  view: t('skills.admin.runtime.view', '查看'),
  empty: {
    title: t('skills.admin.runtime.emptyTitle', '暂无运行监控数据'),
    description: t('skills.admin.runtime.emptyDescription', '技能中心运行指标、告警和排队情况会展示在这里。'),
  },
  selectionPlaceholder: t('skills.admin.runtime.selectionPlaceholder', '选择一条技能运行记录后，这里会展示延迟、错误和队列指标。'),
  detail: {
    successRate: t('skills.admin.runtime.detail.successRate', '成功率'),
    p95Latency: t('skills.admin.runtime.detail.p95Latency', 'P95 延迟'),
    errorRate: t('skills.admin.runtime.detail.errorRate', '错误率'),
    queueDepth: t('skills.admin.runtime.detail.queueDepth', '队列深度'),
    lastRunAt: t('skills.admin.runtime.detail.lastRunAt', '最近运行'),
    lastAlertAt: t('skills.admin.runtime.detail.lastAlertAt', '最近告警'),
    lastError: t('skills.admin.runtime.detail.lastError', '最近错误'),
    noRecentError: t('skills.admin.runtime.detail.noRecentError', '当前技能没有最近错误记录。'),
  },
  events: {
    title: t('skills.admin.runtime.events.title', '最近告警 / 事件'),
    description: t('skills.admin.runtime.events.description', '默认展示全局事件；选中技能后优先过滤关联事件。'),
    global: t('skills.admin.runtime.events.global', '全局事件'),
    empty: t('skills.admin.runtime.events.empty', '当前没有告警事件。'),
  },
  health: {
    healthy: t('skills.admin.runtime.health.healthy', '健康'),
    warning: t('skills.admin.runtime.health.warning', '预警'),
    critical: t('skills.admin.runtime.health.critical', '严重'),
  },
  event: {
    info: t('skills.admin.runtime.event.info', '信息'),
    warning: t('skills.admin.runtime.event.warning', '预警'),
    critical: t('skills.admin.runtime.event.critical', '严重'),
  },
  columns: {
    skillName: t('skills.admin.runtime.columns.skillName', '技能 / 版本'),
    healthStatus: t('skills.admin.runtime.columns.healthStatus', '健康状态'),
    requests24h: t('skills.admin.runtime.columns.requests24h', '调用量'),
    successRate: t('skills.admin.runtime.columns.successRate', '成功率'),
    p95Latency: t('skills.admin.runtime.columns.p95Latency', 'P95 延迟'),
    errorRate: t('skills.admin.runtime.columns.errorRate', '错误率'),
    queueDepth: t('skills.admin.runtime.columns.queueDepth', '队列深度'),
    lastRunAt: t('skills.admin.runtime.columns.lastRunAt', '最近运行'),
    lastError: t('skills.admin.runtime.columns.lastError', '最近错误'),
    actions: t('common.actions', '操作'),
  },
  metrics: {
    activeSkills: t('skills.admin.runtime.metrics.activeSkills', '活跃技能'),
    totalSkills: t('skills.admin.runtime.metrics.totalSkills', { count: summary.total_skills }),
    requests24h: t('skills.admin.runtime.metrics.requests24h', '24h 请求'),
    requestsHint: t('skills.admin.runtime.metrics.requestsHint', '按当前筛选窗口统计'),
    averageSuccessRate: t('skills.admin.runtime.metrics.averageSuccessRate', '平均成功率'),
    warningCount: t('skills.admin.runtime.metrics.warningCount', { count: summary.warning_count }),
    p95Latency: t('skills.admin.runtime.metrics.p95Latency', 'P95 延迟'),
    latencyHint: t('skills.admin.runtime.metrics.latencyHint', '延迟高时优先排查队列和依赖'),
    criticalAlerts: t('skills.admin.runtime.metrics.criticalAlerts', '严重告警'),
    criticalHint: t('skills.admin.runtime.metrics.criticalHint', '建议优先处理影响线上可用性的技能'),
  },
}))

const healthStatusOptions = computed(() => [
  { value: 'all', label: t('common.all', '全部') },
  { value: 'healthy', label: runtimeText.value.health.healthy },
  { value: 'warning', label: runtimeText.value.health.warning },
  { value: 'critical', label: runtimeText.value.health.critical }
])

const columns = computed<Column[]>(() => [
  { key: 'skill_name', label: runtimeText.value.columns.skillName },
  { key: 'health_status', label: runtimeText.value.columns.healthStatus },
  { key: 'requests_24h', label: runtimeText.value.columns.requests24h },
  { key: 'success_rate', label: runtimeText.value.columns.successRate },
  { key: 'p95_latency_ms', label: runtimeText.value.columns.p95Latency },
  { key: 'error_rate', label: runtimeText.value.columns.errorRate },
  { key: 'queue_depth', label: runtimeText.value.columns.queueDepth },
  { key: 'last_run_at', label: runtimeText.value.columns.lastRunAt },
  { key: 'last_error', label: runtimeText.value.columns.lastError },
  { key: 'actions', label: runtimeText.value.columns.actions }
])

const metricCards = computed(() => [
  {
    key: 'active',
    label: runtimeText.value.metrics.activeSkills,
    value: summary.active_skills,
    hint: runtimeText.value.metrics.totalSkills,
    icon: 'sparkles',
    tone: 'success' as const
  },
  {
    key: 'requests',
    label: runtimeText.value.metrics.requests24h,
    value: summary.requests_24h.toLocaleString(),
    hint: runtimeText.value.metrics.requestsHint,
    icon: 'chartBar',
    tone: 'primary' as const
  },
  {
    key: 'successRate',
    label: runtimeText.value.metrics.averageSuccessRate,
    value: formatPercent(summary.success_rate),
    hint: runtimeText.value.metrics.warningCount,
    icon: 'checkCircle',
    tone: 'success' as const
  },
  {
    key: 'latency',
    label: runtimeText.value.metrics.p95Latency,
    value: formatMilliseconds(summary.p95_latency_ms),
    hint: runtimeText.value.metrics.latencyHint,
    icon: 'clock',
    tone: 'warning' as const
  },
  {
    key: 'critical',
    label: runtimeText.value.metrics.criticalAlerts,
    value: summary.critical_count,
    hint: runtimeText.value.metrics.criticalHint,
    icon: 'exclamationTriangle',
    tone: 'danger' as const
  }
])

const filteredEvents = computed(() => {
  if (!selectedRuntime.value) return events.value
  const matched = events.value.filter((event) => event.skill_id === selectedRuntime.value?.skill_id)
  return matched.length ? matched : events.value
})

function healthStatusLabel(status: SkillRuntimeHealth): string {
  return {
    healthy: runtimeText.value.health.healthy,
    warning: runtimeText.value.health.warning,
    critical: runtimeText.value.health.critical
  }[status]
}

function eventLevelLabel(level: SkillRuntimeEvent['level']): string {
  return {
    info: runtimeText.value.event.info,
    warning: runtimeText.value.event.warning,
    critical: runtimeText.value.event.critical
  }[level]
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`
}

function formatMilliseconds(value: number): string {
  return `${value.toFixed(0)} ms`
}

function formatTime(value?: string | null): string {
  if (!value) return '-'
  return new Date(value).toLocaleString()
}

function selectRuntime(row: SkillRuntimeItem) {
  selectedRuntime.value = row
}

function updateHealthStatusFilter(value: string | number | boolean | null) {
  const next = String(value ?? 'all')
  filters.health_status = next === 'healthy' || next === 'warning' || next === 'critical' ? next : 'all'
}

async function loadRuntime() {
  loading.value = true
  try {
    const response = await adminSkillsAPI.getRuntimeOverview(pagination.page, pagination.page_size, {
      search: filters.search.trim() || undefined,
      health_status: filters.health_status
    })
    runtimeItems.value = response.items
    events.value = response.events
    Object.assign(pagination, response)
    Object.assign(summary, response.summary)

    if (selectedRuntime.value) {
      selectedRuntime.value = response.items.find((item) => item.id === selectedRuntime.value?.id) ?? response.items[0] ?? null
    } else {
      selectedRuntime.value = response.items[0] ?? null
    }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('common.error', '加载失败')))
  } finally {
    loading.value = false
  }
}

function resetFilters() {
  filters.search = ''
  filters.health_status = 'all'
  pagination.page = 1
  void loadRuntime()
}

function handlePageChange(page: number) {
  pagination.page = page
  void loadRuntime()
}

function handlePageSizeChange(size: number) {
  pagination.page_size = size
  pagination.page = 1
  void loadRuntime()
}

onMounted(async () => {
  await loadRuntime()
})
</script>
