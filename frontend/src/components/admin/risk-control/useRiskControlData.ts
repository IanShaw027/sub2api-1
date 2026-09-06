import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ContentModerationAPIKeyStatus,
  ContentModerationConfig,
  ContentModerationLog,
  ContentModerationRuntimeStatus,
} from '@/api/admin/riskControl'
import type { AdminGroup, Proxy, SelectOption } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { ConfigFormState } from './types'
import {
  canUnbanRow,
  computeApiKeyHealthSummary,
  computeModelFilterSummary,
  computeModelFilterTooltip,
  normalizeFromDate,
  normalizeKeywordBlockingMode,
  normalizeModelFilter,
  normalizeToDate,
  riskThresholdDefaults,
  riskThresholdsFromConfig,
} from './riskControlUtils'

/**
 * 风控页面的数据加载与筛选逻辑：配置、运行时状态、审计记录分页/筛选、解封与哈希清理。
 */
export function useRiskControlData() {
  const { t } = useI18n()
  const appStore = useAppStore()
  const defaultBlockMessage = () => t('admin.riskControl.defaultBlockMessage')

  const loading = ref(true)
  const statusLoading = ref(false)
  const logsLoading = ref(false)
  const hashActionLoading = ref(false)
  const unbanningUserID = ref<number | null>(null)
  const flaggedHashInput = ref('')
  // 已保存 API Key 的“待删除”标记；页面级状态，设置弹层多次打开/关闭之间保持不重置，
  // 与原单文件实现的行为一致。设置弹层内的增删操作与本视图共享同一份引用。
  const pendingDeleteApiKeyHashes = ref<string[]>([])

  const groups = ref<AdminGroup[]>([])
  const proxies = ref<Proxy[]>([])
  const logs = ref<ContentModerationLog[]>([])
  const status = ref<ContentModerationRuntimeStatus | null>(null)
  let statusTimer: number | null = null

  const configForm = reactive<ConfigFormState>({
    enabled: false,
    mode: 'pre_block',
    base_url: 'https://api.openai.com',
    model: 'omni-moderation-latest',
    proxy_id: null,
    api_keys_text: '',
    api_key_configured: false,
    api_key_masked: '',
    api_key_count: 0,
    api_key_masks: [],
    api_key_statuses: [],
    api_keys_mode: 'append',
    clear_api_key: false,
    timeout_ms: 3000,
    retry_count: 2,
    sample_rate: 100,
    all_groups: true,
    group_ids: [],
    record_non_hits: false,
    worker_count: 4,
    queue_size: 32768,
    block_status: 403,
    block_message: defaultBlockMessage(),
    email_on_hit: true,
    auto_ban_enabled: true,
    cyber_policy_exclude_from_ban_count: false,
    ban_threshold: 10,
    violation_window_hours: 720,
    hit_retention_days: 180,
    non_hit_retention_days: 3,
    pre_hash_check_enabled: false,
    thresholds: { ...riskThresholdDefaults },
    blocked_keywords_text: '',
    keyword_blocking_mode: 'keyword_and_api',
    model_filter_type: 'all',
    model_filter_models: [],
  })

  const pagination = reactive({
    page: 1,
    page_size: 20,
    total: 0,
    pages: 1,
  })

  const filters = reactive({
    result: '',
    group_id: 0,
    endpoint: '',
    search: '',
    from: '',
    to: '',
  })

  function applyConfig(config: ContentModerationConfig) {
    configForm.enabled = config.enabled
    configForm.mode = config.mode
    configForm.base_url = config.base_url || 'https://api.openai.com'
    configForm.model = config.model || 'omni-moderation-latest'
    configForm.proxy_id = config.proxy_id || null
    configForm.api_keys_text = ''
    configForm.api_key_configured = config.api_key_configured
    configForm.api_key_masked = config.api_key_masked || ''
    configForm.api_key_count = config.api_key_count || 0
    configForm.api_key_masks = Array.isArray(config.api_key_masks) ? [...config.api_key_masks] : []
    configForm.api_key_statuses = Array.isArray(config.api_key_statuses) ? [...config.api_key_statuses] : []
    configForm.api_keys_mode = 'append'
    configForm.clear_api_key = false
    pendingDeleteApiKeyHashes.value = []
    configForm.timeout_ms = config.timeout_ms || 3000
    configForm.retry_count = config.retry_count ?? 2
    configForm.sample_rate = config.sample_rate ?? 100
    configForm.all_groups = config.all_groups
    configForm.group_ids = Array.isArray(config.group_ids) ? [...config.group_ids] : []
    configForm.record_non_hits = config.record_non_hits
    configForm.worker_count = config.worker_count || 4
    configForm.queue_size = config.queue_size || 32768
    configForm.block_status = config.block_status || 403
    configForm.block_message = config.block_message || defaultBlockMessage()
    configForm.email_on_hit = config.email_on_hit ?? true
    configForm.auto_ban_enabled = config.auto_ban_enabled ?? true
    configForm.cyber_policy_exclude_from_ban_count = config.cyber_policy_exclude_from_ban_count ?? false
    configForm.ban_threshold = config.ban_threshold || 10
    configForm.violation_window_hours = config.violation_window_hours || 720
    configForm.hit_retention_days = config.hit_retention_days || 180
    configForm.non_hit_retention_days = Math.min(Math.max(config.non_hit_retention_days || 3, 1), 3)
    configForm.pre_hash_check_enabled = config.pre_hash_check_enabled ?? false
    configForm.thresholds = riskThresholdsFromConfig(config.thresholds)
    configForm.blocked_keywords_text = Array.isArray(config.blocked_keywords) ? config.blocked_keywords.join('\n') : ''
    configForm.keyword_blocking_mode = normalizeKeywordBlockingMode(config.keyword_blocking_mode)
    const modelFilter = normalizeModelFilter(config.model_filter)
    configForm.model_filter_type = modelFilter.type
    configForm.model_filter_models = modelFilter.models
  }

  function prunePendingDeleteAPIKeyHashes(rows: ContentModerationAPIKeyStatus[]) {
    const currentHashes = new Set(rows.map((row) => row.key_hash).filter(Boolean))
    pendingDeleteApiKeyHashes.value = pendingDeleteApiKeyHashes.value.filter((hash) => currentHashes.has(hash))
  }

  async function loadAll() {
    loading.value = true
    try {
      const [config, groupItems, runtimeStatus, proxyItems] = await Promise.all([
        adminAPI.riskControl.getConfig(),
        adminAPI.groups.getAll(),
        adminAPI.riskControl.getStatus(),
        // 代理列表加载失败不阻塞风控页面（仅影响下拉可选项）
        adminAPI.proxies.getAll().catch(() => [] as Proxy[]),
      ])
      applyConfig(config)
      groups.value = groupItems
      status.value = runtimeStatus
      proxies.value = proxyItems
      if (Array.isArray(runtimeStatus.api_key_statuses)) {
        configForm.api_key_statuses = [...runtimeStatus.api_key_statuses]
        prunePendingDeleteAPIKeyHashes(runtimeStatus.api_key_statuses)
      }
      await loadLogs()
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.loadFailed')))
    } finally {
      loading.value = false
    }
  }

  async function loadStatus(silent = true) {
    statusLoading.value = true
    try {
      const runtimeStatus = await adminAPI.riskControl.getStatus()
      status.value = runtimeStatus
      if (Array.isArray(runtimeStatus.api_key_statuses)) {
        configForm.api_key_statuses = [...runtimeStatus.api_key_statuses]
        prunePendingDeleteAPIKeyHashes(runtimeStatus.api_key_statuses)
      }
    } catch (err: unknown) {
      if (!silent) {
        appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.statusFailed')))
      }
    } finally {
      statusLoading.value = false
    }
  }

  async function loadLogs() {
    logsLoading.value = true
    try {
      const params = {
        page: pagination.page,
        page_size: pagination.page_size,
        result: filters.result || undefined,
        group_id: filters.group_id || undefined,
        endpoint: filters.endpoint || undefined,
        search: filters.search || undefined,
        from: normalizeFromDate(filters.from),
        to: normalizeToDate(filters.to),
      }
      const result = await adminAPI.riskControl.listLogs(params)
      logs.value = result.items
      pagination.total = result.total
      pagination.page = result.page
      pagination.page_size = result.page_size
      pagination.pages = result.pages
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.logsFailed')))
    } finally {
      logsLoading.value = false
    }
  }

  function reloadLogsFromFirstPage() {
    pagination.page = 1
    void loadLogs()
  }

  function onPageChange(page: number) {
    pagination.page = page
    void loadLogs()
  }

  function onPageSizeChange(pageSize: number) {
    pagination.page = 1
    pagination.page_size = pageSize
    void loadLogs()
  }

  async function unbanUser(row: ContentModerationLog) {
    if (!row.user_id || unbanningUserID.value !== null) return
    unbanningUserID.value = row.user_id
    try {
      const result = await adminAPI.riskControl.unbanUser(row.user_id)
      logs.value = logs.value.map((item) => {
        if (item.user_id !== row.user_id) return item
        return { ...item, user_status: result.status }
      })
      appStore.showSuccess(t('admin.riskControl.unbanSuccess'))
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.unbanFailed')))
    } finally {
      unbanningUserID.value = null
    }
  }

  const resultOptions = computed<SelectOption[]>(() => [
    { value: '', label: t('admin.riskControl.result.all') },
    { value: 'hit', label: t('admin.riskControl.result.hit') },
    { value: 'blocked', label: t('admin.riskControl.result.blocked') },
    { value: 'pass', label: t('admin.riskControl.result.pass') },
    { value: 'error', label: t('admin.riskControl.result.error') },
  ])

  const endpointOptions = computed<SelectOption[]>(() => [
    { value: '', label: t('admin.riskControl.filters.allEndpoints') },
    { value: '/v1/messages', label: '/v1/messages' },
    { value: '/v1/responses', label: '/v1/responses' },
    { value: '/v1/chat/completions', label: '/v1/chat/completions' },
    { value: '/v1beta/models', label: '/v1beta/models' },
    { value: '/v1/images/generations', label: '/v1/images/generations' },
    { value: '/v1/images/edits', label: '/v1/images/edits' },
  ])

  const groupFilterOptions = computed<SelectOption[]>(() => [
    { value: 0, label: t('admin.riskControl.filters.allGroups') },
    ...groups.value.map((group) => ({
      value: group.id,
      label: `${group.name} (${group.platform})`,
    })),
  ])

  // 供记录表头部的“当前过滤”徽标使用；设置弹层内的同名派生值在 useRiskControlSettings 中
  // 基于同一份 configForm 独立计算，二者结果始终一致。
  const modelFilterSummary = computed(() => computeModelFilterSummary(t, configForm))

  const modelFilterTooltip = computed(() => computeModelFilterTooltip(t, configForm))

  // 供概览统计卡片使用。已保存 Key 状态列表在设置弹层中也会独立计算一份（依赖同一份
  // status/configForm），两处结果始终一致；健康摘要则额外依赖 pendingDeleteApiKeyHashes。
  const savedApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => {
    const rows = status.value?.api_key_statuses?.length
      ? status.value.api_key_statuses
      : configForm.api_key_statuses
    return Array.isArray(rows) ? rows : []
  })

  const apiKeyHealthSummary = computed(() => (
    computeApiKeyHealthSummary(t, configForm, savedApiKeyRows.value, pendingDeleteApiKeyHashes.value)
  ))

  const selectedGroupCount = computed(() => String(configForm.group_ids.length))

  const isFlaggedHashInputValid = computed(() => /^[a-fA-F0-9]{64}$/.test(flaggedHashInput.value.trim()))

  async function deleteFlaggedHash() {
    if (!isFlaggedHashInputValid.value || hashActionLoading.value) return
    hashActionLoading.value = true
    try {
      const result = await adminAPI.riskControl.deleteFlaggedHash(flaggedHashInput.value)
      flaggedHashInput.value = ''
      await loadStatus(true)
      appStore.showSuccess(result.deleted ? t('admin.riskControl.flaggedHashDeleted') : t('admin.riskControl.flaggedHashNotFound'))
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.flaggedHashDeleteFailed')))
    } finally {
      hashActionLoading.value = false
    }
  }

  async function clearFlaggedHashes() {
    if (hashActionLoading.value) return
    const confirmed = window.confirm(t('admin.riskControl.clearFlaggedHashesConfirm'))
    if (!confirmed) return
    hashActionLoading.value = true
    try {
      const result = await adminAPI.riskControl.clearFlaggedHashes()
      await loadStatus(true)
      appStore.showSuccess(t('admin.riskControl.flaggedHashesCleared', { count: result.deleted }))
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.flaggedHashesClearFailed')))
    } finally {
      hashActionLoading.value = false
    }
  }

  onMounted(() => {
    void loadAll()
    statusTimer = window.setInterval(() => {
      void loadStatus(true)
    }, 15000)
  })

  onUnmounted(() => {
    if (statusTimer !== null) {
      window.clearInterval(statusTimer)
      statusTimer = null
    }
  })

  return {
    defaultBlockMessage,
    loading,
    statusLoading,
    logsLoading,
    hashActionLoading,
    unbanningUserID,
    flaggedHashInput,
    pendingDeleteApiKeyHashes,
    apiKeyHealthSummary,
    selectedGroupCount,
    groups,
    proxies,
    logs,
    status,
    configForm,
    pagination,
    filters,
    resultOptions,
    endpointOptions,
    groupFilterOptions,
    modelFilterSummary,
    modelFilterTooltip,
    applyConfig,
    loadAll,
    loadStatus,
    loadLogs,
    reloadLogsFromFirstPage,
    onPageChange,
    onPageSizeChange,
    unbanUser,
    canUnbanRow,
    isFlaggedHashInputValid,
    deleteFlaggedHash,
    clearFlaggedHashes,
  }
}
