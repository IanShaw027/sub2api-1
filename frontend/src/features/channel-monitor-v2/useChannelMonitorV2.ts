/**
 * useChannelMonitorV2 — all state/data-loading/formatting logic for the
 * user-facing Channel Monitor V2 page (`/monitor`). Extracted out of
 * ChannelStatusV2View.vue so the view stays a thin template composition
 * (glass-ui-redesign task 12.2: view must shrink to <=400 lines).
 *
 * Behaviour is unchanged from the pre-refactor view — this is a pure
 * extraction, not a rewrite of business logic.
 */
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import { isChannelMonitorThroughputHidden } from '@/utils/featureFlags'
import * as api from '@/api/channelMonitorV2'
import type {
  HealthState,
  MonitorDimensions,
  MonitorErrorRow,
  MonitorFilter,
  MonitorHealth,
  MonitorMatrixGroupBy,
  MonitorMatrixResponse,
  MonitorModelRow,
  MonitorRange,
  MonitorSnapshot,
  MonitorUserRow,
} from '@/api/channelMonitorV2'
import {
  formatLatencyKpiSecondary,
  formatLatencyPrivacy,
  formatMonitorMs,
  formatMonitorPercent,
  formatMonitorThroughput,
  formatMonitorTokensPerSecond,
  tokensPerSecondFromTpm,
  healthScoreClass,
  monitorErrorCategoryLabel,
} from '@/features/channel-monitor-v2/monitorFormat'

export type MonitorTab = 'models' | 'errors' | 'users'
export type MonitorHealthMode = 'overall' | 'success' | 'ttft' | 'cache'
export type MonitorTrendView = 'pulse' | 'line'

export function useChannelMonitorV2() {
  const route = useRoute()
  const router = useRouter()
  const authStore = useAuthStore()
  const appStore = useAppStore()
  const { t, te, locale } = useI18n()
  const isAdmin = computed(() => authStore.isAdmin)
  /** Admins always see RPM/TPM; users honor the hide-throughput system setting. */
  const showThroughput = computed(() => isAdmin.value || !isChannelMonitorThroughputHidden())

  const ranges = computed(() => [
    { value: '90m' as MonitorRange, label: t('channelMonitorV2.ranges.90m') },
    { value: '24h' as MonitorRange, label: t('channelMonitorV2.ranges.24h') },
    { value: '7d' as MonitorRange, label: t('channelMonitorV2.ranges.7d') },
    { value: '30d' as MonitorRange, label: t('channelMonitorV2.ranges.30d') },
  ])
  const tabs = computed(() => [
    { value: 'models' as MonitorTab, label: t('channelMonitorV2.tabs.models') },
    { value: 'errors' as MonitorTab, label: t('channelMonitorV2.tabs.errors') },
    { value: 'users' as MonitorTab, label: t('channelMonitorV2.tabs.users') },
  ])
  const matrixGroupOptions = computed(() => [
    { value: 'platform' as MonitorMatrixGroupBy, label: t('channelMonitorV2.groupBy.platform') },
    { value: 'platform_group' as MonitorMatrixGroupBy, label: t('channelMonitorV2.groupBy.platformGroup') },
    { value: 'platform_model' as MonitorMatrixGroupBy, label: t('channelMonitorV2.groupBy.platformModel') },
    { value: 'platform_group_model' as MonitorMatrixGroupBy, label: t('channelMonitorV2.groupBy.platformGroupModel') },
  ])
  const healthModeOptions = computed(() => [
    { value: 'overall' as MonitorHealthMode, label: t('channelMonitorV2.healthMode.overall') },
    { value: 'success' as MonitorHealthMode, label: t('channelMonitorV2.healthMode.success') },
    { value: 'ttft' as MonitorHealthMode, label: t('channelMonitorV2.healthMode.ttft') },
    { value: 'cache' as MonitorHealthMode, label: t('channelMonitorV2.healthMode.cache') },
  ])
  const trendViewOptions = computed(() => [
    { value: 'pulse' as MonitorTrendView, label: t('channelMonitorV2.trendView.pulse') },
    { value: 'line' as MonitorTrendView, label: t('channelMonitorV2.trendView.line') },
  ])

  const filter = ref<MonitorFilter>({
    range: parseRange(route.query.range),
    platforms: csv(route.query.platform),
    groupIds: csv(route.query.group).map(Number).filter(Boolean),
    models: csv(route.query.model),
  })
  const activeTab = ref<MonitorTab>(
    (['models', 'errors', 'users'].includes(String(route.query.tab)) ? route.query.tab : 'models') as MonitorTab
  )
  const matrixGroupBy = ref<MonitorMatrixGroupBy>(parseMatrixGroupBy(route.query.group_by))
  const healthMode = ref<MonitorHealthMode>(parseHealthMode(route.query.health_mode))
  const trendView = ref<MonitorTrendView>(parseTrendView(route.query.trend_view))
  const dimensions = ref<MonitorDimensions>({ platforms: [], groups: [], models: [] })
  const snapshot = ref<MonitorSnapshot | null>(null)
  const matrix = ref<MonitorMatrixResponse | null>(null)
  const modelRows = ref<MonitorModelRow[]>([])
  const errorRows = ref<MonitorErrorRow[]>([])
  const userRows = ref<MonitorUserRow[]>([])
  const loading = ref(false)
  const tabLoading = ref(false)
  const refreshing = ref(false)
  const expandedErrors = ref(new Set<string>())
  let controller: AbortController | null = null
  let sequence = 0
  let autoRefreshTimer: number | null = null

  const hasDimensionFilter = computed(
    () => filter.value.platforms.length + filter.value.groupIds.length + filter.value.models.length > 0
  )
  // Full platform catalog (never pruned). Groups/models cascade by selected platforms
  // so choosing a platform narrows the other pickers without collapsing platforms.
  const platformOptions = computed(() =>
    (dimensions.value.platforms || []).map((item) => ({
      value: item.value,
      label: item.label,
    }))
  )
  const selectedPlatforms = computed(() => new Set(filter.value.platforms))
  const groupOptions = computed(() =>
    (dimensions.value.groups || [])
      .filter(
        (item) =>
          selectedPlatforms.value.size === 0 ||
          !item.platform ||
          selectedPlatforms.value.has(item.platform),
      )
      .map((item) => ({
        value: String(item.id),
        label: item.platform ? `${item.platform} / ${item.name || `#${item.id}`}` : item.name || `#${item.id}`,
      }))
  )
  const modelOptions = computed(() =>
    (dimensions.value.models || [])
      .filter(
        (item) =>
          selectedPlatforms.value.size === 0 ||
          !item.platform ||
          selectedPlatforms.value.has(item.platform),
      )
      .map((item) => ({
        value: item.value,
        label:
          item.platform && !item.label.includes(item.platform)
            ? `${item.platform} / ${item.label}`
            : item.label,
      }))
  )
  const selectedGroupIds = computed({
    get: () => filter.value.groupIds.map(String),
    set: (value: string[]) => {
      filter.value.groupIds = value.map(Number).filter((id) => Number.isInteger(id) && id > 0)
    },
  })
  // Soft-prune group/model selections that fall outside the platform cascade.
  // Do NOT wipe when options are temporarily empty (loading); only drop invalid ids.
  watch(
    [groupOptions, modelOptions],
    () => {
      if (groupOptions.value.length > 0) {
        const allowed = new Set(groupOptions.value.map((item) => item.value))
        const next = filter.value.groupIds.filter((id) => allowed.has(String(id)))
        if (next.length !== filter.value.groupIds.length) {
          filter.value.groupIds = next
        }
      }
      if (modelOptions.value.length > 0) {
        const allowed = new Set(modelOptions.value.map((item) => item.value))
        const next = filter.value.models.filter((model) => allowed.has(model))
        if (next.length !== filter.value.models.length) {
          filter.value.models = next
        }
      }
    },
    { flush: 'post' },
  )
  const activeRowsEmpty = computed(() =>
    activeTab.value === 'models'
      ? modelRows.value.length === 0
      : activeTab.value === 'errors'
        ? errorRows.value.length === 0
        : userRows.value.length === 0
  )
  /** First-upgrade backfill toward 90m/24h/7d/30d; banner hides when backend omits bootstrap. */
  const bootstrapActive = computed(() => Boolean(snapshot.value?.coverage?.bootstrap?.active))
  const bootstrapPercent = computed(() => {
    const raw = snapshot.value?.coverage?.bootstrap?.progress_percent
    if (typeof raw !== 'number' || Number.isNaN(raw)) return 0
    return Math.min(100, Math.max(0, Math.round(raw)))
  })
  const matrixRows = computed(() => {
    const items = matrix.value?.items || []
    // platform_group views should only show real groups, never bare platform placeholders.
    if (matrixGroupBy.value === 'platform_group' || matrixGroupBy.value === 'platform_group_model') {
      return items.filter((row) => row.group_id != null && Number(row.group_id) > 0)
    }
    return items
  })

  function csv(value: unknown) {
    return typeof value === 'string' ? value.split(',').filter(Boolean) : []
  }
  function parseRange(value: unknown): MonitorRange {
    return ['90m', '24h', '7d', '30d'].includes(String(value)) ? (value as MonitorRange) : '90m'
  }
  function parseMatrixGroupBy(value: unknown): MonitorMatrixGroupBy {
    const allowed: MonitorMatrixGroupBy[] = [
      'platform',
      'platform_group',
      'platform_model',
      'platform_group_model',
    ]
    return allowed.includes(value as MonitorMatrixGroupBy)
      ? (value as MonitorMatrixGroupBy)
      : 'platform_group'
  }
  function parseHealthMode(value: unknown): MonitorHealthMode {
    const allowed: MonitorHealthMode[] = ['overall', 'success', 'ttft', 'cache']
    return allowed.includes(value as MonitorHealthMode) ? (value as MonitorHealthMode) : 'overall'
  }
  function parseTrendView(value: unknown): MonitorTrendView {
    return value === 'line' ? 'line' : 'pulse'
  }
  function syncQuery() {
    void router.replace({
      query: {
        range: filter.value.range,
        platform: filter.value.platforms.join(',') || undefined,
        group: filter.value.groupIds.join(',') || undefined,
        model: filter.value.models.join(',') || undefined,
        group_by: matrixGroupBy.value,
        health_mode: healthMode.value,
        trend_view: trendView.value === 'line' ? 'line' : undefined,
        tab: activeTab.value,
      },
    })
  }
  /** Dimensions catalog: range only — never re-filtered by platform/group/model selection. */
  async function loadDimensions(signal?: AbortSignal, id = sequence) {
    const rangeOnly: MonitorFilter = {
      range: filter.value.range,
      platforms: [],
      groupIds: [],
      models: [],
    }
    const next = await api.getDimensions(rangeOnly, isAdmin.value, signal)
    if (id !== sequence) return
    dimensions.value = next
  }

  async function loadMetrics(signal?: AbortSignal, id = sequence) {
    const [nextSnapshot, nextMatrix] = await Promise.all([
      api.getSnapshot(filter.value, isAdmin.value, signal),
      api.getMatrix(filter.value, matrixGroupBy.value, isAdmin.value, signal),
    ])
    if (id !== sequence) return
    snapshot.value = nextSnapshot
    matrix.value = nextMatrix
    scheduleAutoRefresh()
    await loadTab(signal, id)
  }

  async function reload(silent = true) {
    controller?.abort()
    const request = new AbortController()
    controller = request
    const id = ++sequence
    refreshing.value = true
    if (!silent) loading.value = true
    try {
      // Catalog + metrics in parallel; catalog ignores dimension filters so options never shrink.
      await Promise.all([
        loadDimensions(request.signal, id),
        loadMetrics(request.signal, id),
      ])
    } catch (error) {
      if ((error as { name?: string }).name !== 'CanceledError') {
        appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
      }
    } finally {
      if (id === sequence) {
        loading.value = false
        tabLoading.value = false
        refreshing.value = false
      }
    }
  }

  /** When only range changes, still refresh dimensions; dimension filters only re-load metrics. */
  async function reloadMetricsOnly(silent = true) {
    controller?.abort()
    const request = new AbortController()
    controller = request
    const id = ++sequence
    refreshing.value = true
    if (!silent) loading.value = true
    try {
      await loadMetrics(request.signal, id)
    } catch (error) {
      if ((error as { name?: string }).name !== 'CanceledError') {
        appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.loadFailed')))
      }
    } finally {
      if (id === sequence) {
        loading.value = false
        tabLoading.value = false
        refreshing.value = false
      }
    }
  }
  async function loadTab(signal?: AbortSignal, id = sequence) {
    tabLoading.value = true
    try {
      if (activeTab.value === 'models') {
        modelRows.value = (await api.getModels(filter.value, isAdmin.value, signal)).items || []
      } else if (activeTab.value === 'errors') {
        errorRows.value = (await api.getErrors(filter.value, isAdmin.value, signal)).items || []
      } else {
        userRows.value = (await api.getUsers(filter.value, isAdmin.value, signal)).items || []
      }
    } catch (error) {
      const e = error as { name?: string; code?: string }
      if (e?.name === 'AbortError' || e?.name === 'CanceledError' || e?.code === 'ERR_CANCELED') return
      appStore.showError(extractApiErrorMessage(error, t('channelMonitorV2.detailLoadFailed')))
    } finally {
      if (id === sequence) tabLoading.value = false
    }
  }
  function setRange(value: MonitorRange) {
    filter.value.range = value
  }
  function clearDimensions() {
    // Replace arrays so deep watch always fires and metrics reload full window.
    filter.value = {
      ...filter.value,
      platforms: [],
      groupIds: [],
      models: [],
    }
  }
  function scheduleAutoRefresh() {
    if (autoRefreshTimer) {
      window.clearInterval(autoRefreshTimer)
      autoRefreshTimer = null
    }
    // Poll faster while first-upgrade bootstrap is filling 90m→30d so the progress bar moves.
    const seconds = bootstrapActive.value
      ? 10
      : snapshot.value?.config?.refresh_interval_seconds || 300
    autoRefreshTimer = window.setInterval(() => {
      if (!loading.value && !refreshing.value) {
        void reload(true)
      }
    }, Math.max(bootstrapActive.value ? 10 : 60, seconds) * 1000)
  }
  function drillModel(row: MonitorModelRow) {
    filter.value.platforms = [row.platform]
    filter.value.models = [row.model]
  }
  function formatRate(value: number) {
    return formatMonitorThroughput(value)
  }
  function exactRate(value: number) {
    return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 2 }).format(value || 0)
  }
  function formatTps(tpm: number | null | undefined) {
    return formatMonitorTokensPerSecond(tpm)
  }
  function exactTps(tpm: number | null | undefined) {
    return Intl.NumberFormat(locale.value || undefined, { maximumFractionDigits: 3 }).format(
      tokensPerSecondFromTpm(tpm),
    )
  }
  function formatPercent(value: number) {
    return formatMonitorPercent(value)
  }
  function formatMs(value: number | null) {
    return formatMonitorMs(value)
  }
  function latencyDetail(metric: {
    p50_ms: number | null
    p90_ms?: number | null
    p95_ms: number | null
    avg_ms?: number | null
  }) {
    return formatLatencyPrivacy(metric.p50_ms, metric.p90_ms, metric.avg_ms, metric.p95_ms)
  }
  /** KPI secondary: AVG · P90 under the P50 primary value. */
  function latencyKpiSecondary(metric: {
    p90_ms?: number | null
    p95_ms: number | null
    avg_ms?: number | null
  }) {
    return formatLatencyKpiSecondary(metric.avg_ms, metric.p90_ms, metric.p95_ms)
  }
  function formatTime(value: string) {
    return new Intl.DateTimeFormat(locale.value || undefined, {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    }).format(new Date(value))
  }
  function statusDot(health?: MonitorHealth | HealthState) {
    if (!health || typeof health === 'string') {
      return `status-dot health-${health || 'unknown'}`
    }
    // Prefer multi-band score when available; otherwise fall back to the coarse
    // overall state for mixed-version/older payloads.
    const klass =
      health.score != null
        ? healthScoreClass(health, 'overall', 0)
        : `health-${health.overall || 'unknown'}`
    return `status-dot ${klass}`
  }
  function errorLabel(value: string) {
    const key = `channelMonitorV2.errorCategories.${value}`
    return te(key) ? t(key) : monitorErrorCategoryLabel(value)
  }
  function toggleError(category: string) {
    const next = new Set(expandedErrors.value)
    if (next.has(category)) next.delete(category)
    else next.add(category)
    expandedErrors.value = next
  }

  let lastRange: MonitorRange = filter.value.range
  watch(
    filter,
    () => {
      syncQuery()
      const rangeChanged = filter.value.range !== lastRange
      lastRange = filter.value.range
      if (rangeChanged) void reload(true)
      else void reloadMetricsOnly(true)
    },
    { deep: true }
  )
  watch(matrixGroupBy, () => {
    syncQuery()
    void reloadMetricsOnly(true)
  })
  watch(healthMode, syncQuery)
  watch(trendView, syncQuery)
  watch(activeTab, () => {
    syncQuery()
    void loadTab()
  })
  onMounted(() => void reload(false))
  onBeforeUnmount(() => {
    controller?.abort()
    if (autoRefreshTimer) window.clearInterval(autoRefreshTimer)
  })

  return {
    isAdmin,
    showThroughput,
    ranges,
    tabs,
    matrixGroupOptions,
    healthModeOptions,
    trendViewOptions,
    filter,
    activeTab,
    matrixGroupBy,
    healthMode,
    trendView,
    dimensions,
    snapshot,
    matrix,
    modelRows,
    errorRows,
    userRows,
    loading,
    tabLoading,
    refreshing,
    expandedErrors,
    hasDimensionFilter,
    platformOptions,
    groupOptions,
    modelOptions,
    selectedGroupIds,
    activeRowsEmpty,
    bootstrapActive,
    bootstrapPercent,
    matrixRows,
    reload,
    setRange,
    clearDimensions,
    drillModel,
    formatRate,
    exactRate,
    formatTps,
    exactTps,
    formatPercent,
    formatMs,
    latencyDetail,
    latencyKpiSecondary,
    formatTime,
    statusDot,
    errorLabel,
    toggleError,
  }
}
