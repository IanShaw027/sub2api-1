import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'

export type SkillReviewStatus = 'pending' | 'approved' | 'rejected'
export type SkillVisibility = 'public' | 'private' | 'force_private'
export type SkillRiskLevel = 'low' | 'medium' | 'high'
export type SkillGovernanceStatus = 'online' | 'disabled' | 'force_private' | 'draft'
export type SkillRuntimeHealth = 'healthy' | 'warning' | 'critical'
export type SkillSettlementStatus = 'pending' | 'ready' | 'settled' | 'frozen' | 'rejected'
export type SkillAdminAction = 'approve' | 'reject' | 'disable' | 'force-private'

export interface SkillReviewFilters {
  search?: string
  review_status?: SkillReviewStatus | 'all'
  risk_level?: SkillRiskLevel | 'all'
  visibility?: SkillVisibility | 'all'
}

export interface SkillGovernanceFilters {
  search?: string
  governance_status?: SkillGovernanceStatus | 'all'
  review_status?: SkillReviewStatus | 'all'
  visibility?: SkillVisibility | 'all'
}

export interface SkillRuntimeFilters {
  search?: string
  health_status?: SkillRuntimeHealth | 'all'
}

export interface SkillSettlementFilters {
  search?: string
  settlement_status?: SkillSettlementStatus | 'all'
}

export interface SkillActionPayload {
  note?: string
  reason?: string
}

export interface SkillActionReceipt {
  action: SkillAdminAction
  message: string
  status: string
  operated_at: string
}

export interface SkillReviewItem {
  id: number
  skill_id: number
  skill_name: string
  skill_slug: string
  version_id: number
  version_name: string
  latest_published_version: string | null
  review_status: SkillReviewStatus
  visibility: SkillVisibility
  risk_level: SkillRiskLevel
  category: string | null
  author_name: string | null
  summary: string | null
  changelog: string | null
  review_note: string | null
  rejection_reason: string | null
  reviewer_name: string | null
  tags: string[]
  requests_24h: number
  revenue_30d: number
  submitted_at: string
  reviewed_at: string | null
  updated_at: string
}

export interface SkillReviewSummary {
  pending_count: number
  approved_count: number
  rejected_count: number
  high_risk_count: number
}

export interface SkillReviewListResponse extends BasePaginationResponse<SkillReviewItem> {
  summary: SkillReviewSummary
}

export interface SkillGovernanceItem {
  id: number
  skill_id: number
  skill_name: string
  skill_slug: string
  current_version: string
  latest_published_version: string | null
  governance_status: SkillGovernanceStatus
  latest_review_status: SkillReviewStatus
  visibility: SkillVisibility
  category: string | null
  author_name: string | null
  review_note: string | null
  tags: string[]
  requests_24h: number
  success_rate: number
  revenue_30d: number
  created_at: string
  updated_at: string
}

export interface SkillGovernanceSummary {
  total_count: number
  online_count: number
  force_private_count: number
  disabled_count: number
  pending_versions_count: number
}

export interface SkillGovernanceListResponse extends BasePaginationResponse<SkillGovernanceItem> {
  summary: SkillGovernanceSummary
}

export interface SkillRuntimeItem {
  id: number
  skill_id: number
  skill_name: string
  skill_slug: string
  current_version: string
  health_status: SkillRuntimeHealth
  requests_24h: number
  success_rate: number
  avg_latency_ms: number
  p95_latency_ms: number
  error_rate: number
  queue_depth: number
  last_error: string | null
  last_run_at: string | null
  last_alert_at: string | null
}

export interface SkillRuntimeSummary {
  total_skills: number
  active_skills: number
  requests_24h: number
  success_rate: number
  p95_latency_ms: number
  warning_count: number
  critical_count: number
}

export interface SkillRuntimeEvent {
  id: number
  skill_id: number | null
  skill_name: string | null
  level: 'info' | 'warning' | 'critical'
  message: string
  metric_name: string | null
  metric_value: number | null
  created_at: string
}

export interface SkillRuntimeOverviewResponse extends BasePaginationResponse<SkillRuntimeItem> {
  summary: SkillRuntimeSummary
  events: SkillRuntimeEvent[]
}

export interface SkillSettlementItem {
  id: number
  skill_id: number
  skill_name: string
  skill_slug: string
  author_name: string | null
  period_label: string
  settlement_status: SkillSettlementStatus
  gross_amount: number
  platform_fee_amount: number
  payout_amount: number
  frozen_amount: number
  currency: string
  note: string | null
  created_at: string
  updated_at: string
}

export interface SkillSettlementSummary {
  pending_amount: number
  settled_amount: number
  frozen_amount: number
  pending_skill_count: number
  currency: string
}

export interface SkillSettlementListResponse extends BasePaginationResponse<SkillSettlementItem> {
  summary: SkillSettlementSummary
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function asString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

function asNullableString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value : null
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function asNullableNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0) : []
}

function asPercent(value: unknown, fallback = 0): number {
  const numeric = asNumber(value, fallback)
  if (numeric > 0 && numeric <= 1) {
    return Number((numeric * 100).toFixed(2))
  }
  return numeric
}

function pickFirstValue(source: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    if (key in source && source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

function resolveNestedRecord(source: unknown, keys: string[]): Record<string, unknown> {
  if (!isRecord(source)) return {}
  for (const key of keys) {
    if (isRecord(source[key])) {
      return source[key]
    }
  }
  return {}
}

function resolveItemsArray(raw: unknown): unknown[] {
  if (Array.isArray(raw)) return raw
  if (!isRecord(raw)) return []

  for (const key of ['items', 'list', 'records', 'rows', 'data']) {
    if (Array.isArray(raw[key])) {
      return raw[key] as unknown[]
    }
  }

  const dataRecord = resolveNestedRecord(raw, ['data', 'result'])
  for (const key of ['items', 'list', 'records', 'rows']) {
    if (Array.isArray(dataRecord[key])) {
      return dataRecord[key] as unknown[]
    }
  }

  return []
}

function normalizePagedResponse<T>(
  raw: unknown,
  page: number,
  pageSize: number,
  mapper: (item: unknown) => T
): BasePaginationResponse<T> {
  const items = resolveItemsArray(raw)
  if (Array.isArray(raw)) {
    const total = items.length
    const safePageSize = Math.max(1, pageSize)
    const pages = Math.max(1, Math.ceil(total / safePageSize))
    const safePage = Math.min(Math.max(1, page), pages)
    const start = (safePage - 1) * safePageSize
    return {
      items: items.slice(start, start + safePageSize).map(mapper),
      total,
      page: safePage,
      page_size: safePageSize,
      pages
    }
  }

  const source = isRecord(raw) ? raw : {}
  const dataRecord = resolveNestedRecord(raw, ['data', 'result'])
  const metaSource = Object.keys(dataRecord).length > 0 ? dataRecord : source
  return {
    items: items.map(mapper),
    total: asNumber(pickFirstValue(metaSource, ['total', 'count']), items.length),
    page: asNumber(pickFirstValue(metaSource, ['page', 'current_page']), page),
    page_size: asNumber(pickFirstValue(metaSource, ['page_size', 'pageSize', 'per_page']), pageSize),
    pages: asNumber(
      pickFirstValue(metaSource, ['pages', 'total_pages']),
      Math.max(1, Math.ceil(items.length / Math.max(1, pageSize)))
    )
  }
}

function normalizeReviewStatus(value: unknown): SkillReviewStatus {
  const normalized = asString(value).trim().toLowerCase()
  if (['approved', 'passed', 'published', 'released', 'live'].includes(normalized)) return 'approved'
  if (['rejected', 'declined', 'failed'].includes(normalized)) return 'rejected'
  return 'pending'
}

function normalizeVisibility(value: unknown): SkillVisibility {
  const normalized = asString(value).trim().toLowerCase()
  if (['force_private', 'forced_private', 'private_locked'].includes(normalized)) return 'force_private'
  if (normalized === 'public') return 'public'
  return 'private'
}

function normalizeRiskLevel(value: unknown): SkillRiskLevel {
  const normalized = asString(value).trim().toLowerCase()
  if (normalized === 'high') return 'high'
  if (normalized === 'medium') return 'medium'
  return 'low'
}

function normalizeGovernanceStatus(value: unknown): SkillGovernanceStatus {
  const normalized = asString(value).trim().toLowerCase()
  if (['disabled', 'offline', 'blocked', 'banned'].includes(normalized)) return 'disabled'
  if (['force_private', 'forced_private', 'private_locked'].includes(normalized)) return 'force_private'
  if (['draft', 'pending', 'review'].includes(normalized)) return 'draft'
  return 'online'
}

function normalizeRuntimeHealth(value: unknown): SkillRuntimeHealth {
  const normalized = asString(value).trim().toLowerCase()
  if (['critical', 'error', 'outage'].includes(normalized)) return 'critical'
  if (['warning', 'degraded', 'slow'].includes(normalized)) return 'warning'
  return 'healthy'
}

function normalizeSettlementStatus(value: unknown): SkillSettlementStatus {
  const normalized = asString(value).trim().toLowerCase()
  if (['ready', 'audited', 'approved'].includes(normalized)) return 'ready'
  if (['settled', 'paid', 'completed'].includes(normalized)) return 'settled'
  if (['frozen', 'hold', 'held'].includes(normalized)) return 'frozen'
  if (['rejected', 'disputed'].includes(normalized)) return 'rejected'
  return 'pending'
}

function resolveOptionalAmount(source: Record<string, unknown>, amountKeys: string[], centsKeys: string[]): number | null {
  const directAmount = asNullableNumber(pickFirstValue(source, amountKeys))
  if (directAmount !== null) return directAmount

  const centsAmount = asNullableNumber(pickFirstValue(source, centsKeys))
  return centsAmount === null ? null : Number((centsAmount / 100).toFixed(2))
}

function resolveAmount(source: Record<string, unknown>, amountKeys: string[], centsKeys: string[]): number {
  return resolveOptionalAmount(source, amountKeys, centsKeys) ?? 0
}

function normalizeReviewItem(raw: unknown): SkillReviewItem {
  const source = isRecord(raw) ? raw : {}
  const metadata = resolveNestedRecord(source, ['metadata'])
  const skillID = asNumber(pickFirstValue(source, ['skill_id', 'entity_id', 'id']), Date.now())
  return {
    id: asNumber(pickFirstValue(source, ['review_id', 'id', 'version_id']), Date.now()),
    skill_id: skillID,
    skill_name: asString(pickFirstValue(source, ['skill_name', 'name', 'title']), '未命名技能'),
    skill_slug: asString(pickFirstValue(source, ['skill_slug', 'slug']), `skill-${skillID}`),
    version_id: asNumber(pickFirstValue(source, ['version_id', 'release_id', 'revision_id']), skillID),
    version_name: asString(pickFirstValue(source, ['version_name', 'version', 'semantic_version']), 'v0.0.0-draft'),
    latest_published_version: asNullableString(pickFirstValue(source, ['latest_published_version', 'latest_version'])),
    review_status: normalizeReviewStatus(pickFirstValue(source, ['review_status', 'status', 'moderation_status'])),
    visibility: normalizeVisibility(pickFirstValue(source, ['visibility', 'moderation_visibility'])),
    risk_level: normalizeRiskLevel(pickFirstValue(source, ['risk_level', 'risk'])),
    category: asNullableString(pickFirstValue(source, ['category', 'category_name'])),
    author_name: asNullableString(pickFirstValue(source, ['author_name', 'owner_name', 'publisher_name'])),
    summary: asNullableString(pickFirstValue({ ...metadata, ...source }, ['summary', 'description'])),
    changelog: asNullableString(pickFirstValue({ ...metadata, ...source }, ['changelog', 'release_notes'])),
    review_note: asNullableString(pickFirstValue(source, ['review_note', 'moderation_note'])),
    rejection_reason: asNullableString(pickFirstValue(source, ['rejection_reason', 'reject_reason'])),
    reviewer_name: asNullableString(pickFirstValue(source, ['reviewer_name', 'reviewed_by'])),
    tags: asStringArray(pickFirstValue({ ...metadata, ...source }, ['tags', 'labels'])),
    requests_24h: asNumber(pickFirstValue(source, ['requests_24h', 'calls_24h', 'run_count_24h']), 0),
    revenue_30d: resolveAmount(source, ['revenue_30d', 'revenue_amount_30d'], ['revenue_30d_cents', 'revenue_amount_30d_cents']),
    submitted_at: asString(pickFirstValue(source, ['submitted_at', 'created_at', 'requested_at']), new Date().toISOString()),
    reviewed_at: asNullableString(pickFirstValue(source, ['reviewed_at', 'decided_at'])),
    updated_at: asString(pickFirstValue(source, ['updated_at', 'submitted_at', 'created_at']), new Date().toISOString())
  }
}

function normalizeReviewSummary(raw: unknown, items: SkillReviewItem[]): SkillReviewSummary {
  const source = resolveNestedRecord(raw, ['summary', 'stats', 'overview'])
  return {
    pending_count: asNumber(source.pending_count, items.filter((item) => item.review_status === 'pending').length),
    approved_count: asNumber(source.approved_count, items.filter((item) => item.review_status === 'approved').length),
    rejected_count: asNumber(source.rejected_count, items.filter((item) => item.review_status === 'rejected').length),
    high_risk_count: asNumber(source.high_risk_count, items.filter((item) => item.risk_level === 'high').length)
  }
}

function normalizeGovernanceItem(raw: unknown): SkillGovernanceItem {
  const source = isRecord(raw) ? raw : {}
  const metadata = resolveNestedRecord(source, ['metadata'])
  const skillID = asNumber(pickFirstValue(source, ['skill_id', 'id']), Date.now())
  return {
    id: asNumber(pickFirstValue(source, ['id', 'skill_id']), skillID),
    skill_id: skillID,
    skill_name: asString(pickFirstValue(source, ['skill_name', 'name', 'title']), '未命名技能'),
    skill_slug: asString(pickFirstValue(source, ['skill_slug', 'slug']), `skill-${skillID}`),
    current_version: asString(pickFirstValue(source, ['current_version', 'version']), 'v0.0.0-draft'),
    latest_published_version: asNullableString(pickFirstValue(source, ['latest_published_version', 'published_version'])),
    governance_status: normalizeGovernanceStatus(pickFirstValue(source, ['governance_status', 'status', 'moderation_status'])),
    latest_review_status: normalizeReviewStatus(pickFirstValue(source, ['latest_review_status', 'review_status'])),
    visibility: normalizeVisibility(pickFirstValue(source, ['visibility', 'moderation_visibility'])),
    category: asNullableString(pickFirstValue(source, ['category', 'category_name'])),
    author_name: asNullableString(pickFirstValue(source, ['author_name', 'owner_name', 'publisher_name'])),
    review_note: asNullableString(pickFirstValue(source, ['review_note', 'moderation_note'])),
    tags: asStringArray(pickFirstValue({ ...metadata, ...source }, ['tags', 'labels'])),
    requests_24h: asNumber(pickFirstValue(source, ['requests_24h', 'calls_24h', 'run_count_24h']), 0),
    success_rate: asPercent(pickFirstValue(source, ['success_rate']), 0),
    revenue_30d: resolveAmount(source, ['revenue_30d', 'revenue_amount_30d'], ['revenue_30d_cents', 'revenue_amount_30d_cents']),
    created_at: asString(pickFirstValue(source, ['created_at']), new Date().toISOString()),
    updated_at: asString(pickFirstValue(source, ['updated_at', 'created_at']), new Date().toISOString())
  }
}

function normalizeGovernanceSummary(raw: unknown, items: SkillGovernanceItem[]): SkillGovernanceSummary {
  const source = resolveNestedRecord(raw, ['summary', 'stats', 'overview'])
  return {
    total_count: asNumber(source.total_count, items.length),
    online_count: asNumber(source.online_count, items.filter((item) => item.governance_status === 'online').length),
    force_private_count: asNumber(
      source.force_private_count,
      items.filter((item) => item.governance_status === 'force_private').length
    ),
    disabled_count: asNumber(source.disabled_count, items.filter((item) => item.governance_status === 'disabled').length),
    pending_versions_count: asNumber(
      source.pending_versions_count,
      items.filter((item) => item.latest_review_status === 'pending').length
    )
  }
}

function normalizeRuntimeItem(raw: unknown): SkillRuntimeItem {
  const source = isRecord(raw) ? raw : {}
  const skillID = asNumber(pickFirstValue(source, ['skill_id', 'id']), Date.now())
  return {
    id: asNumber(pickFirstValue(source, ['id', 'skill_id']), skillID),
    skill_id: skillID,
    skill_name: asString(pickFirstValue(source, ['skill_name', 'name', 'title']), '未命名技能'),
    skill_slug: asString(pickFirstValue(source, ['skill_slug', 'slug']), `skill-${skillID}`),
    current_version: asString(pickFirstValue(source, ['current_version', 'version']), 'v0.0.0'),
    health_status: normalizeRuntimeHealth(pickFirstValue(source, ['health_status', 'status'])),
    requests_24h: asNumber(pickFirstValue(source, ['requests_24h', 'calls_24h', 'run_count_24h']), 0),
    success_rate: asPercent(pickFirstValue(source, ['success_rate']), 0),
    avg_latency_ms: asNumber(pickFirstValue(source, ['avg_latency_ms', 'latency_ms']), 0),
    p95_latency_ms: asNumber(pickFirstValue(source, ['p95_latency_ms', 'latency_p95_ms']), 0),
    error_rate: asPercent(pickFirstValue(source, ['error_rate']), 0),
    queue_depth: asNumber(pickFirstValue(source, ['queue_depth', 'pending_jobs']), 0),
    last_error: asNullableString(pickFirstValue(source, ['last_error', 'last_error_message'])),
    last_run_at: asNullableString(pickFirstValue(source, ['last_run_at', 'last_request_at'])),
    last_alert_at: asNullableString(pickFirstValue(source, ['last_alert_at', 'last_warning_at']))
  }
}

function normalizeRuntimeSummary(raw: unknown, items: SkillRuntimeItem[]): SkillRuntimeSummary {
  const source = resolveNestedRecord(raw, ['summary', 'stats', 'overview'])
  return {
    total_skills: asNumber(source.total_skills, items.length),
    active_skills: asNumber(source.active_skills, items.filter((item) => item.requests_24h > 0).length),
    requests_24h: asNumber(source.requests_24h, items.reduce((sum, item) => sum + item.requests_24h, 0)),
    success_rate: asPercent(
      source.success_rate,
      items.length ? items.reduce((sum, item) => sum + item.success_rate, 0) / items.length : 0
    ),
    p95_latency_ms: asNumber(
      source.p95_latency_ms,
      items.length ? Math.max(...items.map((item) => item.p95_latency_ms)) : 0
    ),
    warning_count: asNumber(source.warning_count, items.filter((item) => item.health_status === 'warning').length),
    critical_count: asNumber(source.critical_count, items.filter((item) => item.health_status === 'critical').length)
  }
}

function normalizeRuntimeEvent(raw: unknown): SkillRuntimeEvent {
  const source = isRecord(raw) ? raw : {}
  const level = asString(pickFirstValue(source, ['level', 'severity']), 'info').trim().toLowerCase()
  return {
    id: asNumber(pickFirstValue(source, ['id']), Date.now()),
    skill_id: asNullableNumber(pickFirstValue(source, ['skill_id'])),
    skill_name: asNullableString(pickFirstValue(source, ['skill_name', 'name'])),
    level: level === 'critical' ? 'critical' : level === 'warning' ? 'warning' : 'info',
    message: asString(pickFirstValue(source, ['message', 'description']), '暂无事件说明'),
    metric_name: asNullableString(pickFirstValue(source, ['metric_name', 'metric'])),
    metric_value: asNullableNumber(pickFirstValue(source, ['metric_value', 'value'])),
    created_at: asString(pickFirstValue(source, ['created_at', 'timestamp']), new Date().toISOString())
  }
}

function normalizeSettlementItem(raw: unknown): SkillSettlementItem {
  const source = isRecord(raw) ? raw : {}
  const skillID = asNumber(pickFirstValue(source, ['skill_id', 'id']), Date.now())
  return {
    id: asNumber(pickFirstValue(source, ['id', 'settlement_id']), Date.now()),
    skill_id: skillID,
    skill_name: asString(pickFirstValue(source, ['skill_name', 'name', 'title']), '未命名技能'),
    skill_slug: asString(pickFirstValue(source, ['skill_slug', 'slug']), `skill-${skillID}`),
    author_name: asNullableString(pickFirstValue(source, ['author_name', 'owner_name', 'publisher_name'])),
    period_label: asString(pickFirstValue(source, ['period_label', 'settlement_period', 'billing_cycle']), '当前周期'),
    settlement_status: normalizeSettlementStatus(pickFirstValue(source, ['settlement_status', 'status'])),
    gross_amount: resolveAmount(source, ['gross_amount', 'gross_income'], ['gross_amount_cents', 'gross_income_cents']),
    platform_fee_amount: resolveAmount(
      source,
      ['platform_fee_amount', 'platform_fee'],
      ['platform_fee_amount_cents', 'platform_fee_cents']
    ),
    payout_amount: resolveAmount(source, ['payout_amount', 'net_amount'], ['payout_amount_cents', 'net_amount_cents']),
    frozen_amount: resolveAmount(source, ['frozen_amount'], ['frozen_amount_cents']),
    currency: asString(pickFirstValue(source, ['currency']), 'CNY'),
    note: asNullableString(pickFirstValue(source, ['note', 'memo'])),
    created_at: asString(pickFirstValue(source, ['created_at']), new Date().toISOString()),
    updated_at: asString(pickFirstValue(source, ['updated_at', 'created_at']), new Date().toISOString())
  }
}

function normalizeSettlementSummary(raw: unknown, items: SkillSettlementItem[]): SkillSettlementSummary {
  const source = resolveNestedRecord(raw, ['summary', 'stats', 'overview'])
  const pendingFallback = items
    .filter((item) => item.settlement_status === 'pending' || item.settlement_status === 'ready')
    .reduce((sum, item) => sum + item.payout_amount, 0)
  const settledFallback = items
    .filter((item) => item.settlement_status === 'settled')
    .reduce((sum, item) => sum + item.payout_amount, 0)
  const frozenFallback = items.reduce((sum, item) => sum + item.frozen_amount, 0)
  return {
    pending_amount: resolveOptionalAmount(source, ['pending_amount'], ['pending_amount_cents']) ?? pendingFallback,
    settled_amount: resolveOptionalAmount(source, ['settled_amount'], ['settled_amount_cents']) ?? settledFallback,
    frozen_amount: resolveOptionalAmount(source, ['frozen_amount'], ['frozen_amount_cents']) ?? frozenFallback,
    pending_skill_count: asNumber(
      source.pending_skill_count,
      items.filter((item) => item.settlement_status === 'pending' || item.settlement_status === 'ready').length
    ),
    currency: asString(pickFirstValue(source, ['currency']), items[0]?.currency || 'CNY')
  }
}

function normalizeActionReceipt(raw: unknown, action: SkillAdminAction): SkillActionReceipt {
  const source = isRecord(raw) ? raw : {}
  return {
    action,
    message: asString(pickFirstValue(source, ['message']), '操作已提交'),
    status: asString(pickFirstValue(source, ['status', 'review_status', 'governance_status']), 'accepted'),
    operated_at: asString(pickFirstValue(source, ['operated_at', 'updated_at']), new Date().toISOString())
  }
}

export async function listReviews(
  page = 1,
  pageSize = 20,
  filters?: SkillReviewFilters,
  options?: { signal?: AbortSignal }
): Promise<SkillReviewListResponse> {
  const { data } = await apiClient.get('/admin/skills/reviews', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.review_status && filters.review_status !== 'all' ? { review_status: filters.review_status } : {}),
      ...(filters?.risk_level && filters.risk_level !== 'all' ? { risk_level: filters.risk_level } : {}),
      ...(filters?.visibility && filters.visibility !== 'all' ? { visibility: filters.visibility } : {})
    },
    signal: options?.signal
  })

  const paged = normalizePagedResponse(data, page, pageSize, normalizeReviewItem)
  return {
    ...paged,
    summary: normalizeReviewSummary(data, paged.items)
  }
}

export async function approveReview(reviewID: number, payload?: SkillActionPayload): Promise<SkillActionReceipt> {
  const { data } = await apiClient.post(`/admin/skills/reviews/${reviewID}/approve`, payload ?? {})
  return normalizeActionReceipt(data, 'approve')
}

export async function rejectReview(reviewID: number, payload?: SkillActionPayload): Promise<SkillActionReceipt> {
  const { data } = await apiClient.post(`/admin/skills/reviews/${reviewID}/reject`, payload ?? {})
  return normalizeActionReceipt(data, 'reject')
}

export async function listGovernanceSkills(
  page = 1,
  pageSize = 20,
  filters?: SkillGovernanceFilters,
  options?: { signal?: AbortSignal }
): Promise<SkillGovernanceListResponse> {
  const { data } = await apiClient.get('/admin/skills/governance', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.governance_status && filters.governance_status !== 'all'
        ? { governance_status: filters.governance_status }
        : {}),
      ...(filters?.review_status && filters.review_status !== 'all' ? { review_status: filters.review_status } : {}),
      ...(filters?.visibility && filters.visibility !== 'all' ? { visibility: filters.visibility } : {})
    },
    signal: options?.signal
  })

  const paged = normalizePagedResponse(data, page, pageSize, normalizeGovernanceItem)
  return {
    ...paged,
    summary: normalizeGovernanceSummary(data, paged.items)
  }
}

export async function disableSkill(skillID: number, payload?: SkillActionPayload): Promise<SkillActionReceipt> {
  const { data } = await apiClient.post(`/admin/skills/${skillID}/disable`, payload ?? {})
  return normalizeActionReceipt(data, 'disable')
}

export async function forceSkillPrivate(skillID: number, payload?: SkillActionPayload): Promise<SkillActionReceipt> {
  const { data } = await apiClient.post(`/admin/skills/${skillID}/force-private`, payload ?? {})
  return normalizeActionReceipt(data, 'force-private')
}

export async function getRuntimeOverview(
  page = 1,
  pageSize = 20,
  filters?: SkillRuntimeFilters,
  options?: { signal?: AbortSignal }
): Promise<SkillRuntimeOverviewResponse> {
  const { data } = await apiClient.get('/admin/skills/runtime', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.health_status && filters.health_status !== 'all' ? { health_status: filters.health_status } : {})
    },
    signal: options?.signal
  })

  const paged = normalizePagedResponse(data, page, pageSize, normalizeRuntimeItem)
  const source = isRecord(data) ? data : {}
  const eventsSource = resolveNestedRecord(data, ['events', 'recent_events'])
  const eventsArray = Array.isArray(source.events)
    ? source.events
    : Array.isArray(source.recent_events)
      ? source.recent_events
      : Array.isArray(eventsSource.items)
        ? eventsSource.items
        : []

  return {
    ...paged,
    summary: normalizeRuntimeSummary(data, paged.items),
    events: eventsArray.map(normalizeRuntimeEvent)
  }
}

export async function listSettlements(
  page = 1,
  pageSize = 20,
  filters?: SkillSettlementFilters,
  options?: { signal?: AbortSignal }
): Promise<SkillSettlementListResponse> {
  const { data } = await apiClient.get('/admin/skills/settlements', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.settlement_status && filters.settlement_status !== 'all'
        ? { settlement_status: filters.settlement_status }
        : {})
    },
    signal: options?.signal
  })

  const paged = normalizePagedResponse(data, page, pageSize, normalizeSettlementItem)
  return {
    ...paged,
    summary: normalizeSettlementSummary(data, paged.items)
  }
}

const adminSkillsAPI = {
  listReviews,
  approveReview,
  rejectReview,
  listGovernanceSkills,
  disableSkill,
  forceSkillPrivate,
  getRuntimeOverview,
  listSettlements
}

export default adminSkillsAPI
