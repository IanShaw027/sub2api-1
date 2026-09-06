import { computed, ref, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ContentModerationAPIKeyStatus,
  ContentModerationModelFilterType,
  ContentModerationRuntimeStatus,
  ContentModerationTestAuditResult,
  ModerationMode,
  UpdateContentModerationConfig,
} from '@/api/admin/riskControl'
import type { AdminGroup, SelectOption } from '@/types'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'
import type {
  APIKeysWriteMode,
  ConfigFormState,
  KeywordNoticeView,
  ModerationScoreRow,
  RiskThresholdRow,
  SettingsTab,
} from './types'
import {
  apiKeyHealthStatusLabel,
  buildModelFilterPayload,
  buildRiskThresholdPayload,
  computeModelFilterModelCount,
  computeModelFilterSummary,
  fileToDataURL,
  formatDateTime,
  maxModerationTestImages,
  maxModerationTestImageSize,
  maxVisibleApiKeyRows,
  moderationModeLabel,
  parseApiKeys,
  parseBlockedKeywords,
  riskThresholdCategories,
  riskThresholdDefaults,
} from './riskControlUtils'
import type { ContentModerationConfig } from '@/api/admin/riskControl'

/**
 * 风控设置弹层的表单状态：API Key 管理、审计测试、群组/模型范围、风险阈值、关键词、保存逻辑。
 */
export function useRiskControlSettings(
  configForm: ConfigFormState,
  status: Ref<ContentModerationRuntimeStatus | null>,
  groups: Ref<AdminGroup[]>,
  // Parent-owned state persists across modal openings; writes use the modal's v-model.
  pendingDeleteApiKeyHashes: Ref<string[]>,
  refreshers: {
    loadStatus: (silent?: boolean) => Promise<void>
    loadLogs: () => Promise<void>
    applyConfig: (config: ContentModerationConfig) => void
    close: () => void
  },
) {
  const { t } = useI18n()
  const appStore = useAppStore()

  const saving = ref(false)
  const apiKeyTesting = ref(false)
  const activeSettingsTab = ref<SettingsTab>('basic')
  const groupSearch = ref('')
  const testedApiKeyStatuses = ref<ContentModerationAPIKeyStatus[]>([])
  const apiKeyRowsExpanded = ref<boolean>(false)
  const moderationTestPrompt = ref('')
  const moderationTestImages = ref<string[]>([])
  const moderationTestResult = ref<ContentModerationTestAuditResult | null>(null)

  const settingsTabs = computed<Array<{ id: SettingsTab; label: string }>>(() => [
    { id: 'basic', label: t('admin.riskControl.tabs.basic') },
    { id: 'scope', label: t('admin.riskControl.tabs.scope') },
    { id: 'runtime', label: t('admin.riskControl.tabs.runtime') },
    { id: 'response', label: t('admin.riskControl.tabs.response') },
    { id: 'riskThresholds', label: t('admin.riskControl.tabs.riskThresholds') },
    { id: 'keywords', label: t('admin.riskControl.tabs.keywords') },
    { id: 'retention', label: t('admin.riskControl.tabs.retention') },
  ])

  const modeOptions = computed<SelectOption[]>(() => [
    { value: 'pre_block', label: t('admin.riskControl.modePreBlock') },
    { value: 'observe', label: t('admin.riskControl.modeObserve') },
    { value: 'off', label: t('admin.riskControl.modeOff') },
  ])

  const keywordBlockingModeOptions = computed(() => [
    {
      value: 'keyword_and_api' as const,
      label: t('admin.riskControl.keywordModeKeywordAndApi'),
      description: t('admin.riskControl.keywordModeKeywordAndApiDesc'),
    },
    {
      value: 'keyword_only' as const,
      label: t('admin.riskControl.keywordModeKeywordOnly'),
      description: t('admin.riskControl.keywordModeKeywordOnlyDesc'),
    },
    {
      value: 'api_only' as const,
      label: t('admin.riskControl.keywordModeApiOnly'),
      description: t('admin.riskControl.keywordModeApiOnlyDesc'),
    },
  ])

  const modelFilterOptions = computed<Array<{ value: ContentModerationModelFilterType; label: string; description: string }>>(() => [
    {
      value: 'all',
      label: t('admin.riskControl.modelFilterAll'),
      description: t('admin.riskControl.modelFilterAllDesc'),
    },
    {
      value: 'include',
      label: t('admin.riskControl.modelFilterInclude'),
      description: t('admin.riskControl.modelFilterIncludeDesc'),
    },
    {
      value: 'exclude',
      label: t('admin.riskControl.modelFilterExclude'),
      description: t('admin.riskControl.modelFilterExcludeDesc'),
    },
  ])

  const keywordNoticeTones = {
    info: {
      icon: 'infoCircle' as const,
      toneClass: 'border-[color-mix(in_oklch,var(--accent)_18%,transparent)] bg-[color-mix(in_oklch,var(--accent)_7.2%,transparent)]  ',
      iconClass: 'mt-0.5 flex-shrink-0 text-accent ',
      titleClass: 'text-accent ',
    },
    warning: {
      icon: 'exclamationTriangle' as const,
      toneClass: 'border-[color-mix(in_oklch,var(--warning)_35%,transparent)] bg-[color-mix(in_oklch,var(--warning)_18%,transparent)]  ',
      iconClass: 'mt-0.5 flex-shrink-0 text-warning-500 ',
      titleClass: 'text-warning-text ',
    },
  }

  const keywordNotice = computed<KeywordNoticeView>(() => {
    const strategy = configForm.keyword_blocking_mode
    if (strategy === 'api_only') {
      return {
        ...keywordNoticeTones.info,
        title: t('admin.riskControl.keywordModeApiOnlyNotice'),
        description: t('admin.riskControl.keywordModeApiOnlyDesc'),
      }
    }
    if (configForm.mode !== 'pre_block') {
      return {
        ...keywordNoticeTones.warning,
        title: t('admin.riskControl.blockedKeywordsModeWarning', { mode: modeLabel(configForm.mode) }),
        description: t('admin.riskControl.blockedKeywordsDescription'),
      }
    }
    if (strategy === 'keyword_only') {
      return {
        ...keywordNoticeTones.info,
        title: t('admin.riskControl.keywordModeKeywordOnlyNotice'),
        description: t('admin.riskControl.keywordModeKeywordOnlyDesc'),
      }
    }
    return {
      ...keywordNoticeTones.info,
      title: t('admin.riskControl.blockedKeywordsPreBlockHint'),
      description: t('admin.riskControl.blockedKeywordsDescription'),
    }
  })

  const filteredGroups = computed(() => {
    const keyword = groupSearch.value.trim().toLowerCase()
    if (!keyword) return groups.value
    return groups.value.filter((group) => {
      return group.name.toLowerCase().includes(keyword) || String(group.platform).toLowerCase().includes(keyword)
    })
  })

  const inputApiKeyCount = computed(() => parseApiKeys(configForm.api_keys_text).length)

  const blockedKeywordList = computed(() => parseBlockedKeywords(configForm.blocked_keywords_text))

  const blockedKeywordCount = computed(() => blockedKeywordList.value.length)

  const pendingDeletedApiKeyCount = computed(() => pendingDeleteApiKeyHashes.value.length)

  const effectiveStoredApiKeyCount = computed(() => Math.max(0, configForm.api_key_count - pendingDeletedApiKeyCount.value))

  const apiKeysPlaceholder = computed(() => (
    configForm.api_keys_mode === 'replace'
      ? t('admin.riskControl.apiKeysPlaceholderReplace')
      : t('admin.riskControl.apiKeysPlaceholder')
  ))

  const apiKeysModeHint = computed(() => (
    configForm.api_keys_mode === 'replace'
      ? t('admin.riskControl.apiKeysModeReplaceHint')
      : t('admin.riskControl.apiKeysModeAppendHint')
  ))

  const hasModerationAuditInput = computed(() => {
    return moderationTestPrompt.value.trim() !== '' || moderationTestImages.value.length > 0
  })

  const storedApiKeyTestButtonText = computed(() => {
    if (apiKeyTesting.value) return t('admin.riskControl.testingApiKeys')
    if (hasModerationAuditInput.value) return t('admin.riskControl.testContentWithStoredApiKey')
    return t('admin.riskControl.testStoredApiKeys')
  })

  const savedApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => {
    const rows = status.value?.api_key_statuses?.length
      ? status.value.api_key_statuses
      : configForm.api_key_statuses
    return Array.isArray(rows) ? rows : []
  })

  const apiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => [
    ...savedApiKeyRows.value,
    ...testedApiKeyStatuses.value,
  ])

  const visibleApiKeyRows = computed<ContentModerationAPIKeyStatus[]>(() => {
    if (apiKeyRowsExpanded.value) return apiKeyRows.value
    return apiKeyRows.value.slice(0, maxVisibleApiKeyRows)
  })

  const hiddenApiKeyRowCount = computed<number>(() => Math.max(0, apiKeyRows.value.length - visibleApiKeyRows.value.length))

  const canToggleApiKeyRows = computed<boolean>(() => apiKeyRows.value.length > maxVisibleApiKeyRows)

  const moderationScoreRows = computed<ModerationScoreRow[]>(() => {
    const result = moderationTestResult.value
    if (!result) return []
    return Object.entries(result.category_scores || {})
      .map(([category, score]) => {
        const threshold = result.thresholds?.[category] ?? 1
        return {
          category,
          score,
          threshold,
          hit: score >= threshold,
        }
      })
      .sort((a, b) => b.score - a.score)
  })

  const riskThresholdRows = computed<RiskThresholdRow[]>(() => (
    riskThresholdCategories.map((category) => ({
      category,
      value: configForm.thresholds[category] ?? riskThresholdDefaults[category],
      defaultValue: riskThresholdDefaults[category],
    }))
  ))

  const modelFilterModelCount = computed(() => computeModelFilterModelCount(configForm))

  const modelFilterSummary = computed(() => computeModelFilterSummary(t, configForm))

  function modeLabel(mode: ModerationMode): string {
    return moderationModeLabel(t, mode)
  }

  function modeDescription(mode: ModerationMode): string {
    const descriptions: Record<ModerationMode, string> = {
      pre_block: t('admin.riskControl.modePreBlockDesc'),
      observe: t('admin.riskControl.modeObserveDesc'),
      off: t('admin.riskControl.modeOffDesc'),
    }
    return descriptions[mode] ?? ''
  }

  function apiKeyStatusLabel(statusValue: ContentModerationAPIKeyStatus['status']): string {
    return apiKeyHealthStatusLabel(t, statusValue)
  }

  function apiKeyStatusMeta(row: ContentModerationAPIKeyStatus): string {
    const parts: string[] = []
    parts.push(t('admin.riskControl.apiKeyFailureCount', { count: row.failure_count || 0 }))
    if (row.last_latency_ms > 0) {
      parts.push(t('admin.riskControl.apiKeyLatency', { ms: row.last_latency_ms }))
    }
    if (row.last_http_status > 0) {
      parts.push(t('admin.riskControl.apiKeyHTTPStatus', { status: row.last_http_status }))
    }
    if (row.frozen_until) {
      parts.push(t('admin.riskControl.apiKeyFrozenUntil', { time: formatDateTime(row.frozen_until) }))
    } else if (row.last_checked_at) {
      parts.push(t('admin.riskControl.apiKeyLastChecked', { time: formatDateTime(row.last_checked_at) }))
    } else {
      parts.push(t('admin.riskControl.apiKeyNotTested'))
    }
    return parts.join(' / ')
  }

  function toggleClearApiKey() {
    configForm.clear_api_key = !configForm.clear_api_key
    if (configForm.clear_api_key) {
      configForm.api_keys_text = ''
      configForm.api_keys_mode = 'append'
      testedApiKeyStatuses.value = []
      pendingDeleteApiKeyHashes.value = []
    }
  }

  function setAPIKeysMode(mode: APIKeysWriteMode) {
    configForm.api_keys_mode = mode
    if (mode === 'replace') {
      pendingDeleteApiKeyHashes.value = []
    }
  }

  function setModelFilterType(type: ContentModerationModelFilterType) {
    configForm.model_filter_type = type
    if (type === 'all') {
      configForm.model_filter_models = []
    }
  }

  async function testApiKeys(useInputKeys: boolean) {
    const keys = useInputKeys ? parseApiKeys(configForm.api_keys_text) : []
    if (useInputKeys && keys.length === 0) {
      appStore.showError(t('admin.riskControl.apiKeyTestNoInput'))
      return
    }
    apiKeyTesting.value = true
    try {
      const result = await adminAPI.riskControl.testAPIKeys({
        api_keys: keys,
        base_url: configForm.base_url,
        model: configForm.model,
        timeout_ms: Number(configForm.timeout_ms) || 3000,
        // 与保存语义一致：0 强制直连，>0 指定代理，确保测试与实际审计走同一条链路
        proxy_id: configForm.proxy_id ?? 0,
        prompt: moderationTestPrompt.value,
        images: moderationTestImages.value,
      })
      moderationTestResult.value = result.audit_result ?? null
      if (useInputKeys) {
        testedApiKeyStatuses.value = result.items.map((item) => ({ ...item, configured: false }))
      } else {
        mergeConfiguredAPIKeyStatuses(result.items)
        testedApiKeyStatuses.value = []
        await refreshers.loadStatus(true)
      }
      appStore.showSuccess(t('admin.riskControl.apiKeyTestDone', { count: result.items.length }))
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.apiKeyTestFailed')))
    } finally {
      apiKeyTesting.value = false
    }
  }

  function mergeConfiguredAPIKeyStatuses(items: ContentModerationAPIKeyStatus[]) {
    if (!hasModerationAuditInput.value || configForm.api_key_statuses.length === 0) {
      configForm.api_key_statuses = items
      return
    }
    const updates = new Map(items.map((item) => [item.key_hash, item]))
    configForm.api_key_statuses = configForm.api_key_statuses.map((item) => updates.get(item.key_hash) ?? item)
  }

  function toggleDeleteStoredApiKey(row: ContentModerationAPIKeyStatus) {
    if (!row.configured || !row.key_hash) return
    pendingDeleteApiKeyHashes.value = pendingDeleteApiKeyHashes.value.includes(row.key_hash)
      ? pendingDeleteApiKeyHashes.value.filter((hash) => hash !== row.key_hash)
      : [...pendingDeleteApiKeyHashes.value, row.key_hash]
  }

  function isStoredApiKeyPendingDelete(row: ContentModerationAPIKeyStatus): boolean {
    return row.configured && row.key_hash !== '' && pendingDeleteApiKeyHashes.value.includes(row.key_hash)
  }

  function clearModerationTestInput() {
    moderationTestPrompt.value = ''
    moderationTestImages.value = []
    moderationTestResult.value = null
  }

  function removeModerationTestImage(index: number) {
    moderationTestImages.value.splice(index, 1)
  }

  async function handleModerationImageUpload(event: Event) {
    const input = event.target as HTMLInputElement
    await addModerationTestFiles(input.files)
    input.value = ''
  }

  async function handleModerationImageDrop(event: DragEvent) {
    await addModerationTestFiles(event.dataTransfer?.files ?? null)
  }

  async function handleModerationImagePaste(event: ClipboardEvent) {
    const files = Array.from(event.clipboardData?.files ?? []).filter((file) => file.type.startsWith('image/'))
    if (files.length === 0) return
    event.preventDefault()
    await addModerationTestFiles(files)
  }

  async function addModerationTestFiles(files: FileList | File[] | null) {
    if (!files) return
    const items = Array.from(files).filter((file) => file.type.startsWith('image/'))
    for (const file of items) {
      if (moderationTestImages.value.length >= maxModerationTestImages) {
        appStore.showError(t('admin.riskControl.auditTestImageLimit', { count: maxModerationTestImages }))
        return
      }
      if (file.size > maxModerationTestImageSize) {
        appStore.showError(t('admin.riskControl.auditTestImageTooLarge'))
        continue
      }
      try {
        moderationTestImages.value.push(await fileToDataURL(file))
      } catch {
        appStore.showError(t('admin.riskControl.auditTestImageReadFailed'))
      }
    }
  }

  function toggleGroup(groupID: number) {
    const index = configForm.group_ids.indexOf(groupID)
    if (index >= 0) {
      configForm.group_ids.splice(index, 1)
    } else {
      configForm.group_ids.push(groupID)
    }
  }

  function isGroupSelected(groupID: number): boolean {
    return configForm.group_ids.includes(groupID)
  }

  function resetRiskThresholds() {
    configForm.thresholds = { ...riskThresholdDefaults }
  }

  async function saveConfig() {
    saving.value = true
    try {
      const modelFilterPayload = buildModelFilterPayload(configForm.model_filter_type, configForm.model_filter_models)
      if (modelFilterPayload.type !== 'all' && modelFilterPayload.models.length === 0) {
        appStore.showError(t('admin.riskControl.modelFilterModelsRequired'))
        return
      }
      const payload: UpdateContentModerationConfig = {
        enabled: configForm.enabled,
        mode: configForm.mode,
        base_url: configForm.base_url,
        model: configForm.model,
        // 后端语义：0 清除代理（直连），>0 指定代理
        proxy_id: configForm.proxy_id ?? 0,
        timeout_ms: Number(configForm.timeout_ms) || 3000,
        retry_count: Number(configForm.retry_count) || 0,
        sample_rate: Number(configForm.sample_rate) || 0,
        all_groups: configForm.all_groups,
        group_ids: configForm.all_groups ? [] : [...configForm.group_ids],
        record_non_hits: configForm.record_non_hits,
        clear_api_key: configForm.clear_api_key,
        worker_count: Number(configForm.worker_count) || 4,
        queue_size: Number(configForm.queue_size) || 32768,
        block_status: Number(configForm.block_status) || 403,
        block_message: configForm.block_message || t('admin.riskControl.defaultBlockMessage'),
        email_on_hit: configForm.email_on_hit,
        auto_ban_enabled: configForm.auto_ban_enabled,
        cyber_policy_exclude_from_ban_count: configForm.cyber_policy_exclude_from_ban_count,
        ban_threshold: Number(configForm.ban_threshold) || 10,
        violation_window_hours: Number(configForm.violation_window_hours) || 720,
        hit_retention_days: Number(configForm.hit_retention_days) || 180,
        non_hit_retention_days: Math.min(Math.max(Number(configForm.non_hit_retention_days) || 3, 1), 3),
        pre_hash_check_enabled: configForm.pre_hash_check_enabled,
        thresholds: buildRiskThresholdPayload(configForm.thresholds),
        blocked_keywords: blockedKeywordList.value,
        keyword_blocking_mode: configForm.keyword_blocking_mode,
        model_filter: modelFilterPayload,
      }
      const keys = parseApiKeys(configForm.api_keys_text)
      if (!payload.clear_api_key && configForm.api_keys_mode === 'replace' && keys.length === 0) {
        appStore.showError(t('admin.riskControl.apiKeysReplaceNoInput'))
        return
      }
      if (keys.length > 0) {
        payload.api_keys = keys
        payload.api_keys_mode = configForm.api_keys_mode
        payload.clear_api_key = false
      }
      if (!payload.clear_api_key && configForm.api_keys_mode !== 'replace' && pendingDeleteApiKeyHashes.value.length > 0) {
        payload.delete_api_key_hashes = [...pendingDeleteApiKeyHashes.value]
      }

      const updated = await adminAPI.riskControl.updateConfig(payload)
      refreshers.applyConfig(updated)
      testedApiKeyStatuses.value = []
      apiKeyRowsExpanded.value = false
      refreshers.close()
      appStore.showSuccess(t('admin.riskControl.saved'))
      await Promise.all([refreshers.loadStatus(true), refreshers.loadLogs()])
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t('admin.riskControl.saveFailed')))
    } finally {
      saving.value = false
    }
  }

  return {
    saving,
    apiKeyTesting,
    activeSettingsTab,
    groupSearch,
    testedApiKeyStatuses,
    apiKeyRowsExpanded,
    moderationTestPrompt,
    moderationTestImages,
    moderationTestResult,
    settingsTabs,
    modeOptions,
    keywordBlockingModeOptions,
    modelFilterOptions,
    keywordNotice,
    filteredGroups,
    inputApiKeyCount,
    blockedKeywordList,
    blockedKeywordCount,
    pendingDeletedApiKeyCount,
    effectiveStoredApiKeyCount,
    apiKeysPlaceholder,
    apiKeysModeHint,
    hasModerationAuditInput,
    storedApiKeyTestButtonText,
    savedApiKeyRows,
    apiKeyRows,
    visibleApiKeyRows,
    hiddenApiKeyRowCount,
    canToggleApiKeyRows,
    moderationScoreRows,
    riskThresholdRows,
    modelFilterModelCount,
    modelFilterSummary,
    modeLabel,
    modeDescription,
    apiKeyStatusLabel,
    apiKeyStatusMeta,
    toggleClearApiKey,
    setAPIKeysMode,
    setModelFilterType,
    testApiKeys,
    toggleDeleteStoredApiKey,
    isStoredApiKeyPendingDelete,
    clearModerationTestInput,
    removeModerationTestImage,
    handleModerationImageUpload,
    handleModerationImageDrop,
    handleModerationImagePaste,
    toggleGroup,
    isGroupSelected,
    resetRiskThresholds,
    saveConfig,
  }
}
