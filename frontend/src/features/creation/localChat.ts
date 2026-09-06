import type { GroupPlatform } from '@/types'
import type { CreationMessage, CreationSession, CreationTokenUsage } from './types'

export interface ChatToolDefinition {
  name: string
  description: string
  inputSchema: Record<string, unknown>
}
export interface ChatToolCall {
  id: string
  name: string
  arguments: Record<string, unknown>
}
export interface ChatToolResult {
  toolCallId: string
  name: string
  content: string
  data?: unknown
  isError?: boolean
}
export interface ChatSource { url: string; title?: string }
export type ChatThinkingBlock = { type: 'thinking'; thinking: string; signature: string } | { type: 'redacted_thinking'; data: string }
export interface ChatTurn {
  content: string
  toolCalls: ChatToolCall[]
  toolResults: ChatToolResult[]
  thinkingBlocks?: ChatThinkingBlock[]
}
export interface ChatSettings {
  temperature?: number
  maxTokens?: number
  systemPrompt?: string
  reasoningEffort?: 'auto' | 'none' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max'
}
export interface LocalChatMessage {
  id: string
  role: 'user' | 'assistant' | 'system'
  content: string
  files: File[]
  createdAt: string
  model?: string
  status: 'streaming' | 'completed' | 'stopped' | 'error'
  input_tokens?: number
  output_tokens?: number
  usage?: CreationTokenUsage
  error?: string
  reasoning?: string
  sources?: ChatSource[]
  toolCalls?: ChatToolCall[]
  toolResults?: ChatToolResult[]
  turns?: ChatTurn[]
  vote?: 'up' | 'down' | null
  rawContent?: unknown
  legacyMessage?: CreationMessage
}
export type ChatMessage = LocalChatMessage
export interface LocalChatDraft { text: string; files: File[] }
export interface LocalChatSession {
  id: string
  userId: number
  title: string
  groupId: number
  model: string
  platform: GroupPlatform
  settings: ChatSettings
  createdAt: string
  updatedAt: string
  messages: LocalChatMessage[]
  draft: LocalChatDraft
  persisted: boolean
  branchOf?: string
  legacySessionId?: number
  legacySession?: CreationSession
}

export function createLocalChatId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export class LocalChatStorageError extends Error {
  constructor(public readonly code: 'quota' | 'unavailable' | 'write', public readonly cause?: unknown) {
    super(code === 'quota'
      ? 'Browser storage is full. This conversation is not saved. Export it or free local storage before leaving.'
      : code === 'unavailable'
        ? 'Browser storage is unavailable. Enable IndexedDB to save local conversations.'
        : 'Could not save this conversation locally. Export it before leaving and retry saving, not generating.')
    this.name = 'LocalChatStorageError'
  }
}

function storageError(error: unknown): LocalChatStorageError {
  if (error instanceof LocalChatStorageError) return error
  return new LocalChatStorageError(error && typeof error === 'object' && 'name' in error && error.name === 'QuotaExceededError' ? 'quota' : 'write', error)
}

function jsonSnapshot<T>(value: T): T {
  return value === undefined ? value : JSON.parse(JSON.stringify(value)) as T
}

export function snapshotChatSession(session: LocalChatSession): LocalChatSession {
  return {
    ...session,
    settings: { ...session.settings },
    legacySession: jsonSnapshot(session.legacySession),
    draft: { text: session.draft.text, files: [...session.draft.files] },
    messages: session.messages.map(message => ({
      ...message, files: [...message.files], usage: message.usage ? { ...message.usage } : undefined,
      sources: jsonSnapshot(message.sources), toolCalls: jsonSnapshot(message.toolCalls),
      toolResults: jsonSnapshot(message.toolResults), turns: jsonSnapshot(message.turns), rawContent: jsonSnapshot(message.rawContent), legacyMessage: jsonSnapshot(message.legacyMessage),
    })),
  }
}

let database: Promise<IDBDatabase> | null = null
function openDatabase(): Promise<IDBDatabase> {
  if (database) return database
  database = new Promise<IDBDatabase>((resolve, reject) => {
    if (typeof indexedDB === 'undefined') { reject(new LocalChatStorageError('unavailable')); return }
    const request = indexedDB.open('sub2api-local-chat-v1', 1)
    request.onupgradeneeded = () => {
      const sessions = request.result.createObjectStore('sessions', { keyPath: ['userId', 'id'] })
      sessions.createIndex('userId', 'userId')
      request.result.createObjectStore('selection', { keyPath: 'userId' })
    }
    request.onsuccess = () => {
      request.result.onversionchange = () => { request.result.close(); database = null }
      resolve(request.result)
    }
    request.onerror = () => reject(new LocalChatStorageError('unavailable', request.error))
    request.onblocked = () => reject(new LocalChatStorageError('unavailable'))
  }).catch(error => { database = null; throw error })
  return database
}

async function read<T>(name: string, query: (store: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await openDatabase()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(name, 'readonly')
    const request = query(tx.objectStore(name))
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(storageError(request.error))
    tx.onabort = () => reject(storageError(tx.error))
  })
}
async function write(name: string, apply: (store: IDBObjectStore) => void): Promise<void> {
  try {
    const db = await openDatabase()
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(name, 'readwrite')
      tx.oncomplete = () => resolve()
      tx.onabort = () => reject(storageError(tx.error))
      tx.onerror = event => reject(storageError((event.target as IDBRequest | null)?.error ?? tx.error))
      try { apply(tx.objectStore(name)) } catch (error) { tx.abort(); reject(storageError(error)) }
    })
  } catch (error) { throw storageError(error) }
}

export const localChatStorage = {
  async listSessions(userId: number): Promise<LocalChatSession[]> {
    const result = await read<LocalChatSession[]>('sessions', store => store.index('userId').getAll(userId))
    return result.sort((a, b) => b.updatedAt.localeCompare(a.updatedAt))
  },
  async saveSession(session: LocalChatSession): Promise<void> {
    const snapshot = snapshotChatSession(session)
    await write('sessions', store => store.put({ ...snapshot, persisted: true }))
  },
  async deleteSession(userId: number, id: string): Promise<void> {
    await write('sessions', store => store.delete([userId, id]))
  },
  async getSelection(userId: number): Promise<string | null> {
    const result = await read<{ userId: number; sessionId: string | null } | undefined>('selection', store => store.get(userId))
    return result?.sessionId ?? null
  },
  async setSelection(userId: number, sessionId: string | null): Promise<void> {
    await write('selection', store => store.put({ userId, sessionId }))
  },
}
