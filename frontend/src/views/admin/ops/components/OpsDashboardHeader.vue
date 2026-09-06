<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import { CONCRETE_PLATFORM_OPTIONS } from '@/constants/platforms'
import GlassCard from '@/components/ui/GlassCard.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import { adminAPI } from '@/api'
import type { OpsDashboardOverview, OpsMetricThresholds } from '@/api/admin/ops'
import type { OpsRequestDetailsPreset } from './OpsRequestDetailsModal.vue'
import { formatNumber } from '@/utils/format'
import HealthScoreCard from './header/HealthScoreCard.vue'
import RealtimeTrafficCard from './header/RealtimeTrafficCard.vue'
import MetricSummaryCards from './header/MetricSummaryCards.vue'
import SystemHealthCards from './header/SystemHealthCards.vue'
import JobHeartbeatsDialog from './header/JobHeartbeatsDialog.vue'
import CustomTimeRangeDialog from './header/CustomTimeRangeDialog.vue'
import { useOpsThresholds } from './header/useOpsThresholds'
import { useOpsHealthScore } from './header/useOpsHealthScore'
import { useOpsRealtimeTraffic } from './header/useOpsRealtimeTraffic'
import { useOpsSystemHealth } from './header/useOpsSystemHealth'

interface Props {
  overview?: OpsDashboardOverview | null
  platform: string
  groupId: number | null
  timeRange: string
  queryMode: string
  loading: boolean
  lastUpdated: Date | null
  thresholds?: OpsMetricThresholds | null // 阈值配置
  autoRefreshEnabled?: boolean
  autoRefreshCountdown?: number
  fullscreen?: boolean
  customStartTime?: string | null
  customEndTime?: string | null
}

interface Emits {
  (e: 'update:platform', value: string): void
  (e: 'update:group', value: number | null): void
  (e: 'update:timeRange', value: string): void
  (e: 'update:queryMode', value: string): void
  (e: 'update:customTimeRange', startTime: string, endTime: string): void
  (e: 'refresh'): void
  (e: 'openRequestDetails', preset?: OpsRequestDetailsPreset): void
  (e: 'openErrorDetails', kind: 'request' | 'upstream'): void
  (e: 'openSettings'): void
  (e: 'openAlertRules'): void
  (e: 'enterFullscreen'): void
  (e: 'exitFullscreen'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()

const overview = computed(() => props.overview ?? null)

// --- Filters ---

const showCustomTimeRangeDialog = ref(false)
const customStartTimeInput = ref('')
const customEndTimeInput = ref('')

function formatCustomTimeRangeLabel(startTime: string, endTime: string): string {
  const start = new Date(startTime)
  const end = new Date(endTime)
  const formatDate = (d: Date) => {
    const month = String(d.getMonth() + 1).padStart(2, '0')
    const day = String(d.getDate()).padStart(2, '0')
    const hour = String(d.getHours()).padStart(2, '0')
    const minute = String(d.getMinutes()).padStart(2, '0')
    return `${month}-${day} ${hour}:${minute}`
  }
  return `${formatDate(start)} ~ ${formatDate(end)}`
}

const groups = ref<Array<{ id: number; name: string; platform: string }>>([])

const platformOptions = computed(() => [
  { value: '', label: t('common.all') },
  ...CONCRETE_PLATFORM_OPTIONS
])

const timeRangeOptions = computed(() => [
  { value: '5m', label: t('admin.ops.timeRange.5m') },
  { value: '30m', label: t('admin.ops.timeRange.30m') },
  { value: '1h', label: t('admin.ops.timeRange.1h') },
  { value: '6h', label: t('admin.ops.timeRange.6h') },
  { value: '24h', label: t('admin.ops.timeRange.24h') },
  {
    value: 'custom',
    label: props.timeRange === 'custom' && props.customStartTime && props.customEndTime
      ? `${t('admin.ops.timeRange.custom')} (${formatCustomTimeRangeLabel(props.customStartTime, props.customEndTime)})`
      : t('admin.ops.timeRange.custom')
  }
])

const queryModeOptions = computed(() => [
  { value: 'auto', label: t('admin.ops.queryMode.auto') },
  { value: 'raw', label: t('admin.ops.queryMode.raw') },
  { value: 'preagg', label: t('admin.ops.queryMode.preagg') }
])

const groupOptions = computed(() => {
  const filtered = props.platform ? groups.value.filter((g) => g.platform === props.platform) : groups.value
  return [{ value: null, label: t('common.all') }, ...filtered.map((g) => ({ value: g.id, label: g.name }))]
})

watch(
  () => props.platform,
  (newPlatform) => {
    if (!newPlatform) return
    const currentGroup = groups.value.find((g) => g.id === props.groupId)
    if (currentGroup && currentGroup.platform !== newPlatform) {
      emit('update:group', null)
    }
  }
)

onMounted(async () => {
  try {
    const list = await adminAPI.groups.getAll()
    groups.value = list.map((g) => ({ id: g.id, name: g.name, platform: g.platform }))
  } catch (e) {
    console.error('[OpsDashboardHeader] Failed to load groups', e)
    groups.value = []
  }
})

function handlePlatformChange(val: string | number | boolean | null) {
  emit('update:platform', String(val || ''))
}

function handleGroupChange(val: string | number | boolean | null) {
  if (val === null || val === '' || typeof val === 'boolean') {
    emit('update:group', null)
    return
  }
  const id = typeof val === 'number' ? val : Number.parseInt(String(val), 10)
  emit('update:group', Number.isFinite(id) && id > 0 ? id : null)
}

function handleTimeRangeChange(val: string | number | boolean | null) {
  const newValue = String(val || '1h')
  if (newValue === 'custom') {
    // 初始化为最近1小时
    const now = new Date()
    const oneHourAgo = new Date(now.getTime() - 60 * 60 * 1000)
    customStartTimeInput.value = oneHourAgo.toISOString().slice(0, 16)
    customEndTimeInput.value = now.toISOString().slice(0, 16)
    showCustomTimeRangeDialog.value = true
  } else {
    emit('update:timeRange', newValue)
  }
}

function handleCustomTimeRangeConfirm() {
  if (!customStartTimeInput.value || !customEndTimeInput.value) return
  const startTime = new Date(customStartTimeInput.value).toISOString()
  const endTime = new Date(customEndTimeInput.value).toISOString()
  // Emit custom time range first so the parent can build correct API params
  // when it reacts to timeRange switching to "custom".
  emit('update:customTimeRange', startTime, endTime)
  emit('update:timeRange', 'custom')
  showCustomTimeRangeDialog.value = false
}

function handleCustomTimeRangeCancel() {
  showCustomTimeRangeDialog.value = false
  // 如果当前不是 custom，不需要做任何事
  // 如果当前是 custom，保持不变
}

function handleQueryModeChange(val: string | number | boolean | null) {
  emit('update:queryMode', String(val || 'auto'))
}

function openDetails(preset?: OpsRequestDetailsPreset) {
  emit('openRequestDetails', preset)
}

function openErrorDetails(kind: 'request' | 'upstream') {
  emit('openErrorDetails', kind)
}

// --- Threshold checking helpers ---

const {
  getSLAThresholdLevel,
  getTTFTThresholdLevel,
  getRequestErrorRateThresholdLevel,
  getUpstreamErrorRateThresholdLevel,
  getThresholdColorClass
} = useOpsThresholds({ thresholds: () => props.thresholds })

// --- Realtime / Overview labels ---

const totalRequestsLabel = computed(() => formatNumber(overview.value?.request_count_total ?? 0))
const totalTokensLabel = computed(() => formatNumber(overview.value?.token_consumed ?? 0))

const {
  realtimeWindow,
  availableRealtimeWindows,
  displayRealTimeQps,
  displayRealTimeTps,
  realtimeQpsPeakLabel,
  realtimeTpsPeakLabel,
  realtimeQpsAvgLabel,
  realtimeTpsAvgLabel,
  loadRealtimeTrafficSummary
} = useOpsRealtimeTraffic({
  timeRange: () => props.timeRange,
  platform: () => props.platform,
  groupId: () => props.groupId,
  autoRefreshEnabled: () => props.autoRefreshEnabled,
  autoRefreshCountdown: () => props.autoRefreshCountdown,
  loading: () => props.loading
})

function handleToolbarRefresh() {
  loadRealtimeTrafficSummary()
  emit('refresh')
}

const qpsAvgLabel = computed(() => {
  const v = overview.value?.qps?.avg
  if (typeof v !== 'number') return '-'
  return v.toFixed(1)
})

const tpsAvgLabel = computed(() => {
  const v = overview.value?.tps?.avg
  if (typeof v !== 'number') return '-'
  return v.toFixed(1)
})

const slaPercent = computed(() => {
  const v = overview.value?.sla
  if (typeof v !== 'number') return null
  if ((overview.value?.request_count_sla ?? 0) <= 0) return null
  return v * 100
})

const errorRatePercent = computed(() => {
  const v = overview.value?.error_rate
  if (typeof v !== 'number') return null
  return v * 100
})

const upstreamErrorRatePercent = computed(() => {
  const v = overview.value?.upstream_error_rate
  if (typeof v !== 'number') return null
  return v * 100
})

const durationP99Ms = computed(() => overview.value?.duration?.p99_ms ?? null)
const durationP95Ms = computed(() => overview.value?.duration?.p95_ms ?? null)
const durationP90Ms = computed(() => overview.value?.duration?.p90_ms ?? null)
const durationP50Ms = computed(() => overview.value?.duration?.p50_ms ?? null)
const durationAvgMs = computed(() => overview.value?.duration?.avg_ms ?? null)
const durationMaxMs = computed(() => overview.value?.duration?.max_ms ?? null)

const ttftP99Ms = computed(() => overview.value?.ttft?.p99_ms ?? null)
const ttftP95Ms = computed(() => overview.value?.ttft?.p95_ms ?? null)
const ttftP90Ms = computed(() => overview.value?.ttft?.p90_ms ?? null)
const ttftP50Ms = computed(() => overview.value?.ttft?.p50_ms ?? null)
const ttftAvgMs = computed(() => overview.value?.ttft?.avg_ms ?? null)
const ttftMaxMs = computed(() => overview.value?.ttft?.max_ms ?? null)

// --- Health Score & Diagnosis (primary) ---

const {
  isSystemIdle,
  healthScoreColor,
  healthScoreClass,
  circleSize,
  strokeWidth,
  radius,
  circumference,
  dashOffset,
  diagnosisReport
} = useOpsHealthScore({
  overview: () => overview.value,
  fullscreen: () => props.fullscreen,
  t
})

// --- System health (secondary) ---

const {
  systemMetrics,
  formatTimeShort,
  cpuPercentValue,
  cpuPercentClass,
  memPercentValue,
  memPercentClass,
  dbConnActiveValue,
  dbConnIdleValue,
  dbConnWaitingValue,
  dbConnOpenValue,
  dbMaxOpenConnsValue,
  dbMiddleLabel,
  dbMiddleClass,
  redisConnTotalValue,
  redisConnIdleValue,
  redisConnActiveValue,
  redisPoolSizeValue,
  redisMiddleLabel,
  redisMiddleClass,
  goroutineCountValue,
  goroutinesWarnThreshold,
  goroutinesCriticalThreshold,
  goroutineStatusLabel,
  goroutineStatusClass,
  jobHeartbeats,
  jobsWarnCount,
  jobsStatusLabel,
  jobsStatusClass,
  showJobsDetails,
  openJobsDetails
} = useOpsSystemHealth({
  overview: () => overview.value,
  t
})
</script>

<template>
  <div class="ops-header-shell">
  <PageHeader>
    <template #title>
      <span class="inline-flex items-center gap-2">
        <svg class="h-5 w-5 text-accent-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            stroke-width="2"
            d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 01-2 2h-2a2 2 0 01-2-2z"
          />
        </svg>
        {{ t('admin.ops.title') }}
      </span>
    </template>
    <template v-if="!props.fullscreen" #description>
      <span class="flex flex-wrap items-center gap-3">
        <span class="flex items-center gap-1.5" :title="props.loading ? t('admin.ops.loadingText') : t('admin.ops.ready')">
          <span class="relative flex h-2 w-2">
            <span class="relative inline-flex h-2 w-2 rounded-full" :class="props.loading ? 'bg-muted' : 'bg-success'"></span>
          </span>
          {{ props.loading ? t('admin.ops.loadingText') : t('admin.ops.ready') }}
        </span>

        <span>·</span>
        <span>{{ t('common.refresh') }}: {{ props.lastUpdated ? props.lastUpdated.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit' }).replace(/\//g, '-') : t('common.unknown') }}</span>

        <template v-if="props.autoRefreshEnabled && props.autoRefreshCountdown !== undefined">
          <span>·</span>
          <span>{{ t('admin.ops.autoRefreshRemaining', { seconds: props.autoRefreshCountdown }) }}</span>
        </template>
      </span>
    </template>
    <template v-if="!props.fullscreen" #actions>
      <div class="flex flex-wrap items-center gap-3">
          <Select
            :model-value="platform"
            :options="platformOptions"
            class="w-full sm:w-[140px]"
            @update:model-value="handlePlatformChange"
          />

          <Select
            :model-value="groupId"
            :options="groupOptions"
            class="w-full sm:w-[160px]"
            @update:model-value="handleGroupChange"
          />

          <div class="mx-1 hidden h-4 w-[1px] bg-surface-3  sm:block"></div>

          <Select
            :model-value="timeRange"
            :options="timeRangeOptions"
            class="relative w-full sm:w-[150px]"
            @update:model-value="handleTimeRangeChange"
          />

        <Select
          v-if="false"
          :model-value="queryMode"
          :options="queryModeOptions"
          class="relative w-full sm:w-[170px]"
          @update:model-value="handleQueryModeChange"
        />

        <button
          v-if="!props.fullscreen"
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-surface-2 text-muted transition-colors hover:bg-surface-3   "
          :disabled="loading"
          :title="t('common.refresh')"
          @click="handleToolbarRefresh"
        >
          <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
            />
          </svg>
        </button>

        <div v-if="!props.fullscreen" class="mx-1 hidden h-4 w-[1px] bg-surface-3  sm:block"></div>

        <!-- Alert Rules Button (hidden in fullscreen) -->
        <button
          v-if="!props.fullscreen"
          type="button"
          class="flex h-8 items-center gap-1.5 rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] px-3 text-xs font-bold text-accent transition-colors hover:bg-accent-200   "
          :title="t('admin.ops.alertRules.title')"
          @click="emit('openAlertRules')"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 17h5l-1.405-1.405A2.032 2.032 0 0118 14.158V11a6.002 6.002 0 00-4-5.659V5a2 2 0 10-4 0v.341C7.67 6.165 6 8.388 6 11v3.159c0 .538-.214 1.055-.595 1.436L4 17h5m6 0v1a3 3 0 11-6 0v-1m6 0H9" />
          </svg>
          <span class="hidden sm:inline">{{ t('admin.ops.alertRules.manage') }}</span>
        </button>

        <!-- Settings Button (hidden in fullscreen) -->
        <button
          v-if="!props.fullscreen"
          type="button"
          class="flex h-8 items-center gap-1.5 rounded-lg bg-surface-2 px-3 text-xs font-bold text-foreground transition-colors hover:bg-surface-3   "
          :title="t('admin.ops.settings.title')"
          @click="emit('openSettings')"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
          </svg>
          <span class="hidden sm:inline">{{ t('common.settings') }}</span>
        </button>

        <!-- Enter Fullscreen Button (hidden in fullscreen mode) -->
        <button
          v-if="!props.fullscreen"
          type="button"
          class="flex h-8 w-8 items-center justify-center rounded-lg bg-surface-2 text-foreground transition-colors hover:bg-surface-3   "
          :title="t('admin.ops.fullscreen.enter')"
          @click="emit('enterFullscreen')"
        >
          <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
          </svg>
        </button>
      </div>
    </template>
  </PageHeader>
  </div>

  <GlassCard :class="['flex flex-col gap-4 !rounded-xl', props.fullscreen ? '!p-8' : '!p-6']" padding="sm">
    <div v-if="overview" class="grid grid-cols-1 gap-6 lg:grid-cols-12">
      <!-- Left: Health + Realtime -->
      <div :class="['rounded-xl bg-surface-2  lg:col-span-5', props.fullscreen ? 'p-6' : 'p-4']">
        <div class="grid h-full grid-cols-1 gap-6 md:grid-cols-[200px_1fr] md:items-center">
          <HealthScoreCard
            :fullscreen="props.fullscreen"
            :is-system-idle="isSystemIdle"
            :health-score-raw="overview.health_score"
            :health-score-color="healthScoreColor"
            :health-score-class="healthScoreClass"
            :circle-size="circleSize"
            :stroke-width="strokeWidth"
            :radius="radius"
            :circumference="circumference"
            :dash-offset="dashOffset"
            :diagnosis-report="diagnosisReport"
          />

          <RealtimeTrafficCard
            v-model="realtimeWindow"
            :fullscreen="props.fullscreen"
            :available-realtime-windows="availableRealtimeWindows"
            :display-real-time-qps="displayRealTimeQps"
            :display-real-time-tps="displayRealTimeTps"
            :realtime-qps-peak-label="realtimeQpsPeakLabel"
            :realtime-tps-peak-label="realtimeTpsPeakLabel"
            :realtime-qps-avg-label="realtimeQpsAvgLabel"
            :realtime-tps-avg-label="realtimeTpsAvgLabel"
          />
        </div>
      </div>

      <MetricSummaryCards
        :fullscreen="props.fullscreen"
        :overview="overview"
        :total-requests-label="totalRequestsLabel"
        :total-tokens-label="totalTokensLabel"
        :qps-avg-label="qpsAvgLabel"
        :tps-avg-label="tpsAvgLabel"
        :sla-percent="slaPercent"
        :error-rate-percent="errorRatePercent"
        :upstream-error-rate-percent="upstreamErrorRatePercent"
        :duration-p99-ms="durationP99Ms"
        :duration-p95-ms="durationP95Ms"
        :duration-p90-ms="durationP90Ms"
        :duration-p50-ms="durationP50Ms"
        :duration-avg-ms="durationAvgMs"
        :duration-max-ms="durationMaxMs"
        :ttft-p99-ms="ttftP99Ms"
        :ttft-p95-ms="ttftP95Ms"
        :ttft-p90-ms="ttftP90Ms"
        :ttft-p50-ms="ttftP50Ms"
        :ttft-avg-ms="ttftAvgMs"
        :ttft-max-ms="ttftMaxMs"
        :getSLAThresholdLevel="getSLAThresholdLevel"
        :getTTFTThresholdLevel="getTTFTThresholdLevel"
        :get-request-error-rate-threshold-level="getRequestErrorRateThresholdLevel"
        :get-upstream-error-rate-threshold-level="getUpstreamErrorRateThresholdLevel"
        :get-threshold-color-class="getThresholdColorClass"
        @open-request-details="openDetails"
        @open-error-details="openErrorDetails"
      />
    </div>

    <!-- Integrated: System health (cards) -->
    <div v-if="overview" class="mt-2 border-t border-line pt-4 ">
      <SystemHealthCards
        :fullscreen="props.fullscreen"
        :system-metrics="systemMetrics"
        :cpu-percent-value="cpuPercentValue"
        :cpu-percent-class="cpuPercentClass"
        :mem-percent-value="memPercentValue"
        :mem-percent-class="memPercentClass"
        :db-conn-active-value="dbConnActiveValue"
        :db-conn-idle-value="dbConnIdleValue"
        :db-conn-waiting-value="dbConnWaitingValue"
        :db-conn-open-value="dbConnOpenValue"
        :db-max-open-conns-value="dbMaxOpenConnsValue"
        :db-middle-label="dbMiddleLabel"
        :db-middle-class="dbMiddleClass"
        :redis-conn-total-value="redisConnTotalValue"
        :redis-conn-idle-value="redisConnIdleValue"
        :redis-conn-active-value="redisConnActiveValue"
        :redis-pool-size-value="redisPoolSizeValue"
        :redis-middle-label="redisMiddleLabel"
        :redis-middle-class="redisMiddleClass"
        :goroutine-count-value="goroutineCountValue"
        :goroutines-warn-threshold="goroutinesWarnThreshold"
        :goroutines-critical-threshold="goroutinesCriticalThreshold"
        :goroutine-status-label="goroutineStatusLabel"
        :goroutine-status-class="goroutineStatusClass"
        :job-heartbeats-count="jobHeartbeats.length"
        :jobs-warn-count="jobsWarnCount"
        :jobs-status-label="jobsStatusLabel"
        :jobs-status-class="jobsStatusClass"
        @open-jobs-details="openJobsDetails"
      />
    </div>

    <JobHeartbeatsDialog
      :show="showJobsDetails"
      :job-heartbeats="jobHeartbeats"
      :format-time-short="formatTimeShort"
      @close="showJobsDetails = false"
    />

    <CustomTimeRangeDialog
      :show="showCustomTimeRangeDialog"
      v-model:start-time="customStartTimeInput"
      v-model:end-time="customEndTimeInput"
      @confirm="handleCustomTimeRangeConfirm"
      @cancel="handleCustomTimeRangeCancel"
    />
  </GlassCard>
</template>

<style scoped>
/* Allow the PageHeader row to stack (title above, toolbar below) instead of
   squeezing the h1 to near-zero width when the actions slot's filter/select
   controls go full-width on narrow viewports (see deviations.md).
   Note: PageHeader is invoked as this component's own root node, so its
   rendered root (.ui-page-header) receives BOTH this component's scoped
   data-v attribute and PageHeader's own — meaning a bare `:deep(.ui-page-header)`
   compiles to a descendant selector that can never match its own root element.
   The `.ops-header-shell` wrapper below gives :deep() a real ancestor to key off. */
.ops-header-shell :deep(.ui-page-header) {
  flex-wrap: wrap;
}

@media (max-width: 640px) {
  .ops-header-shell :deep(.ui-page-header-actions) {
    flex-basis: 100%;
    width: 100%;
  }
}
</style>
