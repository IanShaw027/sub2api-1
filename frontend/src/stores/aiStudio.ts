import { computed, reactive, ref } from 'vue'
import { defineStore } from 'pinia'
import { apiClient } from '@/api/client'
import { getRuntimeInfo, type AiRuntimeInfo } from '@/api/ai'
import type { AiChatMessage, AiLineOption, BasePaginationResponse } from '@/types'
import { getSessionUser } from '@/utils/authSession'

const SELECTED_LINE_KEY = 'sub2api_ai_selected_line_v1'
const SELECTED_KEY_BY_LINE_KEY = 'sub2api_ai_selected_key_by_line_v1'

export interface AiChatSessionSummary {
  id: number
  title: string
  status: string
  line_id: number | null
  last_message_at: string | null
  created_at: string
  updated_at: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null
}

function asString(value: unknown, fallback = ''): string {
  if (typeof value === 'string') return value
  if (typeof value === 'number' && Number.isFinite(value)) return String(value)
  return fallback
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

  const dataRecord = resolveNestedRecord(raw, ['data', 'result', 'pagination'])
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

function normalizeChatSession(raw: unknown): AiChatSessionSummary {
  const source = isRecord(raw) ? raw : {}
  const trace = resolveNestedRecord(source, ['trace'])
  const createdAt = asString(pickFirstValue(source, ['created_at']), new Date().toISOString())
  return {
    id: asNumber(pickFirstValue(source, ['id']), Date.now()),
    title: asString(pickFirstValue(source, ['title']), '未命名会话'),
    status: asString(pickFirstValue(source, ['status']), 'active'),
    line_id: asNullableNumber(pickFirstValue({ ...trace, ...source }, ['line_id', 'group_id'])),
    last_message_at: asNullableString(pickFirstValue(source, ['last_message_at'])),
    created_at: createdAt,
    updated_at: asString(pickFirstValue(source, ['updated_at']), createdAt)
  }
}

function normalizeChatMessage(raw: unknown): AiChatMessage {
  const source = isRecord(raw) ? raw : {}
  const trace = resolveNestedRecord(source, ['trace'])
  const role = asString(pickFirstValue(source, ['role']), 'assistant')
  return {
    id: asString(pickFirstValue(source, ['id']), `message-${Date.now()}`),
    role: role === 'system' || role === 'user' ? role : 'assistant',
    content: asString(pickFirstValue(source, ['content'])),
    created_at: asString(pickFirstValue(source, ['created_at']), new Date().toISOString()),
    line_id: asNullableNumber(pickFirstValue({ ...trace, ...source }, ['line_id', 'group_id'])),
    line_name: asNullableString(pickFirstValue(source, ['line_name', 'group_name'])),
    model: asNullableString(pickFirstValue(source, ['model']))
  }
}

function createEmptyPagination<T>(pageSize: number): BasePaginationResponse<T> {
  return {
    items: [],
    total: 0,
    page: 1,
    page_size: pageSize,
    pages: 1
  }
}

function storageScopeSuffix(userId: number | null): string {
  return userId && userId > 0 ? `user_${userId}` : 'anonymous'
}

function scopedSelectedLineKey(userId: number | null): string {
  return `${SELECTED_LINE_KEY}:${storageScopeSuffix(userId)}`
}

function scopedSelectedKeyMapKey(userId: number | null): string {
  return `${SELECTED_KEY_BY_LINE_KEY}:${storageScopeSuffix(userId)}`
}

function readStoredLineId(userId: number | null): number | null {
  try {
    const raw = localStorage.getItem(scopedSelectedLineKey(userId))
    if (!raw) return null
    const value = Number(raw)
    return Number.isFinite(value) && value > 0 ? value : null
  } catch {
    return null
  }
}

function persistStoredLineId(lineId: number | null, userId: number | null): void {
  try {
    const storageKey = scopedSelectedLineKey(userId)
    if (lineId === null) {
      localStorage.removeItem(storageKey)
    } else {
      localStorage.setItem(storageKey, String(lineId))
    }
  } catch {
    // ignore persistence failures
  }
}

function readStoredKeyMap(userId: number | null): Record<string, number> {
  try {
    const raw = localStorage.getItem(scopedSelectedKeyMapKey(userId))
    if (!raw) return {}
    const parsed = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object') return {}
    return Object.fromEntries(
      Object.entries(parsed).flatMap(([lineId, value]) => {
        const keyId = typeof value === 'number' ? value : Number(value)
        return Number.isFinite(keyId) && keyId > 0 ? [[lineId, keyId]] : []
      })
    )
  } catch {
    return {}
  }
}

function persistStoredKeyMap(map: Record<string, number>, userId: number | null): void {
  try {
    localStorage.setItem(scopedSelectedKeyMapKey(userId), JSON.stringify(map))
  } catch {
    // ignore persistence failures
  }
}

function getCurrentUserId(): number | null {
  const id = getSessionUser()?.id
  return typeof id === 'number' && Number.isFinite(id) ? id : null
}

export const useAiStudioStore = defineStore('aiStudio', () => {
  const scopedUserId = ref<number | null>(getCurrentUserId())
  const lines = ref<AiLineOption[]>([])
  const selectedLineId = ref<number | null>(readStoredLineId(scopedUserId.value))
  const selectedKeyId = ref<number | null>(null)
  const selectedKeyMap = ref<Record<string, number>>(readStoredKeyMap(scopedUserId.value))
  const loadingLines = ref(false)
  const loadingRuntime = ref(false)
  const loaded = ref(false)
  const loadedForUserId = ref<number | null>(null)
  const lastLoadedAt = ref<string | null>(null)
  const runtimeInfo = ref<AiRuntimeInfo | null>(null)
  const runtimeLoadedAt = ref<string | null>(null)
  const runtimeLoadedForUserId = ref<number | null>(null)
  const chatSessions = reactive<BasePaginationResponse<AiChatSessionSummary>>(createEmptyPagination<AiChatSessionSummary>(8))
  const activeSessionId = ref<number | null>(null)
  const sessionMessages = ref<AiChatMessage[]>([])
  const loadingChatSessions = ref(false)
  const loadingSessionMessages = ref(false)
  const creatingSession = ref(false)

  function clearScopedRuntimeState(): void {
    lines.value = []
    loadingLines.value = false
    loadingRuntime.value = false
    loaded.value = false
    loadedForUserId.value = null
    lastLoadedAt.value = null
    runtimeInfo.value = null
    runtimeLoadedAt.value = null
    runtimeLoadedForUserId.value = null
    assignPagination(chatSessions, createEmptyPagination<AiChatSessionSummary>(chatSessions.page_size))
    activeSessionId.value = null
    sessionMessages.value = []
    loadingChatSessions.value = false
    loadingSessionMessages.value = false
    creatingSession.value = false
  }

  const availableLines = computed(() => lines.value.filter((line) => line.key_count > 0))
  const selectedLine = computed(() => availableLines.value.find((line) => line.group_id === selectedLineId.value) ?? null)
  const selectedLineLabel = computed(() => selectedLine.value?.label ?? '')
  const activeSession = computed(
    () => chatSessions.items.find((session) => session.id === activeSessionId.value) ?? null
  )

  function syncStorageScope(): number | null {
    const currentUserId = getCurrentUserId()
    if (currentUserId === scopedUserId.value) {
      return currentUserId
    }

    scopedUserId.value = currentUserId
    selectedLineId.value = readStoredLineId(currentUserId)
    selectedKeyMap.value = readStoredKeyMap(currentUserId)
    selectedKeyId.value = null
    clearScopedRuntimeState()
    return currentUserId
  }

  function setSelectedLine(lineId: number | null): void {
    const currentUserId = syncStorageScope()
    selectedLineId.value = lineId
    persistStoredLineId(lineId, currentUserId)
    syncSelectedKey()
  }

  function persistSelectedKeyForLine(lineId: number | null, keyId: number | null): void {
    const currentUserId = syncStorageScope()
    if (!lineId || !keyId) return
    const next = { ...selectedKeyMap.value, [String(lineId)]: keyId }
    selectedKeyMap.value = next
    persistStoredKeyMap(next, currentUserId)
  }

  function isKeyInLine(line: AiLineOption | null, keyId: number | null): boolean {
    if (!line || !keyId) return false
    return line.keys.some((item) => item.id === keyId) || line.key_ids.includes(keyId)
  }

  function defaultKeyForLine(line: AiLineOption | null): number | null {
    if (!line) return null
    return line.default_key_id ?? line.keys[0]?.id ?? line.key_ids[0] ?? null
  }

  function syncSelectedKey(): void {
    syncStorageScope()
    const current = selectedLine.value
    if (!current) {
      selectedKeyId.value = null
      return
    }
    const stored = selectedLineId.value ? selectedKeyMap.value[String(selectedLineId.value)] ?? null : null
    const nextKey = isKeyInLine(current, stored) ? stored : defaultKeyForLine(current)
    selectedKeyId.value = nextKey
    persistSelectedKeyForLine(current.group_id, nextKey)
  }

  function setLines(nextLines: AiLineOption[]): void {
    lines.value = [...nextLines]
  }

  async function loadRuntimeInfo(force = false): Promise<AiRuntimeInfo | null> {
    const currentUserId = syncStorageScope()
    if (!force && runtimeInfo.value && runtimeLoadedForUserId.value === currentUserId) {
      return runtimeInfo.value
    }
    loadingRuntime.value = true
    try {
      runtimeInfo.value = await getRuntimeInfo()
      runtimeLoadedAt.value = new Date().toISOString()
      runtimeLoadedForUserId.value = currentUserId
      return runtimeInfo.value
    } catch (error) {
      runtimeInfo.value = null
      runtimeLoadedAt.value = null
      runtimeLoadedForUserId.value = null
      throw error
    } finally {
      loadingRuntime.value = false
    }
  }

  function ensureSelection(): void {
    const currentUserId = syncStorageScope()
    if (!loaded.value && lines.value.length === 0 && runtimeInfo.value === null) {
      selectedKeyId.value = null
      return
    }
    if (availableLines.value.length === 0) {
      selectedLineId.value = null
      selectedKeyId.value = null
      persistStoredLineId(null, currentUserId)
      return
    }

    const persisted = selectedLineId.value
    const persistedLine = persisted ? availableLines.value.find((line) => line.group_id === persisted) : null
    const runtimeDefaultLineId = runtimeInfo.value?.default_line?.group_id ?? null
    const runtimeDefaultLine = runtimeDefaultLineId
      ? availableLines.value.find((line) => line.group_id === runtimeDefaultLineId) ?? null
      : null
    const nextLine = persistedLine ?? runtimeDefaultLine ?? availableLines.value[0]
    selectedLineId.value = nextLine.group_id
    persistStoredLineId(nextLine.group_id, currentUserId)
    const storedKey = selectedKeyMap.value[String(nextLine.group_id)] ?? null
    const nextKey = isKeyInLine(nextLine, storedKey) ? storedKey : defaultKeyForLine(nextLine)
    selectedKeyId.value = nextKey
    persistSelectedKeyForLine(nextLine.group_id, nextKey)
  }

  async function loadRuntimeLines(force = false): Promise<AiLineOption[]> {
    const currentUserId = syncStorageScope()
    if (loaded.value && !force && loadedForUserId.value === currentUserId) {
      ensureSelection()
      return availableLines.value
    }

    loadingLines.value = true
    try {
      const runtime = await loadRuntimeInfo(force)
      setLines(runtime?.lines ?? [])
      loaded.value = true
      loadedForUserId.value = currentUserId
      lastLoadedAt.value = new Date().toISOString()
      ensureSelection()
      return availableLines.value
    } finally {
      loadingLines.value = false
    }
  }

  function assignPagination<T>(
    target: BasePaginationResponse<T>,
    next: BasePaginationResponse<T>
  ): void {
    target.items = next.items
    target.total = next.total
    target.page = next.page
    target.page_size = next.page_size
    target.pages = next.pages
  }

  function setActiveSession(sessionId: number | null): void {
    activeSessionId.value = sessionId
    if (sessionId === null) {
      sessionMessages.value = []
    }
  }

  function upsertChatSession(session: AiChatSessionSummary): void {
    const index = chatSessions.items.findIndex((item) => item.id === session.id)
    if (index >= 0) {
      chatSessions.items.splice(index, 1, session)
      return
    }
    chatSessions.items.unshift(session)
    chatSessions.total = Math.max(chatSessions.total + 1, chatSessions.items.length)
  }

  async function loadChatSessions(
    page = chatSessions.page,
    pageSize = chatSessions.page_size
  ): Promise<AiChatSessionSummary[]> {
    loadingChatSessions.value = true
    try {
      const { data } = await apiClient.get('/user/ai/sessions', {
        params: {
          page,
          page_size: pageSize
        }
      })
      const paged = normalizePagedResponse(data, page, pageSize, normalizeChatSession)
      assignPagination(chatSessions, paged)
      return paged.items
    } finally {
      loadingChatSessions.value = false
    }
  }

  async function createChatSession(title: string): Promise<AiChatSessionSummary> {
    creatingSession.value = true
    try {
      const { data } = await apiClient.post('/user/ai/sessions', {
        title: title.trim() || '未命名会话'
      })
      const session = normalizeChatSession(data)
      upsertChatSession(session)
      setActiveSession(session.id)
      return session
    } finally {
      creatingSession.value = false
    }
  }

  async function loadChatSession(sessionId: number): Promise<AiChatMessage[]> {
    setActiveSession(sessionId)
    loadingSessionMessages.value = true
    try {
      const [{ data: sessionData }, { data: messageData }] = await Promise.all([
        apiClient.get(`/user/ai/sessions/${sessionId}`),
        apiClient.get(`/user/ai/sessions/${sessionId}/messages`, {
          params: {
            page: 1,
            page_size: 200,
            sort_by: 'created_at',
            sort_order: 'asc'
          }
        })
      ])
      upsertChatSession(normalizeChatSession(sessionData))
      const pagedMessages = normalizePagedResponse(messageData, 1, 200, normalizeChatMessage)
      sessionMessages.value = pagedMessages.items
      return sessionMessages.value
    } finally {
      loadingSessionMessages.value = false
    }
  }

  function reset(): void {
    const currentUserId = syncStorageScope()
    selectedLineId.value = null
    selectedKeyId.value = null
    selectedKeyMap.value = {}
    clearScopedRuntimeState()
    persistStoredLineId(null, currentUserId)
    persistStoredKeyMap({}, currentUserId)
  }

  function setSelectedKey(keyId: number | null): void {
    syncStorageScope()
    selectedKeyId.value = keyId
    persistSelectedKeyForLine(selectedLineId.value, keyId)
  }

  return {
    lines,
    selectedLineId,
    selectedKeyId,
    loadingLines,
    loadingRuntime,
    loaded,
    lastLoadedAt,
    runtimeInfo,
    runtimeLoadedAt,
    availableLines,
    selectedLine,
    selectedLineLabel,
    chatSessions,
    activeSessionId,
    activeSession,
    sessionMessages,
    loadingChatSessions,
    loadingSessionMessages,
    creatingSession,
    loadRuntimeInfo,
    loadRuntimeLines,
    loadChatSessions,
    createChatSession,
    loadChatSession,
    setSelectedLine,
    setSelectedKey,
    setActiveSession,
    syncSelectedKey,
    ensureSelection,
    reset
  }
})
