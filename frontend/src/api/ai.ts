import { apiClient } from './client'
import type {
  AiArtwork,
  AiArtworkListFilters,
  AiLineOption,
  AiLineKeyOption,
  AiArtworkStatus,
  AiChatMessage,
  AiChatResponse,
  AiPromptListFilters,
  AiPromptStatus,
  AiPromptTemplate,
  AiVisibility,
  BasePaginationResponse,
  CreateAiArtworkRequest,
  CreateAiChatRequest,
  CreateAiPromptRequest,
  UpdateAiArtworkRequest,
  UpdateAiPromptRequest,
  User
} from '@/types'
import { getSessionUser } from '@/utils/authSession'

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
  return typeof value === 'boolean' ? value : fallback
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function asNumberArray(value: unknown): number[] {
  return Array.isArray(value)
    ? value
        .map((item) => (typeof item === 'number' && Number.isFinite(item) ? item : Number(item)))
        .filter((item) => Number.isFinite(item) && item > 0)
    : []
}

function traceGroupID(value: unknown): number | null {
  if (!isRecord(value)) return null
  return asNullableNumber(value.group_id)
}

function normalizeLineOption(raw: unknown): AiLineOption | null {
  const source = isRecord(raw) ? raw : {}
  const groupId = asNullableNumber(source.group_id ?? source.line_id ?? source.id)
  if (!groupId) return null
  const defaultKeyId = asNullableNumber(source.default_key_id ?? source.key_id ?? source.primary_key_id)
  const keys = Array.isArray(source.keys)
    ? source.keys
        .map((item): AiLineKeyOption | null => {
          if (!isRecord(item)) return null
          const id = asNullableNumber(item.id ?? item.key_id ?? item.value)
          if (!id) return null
          const name = asString(item.name ?? item.label ?? item.display_name ?? item.key_name, `Key ${id}`)
          return { id, name }
        })
        .filter((item): item is AiLineKeyOption => !!item)
    : []
  const keyIds = asNumberArray(source.key_ids ?? source.keys ?? source.available_key_ids)
  if (keys.length > 0 && keyIds.length === 0) {
    keyIds.push(...keys.map((item) => item.id))
  }
  if (keyIds.length === 0 && defaultKeyId) {
    keyIds.push(defaultKeyId)
  }
  const normalizedKeyCount = asNumber(source.key_count ?? source.available_key_count, keyIds.length)
  const label = asString(source.label ?? source.display_name ?? source.group_name ?? source.name, `Line ${groupId}`)
  const platform = asString(source.platform, 'openai') as AiLineOption['platform']
  return {
    group_id: groupId,
    label,
    platform,
    description: asNullableString(source.description),
    keys,
    key_ids: keyIds,
    key_count: Math.max(normalizedKeyCount, keyIds.length),
    default_key_id: defaultKeyId ?? keyIds[0] ?? null
  }
}

function extractRuntimeLineSources(raw: unknown): unknown[] {
  if (Array.isArray(raw)) return raw
  if (!isRecord(raw)) return []
  const candidates = [raw.lines, raw.items, raw.data]
  for (const candidate of candidates) {
    if (Array.isArray(candidate)) return candidate
    if (isRecord(candidate) && Array.isArray(candidate.items)) return candidate.items
    if (isRecord(candidate) && Array.isArray(candidate.lines)) return candidate.lines
  }
  return []
}

function normalizeRuntimeLines(raw: unknown): AiLineOption[] {
  const sources = extractRuntimeLineSources(raw)
  return sources.map(normalizeLineOption).filter((item): item is AiLineOption => !!item && item.key_count > 0)
}

type AiChatEntryMode = 'responses' | 'chat_completions'

function asChatEntry(value: unknown): AiChatEntryMode | null {
  const normalized = asString(value).trim().toLowerCase()
  if (normalized === 'responses') return 'responses'
  if (normalized === 'chat_completions' || normalized === 'chat-completions') return 'chat_completions'
  return null
}

function normalizeChatEntries(...candidates: unknown[]): AiChatEntryMode[] {
  const entries: AiChatEntryMode[] = []
  for (const candidate of candidates) {
    if (!Array.isArray(candidate)) continue
    for (const item of candidate) {
      const entry = asChatEntry(item)
      if (entry && !entries.includes(entry)) {
        entries.push(entry)
      }
    }
  }
  return entries
}

function normalizeRuntimeChat(raw: unknown): { supported_entries: AiChatEntryMode[]; forced_entry: AiChatEntryMode | null } {
  const source = isRecord(raw) ? raw : {}
  const chat = isRecord(source.chat) ? source.chat : {}
  const capabilities = isRecord(source.capabilities) ? source.capabilities : {}
  const supportedEntries = normalizeChatEntries(
    chat.supported_entries,
    chat.entries,
    chat.entry_modes,
    chat.available_entries,
    capabilities.supported_entries
  )

  if (asBoolean(chat.chat_completions_enabled ?? chat.supports_chat_completions ?? capabilities.chat_completions, false)) {
    supportedEntries.push('chat_completions')
  }
  if (!supportedEntries.includes('responses')) {
    supportedEntries.unshift('responses')
  }

  const forcedEntry =
    asBoolean(chat.responses_only ?? chat.use_responses_only ?? chat.lock_responses, false)
      ? 'responses'
      : asChatEntry(chat.forced_entry ?? chat.default_entry ?? chat.entry)

  return {
    supported_entries: supportedEntries.filter((entry, index, entries) => entries.indexOf(entry) === index),
    forced_entry: forcedEntry
  }
}

export interface AiRuntimeInfo {
  source_domain?: string | null
  default_line?: AiLineOption | null
  media?: {
    enabled?: boolean
    bucket?: string | null
    public_base_url?: string | null
    presign_expiry_minutes?: number
    max_upload_size_bytes?: number
    default_visibility?: string | null
    upload_endpoint?: string | null
    public_endpoint_template?: string | null
    thumbnail_endpoint_template?: string | null
    download_endpoint_template?: string | null
    thumbnail_download_template?: string | null
    supported_biz_types?: string[]
    thumbnail_enabled?: boolean
  }
  image_edit?: {
    enabled?: boolean
    source_domain?: string | null
    upload_biz_type?: string | null
    thumbnail_route?: string | null
    download_route?: string | null
    thumbnail_download_route?: string | null
  }
  chat?: {
    supported_entries?: AiChatEntryMode[]
    forced_entry?: AiChatEntryMode | null
  }
  lines?: AiLineOption[]
}

function normalizeRuntimeInfo(raw: unknown): AiRuntimeInfo {
  const source = isRecord(raw) ? raw : {}
  const media = isRecord(source.media) ? source.media : {}
  const imageEdit = isRecord(source.image_edit) ? source.image_edit : {}
  const defaultLine = normalizeLineOption(source.default_line)
  const chat = normalizeRuntimeChat(source)
  return {
    source_domain: asNullableString(source.source_domain),
    default_line: defaultLine,
    media: {
      enabled: asBoolean(media.enabled, false),
      bucket: asNullableString(media.bucket),
      public_base_url: asNullableString(media.public_base_url),
      presign_expiry_minutes: asNumber(media.presign_expiry_minutes, 0),
      max_upload_size_bytes: asNumber(media.max_upload_size_bytes, 0),
      default_visibility: asNullableString(media.default_visibility),
      upload_endpoint: asNullableString(media.upload_endpoint),
      public_endpoint_template: asNullableString(media.public_endpoint_template),
      thumbnail_endpoint_template: asNullableString(media.thumbnail_endpoint_template),
      download_endpoint_template: asNullableString(media.download_endpoint_template),
      thumbnail_download_template: asNullableString(media.thumbnail_download_template),
      supported_biz_types: asStringArray(media.supported_biz_types),
      thumbnail_enabled: asBoolean(media.thumbnail_enabled, false)
    },
    image_edit: {
      enabled: asBoolean(imageEdit.enabled, false),
      source_domain: asNullableString(imageEdit.source_domain),
      upload_biz_type: asNullableString(imageEdit.upload_biz_type),
      thumbnail_route: asNullableString(imageEdit.thumbnail_route),
      download_route: asNullableString(imageEdit.download_route),
      thumbnail_download_route: asNullableString(imageEdit.thumbnail_download_route)
    },
    chat,
    lines: normalizeRuntimeLines(source)
  }
}

function currentUser(): User | null {
  return getSessionUser()
}

function normalizePagedResponse<T>(
  raw: unknown,
  page: number,
  pageSize: number,
  mapper: (item: unknown) => T
): BasePaginationResponse<T> {
  if (Array.isArray(raw)) {
    const total = raw.length
    const safePageSize = Math.max(1, pageSize)
    const pages = Math.max(1, Math.ceil(total / safePageSize))
    const safePage = Math.min(Math.max(1, page), pages)
    const start = (safePage - 1) * safePageSize
    return {
      items: raw.slice(start, start + safePageSize).map(mapper),
      total,
      page: safePage,
      page_size: safePageSize,
      pages
    }
  }

  if (isRecord(raw) && Array.isArray(raw.items)) {
    return {
      items: raw.items.map(mapper),
      total: asNumber(raw.total, raw.items.length),
      page: asNumber(raw.page, page),
      page_size: asNumber(raw.page_size, pageSize),
      pages: asNumber(raw.pages, Math.max(1, Math.ceil(raw.items.length / Math.max(1, pageSize))))
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

function promptStatusFromRaw(source: Record<string, unknown>): AiPromptStatus {
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  const status = asString(source.status || metadata.status).trim().toLowerCase()
  if (status === 'draft' || status === 'published' || status === 'archived' || status === 'hidden') {
    return status
  }
  const moderation = asString(source.moderation_state).trim().toLowerCase()
  if (moderation === 'blocked') return 'hidden'
  if (moderation === 'forced_private') return 'archived'
  return asString(source.visibility) === 'public' ? 'published' : 'draft'
}

function normalizePrompt(raw: unknown): AiPromptTemplate {
  const source = isRecord(raw) ? raw : {}
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  const createdAt = asString(source.created_at, new Date().toISOString())
  const updatedAt = asString(source.updated_at, createdAt)
  const current = currentUser()
  return {
    id: asNumber(source.id, Date.now()),
    title: asString(source.title, 'Untitled Prompt'),
    content: asString(source.content),
    description: asNullableString(source.description ?? metadata.description),
    tags: asStringArray(source.tags ?? metadata.tags),
    visibility: (asString(source.visibility) === 'public' ? 'public' : 'private') as AiVisibility,
    status: promptStatusFromRaw({ ...source, metadata }),
    line_id: asNullableNumber(source.group_id ?? source.line_id) ?? traceGroupID(source.trace),
    line_name: asNullableString(source.group_name ?? source.line_name),
    owner_id: asNullableNumber(source.user_id ?? source.owner_id),
    owner_name: asNullableString(metadata.owner_name ?? source.owner_name ?? source.user_name),
    is_mine: current ? asNumber(source.user_id ?? source.owner_id) === current.id : false,
    cloned_from_id: asNullableNumber(metadata.cloned_from_id),
    usage_count: asNumber(metadata.usage_count, 0),
    featured: asBoolean(metadata.featured, false),
    created_at: createdAt,
    updated_at: updatedAt
  }
}

function normalizeArtwork(raw: unknown): AiArtwork {
  const source = isRecord(raw) ? raw : {}
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  const createdAt = asString(source.created_at, new Date().toISOString())
  const updatedAt = asString(source.updated_at, createdAt)
  const imageUrl = asString(source.source_url || source.image_url || metadata.image_url)
  const thumbnailUrl = asNullableString(metadata.thumbnail_url || source.thumbnail_url) ?? imageUrl
  const rawStatus = asString(source.status).trim().toLowerCase()
  const normalizedStatus = (
    rawStatus === 'ready'
      ? 'succeeded'
      : ['pending', 'succeeded', 'failed', 'hidden', 'deleted'].includes(rawStatus)
        ? rawStatus
        : 'pending'
  ) as AiArtworkStatus
  return {
    id: asNumber(source.id, Date.now()),
    title: asString(metadata.title ?? source.title, 'Untitled Artwork'),
    prompt: asString(metadata.prompt ?? source.prompt),
    negative_prompt: asNullableString(metadata.negative_prompt ?? source.negative_prompt),
    visibility: (asString(source.visibility) === 'private' ? 'private' : 'public') as AiVisibility,
    status: normalizedStatus,
    image_url: imageUrl,
    thumbnail_url: asNullableString(thumbnailUrl),
    line_id: asNullableNumber(source.group_id ?? source.line_id) ?? traceGroupID(source.trace),
    line_name: asNullableString(source.group_name ?? source.line_name),
    owner_id: asNullableNumber(source.user_id ?? source.owner_id),
    owner_name: asNullableString(metadata.owner_name ?? source.owner_name ?? source.user_name),
    width: asNullableNumber(source.width),
    height: asNullableNumber(source.height),
    size: asNullableString(metadata.size ?? source.size),
    style: asNullableString(metadata.style ?? source.style),
    tags: asStringArray(metadata.tags ?? source.tags),
    featured: asBoolean(metadata.featured, false),
    prompt_template_id: asNullableNumber(source.prompt_template_id),
    likes: asNumber(metadata.likes, 0),
    views: asNumber(metadata.views, 0),
    created_at: createdAt,
    updated_at: updatedAt
  }
}

function normalizeChatMessage(raw: unknown): AiChatMessage {
  const source = isRecord(raw) ? raw : {}
  return {
    id: asString(source.id, `message-${Date.now()}`),
    role: source.role === 'system' || source.role === 'user' ? source.role : 'assistant',
    content: asString(source.content),
    created_at: asString(source.created_at, new Date().toISOString()),
    line_id: asNullableNumber(source.group_id ?? source.line_id) ?? traceGroupID(source.trace),
    line_name: asNullableString(source.group_name ?? source.line_name),
    model: asNullableString(source.model)
  }
}

export async function loadRuntimeLines(): Promise<AiLineOption[] | null> {
  const runtime = await getRuntimeInfo()
  const lines = Array.isArray(runtime.lines) ? runtime.lines : normalizeRuntimeLines(runtime)
  return lines.length > 0 ? lines : null
}

export async function getRuntimeInfo(): Promise<AiRuntimeInfo> {
  const { data } = await apiClient.get('/user/ai/runtime')
  return normalizeRuntimeInfo(data)
}

type UserAiPromptScope = 'mine' | 'library' | 'all'
type UserAiPromptListFilters = AiPromptListFilters & { scope?: UserAiPromptScope }
type UserAiArtworkFilterStatus = AiArtworkListFilters['status'] | 'ready'
type UserAiArtworkListFilters = Omit<AiArtworkListFilters, 'status'> & { status?: UserAiArtworkFilterStatus }

function normalizePromptScope(filters?: UserAiPromptListFilters): UserAiPromptScope {
  const requestedScope = asString(filters?.scope).trim().toLowerCase()
  if (requestedScope === 'library' || requestedScope === 'all') return requestedScope
  if (requestedScope === 'mine') return 'mine'
  if (filters?.mine_only) return 'mine'
  if (filters?.visibility === 'public') return 'library'
  return 'mine'
}

function normalizePromptListParams(filters?: UserAiPromptListFilters): Record<string, unknown> {
  const params: Record<string, unknown> = {
    scope: normalizePromptScope(filters)
  }
  const search = asNullableString(filters?.search)
  const visibility = asString(filters?.visibility).trim()
  const status = asString(filters?.status).trim()
  const groupID = typeof filters?.line_id === 'number' && filters.line_id > 0 ? filters.line_id : null

  if (search) {
    params.search = search
  }
  if (params.scope === 'library') {
    params.visibility = 'public'
  } else if (visibility === 'public' || visibility === 'private') {
    params.visibility = visibility
  }
  if (status && status !== 'all') {
    params.status = status
  }
  if (groupID) {
    params.group_id = groupID
  }
  return params
}

function normalizeArtworkFilterStatus(status: unknown): 'pending' | 'ready' | 'hidden' | 'deleted' | null {
  const normalized = asString(status).trim().toLowerCase()
  switch (normalized) {
    case 'pending':
      return 'pending'
    case 'ready':
    case 'succeeded':
      return 'ready'
    case 'hidden':
      return 'hidden'
    case 'deleted':
      return 'deleted'
    default:
      return null
  }
}

function normalizeArtworkListParams(filters?: UserAiArtworkListFilters): Record<string, unknown> {
  const params: Record<string, unknown> = {}
  const search = asNullableString(filters?.search)
  const visibility = asString(filters?.visibility).trim()
  const status = normalizeArtworkFilterStatus(filters?.status)
  const groupID = typeof filters?.line_id === 'number' && filters.line_id > 0 ? filters.line_id : null

  if (search) {
    params.search = search
  }
  if (visibility === 'public' || visibility === 'private') {
    params.visibility = visibility
  }
  if (status) {
    params.status = status
  }
  if (groupID) {
    params.group_id = groupID
  }
  return params
}

export async function listPrompts(
  page = 1,
  pageSize = 24,
  filters?: UserAiPromptListFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<AiPromptTemplate>> {
  const { data } = await apiClient.get('/user/ai/prompt-templates', {
    params: {
      page,
      page_size: pageSize,
      ...normalizePromptListParams(filters)
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizePrompt)
}

export async function createPrompt(request: CreateAiPromptRequest): Promise<AiPromptTemplate> {
  const { data } = await apiClient.post('/user/ai/prompts', request)
  return normalizePrompt(data)
}

export async function updatePrompt(id: number, request: UpdateAiPromptRequest): Promise<AiPromptTemplate> {
  const { data } = await apiClient.put(`/user/ai/prompts/${id}`, request)
  return normalizePrompt(data)
}

export async function deletePrompt(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete(`/user/ai/prompts/${id}`)
  return data as { message: string }
}

export async function clonePrompt(id: number): Promise<AiPromptTemplate> {
  const { data } = await apiClient.post(`/user/ai/prompts/${id}/clone`)
  return normalizePrompt(data)
}

export async function listArtworks(
  page = 1,
  pageSize = 48,
  filters?: UserAiArtworkListFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<AiArtwork>> {
  const { data } = await apiClient.get('/user/ai/gallery', {
    params: {
      page,
      page_size: pageSize,
      ...normalizeArtworkListParams(filters)
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeArtwork)
}

export async function createArtwork(request: CreateAiArtworkRequest): Promise<AiArtwork> {
  const { data } = await apiClient.post('/user/ai/artworks', request)
  return normalizeArtwork(data)
}

export async function editArtwork(request: CreateAiArtworkRequest): Promise<AiArtwork> {
  const { data } = await apiClient.post('/user/ai/artworks/edit', request)
  return normalizeArtwork(data)
}

export async function updateArtwork(id: number, request: UpdateAiArtworkRequest): Promise<AiArtwork> {
  const { data } = await apiClient.put(`/user/ai/artworks/${id}`, request)
  return normalizeArtwork(data)
}

export async function deleteArtwork(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete(`/user/ai/artworks/${id}`)
  return data as { message: string }
}

export async function chat(request: CreateAiChatRequest & { use_responses?: boolean }): Promise<AiChatResponse> {
  const { data } = await apiClient.post('/user/ai/chat', request)
  const message = isRecord(data) && 'message' in data ? normalizeChatMessage(data.message) : normalizeChatMessage(data)
  return { message }
}

const aiAPI = {
  listPrompts,
  createPrompt,
  updatePrompt,
  deletePrompt,
  clonePrompt,
  listArtworks,
  createArtwork,
  editArtwork,
  updateArtwork,
  deleteArtwork,
  chat,
  loadRuntimeLines,
  getRuntimeInfo
}

export default aiAPI
