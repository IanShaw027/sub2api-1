import type {
  ContentModerationAPIKeyStatus,
  ContentModerationLog,
  ContentModerationModelFilter,
  ContentModerationModelFilterType,
  KeywordBlockingMode,
  ModerationMode,
} from '@/api/admin/riskControl'
import { formatDateTime as formatDateTimeValue } from '@/utils/format'
import type { WorkerSlotState } from './types'

export const maxModerationTestImages = 1
export const maxModerationTestImageSize = 8 * 1024 * 1024
export const maxVisibleApiKeyRows = 3
export const blockedKeywordMax = 10000

/**
 * 风控运行模式的本地化标签，需要调用方传入 i18n 的 t()（保持本文件无 i18n 依赖）。
 */
export function moderationModeLabel(t: (key: string) => string, mode: ModerationMode): string {
  const labels: Record<ModerationMode, string> = {
    pre_block: t('admin.riskControl.modePreBlock'),
    observe: t('admin.riskControl.modeObserve'),
    off: t('admin.riskControl.modeOff'),
  }
  return labels[mode] ?? mode
}

type ModelFilterConfigLike = {
  model_filter_type: ContentModerationModelFilterType
  model_filter_models: string[]
}

/**
 * 模型过滤范围的派生展示信息：计数 / 摘要文案 / 悬浮提示。主视图与设置弹层各自基于
 * 同一份 configForm 独立计算，避免两个 composable 相互耦合。
 */
export function computeModelFilterModelCount(configForm: ModelFilterConfigLike): number {
  return configForm.model_filter_models.length
}

export function computeModelFilterSummary(
  t: (key: string, params?: Record<string, unknown>) => string,
  configForm: ModelFilterConfigLike,
): string {
  if (configForm.model_filter_type === 'include') {
    return t('admin.riskControl.modelFilterIncludeSummary', { count: computeModelFilterModelCount(configForm) })
  }
  if (configForm.model_filter_type === 'exclude') {
    return t('admin.riskControl.modelFilterExcludeSummary', { count: computeModelFilterModelCount(configForm) })
  }
  return t('admin.riskControl.modelFilterAllSummary')
}

export function computeModelFilterTooltip(
  t: (key: string, params?: Record<string, unknown>) => string,
  configForm: ModelFilterConfigLike,
): string {
  const preview = configForm.model_filter_models.slice(0, 6)
  const hidden = Math.max(0, configForm.model_filter_models.length - preview.length)
  const summary = computeModelFilterSummary(t, configForm)
  if (preview.length === 0) return summary
  const suffix = hidden > 0 ? ` +${hidden}` : ''
  return `${summary}: ${preview.join(', ')}${suffix}`
}

export const riskThresholdDefaults: Record<string, number> = {
  harassment: 98,
  'harassment/threatening': 90,
  hate: 65,
  'hate/threatening': 65,
  illicit: 95,
  'illicit/violent': 95,
  'self-harm': 65,
  'self-harm/intent': 85,
  'self-harm/instructions': 65,
  sexual: 65,
  'sexual/minors': 65,
  violence: 95,
  'violence/graphic': 95,
}

export const riskThresholdCategories = Object.keys(riskThresholdDefaults)

export function parseApiKeys(value: string): string[] {
  return value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter((item, index, arr) => item && arr.indexOf(item) === index)
}

export function parseBlockedKeywords(value: string): string[] {
  const seen = new Set<string>()
  const out: string[] = []
  for (const line of value.split(/\r?\n/)) {
    const kw = line.trim()
    if (!kw) continue
    const key = kw.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(kw)
  }
  return out
}

export function normalizeKeywordBlockingMode(value: unknown): KeywordBlockingMode {
  if (value === 'keyword_only' || value === 'api_only' || value === 'keyword_and_api') {
    return value
  }
  return 'keyword_and_api'
}

export function normalizeModelFilterType(value: unknown): ContentModerationModelFilterType {
  if (value === 'include' || value === 'exclude' || value === 'all') {
    return value
  }
  return 'all'
}

export function normalizeModelNames(models: unknown): string[] {
  if (!Array.isArray(models)) return []
  const seen = new Set<string>()
  const out: string[] = []
  for (const item of models) {
    const model = String(item ?? '').trim()
    if (!model) continue
    const key = model.toLowerCase()
    if (seen.has(key)) continue
    seen.add(key)
    out.push(model)
  }
  return out
}

export function normalizeModelFilter(value: unknown): ContentModerationModelFilter {
  if (!value || typeof value !== 'object') {
    return { type: 'all', models: [] }
  }
  const raw = value as Partial<ContentModerationModelFilter>
  const type = normalizeModelFilterType(raw.type)
  const models = type === 'all' ? [] : normalizeModelNames(raw.models)
  return { type, models }
}

export function clampPercent(value: unknown): number {
  const numeric = Number(value)
  if (!Number.isFinite(numeric)) {
    return 0
  }
  return Math.min(100, Math.max(0, numeric))
}

export function formatThresholdPercent(value: number): string {
  return `${clampPercent(value).toFixed(1)}%`
}

export function riskThresholdsFromConfig(thresholds: Record<string, number> | null | undefined): Record<string, number> {
  const out: Record<string, number> = { ...riskThresholdDefaults }
  for (const category of riskThresholdCategories) {
    const value = thresholds?.[category]
    if (Number.isFinite(value)) {
      out[category] = clampPercent(Number(value) * 100)
    }
  }
  return out
}

export function buildRiskThresholdPayload(thresholds: Record<string, number>): Record<string, number> {
  const payload: Record<string, number> = {}
  for (const category of riskThresholdCategories) {
    payload[category] = Number((clampPercent(thresholds[category]) / 100).toFixed(4))
  }
  return payload
}

export function buildModelFilterPayload(type: ContentModerationModelFilterType, models: string[]): ContentModerationModelFilter {
  const normalizedType = normalizeModelFilterType(type)
  if (normalizedType === 'all') {
    return { type: 'all', models: [] }
  }
  return {
    type: normalizedType,
    models: normalizeModelNames(models),
  }
}

export function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value)
}

export function formatDateTime(value: string): string {
  return formatDateTimeValue(value) || '-'
}

export function percent(value: number): string {
  if (!Number.isFinite(value)) return '-'
  return `${(value * 100).toFixed(1)}%`
}

export function percentWidth(value: number): string {
  if (!Number.isFinite(value)) return '0%'
  return `${Math.min(100, Math.max(0, value * 100)).toFixed(1)}%`
}

export function latencyText(value: number | null): string {
  if (value === null || value === undefined) return '-'
  return `${value} ms`
}

export function apiKeyRowKey(row: ContentModerationAPIKeyStatus, index: number): string {
  return `${row.configured ? 'saved' : 'test'}-${row.key_hash || index}`
}

export function apiKeyStatusDotClass(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const classes: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: 'bg-success-500',
    error: 'bg-warning-500',
    frozen: 'bg-danger-500',
    unknown: 'bg-surface-3',
  }
  return classes[statusValue] ?? classes.unknown
}

export function apiKeyStatusBadgeClass(statusValue: ContentModerationAPIKeyStatus['status']): string {
  const classes: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  ',
    error: 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text  ',
    frozen: 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text  ',
    unknown: 'bg-surface-2 text-muted  ',
  }
  return classes[statusValue] ?? classes.unknown
}

/**
 * 已保存 API Key 单条状态的本地化标签，供主视图的健康摘要与设置弹层的 Key 行状态徽标共用。
 */
export function apiKeyHealthStatusLabel(t: (key: string) => string, statusValue: ContentModerationAPIKeyStatus['status']): string {
  const labels: Record<ContentModerationAPIKeyStatus['status'], string> = {
    ok: t('admin.riskControl.apiKeyStatusOk'),
    error: t('admin.riskControl.apiKeyStatusError'),
    frozen: t('admin.riskControl.apiKeyStatusFrozen'),
    unknown: t('admin.riskControl.apiKeyStatusUnknown'),
  }
  return labels[statusValue] ?? labels.unknown
}

export function isStoredApiKeyPendingDelete(row: ContentModerationAPIKeyStatus, pendingDeleteApiKeyHashes: string[]): boolean {
  return row.configured && row.key_hash !== '' && pendingDeleteApiKeyHashes.includes(row.key_hash)
}

type ApiKeyHealthConfigLike = {
  api_key_configured: boolean
  api_key_count: number
}

/**
 * 已保存 API Key 的健康摘要文案（用于概览统计卡片）。依赖已保存 Key 状态列表与“待删除”标记，
 * 后者是页面级的临时状态（在设置弹层多次打开/关闭之间保持），由调用方传入同一份引用。
 */
export function computeApiKeyHealthSummary(
  t: (key: string, params?: Record<string, unknown>) => string,
  configForm: ApiKeyHealthConfigLike,
  savedApiKeyRows: ContentModerationAPIKeyStatus[],
  pendingDeleteApiKeyHashes: string[],
): string {
  if (!configForm.api_key_configured) return ''
  const activeSavedApiKeyRows = savedApiKeyRows.filter((row) => !isStoredApiKeyPendingDelete(row, pendingDeleteApiKeyHashes))
  const effectiveStoredApiKeyCount = Math.max(0, configForm.api_key_count - pendingDeleteApiKeyHashes.length)
  const counts: Record<ContentModerationAPIKeyStatus['status'], number> = {
    ok: 0,
    error: 0,
    frozen: 0,
    unknown: 0,
  }
  for (const row of activeSavedApiKeyRows) {
    counts[row.status] = (counts[row.status] ?? 0) + 1
  }
  if (activeSavedApiKeyRows.length === 0 && effectiveStoredApiKeyCount > 0) {
    counts.unknown = effectiveStoredApiKeyCount
  }
  const badges = (['ok', 'frozen', 'error', 'unknown'] as Array<ContentModerationAPIKeyStatus['status']>)
    .map((item) => ({ status: item, count: counts[item] }))
    .filter((item) => item.count > 0)
  if (badges.length === 0) return t('admin.riskControl.apiKeyStatusUnknown')
  return badges.map((badge) => `${apiKeyHealthStatusLabel(t, badge.status)} ${badge.count}`).join(' · ')
}

export function workerSlotClass(state: WorkerSlotState): string {
  if (state === 'active') {
    return 'border-accent-200 bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-accent   '
  }
  if (state === 'idle') {
    return 'border-[color-mix(in_oklch,var(--success)_35%,transparent)] bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text   '
  }
  return 'border-line bg-surface text-muted   '
}

export function workerDotClass(state: WorkerSlotState): string {
  if (state === 'active') return 'bg-accent-500'
  if (state === 'idle') return 'bg-success-500'
  return 'bg-surface-3 '
}

export function resultBadgeClass(row: ContentModerationLog): string {
  if (row.action === 'block' || row.action === 'keyword_block' || row.action === 'cyber_policy') return 'bg-[color-mix(in_oklch,var(--danger)_14%,transparent)] text-danger-text  '
  if (row.action === 'error' || row.error) return 'bg-[color-mix(in_oklch,var(--warning)_18%,transparent)] text-warning-text  '
  if (row.flagged) return 'bg-accent-500/15 text-accent-700  '
  return 'bg-[color-mix(in_oklch,var(--success)_16%,transparent)] text-success-text  '
}

export function inputSummaryText(row: ContentModerationLog): string {
  return row.input_excerpt || row.error || '-'
}

export function canUnbanRow(row: ContentModerationLog): boolean {
  return Boolean(row.auto_banned && row.user_id && row.user_status === 'disabled')
}

export function normalizeDateTimeLocal(value: string): string | undefined {
  if (!value) return undefined
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return undefined
  return date.toISOString()
}

export function fileToDataURL(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result || ''))
    reader.onerror = () => reject(reader.error)
    reader.readAsDataURL(file)
  })
}
