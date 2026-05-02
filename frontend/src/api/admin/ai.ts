import { apiClient } from '../client'
import type {
  AiArtwork,
  AiArtworkListFilters,
  AiArtworkStatus,
  AiPromptListFilters,
  AiPromptTemplate,
  BasePaginationResponse,
  UpdateAiArtworkRequest,
  UpdateAiPromptRequest
} from '@/types'

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function asString(value: unknown, fallback = ''): string {
  return typeof value === 'string' ? value : fallback
}

function asNumber(value: unknown, fallback = 0): number {
  return typeof value === 'number' && Number.isFinite(value) ? value : fallback
}

function asNullableNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function asNullableString(value: unknown): string | null {
  return typeof value === 'string' && value.trim() ? value : null
}

function traceGroupID(value: unknown): number | null {
  if (!isRecord(value)) return null
  return asNullableNumber(value.group_id)
}

function asStringArray(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function asBoolean(value: unknown, fallback = false): boolean {
  return typeof value === 'boolean' ? value : fallback
}

type AdminAiArtworkFilterStatus = AiArtworkListFilters['status'] | 'ready'
type AdminAiArtworkListFilters = Omit<AiArtworkListFilters, 'status'> & {
  status?: AdminAiArtworkFilterStatus
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

  return { items: [], total: 0, page, page_size: pageSize, pages: 1 }
}

function promptStatusFromRaw(source: Record<string, unknown>): string {
  const status = asString(source.status, '').trim().toLowerCase()
  if (status === 'draft' || status === 'published' || status === 'archived' || status === 'hidden') {
    return status
  }
  const moderation = asString(source.moderation_state, '').trim().toLowerCase()
  if (moderation === 'blocked') return 'hidden'
  if (moderation === 'forced_private') return 'archived'
  return asString(source.visibility, '') === 'public' ? 'published' : 'draft'
}

function normalizePrompt(raw: unknown): AiPromptTemplate {
  const source = isRecord(raw) ? raw : {}
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  return {
    id: asNumber(source.id, Date.now()),
    title: asString(source.title, 'Untitled Prompt'),
    content: asString(source.content),
    description: asNullableString(source.description ?? metadata.description),
    tags: asStringArray(source.tags ?? metadata.tags),
    visibility: asString(source.visibility, 'private') as 'public' | 'private',
    status: promptStatusFromRaw({ ...source, metadata }) as any,
    line_id: asNullableNumber(source.group_id ?? source.line_id) ?? traceGroupID(source.trace),
    line_name: asNullableString(source.group_name ?? source.line_name),
    owner_id: asNullableNumber(source.user_id ?? source.owner_id),
    owner_name: asNullableString(metadata.owner_name ?? source.owner_name),
    is_mine: false,
    cloned_from_id: asNullableNumber(metadata.cloned_from_id),
    usage_count: asNumber(metadata.usage_count, 0),
    featured: asBoolean(metadata.featured, false),
    created_at: asString(source.created_at, new Date().toISOString()),
    updated_at: asString(source.updated_at, asString(source.created_at, new Date().toISOString()))
  }
}

function normalizeArtwork(raw: unknown): AiArtwork {
  const source = isRecord(raw) ? raw : {}
  const metadata = isRecord(source.metadata) ? source.metadata : {}
  const imageUrl = asString(source.source_url || source.image_url || metadata.image_url)
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
    visibility: asString(source.visibility, 'public') as 'public' | 'private',
    status: normalizedStatus,
    image_url: imageUrl,
    thumbnail_url: asNullableString(metadata.thumbnail_url ?? source.thumbnail_url) ?? imageUrl,
    line_id: asNullableNumber(source.group_id ?? source.line_id) ?? traceGroupID(source.trace),
    line_name: asNullableString(source.group_name ?? source.line_name),
    owner_id: asNullableNumber(source.user_id ?? source.owner_id),
    owner_name: asNullableString(metadata.owner_name ?? source.owner_name),
    width: asNullableNumber(source.width),
    height: asNullableNumber(source.height),
    size: asNullableString(metadata.size ?? source.size),
    style: asNullableString(metadata.style ?? source.style),
    tags: asStringArray(metadata.tags ?? source.tags),
    featured: asBoolean(metadata.featured, false),
    prompt_template_id: asNullableNumber(source.prompt_template_id),
    likes: asNumber(metadata.likes, 0),
    views: asNumber(metadata.views, 0),
    created_at: asString(source.created_at, new Date().toISOString()),
    updated_at: asString(source.updated_at, asString(source.created_at, new Date().toISOString()))
  }
}

export async function listPrompts(
  page = 1,
  pageSize = 20,
  filters?: AiPromptListFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<AiPromptTemplate>> {
  const status = typeof filters?.status === 'string' ? filters.status.trim().toLowerCase() : ''
  const { data } = await apiClient.get('/admin/ai/prompts', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.visibility && filters.visibility !== 'all' ? { visibility: filters.visibility } : {}),
      ...(status && status !== 'all' ? { status } : {}),
      ...(typeof filters?.line_id === 'number' ? { line_id: filters.line_id } : {}),
      ...(filters?.mine_only ? { mine_only: true } : {})
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizePrompt)
}

export async function updatePrompt(id: number, request: UpdateAiPromptRequest): Promise<AiPromptTemplate> {
  const { data } = await apiClient.put(`/admin/ai/prompts/${id}`, request)
  return normalizePrompt(data)
}

export async function deletePrompt(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete(`/admin/ai/prompts/${id}`)
  return data as { message: string }
}

export async function listArtworks(
  page = 1,
  pageSize = 24,
  filters?: AdminAiArtworkListFilters,
  options?: { signal?: AbortSignal }
): Promise<BasePaginationResponse<AiArtwork>> {
  const status = typeof filters?.status === 'string' ? filters.status.trim().toLowerCase() : ''
  const normalizedStatus = status === 'succeeded' || status === 'ready'
    ? 'ready'
    : status === 'pending' || status === 'hidden' || status === 'deleted'
      ? status
      : undefined
  const { data } = await apiClient.get('/admin/ai/artworks', {
    params: {
      page,
      page_size: pageSize,
      ...(filters?.search ? { search: filters.search } : {}),
      ...(filters?.visibility && filters.visibility !== 'all' ? { visibility: filters.visibility } : {}),
      ...(normalizedStatus ? { status: normalizedStatus } : {}),
      ...(typeof filters?.line_id === 'number' ? { line_id: filters.line_id } : {}),
      ...(typeof filters?.featured === 'boolean' ? { featured: filters.featured } : {})
    },
    signal: options?.signal
  })
  return normalizePagedResponse(data, page, pageSize, normalizeArtwork)
}

export async function updateArtwork(id: number, request: UpdateAiArtworkRequest): Promise<AiArtwork> {
  const { data } = await apiClient.put(`/admin/ai/artworks/${id}`, request)
  return normalizeArtwork(data)
}

export async function deleteArtwork(id: number): Promise<{ message: string }> {
  const { data } = await apiClient.delete(`/admin/ai/artworks/${id}`)
  return data as { message: string }
}

const adminAIAPI = {
  listPrompts,
  updatePrompt,
  deletePrompt,
  listArtworks,
  updateArtwork,
  deleteArtwork
}

export default adminAIAPI
