import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import {
  type CreateSkillRequest,
  type CreateSkillVersionRequest,
  type SkillContent,
  type SkillDetail,
  type SkillEditorDraft,
  type SkillMarketFilters,
  type SkillMineFilters,
  type SkillPriceMode,
  type SkillRevenueDetail,
  type SkillRevenueOrder,
  type SkillRevenueOrderStatus,
  type SkillRevenuePoint,
  type SkillRevenueSummary,
  type SkillRunActionResult,
  type SkillRunFilters,
  type SkillRunMode,
  type SkillRunRecord,
  type SkillRunStatus,
  type SkillSortKey,
  type SkillStats,
  type SkillStatus,
  type SkillSummary,
  type SkillType,
  type SkillVariableOption,
  type SkillVariableSchemaItem,
  type SkillVariableType,
  type SkillVersionPublishResult,
  type SkillVersionRecord,
  type SkillVersionReviewStatus,
  type SkillVersionStatus,
  type SkillVersionSummary,
  type SkillVisibility,
  type UpdateSkillRequest
} from '@/types/skills'

type SkillScope = 'market' | 'mine'

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
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    if (Number.isFinite(parsed)) return parsed
  }
  return fallback
}

function asNullableNumber(value: unknown): number | null {
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? parsed : null
  }
  return null
}

function asBoolean(value: unknown, fallback = false): boolean {
  if (typeof value === 'boolean') return value
  if (typeof value === 'number') return value !== 0
  if (typeof value === 'string') {
    const normalized = value.trim().toLowerCase()
    if (normalized === 'true' || normalized === '1' || normalized === 'yes') return true
    if (normalized === 'false' || normalized === '0' || normalized === 'no') return false
  }
  return fallback
}

function asStringArray(value: unknown): string[] {
  if (Array.isArray(value)) {
    return value.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
  }
  if (typeof value === 'string' && value.trim()) {
    return value
      .split(',')
      .map((item) => item.trim())
      .filter(Boolean)
  }
  return []
}

function pickFirstValue(source: Record<string, unknown>, keys: string[]): unknown {
  for (const key of keys) {
    if (key in source && source[key] !== undefined && source[key] !== null) {
      return source[key]
    }
  }
  return undefined
}

function currentUserId(): number | null {
  try {
    const raw = localStorage.getItem('auth_user')
    if (!raw) return null
    const parsed = JSON.parse(raw)
    const value = isRecord(parsed) ? parsed.id : null
    return asNullableNumber(value)
  } catch {
    return null
  }
}

function normalizeSkillType(value: unknown): SkillType {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'prompt_image':
    case 'image_prompt':
    case 'image':
      return 'prompt_image'
    case 'script':
    case 'code':
      return 'script'
    case 'prompt_chat':
    case 'chat_prompt':
    case 'chat':
    default:
      return 'prompt_chat'
  }
}

function normalizeVisibility(value: unknown): SkillVisibility {
  return asString(value).trim().toLowerCase() === 'public' ? 'public' : 'private'
}

function normalizeSkillStatus(value: unknown, visibility?: SkillVisibility): SkillStatus {
  const normalized = asString(value).trim().toLowerCase()
  if (normalized === 'draft' || normalized === 'published' || normalized === 'archived' || normalized === 'hidden') {
    return normalized
  }
  return visibility === 'public' ? 'published' : 'draft'
}

function normalizePriceMode(value: unknown): SkillPriceMode {
  const normalized = asString(value).trim().toLowerCase()
  return normalized === 'paid' || normalized === 'sale' || normalized === 'premium' ? 'paid' : 'free'
}

function normalizeVariableType(value: unknown): SkillVariableType {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'text':
    case 'textarea':
      return 'text'
    case 'number':
    case 'integer':
    case 'float':
      return 'number'
    case 'boolean':
    case 'bool':
      return 'boolean'
    case 'select':
    case 'enum':
      return 'select'
    case 'json':
    case 'object':
      return 'json'
    case 'image':
      return 'image'
    case 'file':
      return 'file'
    case 'string':
    default:
      return 'string'
  }
}

function normalizeRunStatus(value: unknown): SkillRunStatus {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'queued':
      return 'queued'
    case 'running':
    case 'processing':
      return 'running'
    case 'failed':
    case 'error':
      return 'failed'
    case 'cancelled':
    case 'canceled':
      return 'cancelled'
    case 'succeeded':
    case 'success':
    case 'completed':
    default:
      return 'succeeded'
  }
}

function normalizeVersionStatus(value: unknown): SkillVersionStatus {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'draft':
      return 'draft'
    case 'deprecated':
      return 'deprecated'
    case 'archived':
      return 'archived'
    case 'published':
    default:
      return 'published'
  }
}

function normalizeReviewStatus(
  value: unknown,
  fallback: {
    status?: SkillVersionStatus
    publishedAt?: string | null
    submittedAt?: string | null
  } = {}
): SkillVersionReviewStatus {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'approved':
      return 'approved'
    case 'pending':
    case 'reviewing':
    case 'under_review':
      return 'pending'
    case 'rejected':
    case 'changes_requested':
      return 'rejected'
    case 'draft':
      return 'draft'
    default:
      break
  }

  if (fallback.status === 'draft') {
    return fallback.publishedAt ? 'rejected' : 'draft'
  }

  if (fallback.status === 'published' || fallback.status === 'deprecated' || fallback.status === 'archived') {
    return fallback.publishedAt ? 'approved' : 'pending'
  }

  if (fallback.publishedAt) return 'approved'
  if (fallback.submittedAt) return 'pending'
  return 'draft'
}

function buildVersionActionFlags(reviewStatus: SkillVersionReviewStatus): Pick<
  SkillVersionSummary,
  'can_submit_review' | 'can_publish' | 'can_test' | 'can_use'
> {
  const canSubmit = reviewStatus === 'draft' || reviewStatus === 'rejected'
  const canOperate = reviewStatus === 'approved'
  return {
    can_submit_review: canSubmit,
    can_publish: canOperate,
    can_test: canOperate,
    can_use: canOperate
  }
}

function normalizeRevenueOrderStatus(value: unknown): SkillRevenueOrderStatus {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'pending':
      return 'pending'
    case 'cancelled':
    case 'canceled':
      return 'cancelled' as SkillRevenueOrderStatus
    case 'refunded':
    case 'refund':
      return 'refunded'
    case 'settled':
      return 'settled'
    case 'transferred':
    case 'paid':
      return 'paid'
    default:
      return 'pending'
  }
}

function normalizeVariableOptions(value: unknown): SkillVariableOption[] {
  if (!Array.isArray(value)) return []
  const result: SkillVariableOption[] = []
  value.forEach((item) => {
    const source = isRecord(item) ? item : {}
    const rawValue = asString(source.value ?? source.key)
    const label = asString(source.label ?? source.name ?? rawValue)
    if (!rawValue && !label) return
    result.push({
      label: label || rawValue,
      value: rawValue || label,
      description: asNullableString(source.description)
    })
  })
  return result
}

function normalizeVariableSchema(value: unknown): SkillVariableSchemaItem[] {
  if (!Array.isArray(value)) return []
  const result: SkillVariableSchemaItem[] = []
  value.forEach((item) => {
    const source = isRecord(item) ? item : {}
    const key = asString(source.key ?? source.name ?? source.id).trim()
    if (!key) return
    result.push({
      key,
      label: asString(source.label ?? source.title ?? key),
      type: normalizeVariableType(source.type),
      required: asBoolean(source.required, false),
      description: asNullableString(source.description ?? source.help_text ?? source.help),
      placeholder: asNullableString(source.placeholder),
      default_value: (source.default_value ?? source.default ?? null) as string | number | boolean | null,
      options: normalizeVariableOptions(source.options ?? source.enum_values)
    })
  })
  return result
}

function normalizeSkillContent(value: unknown, fallbackType: SkillType): SkillContent | null {
  const source = isRecord(value) ? value : {}
  const rawText = typeof value === 'string' && value.trim() ? value : null
  if (rawText === null && !isRecord(value)) return null
  if (rawText === null && Object.keys(source).length === 0) return null
  const type = normalizeSkillType(source.type ?? fallbackType)

  switch (type) {
    case 'prompt_image':
      return {
        type,
        prompt_template: asString(source.prompt_template ?? source.prompt ?? source.template ?? rawText),
        negative_prompt_template: asNullableString(source.negative_prompt_template ?? source.negative_prompt),
        style: asNullableString(source.style),
        size: asNullableString(source.size),
        quality: asNullableString(source.quality),
        image_count: asNullableNumber(source.image_count ?? source.count)
      }
    case 'script':
      return {
        type,
        language: asString(source.language, 'javascript'),
        runtime: asNullableString(source.runtime),
        entrypoint: asNullableString(source.entrypoint ?? source.entry),
        source_code: asString(source.source_code ?? source.code ?? source.script ?? rawText),
        dependencies: asStringArray(source.dependencies),
        timeout_seconds: asNullableNumber(source.timeout_seconds ?? source.timeout)
      }
    case 'prompt_chat':
    default:
      return {
        type: 'prompt_chat',
        system_prompt: asString(source.system_prompt ?? source.system ?? source.prefix),
        user_prompt_template: asString(source.user_prompt_template ?? source.prompt_template ?? source.prompt ?? source.template ?? rawText),
        assistant_prefill: asNullableString(source.assistant_prefill ?? source.assistant),
        model: asNullableString(source.model),
        temperature: asNullableNumber(source.temperature),
        max_tokens: asNullableNumber(source.max_tokens)
      }
  }
}

function normalizePricing(raw: unknown): SkillSummary['pricing'] {
  const source = isRecord(raw) ? raw : {}
  const priceMode = normalizePriceMode(source.mode ?? source.price_mode ?? source.billing_mode)
  return {
    mode: priceMode,
    amount: asNumber(source.amount ?? source.price ?? source.price_amount, 0),
    currency: asString(source.currency, 'CNY'),
    settlement_ratio: asNullableNumber(source.settlement_ratio ?? source.share_ratio)
  }
}

function normalizeStats(raw: unknown): SkillStats {
  const source = isRecord(raw) ? raw : {}
  return {
    installs: asNumber(source.installs ?? source.install_count, 0),
    runs: asNumber(source.runs ?? source.run_count, 0),
    revenue: asNumber(source.revenue ?? source.total_revenue, 0),
    rating: asNullableNumber(source.rating),
    versions: asNumber(source.versions ?? source.version_count, 0)
  }
}

function normalizeVersionSummary(raw: unknown, skillId: number | null = null): SkillVersionSummary | null {
  const source = isRecord(raw) ? raw : {}
  const version = asString(source.version ?? source.version_name)
  if (!version && !asNullableNumber(source.id)) return null
  const createdAt = asString(source.created_at, new Date().toISOString())
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  const publishedAt = asNullableString(pickFirstValue({ ...metadata, ...source }, ['published_at', 'reviewed_at']))
  const submittedAt = asNullableString(pickFirstValue({ ...metadata, ...source }, ['submitted_at']))
  const status = normalizeVersionStatus(source.status)
  const reviewStatus = normalizeReviewStatus(
    pickFirstValue({ ...metadata, ...source }, ['review_status', 'latest_review_status']),
    {
      status,
      publishedAt,
      submittedAt
    }
  )
  return {
    id: asNumber(source.id, Date.now()),
    skill_id: asNullableNumber(source.skill_id ?? skillId),
    version: version || 'v1',
    status,
    review_status: reviewStatus,
    changelog: asString(source.changelog ?? source.release_note),
    source_locked: asBoolean(source.source_locked, false),
    is_current: asBoolean(source.is_current ?? source.current, false),
    created_at: createdAt,
    published_at: publishedAt,
    submitted_at: submittedAt,
    reviewed_at: publishedAt,
    review_note: asNullableString(pickFirstValue({ ...metadata, ...source }, ['review_note', 'rejection_reason'])),
    ...buildVersionActionFlags(reviewStatus)
  }
}

function normalizeAuthor(raw: unknown): SkillSummary['author'] {
  const source = isRecord(raw) ? raw : {}
  return {
    id: asNullableNumber(source.id ?? source.user_id),
    name: asNullableString(source.name ?? source.username ?? source.display_name),
    avatar_url: asNullableString(source.avatar_url)
  }
}

function normalizeSkillSummary(raw: unknown): SkillSummary {
  const source = isRecord(raw) ? raw : {}
  const meta = isRecord(source.metadata) ? source.metadata : {}
  const pricing = normalizePricing(source.pricing ?? meta.pricing ?? source)
  const visibility = normalizeVisibility(source.visibility ?? meta.visibility)
  const status = normalizeSkillStatus(source.status ?? meta.status, visibility)
  const type = normalizeSkillType(source.type ?? meta.type)
  const author = normalizeAuthor(source.author ?? meta.author ?? source)
  const ownerId = asNullableNumber(source.owner_id ?? source.user_id ?? author.id)
  const owned = asBoolean(source.owned ?? source.is_owner, false) || (ownerId !== null && ownerId === currentUserId())
  const installed = asBoolean(source.installed ?? source.is_installed, false)
  const explicitSourceLocked = source.source_locked ?? meta.source_locked
  const sourceLocked = explicitSourceLocked === undefined ? pricing.mode === 'paid' : asBoolean(explicitSourceLocked, pricing.mode === 'paid')
  const canViewSource = asBoolean(source.can_view_source ?? meta.can_view_source, !sourceLocked || owned)
  const skillId = asNumber(source.id, Date.now())
  const createdAt = asString(source.created_at, new Date().toISOString())
  const updatedAt = asString(source.updated_at, createdAt)

  return {
    id: skillId,
    slug: asString(source.slug, `skill-${skillId}`),
    name: asString(source.name ?? source.title, `Skill ${skillId}`),
    tagline: asString(source.tagline ?? source.summary ?? meta.summary),
    description: asString(source.description ?? meta.description),
    type,
    visibility,
    status,
    category: asNullableString(source.category ?? meta.category),
    tags: asStringArray(source.tags ?? meta.tags),
    cover_image_url: asNullableString(source.cover_image_url ?? source.cover ?? meta.cover_image_url),
    pricing,
    source_locked: sourceLocked,
    can_view_source: canViewSource,
    installed,
    owned,
    editable: asBoolean(source.editable ?? source.can_edit, owned),
    author,
    stats: normalizeStats(source.stats ?? meta.stats ?? source.metrics),
    latest_version: normalizeVersionSummary(source.latest_version ?? meta.latest_version, skillId),
    current_version: normalizeVersionSummary(source.current_version ?? source.installed_version ?? meta.current_version, skillId),
    created_at: createdAt,
    updated_at: updatedAt
  }
}

function normalizeSkillDetail(raw: unknown): SkillDetail {
  const summary = normalizeSkillSummary(raw)
  const source = isRecord(raw) ? raw : {}
  const meta = isRecord(source.metadata) ? source.metadata : {}
  const content = normalizeSkillContent(source.content ?? source.source_content ?? source.source ?? meta.content, summary.type)
  const variableSchema = normalizeVariableSchema(
    source.variable_schema ?? source.variables_schema ?? source.variables ?? source.schema ?? meta.variable_schema
  )

  return {
    ...summary,
    variable_schema: variableSchema,
    content,
    metadata: meta,
    examples: asStringArray(source.examples ?? meta.examples),
    readme: asNullableString(source.readme ?? meta.readme),
    install_note: asNullableString(source.install_note ?? meta.install_note),
    can_install: asBoolean(source.can_install, !summary.owned),
    can_run: asBoolean(source.can_run, true)
  }
}

function normalizeVersionRecord(raw: unknown): SkillVersionRecord {
  const summary = normalizeVersionSummary(raw) ?? {
    id: Date.now(),
    skill_id: null,
    version: 'v1',
    status: 'draft' as SkillVersionStatus,
    review_status: 'draft' as SkillVersionReviewStatus,
    changelog: '',
    source_locked: false,
    is_current: false,
    created_at: new Date().toISOString(),
    published_at: null,
    submitted_at: null,
    reviewed_at: null,
    review_note: null,
    can_submit_review: true,
    can_publish: false,
    can_test: false,
    can_use: false
  }
  const source = isRecord(raw) ? raw : {}
  const meta = isRecord(source.metadata) ? source.metadata : {}
  const skillType = normalizeSkillType(source.type ?? source.skill_type)

  return {
    ...summary,
    variable_schema: normalizeVariableSchema(source.variable_schema ?? source.variables ?? source.schema ?? meta.variable_schema),
    content: normalizeSkillContent(source.content ?? source.source_content ?? source.source ?? meta.content, skillType),
    metadata: meta
  }
}

function looksLikeVersionRecordPayload(source: Record<string, unknown>): boolean {
  return (
    source.id !== undefined ||
    source.version !== undefined ||
    source.variable_schema !== undefined ||
    source.content !== undefined ||
    source.source_content !== undefined ||
    source.source !== undefined
  )
}

function normalizeRunActionResult(
  raw: unknown,
  mode: SkillRunMode,
  skillId: number,
  versionId?: number | null
): SkillRunActionResult {
  const source = isRecord(raw) ? raw : {}
  const prepared = isRecord(source.prepared) ? source.prepared : {}
  const run = isRecord(prepared.run) ? prepared.run : {}
  const dispatch = isRecord(source.dispatch) ? source.dispatch : {}
  const merged = { ...dispatch, ...run, ...prepared, ...source }

  return {
    mode,
    skill_id: asNumber(pickFirstValue(merged, ['skill_id']), skillId),
    version_id: asNullableNumber(pickFirstValue(merged, ['version_id'])) ?? versionId ?? null,
    run_id: asNullableNumber(pickFirstValue(merged, ['run_id', 'id'])),
    status: asNullableString(pickFirstValue(merged, ['status'])),
    raw: source
  }
}

function normalizeRunRecord(raw: unknown): SkillRunRecord {
  const source = isRecord(raw) ? raw : {}
  return {
    id: asNumber(source.id, Date.now()),
    skill_id: asNumber(source.skill_id, 0),
    skill_name: asString(source.skill_name ?? source.name, '-'),
    version_id: asNullableNumber(source.version_id),
    version: asNullableString(source.version ?? source.version_name),
    status: normalizeRunStatus(source.status),
    trigger: asNullableString(source.trigger),
    input_preview: asNullableString(source.input_preview ?? source.input_summary),
    output_preview: asNullableString(source.output_preview ?? source.output_summary),
    error_message: asNullableString(source.error_message ?? source.error),
    duration_ms: asNullableNumber(source.duration_ms ?? source.duration),
    cost: asNullableNumber(source.cost ?? source.amount),
    currency: asString(source.currency, 'CNY'),
    created_at: asString(source.created_at, new Date().toISOString()),
    started_at: asNullableString(source.started_at),
    finished_at: asNullableString(source.finished_at)
  }
}

function normalizeRevenueSummary(raw: unknown): SkillRevenueSummary {
  const source = isRecord(raw) ? raw : {}
  return {
    total_revenue: asNumber(source.total_revenue ?? source.revenue, 0),
    total_sales: asNumber(source.total_sales ?? source.sales, 0),
    total_runs: asNumber(source.total_runs ?? source.runs, 0),
    pending_amount: asNumber(source.pending_amount, 0),
    settled_amount: asNumber(source.settled_amount, 0),
    refunded_amount: asNumber(source.refunded_amount ?? source.refunds, 0),
    currency: asString(source.currency, 'CNY')
  }
}

function normalizeRevenuePoint(raw: unknown): SkillRevenuePoint {
  const source = isRecord(raw) ? raw : {}
  return {
    date: asString(source.date, new Date().toISOString().slice(0, 10)),
    revenue: asNumber(source.revenue, 0),
    sales: asNumber(source.sales, 0),
    runs: asNumber(source.runs, 0)
  }
}

function normalizeRevenueOrder(raw: unknown): SkillRevenueOrder {
  const source = isRecord(raw) ? raw : {}
  return {
    id: asNumber(source.id, Date.now()),
    buyer_name: asNullableString(source.buyer_name ?? source.user_name),
    version: asNullableString(source.version ?? source.version_name),
    amount: asNumber(source.amount ?? source.revenue, 0),
    currency: asString(source.currency, 'CNY'),
    status: normalizeRevenueOrderStatus(source.status),
    created_at: asString(source.created_at, new Date().toISOString())
  }
}

function normalizePagedResponse<T>(
  raw: unknown,
  page: number,
  pageSize: number,
  mapper: (item: unknown) => T
): BasePaginationResponse<T> {
  if (Array.isArray(raw)) {
    const total = raw.length
    const pages = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)))
    const safePage = Math.min(Math.max(page, 1), pages)
    const start = (safePage - 1) * pageSize
    return {
      items: raw.slice(start, start + pageSize).map(mapper),
      total,
      page: safePage,
      page_size: pageSize,
      pages
    }
  }

  if (isRecord(raw) && Array.isArray(raw.items)) {
    return {
      items: raw.items.map(mapper),
      total: asNumber(raw.total, raw.items.length),
      page: asNumber(raw.page, page),
      page_size: asNumber(raw.page_size, pageSize),
      pages: asNumber(raw.pages, Math.max(1, Math.ceil(raw.items.length / Math.max(pageSize, 1))))
    }
  }

  return {
    items: [],
    total: 0,
    page,
    page_size: pageSize,
    pages: 1
  }
}

function normalizeSort(value: unknown): SkillSortKey | null {
  const normalized = asString(value).trim().toLowerCase()
  switch (normalized) {
    case 'popular':
    case 'revenue':
    case 'runs':
    case 'price_low':
    case 'price_high':
    case 'latest':
      return normalized
    default:
      return null
  }
}

function buildListParams(scope: SkillScope, filters?: SkillMarketFilters | SkillMineFilters): Record<string, unknown> {
  const params: Record<string, unknown> = { scope }
  const search = asNullableString(filters?.search)
  const type = asString(filters?.type).trim()
  const category = filters && 'category' in filters ? asString(filters.category).trim() : ''
  const status = asString((filters as SkillMineFilters | undefined)?.status).trim()
  const visibility = asString((filters as SkillMineFilters | undefined)?.visibility).trim()
  const priceMode = asString((filters as SkillMarketFilters | undefined)?.price_mode).trim()
  const installed = asString((filters as SkillMarketFilters | undefined)?.installed).trim()
  const sort = normalizeSort(filters?.sort)

  if (search) params.search = search
  if (type && type !== 'all') params.type = type
  if (category && category !== 'all') params.category = category
  if (scope === 'mine') {
    if (status && status !== 'all') params.status = status
    if (visibility && visibility !== 'all') params.visibility = visibility
  } else if (priceMode && priceMode !== 'all') {
    params.price_mode = priceMode
    if (installed && installed !== 'all') {
      params.installed = installed
    }
  } else if (installed && installed !== 'all') {
    params.installed = installed
  }
  if (sort) params.sort = sort
  return params
}

function buildRunParams(filters?: SkillRunFilters): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  const search = asNullableString(filters?.search)
  const status = asString(filters?.status).trim()
  if (search) params.search = search
  if (status && status !== 'all') params.status = status
  if (typeof filters?.version_id === 'number' && filters.version_id > 0) {
    params.version_id = filters.version_id
  }
  return params
}

function serializeSkillPayload(payload: CreateSkillRequest | UpdateSkillRequest): Record<string, unknown> {
  const draft = payload as Partial<SkillEditorDraft> & Partial<CreateSkillRequest>
  const tags = Array.isArray(draft.tags) ? draft.tags : []
  const pricing = {
    mode: draft.pricing?.mode ?? draft.pricing?.mode ?? (draft as { price_mode?: SkillPriceMode }).price_mode ?? 'free',
    amount: draft.pricing?.amount ?? 0,
    currency: draft.pricing?.currency ?? 'CNY',
    settlement_ratio: draft.pricing?.settlement_ratio ?? null
  }

  return {
    slug: draft.slug,
    name: draft.name,
    tagline: draft.tagline,
    description: draft.description,
    type: draft.type,
    visibility: draft.visibility,
    status: draft.status,
    category: draft.category || null,
    cover_image_url: draft.cover_image_url || null,
    tags,
    pricing,
    price_mode: pricing.mode,
    price_amount: pricing.amount,
    currency: pricing.currency,
    settlement_ratio: pricing.settlement_ratio,
    source_locked: draft.source_locked,
    variable_schema: draft.variable_schema ?? [],
    content: draft.content,
    readme: draft.readme || null,
    install_note: draft.install_note || null
  }
}

export async function listSkillMarket(
  page = 1,
  pageSize = 18,
  filters?: SkillMarketFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<SkillSummary>> {
  const { data } = await apiClient.get('/user/skills', {
    params: {
      page,
      page_size: pageSize,
      ...buildListParams('market', filters)
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeSkillSummary)
}

export async function listMySkills(
  page = 1,
  pageSize = 18,
  filters?: SkillMineFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<SkillSummary>> {
  const { data } = await apiClient.get('/user/skills', {
    params: {
      page,
      page_size: pageSize,
      ...buildListParams('mine', filters)
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeSkillSummary)
}

export async function getSkillDetail(id: number, options?: { signal?: AbortSignal }): Promise<SkillDetail> {
  const { data } = await apiClient.get(`/user/skills/${id}`, {
    signal: options?.signal
  })
  return normalizeSkillDetail(data)
}

export async function createSkill(payload: CreateSkillRequest): Promise<SkillDetail> {
  const { data } = await apiClient.post('/user/skills', serializeSkillPayload(payload))
  return normalizeSkillDetail(data)
}

export async function updateSkill(id: number, payload: UpdateSkillRequest): Promise<SkillDetail> {
  const { data } = await apiClient.put(`/user/skills/${id}`, serializeSkillPayload(payload))
  return normalizeSkillDetail(data)
}

export async function installSkill(id: number): Promise<{ message: string; skill_id: number; installed: boolean; install_count: number }> {
  const { data } = await apiClient.post(`/user/skills/${id}/install`)
  return data as { message: string; skill_id: number; installed: boolean; install_count: number }
}

export async function uninstallSkill(id: number): Promise<{ message: string; skill_id: number; installed: boolean; install_count: number }> {
  const { data } = await apiClient.post(`/user/skills/${id}/uninstall`)
  return data as { message: string; skill_id: number; installed: boolean; install_count: number }
}

export async function listSkillVersions(
  skillId: number,
  page = 1,
  pageSize = 20,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<SkillVersionRecord>> {
  const { data } = await apiClient.get(`/user/skills/${skillId}/versions`, {
    params: {
      page,
      page_size: pageSize
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeVersionRecord)
}

export async function createSkillVersion(skillId: number, payload: CreateSkillVersionRequest): Promise<SkillVersionRecord> {
  const { data } = await apiClient.post(`/user/skills/${skillId}/versions`, payload)
  return normalizeVersionRecord(data)
}

export async function updateSkillVersion(skillId: number, versionId: number, payload: Partial<CreateSkillVersionRequest>): Promise<SkillVersionRecord> {
  const { data } = await apiClient.put(`/user/skills/${skillId}/versions/${versionId}`, payload)
  return normalizeVersionRecord(data)
}

export async function submitSkillVersion(skillId: number, versionId: number): Promise<SkillVersionRecord> {
  const { data } = await apiClient.post(`/user/skills/${skillId}/versions/${versionId}/submit`)
  return normalizeVersionRecord(data)
}

export async function publishSkillVersion(versionId: number): Promise<SkillVersionPublishResult> {
  const { data } = await apiClient.post(`/user/skills/versions/${versionId}/publish`)
  const source = isRecord(data) ? data : {}
  const version = looksLikeVersionRecordPayload(source) ? normalizeVersionRecord(source) : null

  return {
    published: asBoolean(source.published, Boolean(version)),
    skill_id: asNullableNumber(source.skill_id ?? version?.skill_id),
    version_id: asNumber(source.version_id ?? source.id ?? version?.id, versionId),
    version,
    raw: source
  }
}

export type SkillRunOptions = {
  versionId?: number | null
  apiKeyId?: number | null
  parameters?: Record<string, unknown>
}

export async function testSkill(skillId: number, options?: SkillRunOptions | number | null): Promise<SkillRunActionResult> {
  const normalized = typeof options === 'number' || options == null
    ? { versionId: options ?? null }
    : options
  const versionId = normalized.versionId
  const { data } = await apiClient.post(
    `/user/skills/${skillId}/test`,
    {
      parameters: normalized.parameters ?? {},
      trace: normalized.apiKeyId && normalized.apiKeyId > 0 ? { api_key_id: normalized.apiKeyId } : undefined
    },
    {
      params: typeof versionId === 'number' && versionId > 0 ? { version_id: versionId } : undefined
    }
  )
  return normalizeRunActionResult(data, 'test', skillId, versionId)
}

export async function useSkill(skillId: number, options?: SkillRunOptions | number | null): Promise<SkillRunActionResult> {
  const normalized = typeof options === 'number' || options == null
    ? { versionId: options ?? null }
    : options
  const versionId = normalized.versionId
  if (!(normalized.apiKeyId && normalized.apiKeyId > 0)) {
    throw new Error('use skill requires an API key for token billing')
  }
  const { data } = await apiClient.post(
    `/user/skills/${skillId}/use`,
    {
      parameters: normalized.parameters ?? {},
      trace: { api_key_id: normalized.apiKeyId }
    },
    {
      params: typeof versionId === 'number' && versionId > 0 ? { version_id: versionId } : undefined
    }
  )
  return normalizeRunActionResult(data, 'use', skillId, versionId)
}

export async function listSkillRuns(
  skillId: number,
  page = 1,
  pageSize = 20,
  filters?: SkillRunFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<SkillRunRecord>> {
  const { data } = await apiClient.get(`/user/skills/${skillId}/runs`, {
    params: {
      page,
      page_size: pageSize,
      ...buildRunParams(filters)
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeRunRecord)
}

export async function getSkillRevenue(skillId: number, options?: { signal?: AbortSignal }): Promise<SkillRevenueDetail> {
  const { data } = await apiClient.get(`/user/skills/${skillId}/revenue`, {
    signal: options?.signal
  })
  const source = isRecord(data) ? data : {}
  return {
    summary: normalizeRevenueSummary(source.summary ?? source),
    trend: Array.isArray(source.trend) ? source.trend.map(normalizeRevenuePoint) : [],
    orders: Array.isArray(source.orders) ? source.orders.map(normalizeRevenueOrder) : []
  }
}

const skillsAPI = {
  listSkillMarket,
  listMySkills,
  getSkillDetail,
  createSkill,
  updateSkill,
  installSkill,
  uninstallSkill,
  listSkillVersions,
  createSkillVersion,
  updateSkillVersion,
  submitSkillVersion,
  publishSkillVersion,
  testSkill,
  useSkill,
  listSkillRuns,
  getSkillRevenue
}

export default skillsAPI
