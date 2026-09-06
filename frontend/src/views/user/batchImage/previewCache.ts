import {
  PREVIEW_CACHE_DB_NAME,
  PREVIEW_CACHE_MAX_AGE_MS,
  PREVIEW_CACHE_MAX_BYTES,
  PREVIEW_CACHE_MAX_ENTRIES,
  PREVIEW_CACHE_STORE_NAME,
  PREVIEW_THUMBNAIL_MAX_EDGE,
  PREVIEW_THUMBNAIL_QUALITY,
} from './constants'
import type { PreviewCacheRecord, PreviewImageSource } from './types'

/**
 * IndexedDB-backed thumbnail cache for batch-image item previews.
 * Extracted from BatchImageGuideView.vue (glass-ui-redesign task 12.12).
 *
 * Kept as a factory (rather than module-level singletons) so each
 * BatchImageGuideView instance gets its own DB-connection promise, matching
 * the original per-component-instance behaviour exactly (important for test
 * isolation across multiple mounts).
 */
export function createPreviewCache() {
  let previewCacheDBPromise: Promise<IDBDatabase | null> | null = null

  function previewCacheSupported() {
    return typeof window !== 'undefined' && 'indexedDB' in window
  }

  function previewCacheKey(batchId: string, customID: string, imageIndex = 0) {
    return [batchId, customID, imageIndex].map(part => encodeURIComponent(String(part))).join(':')
  }

  function idbRequest<T>(request: IDBRequest<T>): Promise<T> {
    return new Promise((resolve, reject) => {
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => reject(request.error)
    })
  }

  function openPreviewCacheDB(): Promise<IDBDatabase | null> {
    if (!previewCacheSupported()) return Promise.resolve(null)
    if (previewCacheDBPromise) return previewCacheDBPromise

    previewCacheDBPromise = new Promise((resolve) => {
      const request = window.indexedDB.open(PREVIEW_CACHE_DB_NAME, 1)
      request.onupgradeneeded = () => {
        const db = request.result
        if (!db.objectStoreNames.contains(PREVIEW_CACHE_STORE_NAME)) {
          const store = db.createObjectStore(PREVIEW_CACHE_STORE_NAME, { keyPath: 'key' })
          store.createIndex('lastAccessedAt', 'lastAccessedAt', { unique: false })
        }
      }
      request.onsuccess = () => resolve(request.result)
      request.onerror = () => resolve(null)
      request.onblocked = () => resolve(null)
    })
    return previewCacheDBPromise
  }

  async function touchCachedPreview(cacheKey: string, lastAccessedAt: number) {
    const db = await openPreviewCacheDB()
    if (!db) return
    const record = await idbRequest<PreviewCacheRecord | undefined>(
      db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).get(cacheKey),
    ).catch(() => undefined)
    if (!record) return
    record.lastAccessedAt = lastAccessedAt
    await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).put(record)).catch(() => null)
  }

  async function deleteCachedPreview(cacheKey: string) {
    const db = await openPreviewCacheDB()
    if (!db) return
    await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).delete(cacheKey)).catch(() => null)
  }

  async function getCachedPreviewBlob(cacheKey: string): Promise<Blob | null> {
    const db = await openPreviewCacheDB()
    if (!db) return null
    const record = await idbRequest<PreviewCacheRecord | undefined>(
      db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).get(cacheKey),
    ).catch(() => undefined)
    if (!record?.blob) return null

    const now = Date.now()
    if (now - record.createdAt > PREVIEW_CACHE_MAX_AGE_MS) {
      void deleteCachedPreview(cacheKey)
      return null
    }
    void touchCachedPreview(cacheKey, now)
    return record.blob
  }

  async function putCachedPreviewBlob(cacheKey: string, blob: Blob) {
    const db = await openPreviewCacheDB()
    if (!db) return
    const now = Date.now()
    const record: PreviewCacheRecord = {
      key: cacheKey,
      blob,
      size: blob.size,
      createdAt: now,
      lastAccessedAt: now,
    }
    await idbRequest(db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME).put(record)).catch(() => null)
    void cleanupPreviewCache()
  }

  async function cleanupPreviewCache() {
    const db = await openPreviewCacheDB()
    if (!db) return
    const records = await idbRequest<PreviewCacheRecord[]>(
      db.transaction(PREVIEW_CACHE_STORE_NAME, 'readonly').objectStore(PREVIEW_CACHE_STORE_NAME).getAll(),
    ).catch(() => [])
    if (!records.length) return

    const now = Date.now()
    const sorted = [...records].sort((a, b) => a.lastAccessedAt - b.lastAccessedAt)
    const deleteKeys = new Set<string>()
    let totalBytes = 0
    let keptCount = 0

    for (const record of sorted) {
      if (now - record.createdAt > PREVIEW_CACHE_MAX_AGE_MS) {
        deleteKeys.add(record.key)
        continue
      }
      totalBytes += record.size || record.blob?.size || 0
      keptCount += 1
    }

    for (const record of sorted) {
      if (deleteKeys.has(record.key)) continue
      if (keptCount <= PREVIEW_CACHE_MAX_ENTRIES && totalBytes <= PREVIEW_CACHE_MAX_BYTES) break
      deleteKeys.add(record.key)
      totalBytes -= record.size || record.blob?.size || 0
      keptCount -= 1
    }

    if (!deleteKeys.size) return
    const store = db.transaction(PREVIEW_CACHE_STORE_NAME, 'readwrite').objectStore(PREVIEW_CACHE_STORE_NAME)
    for (const key of deleteKeys) {
      store.delete(key)
    }
  }

  async function loadPreviewImageSource(blob: Blob): Promise<{ image: PreviewImageSource, width: number, height: number, close: () => void }> {
    if ('createImageBitmap' in window) {
      const bitmap = await window.createImageBitmap(blob)
      return {
        image: bitmap,
        width: bitmap.width,
        height: bitmap.height,
        close: () => bitmap.close(),
      }
    }

    const url = URL.createObjectURL(blob)
    try {
      const image = await new Promise<HTMLImageElement>((resolve, reject) => {
        const img = new Image()
        img.onload = () => resolve(img)
        img.onerror = () => reject(new Error('image unavailable'))
        img.src = url
      })
      return {
        image,
        width: image.naturalWidth || image.width,
        height: image.naturalHeight || image.height,
        close: () => URL.revokeObjectURL(url),
      }
    } catch (error) {
      URL.revokeObjectURL(url)
      throw error
    }
  }

  async function createThumbnailBlob(blob: Blob): Promise<Blob> {
    const source = await loadPreviewImageSource(blob)
    const width = source.width
    const height = source.height
    const scale = Math.min(1, PREVIEW_THUMBNAIL_MAX_EDGE / Math.max(width, height))
    const targetWidth = Math.max(1, Math.round(width * scale))
    const targetHeight = Math.max(1, Math.round(height * scale))
    const canvas = document.createElement('canvas')
    canvas.width = targetWidth
    canvas.height = targetHeight
    const ctx = canvas.getContext('2d')
    if (!ctx) throw new Error('canvas unavailable')
    ctx.drawImage(source.image, 0, 0, targetWidth, targetHeight)
    source.close()
    return await new Promise<Blob>((resolve, reject) => {
      canvas.toBlob((thumbnail) => {
        if (thumbnail) resolve(thumbnail)
        else reject(new Error('thumbnail unavailable'))
      }, 'image/webp', PREVIEW_THUMBNAIL_QUALITY)
    })
  }

  return {
    previewCacheSupported,
    previewCacheKey,
    getCachedPreviewBlob,
    putCachedPreviewBlob,
    cleanupPreviewCache,
    createThumbnailBlob,
  }
}

export type PreviewCache = ReturnType<typeof createPreviewCache>
