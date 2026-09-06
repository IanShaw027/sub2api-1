export type MediaMode = 'image' | 'video'
export type VideoOperation = 'generation' | 'edit' | 'extension'
export type MediaTaskStatus = 'generating' | 'completed' | 'error'

export interface MediaCost {
  status: 'estimated' | 'settled' | 'pending' | 'unavailable' | 'not_billed'
  currency: 'USD'
  amount: number | null
  billing_target: 'balance' | 'subscription' | null
  reason?: string
  usage_id?: number
}

export function createLocalMediaId(): string {
  return globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export interface MediaSettings {
  size?: string
  quality?: string
  aspectRatio?: string
  ratio?: string
  resolution?: string
  duration?: number
  n?: number
}

export interface MediaRequest {
  mode: MediaMode
  videoOperation?: VideoOperation
  prompt: string
  model: string
  groupId: number
  settings: MediaSettings
  files?: File[]
  parentId?: string
}

export interface MediaTask {
  id: string
  userId: number
  mode: MediaMode
  // Legacy video type='edit' means image-to-video, not video editing.
  videoOperation?: VideoOperation
  sourceDuration?: number
  type: 'generation' | 'edit'
  parentId?: string
  prompt: string
  model: string
  groupId: number
  settings: MediaSettings
  status: MediaTaskStatus
  blob?: Blob
  mediaUrl?: string
  revisedPrompt?: string
  error?: string
  createdAt: string
  completedAt?: string
  providerTaskId?: string
  sourceFiles: File[]
  persisted: boolean
  legacyId?: number
  estimate?: MediaCost
  billing?: MediaCost
  observationPersisted?: boolean
  observationWarning?: string
}

export interface MediaDraft extends Omit<MediaRequest, 'mode'> {
  userId: number
  mode: MediaMode
  updatedAt: string
}

export class MediaStorageError extends Error {
  constructor(public readonly code: 'quota' | 'unavailable' | 'write', public readonly cause?: unknown) {
    super(code === 'quota'
      ? 'Browser storage is full. This media was not saved. Download it before leaving, or delete local tasks and save again.'
      : code === 'unavailable'
        ? 'Browser storage is unavailable. Enable local storage to use this workspace.'
        : 'Could not save media in this browser. Download the result before leaving and try saving again.')
    this.name = 'MediaStorageError'
  }
}

const DB_NAME = 'sub2api-local-media-v1'
let database: Promise<IDBDatabase> | null = null

function storageError(error: unknown): MediaStorageError {
  return error instanceof MediaStorageError ? error : new MediaStorageError(
    error && typeof error === 'object' && 'name' in error && error.name === 'QuotaExceededError' ? 'quota' : 'write',
    error,
  )
}

function openDatabase(): Promise<IDBDatabase> {
  if (database) return database
  database = new Promise<IDBDatabase>((resolve, reject) => {
    if (typeof indexedDB === 'undefined') {
      reject(new MediaStorageError('unavailable'))
      return
    }
    const request = indexedDB.open(DB_NAME, 1)
    request.onupgradeneeded = () => {
      const db = request.result
      const tasks = db.createObjectStore('tasks', { keyPath: ['userId', 'id'] })
      tasks.createIndex('userId', 'userId')
      db.createObjectStore('drafts', { keyPath: ['userId', 'mode'] })
    }
    request.onsuccess = () => {
      request.result.onversionchange = () => {
        request.result.close()
        database = null
      }
      resolve(request.result)
    }
    request.onerror = () => reject(new MediaStorageError('unavailable', request.error))
    request.onblocked = () => reject(new MediaStorageError('unavailable'))
  }).catch(error => {
    database = null
    throw error
  })
  return database
}

async function read<T>(store: string, query: (objectStore: IDBObjectStore) => IDBRequest<T>): Promise<T> {
  const db = await openDatabase()
  return new Promise((resolve, reject) => {
    const tx = db.transaction(store, 'readonly')
    const request = query(tx.objectStore(store))
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(storageError(request.error))
    tx.onabort = () => reject(storageError(tx.error))
  })
}

async function write(store: string, apply: (objectStore: IDBObjectStore) => void): Promise<void> {
  try {
    const db = await openDatabase()
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction(store, 'readwrite')
      tx.oncomplete = () => resolve()
      tx.onabort = () => reject(storageError(tx.error))
      tx.onerror = event => reject(storageError((event.target as IDBRequest | null)?.error ?? tx.error))
      try {
        apply(tx.objectStore(store))
      } catch (error) {
        tx.abort()
        reject(storageError(error))
      }
    })
  } catch (error) {
    throw storageError(error)
  }
}

export const localMedia = {
  async listTasks(userId: number): Promise<MediaTask[]> {
    const tasks = await read<MediaTask[]>('tasks', store => store.index('userId').getAll(userId))
    return tasks.sort((a, b) => b.createdAt.localeCompare(a.createdAt))
  },
  async saveTask(task: MediaTask): Promise<void> {
    // Never persist object URLs: they are tied to a document, not the stored Blob.
    const { mediaUrl: _url, ...stored } = task
    await write('tasks', store => store.put({ ...stored, persisted: true }))
  },
  async deleteTasks(userId: number, ids: string[]): Promise<void> {
    await write('tasks', store => ids.forEach(id => store.delete([userId, id])))
  },
  async loadDraft(userId: number, mode: MediaMode): Promise<MediaDraft | null> {
    return await read<MediaDraft | undefined>('drafts', store => store.get([userId, mode])) ?? null
  },
  async saveDraft(draft: MediaDraft): Promise<void> {
    await write('drafts', store => store.put(draft))
  },
}
