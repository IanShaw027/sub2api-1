import { apiClient, buildApiUrl } from '@/api/client'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import type { GroupPlatform } from '@/types'
import { parseSSEBuffer } from './sse'
import {
  ANTHROPIC_STYLE_PLATFORMS,
  type AsyncImageTask,
  type CreationImageJob,
  type CreationImageListResponse,
  type CreationMessage,
  type CreationSession,
  type CreationSessionListResponse,
  type CreationSessionMode,
  type GatewayModelList,
} from './types'

const basePath = '/creation'
const DEFAULT_MAX_TOKENS = 4096

function authHeaders(groupId: number, sessionId?: number): Record<string, string> {
  const token = localStorage.getItem('auth_token')
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (token) headers.Authorization = `Bearer ${token}`
  headers['X-Group-Id'] = String(groupId)
  if (sessionId != null) headers['X-Session-Id'] = String(sessionId)
  return headers
}

export async function listSessions(params?: {
  mode?: CreationSessionMode
  page?: number
  page_size?: number
}): Promise<CreationSessionListResponse> {
  const { data } = await apiClient.get<CreationSessionListResponse>(`${basePath}/sessions`, { params })
  return data
}

export async function createSession(payload: {
  group_id: number
  title?: string
  model?: string
  mode: CreationSessionMode
}): Promise<CreationSession> {
  const { data } = await apiClient.post<CreationSession>(`${basePath}/sessions`, payload)
  return data
}

export async function updateSession(
  id: number,
  payload: Partial<Pick<CreationSession, 'title' | 'model' | 'status'>>,
): Promise<CreationSession> {
  const { data } = await apiClient.patch<CreationSession>(`${basePath}/sessions/${id}`, payload)
  return data
}

export async function deleteSession(id: number): Promise<void> {
  await apiClient.delete(`${basePath}/sessions/${id}`)
}

export async function listSessionMessages(sessionId: number): Promise<CreationMessage[]> {
  const { data } = await apiClient.get<CreationMessage[]>(`${basePath}/sessions/${sessionId}/messages`)
  return data
}

export async function createSessionMessage(
  sessionId: number,
  payload: { role: string; content: unknown; model?: string },
): Promise<CreationMessage> {
  const { data } = await apiClient.post<CreationMessage>(`${basePath}/sessions/${sessionId}/messages`, payload)
  return data
}

function asOptionalString(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  const trimmed = value.trim()
  return trimmed.length > 0 ? trimmed : undefined
}

export function mapCreationImageJob(raw: unknown): CreationImageJob {
  const record = (raw && typeof raw === 'object' ? raw : {}) as Record<string, unknown>
  const mediaUrl =
    asOptionalString(record.media_url) ??
    asOptionalString(record.mediaUrl) ??
    undefined

  return {
    id: Number(record.id) || 0,
    session_id: record.session_id == null ? null : Number(record.session_id),
    user_id: Number(record.user_id) || 0,
    group_id: Number(record.group_id) || 0,
    status:
      record.status === 'completed' ||
      record.status === 'failed' ||
      record.status === 'processing' ||
      record.status === 'pending'
        ? record.status
        : 'pending',
    model: String(record.model ?? ''),
    prompt: String(record.prompt ?? ''),
    media_asset_id: record.media_asset_id == null ? null : Number(record.media_asset_id),
    provider_task_id:
      typeof record.provider_task_id === 'string'
        ? record.provider_task_id
        : record.provider_task_id == null
          ? null
          : String(record.provider_task_id),
    error: typeof record.error === 'string' ? record.error : record.error == null ? null : String(record.error),
    created_at: String(record.created_at ?? ''),
    updated_at: String(record.updated_at ?? ''),
    media_url: mediaUrl,
  }
}

export async function listImages(params?: {
  session_id?: number
  page?: number
  page_size?: number
}): Promise<CreationImageListResponse> {
  const { data } = await apiClient.get<CreationImageListResponse>(`${basePath}/images`, { params })
  const items = Array.isArray(data?.items) ? data.items.map(mapCreationImageJob) : []
  return {
    items,
    total: data?.total ?? items.length,
    page: data?.page ?? 1,
    page_size: data?.page_size ?? items.length,
  }
}

export async function getModels(groupId: number): Promise<GatewayModelList> {
  const { data } = await apiClient.get<GatewayModelList>(`${basePath}/models`, {
    params: { group_id: groupId },
  })
  return data
}

export async function submitImageGenerationAsync(
  groupId: number,
  sessionId: number,
  body: Record<string, unknown>,
): Promise<AsyncImageTask> {
  const response = await authenticatedFetch(buildApiUrl(`${basePath}/images/generations/async?group_id=${groupId}`), {
    method: 'POST',
    headers: authHeaders(groupId, sessionId),
    body: JSON.stringify(body),
  })
  if (!response.ok) {
    const errText = await response.text()
    throw new Error(errText || `Image generation failed (${response.status})`)
  }
  return (await response.json()) as AsyncImageTask
}

export async function getImageTask(groupId: number, taskId: string): Promise<AsyncImageTask> {
  const response = await authenticatedFetch(
    buildApiUrl(`${basePath}/images/tasks/${encodeURIComponent(taskId)}?group_id=${groupId}`),
    {
      method: 'GET',
      headers: authHeaders(groupId),
    },
  )
  if (!response.ok) {
    const errText = await response.text()
    throw new Error(errText || `Failed to poll image task (${response.status})`)
  }
  return (await response.json()) as AsyncImageTask
}

function usesMessagesEndpoint(platform: GroupPlatform): boolean {
  return ANTHROPIC_STYLE_PLATFORMS.has(platform)
}

function buildChatHistory(messages: CreationMessage[]): Array<{ role: string; content: string }> {
  return messages
    .filter((msg) => msg.role === 'user' || msg.role === 'assistant')
    .map((msg) => ({
      role: msg.role,
      content: extractMessageText(msg.content),
    }))
    .filter((msg) => msg.content.trim().length > 0)
}

export function extractMessageText(content: unknown): string {
  if (typeof content === 'string') {
    try {
      const parsed = JSON.parse(content)
      if (typeof parsed === 'string') return parsed
      if (Array.isArray(parsed)) {
        return parsed
          .map((block) => {
            if (typeof block === 'string') return block
            if (block && typeof block === 'object') {
              const record = block as Record<string, unknown>
              if (typeof record.text === 'string') return record.text
              if (typeof record.content === 'string') return record.content
            }
            return ''
          })
          .join('')
      }
      if (parsed && typeof parsed === 'object' && 'text' in parsed) {
        return String((parsed as { text: string }).text)
      }
    } catch {
      return content
    }
    return content
  }

  if (Array.isArray(content)) {
    return content
      .map((block) => {
        if (typeof block === 'string') return block
        if (block && typeof block === 'object') {
          const record = block as Record<string, unknown>
          if (typeof record.text === 'string') return record.text
        }
        return ''
      })
      .join('')
  }

  if (content && typeof content === 'object' && 'text' in content) {
    return String((content as { text: string }).text)
  }

  return String(content ?? '')
}

export async function streamChatCompletions(options: {
  groupId: number
  sessionId: number
  model: string
  messages: CreationMessage[]
  userText: string
  signal?: AbortSignal
  onDelta: (text: string) => void
}): Promise<string> {
  const history = buildChatHistory(options.messages)
  history.push({ role: 'user', content: options.userText })

  const response = await authenticatedFetch(
    buildApiUrl(`${basePath}/chat/completions?group_id=${options.groupId}`),
    {
      method: 'POST',
      headers: authHeaders(options.groupId, options.sessionId),
      body: JSON.stringify({
        model: options.model,
        stream: true,
        messages: history,
      }),
      signal: options.signal,
    },
  )

  if (!response.ok) {
    const errText = await response.text()
    throw new Error(errText || `Chat request failed (${response.status})`)
  }

  return readSSEText(response, options.onDelta)
}

export async function streamMessages(options: {
  groupId: number
  sessionId: number
  platform: GroupPlatform
  model: string
  messages: CreationMessage[]
  userText: string
  signal?: AbortSignal
  onDelta: (text: string) => void
}): Promise<string> {
  const history = buildChatHistory(options.messages)
  history.push({ role: 'user', content: options.userText })

  const response = await authenticatedFetch(buildApiUrl(`${basePath}/messages?group_id=${options.groupId}`), {
    method: 'POST',
    headers: authHeaders(options.groupId, options.sessionId),
    body: JSON.stringify({
      model: options.model,
      max_tokens: DEFAULT_MAX_TOKENS,
      stream: true,
      messages: history.map((msg) => ({
        role: msg.role,
        content: msg.content,
      })),
    }),
    signal: options.signal,
  })

  if (!response.ok) {
    const errText = await response.text()
    throw new Error(errText || `Messages request failed (${response.status})`)
  }

  return readSSEText(response, options.onDelta)
}

export async function streamCreationChat(options: {
  groupId: number
  sessionId: number
  platform: GroupPlatform
  model: string
  messages: CreationMessage[]
  userText: string
  signal?: AbortSignal
  onDelta: (text: string) => void
}): Promise<string> {
  if (usesMessagesEndpoint(options.platform)) {
    return streamMessages(options)
  }
  return streamChatCompletions(options)
}

async function readSSEText(
  response: Response,
  onDelta: (text: string) => void,
): Promise<string> {
  const reader = response.body?.getReader()
  if (!reader) throw new Error('No response body')

  const decoder = new TextDecoder()
  let buffer = ''
  let fullText = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })
    const { deltas, remainder } = parseSSEBuffer(buffer)
    buffer = remainder
    for (const delta of deltas) {
      fullText += delta
      onDelta(delta)
    }
  }

  if (buffer.trim()) {
    const { deltas } = parseSSEBuffer(`${buffer}\n`)
    for (const delta of deltas) {
      fullText += delta
      onDelta(delta)
    }
  }

  return fullText
}

export function extractImageUrlFromTask(task: AsyncImageTask): string | undefined {
  if (task.image_url) return task.image_url
  const result = task.result
  if (!result) return undefined
  if (typeof result === 'string') {
    try {
      return extractImageUrlFromTask({ ...task, result: JSON.parse(result) })
    } catch {
      return undefined
    }
  }
  if (typeof result === 'object' && result !== null) {
    const record = result as Record<string, unknown>
    if (typeof record.url === 'string') return record.url
    const data = record.data
    if (Array.isArray(data) && data[0] && typeof data[0] === 'object') {
      const first = data[0] as Record<string, unknown>
      if (typeof first.url === 'string') return first.url
      if (typeof first.b64_json === 'string') return `data:image/png;base64,${first.b64_json}`
    }
  }
  return undefined
}

export function mapAsyncTaskToImageJob(
  task: AsyncImageTask,
  sessionId: number,
  groupId: number,
  model: string,
  prompt: string,
): CreationImageJob {
  const status =
    task.status === 'completed'
      ? 'completed'
      : task.status === 'failed'
        ? 'failed'
        : task.status === 'processing'
          ? 'processing'
          : 'pending'

  return {
    id: Number(task.task_id?.replace(/\D/g, '').slice(0, 12) || Date.now()),
    session_id: sessionId,
    user_id: 0,
    group_id: groupId,
    status,
    model,
    prompt,
    error: typeof task.error === 'string' ? task.error : null,
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    media_url: extractImageUrlFromTask(task) ?? undefined,
    provider_task_id: task.task_id,
  }
}

export const creationAPI = {
  listSessions,
  createSession,
  updateSession,
  deleteSession,
  listSessionMessages,
  createSessionMessage,
  listImages,
  getModels,
  submitImageGenerationAsync,
  getImageTask,
  streamCreationChat,
  extractMessageText,
  mapCreationImageJob,
}

export default creationAPI
