import { computed, type ComputedRef, type Ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { ContentModerationAPIKeyLoad, ContentModerationRuntimeStatus, ModerationMode } from '@/api/admin/riskControl'
import type { ConfigFormState, OverviewItem, WorkerSlotState } from './types'
import { formatNumber } from './riskControlUtils'

type OverviewDeps = {
  apiKeyHealthSummary: ComputedRef<string>
  modelFilterSummary: ComputedRef<string>
  selectedGroupCount: ComputedRef<string>
  modeLabel: (mode: ModerationMode) => string
}

/**
 * 概览统计卡片、预拦截运行时卡片与 Worker 运行时卡片所需的派生数据。
 */
export function useRiskControlOverview(
  configForm: ConfigFormState,
  status: Ref<ContentModerationRuntimeStatus | null>,
  pagination: { total: number },
  deps: OverviewDeps,
) {
  const { t } = useI18n()

  const runtimeBadgeText = computed(() => {
    if (!status.value?.risk_control_enabled) return t('admin.riskControl.riskSwitchOff')
    if (!configForm.enabled || configForm.mode === 'off') return t('admin.riskControl.overview.disabled')
    return t('admin.riskControl.overview.enabled')
  })

  const runtimeBadgeClass = computed(() => {
    if (!status.value?.risk_control_enabled || !configForm.enabled || configForm.mode === 'off') {
      return 'bg-surface-2 text-muted  '
    }
    return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '
  })

  const overviewItems = computed<OverviewItem[]>(() => [
    {
      key: 'status',
      label: t('admin.riskControl.overview.status'),
      value: configForm.enabled ? t('admin.riskControl.overview.enabled') : t('admin.riskControl.overview.disabled'),
      meta: deps.modeLabel(configForm.mode),
      icon: 'shield',
      iconClass: configForm.enabled
        ? 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '
        : 'bg-surface-2 text-muted  ',
      badge: runtimeBadgeText.value,
      badgeClass: runtimeBadgeClass.value,
    },
    {
      key: 'api-key',
      label: t('admin.riskControl.overview.apiKey'),
      value: configForm.api_key_configured ? t('admin.riskControl.apiKeyCount', { count: configForm.api_key_count }) : t('admin.riskControl.notConfigured'),
      meta: configForm.api_key_configured ? deps.apiKeyHealthSummary.value || configForm.model || '-' : configForm.model || '-',
      icon: 'key',
      iconClass: 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent-600  ',
    },
    {
      key: 'scope',
      label: t('admin.riskControl.overview.groupScope'),
      value: configForm.all_groups ? t('admin.riskControl.allGroups') : deps.selectedGroupCount.value,
      meta: deps.modelFilterSummary.value,
      icon: 'users',
      iconClass: 'bg-accent-500/15 text-accent-600  ',
    },
    {
      key: 'logs',
      label: t('admin.riskControl.overview.logs'),
      value: formatNumber(pagination.total),
      meta: t('admin.riskControl.overview.currentFilter'),
      icon: 'document',
      iconClass: 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text  ',
    },
  ])

  const queueUsagePercent = computed(() => `${Math.min(100, Math.max(0, status.value?.queue_usage_percent ?? 0)).toFixed(1)}%`)

  const queueUsageStyle = computed(() => ({
    width: queueUsagePercent.value,
  }))

  const runtimeMode = computed<ModerationMode>(() => status.value?.mode ?? configForm.mode)

  const showPreBlockRuntimeCard = computed(() => runtimeMode.value === 'pre_block')

  const showWorkerRuntimeCard = computed(() => runtimeMode.value === 'observe')

  const preBlockMetricItems = computed(() => [
    {
      key: 'active',
      label: t('admin.riskControl.preBlockActive'),
      value: formatNumber(status.value?.pre_block_active ?? 0),
      meta: t('admin.riskControl.preBlockActiveHint'),
      class: 'bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] ',
      valueClass: 'text-accent ',
    },
    {
      key: 'checked',
      label: t('admin.riskControl.preBlockChecked'),
      value: formatNumber(status.value?.pre_block_checked ?? 0),
      meta: t('admin.riskControl.preBlockCheckedHint'),
      class: 'bg-surface-2 ',
      valueClass: 'text-foreground ',
    },
    {
      key: 'allowed',
      label: t('admin.riskControl.preBlockAllowed'),
      value: formatNumber(status.value?.pre_block_allowed ?? 0),
      meta: t('admin.riskControl.preBlockAllowedHint'),
      class: 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] ',
      valueClass: 'text-success-text ',
    },
    {
      key: 'blocked',
      label: t('admin.riskControl.preBlockBlocked'),
      value: formatNumber(status.value?.pre_block_blocked ?? 0),
      meta: t('admin.riskControl.preBlockBlockedHint'),
      class: 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] ',
      valueClass: 'text-danger-text ',
    },
    {
      key: 'errors',
      label: t('admin.riskControl.preBlockErrors'),
      value: formatNumber(status.value?.pre_block_errors ?? 0),
      meta: t('admin.riskControl.preBlockErrorsHint'),
      class: 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] ',
      valueClass: 'text-warning-text ',
    },
    {
      key: 'latency',
      label: t('admin.riskControl.preBlockAvgLatency'),
      value: `${formatNumber(status.value?.pre_block_avg_latency_ms ?? 0)} ms`,
      meta: t('admin.riskControl.preBlockAvgLatencyHint'),
      class: 'bg-accent-500/15 ',
      valueClass: 'text-accent-700 ',
    },
  ])

  const preBlockAPIKeyLoads = computed<ContentModerationAPIKeyLoad[]>(() => (
    [...(status.value?.pre_block_api_key_loads ?? [])].sort((a, b) => a.index - b.index)
  ))

  const preBlockAPIKeyMaxTotal = computed(() => Math.max(1, ...preBlockAPIKeyLoads.value.map((item) => item.total || 0)))

  const preBlockAPIKeyLoadSummaryText = computed(() => t('admin.riskControl.preBlockAPIKeyLoadSummary', {
    active: formatNumber(status.value?.pre_block_api_key_active ?? 0),
    available: formatNumber(status.value?.pre_block_api_key_available_count ?? 0),
    total: formatNumber(status.value?.pre_block_api_key_total_calls ?? 0),
    workerActive: formatNumber(status.value?.active_workers ?? 0),
    workerTotal: formatNumber(status.value?.worker_count ?? configForm.worker_count),
  }))

  function preBlockAPIKeyLoadWidth(total: number): string {
    return `${Math.min(100, Math.max(0, (total / preBlockAPIKeyMaxTotal.value) * 100)).toFixed(1)}%`
  }

  const workerSlots = computed(() => {
    const total = Math.max(0, status.value?.worker_count ?? configForm.worker_count)
    const active = Math.max(0, status.value?.active_workers ?? 0)
    const enabled = Boolean(status.value?.risk_control_enabled && status.value?.enabled && status.value?.mode !== 'off')
    return Array.from({ length: total }, (_, index) => ({
      id: index + 1,
      state: (!enabled ? 'disabled' : index < active ? 'active' : 'idle') as WorkerSlotState,
      label: !enabled
        ? t('admin.riskControl.workerDisabled')
        : index < active
          ? t('admin.riskControl.workerActive')
          : t('admin.riskControl.workerIdle'),
    }))
  })

  return {
    overviewItems,
    runtimeBadgeText,
    runtimeBadgeClass,
    queueUsagePercent,
    queueUsageStyle,
    runtimeMode,
    showPreBlockRuntimeCard,
    showWorkerRuntimeCard,
    preBlockMetricItems,
    preBlockAPIKeyLoads,
    preBlockAPIKeyLoadSummaryText,
    preBlockAPIKeyLoadWidth,
    workerSlots,
  }
}
