import { afterEach, describe, expect, it, vi } from 'vitest'
import { reactive } from 'vue'
import { createLocalChatId, snapshotChatSession, type LocalChatSession } from '../localChat'

afterEach(() => { vi.unstubAllGlobals(); vi.resetModules() })

describe('local chat storage', () => {
  it('snapshots reactive messages without losing files, unknown tokens, tools or legacy metadata', () => {
    const file = new File(['image'], 'source.png', { type: 'image/png' })
    const session = reactive<LocalChatSession>({
      id: 'local', userId: 1, title: 'Example', groupId: 2, model: 'model', platform: 'openai', settings: {},
      createdAt: '', updatedAt: '', persisted: false, draft: { text: 'Draft', files: [file] },
      messages: [{ id: 'm', role: 'user', content: 'Hello', files: [file], status: 'completed', createdAt: '',
        rawContent: [{ type: 'image_url', image_url: { url: 'https://example.test/image.png' } }],
        toolResults: [{ toolCallId: 't', name: 'chart', content: 'Chart', data: { type: 'chart', values: [1] } }],
      }],
    })
    const saved = snapshotChatSession(session)
    session.messages[0]!.content = 'Changed'
    session.draft.text = 'Changed draft'
    expect(saved.messages[0]!.content).toBe('Hello')
    expect(saved.draft.text).toBe('Draft')
    expect(saved.messages[0]!.files[0]).toBe(file)
    expect(saved.messages[0]!.input_tokens).toBeUndefined()
    expect(saved.messages[0]!.toolResults?.[0]?.data).toEqual({ type: 'chart', values: [1] })
    expect(saved.messages[0]!.rawContent).toEqual([{ type: 'image_url', image_url: { url: 'https://example.test/image.png' } }])
  })

  it('fails explicitly when IndexedDB is unavailable', async () => {
    vi.stubGlobal('indexedDB', undefined)
    const { localChatStorage } = await import('../localChat')
    await expect(localChatStorage.listSessions(1)).rejects.toMatchObject({ code: 'unavailable' })
  })

  it('does not report a save until its transaction commits and surfaces quota errors', async () => {
    let transaction: { oncomplete?: () => void; onabort?: () => void; onerror?: (event: unknown) => void; error: unknown; objectStore: () => { put: ReturnType<typeof vi.fn> }; abort: ReturnType<typeof vi.fn> }
    const put = vi.fn()
    const db = { transaction: vi.fn(() => {
      transaction = { error: null, objectStore: () => ({ put }), abort: vi.fn() }
      return transaction
    }) }
    vi.stubGlobal('indexedDB', { open: () => {
      const request: { result: unknown; onsuccess?: () => void } = { result: db }
      queueMicrotask(() => request.onsuccess?.())
      return request
    } })
    const { localChatStorage } = await import('../localChat')
    let completed = false
    const saving = localChatStorage.setSelection(12, 'private-session').then(() => { completed = true })
    await new Promise(resolve => setTimeout(resolve, 0))
    expect(completed).toBe(false)
    expect(put).toHaveBeenCalledWith({ userId: 12, sessionId: 'private-session' })
    transaction!.oncomplete?.()
    await saving
    const failing = localChatStorage.setSelection(12, 'next')
    const rejection = expect(failing).rejects.toMatchObject({ code: 'quota' })
    await new Promise(resolve => setTimeout(resolve, 0))
    transaction!.onerror?.({ target: { error: new DOMException('Full', 'QuotaExceededError') } })
    await rejection
  })

  it('uses an HTTP-compatible local ID fallback', () => {
    vi.stubGlobal('crypto', {})
    expect(createLocalChatId()).toMatch(/^\d+-[a-z0-9]+$/)
  })
})
