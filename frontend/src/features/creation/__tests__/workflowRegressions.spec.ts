import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { i18n } from '@/i18n'
import ComposerBar from '../components/ComposerBar.vue'
import type { CreationExchange, CreationExchangeRequest, CreationMessage, CreationSession, CreationTokenUsage } from '../types'

const creationApi = vi.hoisted(() => ({
  listSessions: vi.fn(), createSession: vi.fn(), updateSession: vi.fn(), deleteSession: vi.fn(),
  listSessionMessages: vi.fn(), createSessionExchange: vi.fn(), listImages: vi.fn(),
  getModels: vi.fn(), submitImageGenerationAsync: vi.fn(), getImageTask: vi.fn(), streamCreationChat: vi.fn(),
}))
const groupsApi = vi.hoisted(() => ({ getAvailable: vi.fn() }))

vi.mock('../api', async (importOriginal) => ({
  ...await importOriginal<typeof import('../api')>(),
  default: creationApi,
}))
vi.mock('@/api/groups', () => ({ userGroupsAPI: groupsApi }))

import { useCreationStore } from '../stores/creation'

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => { resolve = res })
  return { promise, resolve }
}

function session(id: number, groupId = 1): CreationSession {
  return {
    id, user_id: 1, group_id: groupId, title: `Session ${id}`, model: 'gpt-4o',
    mode: 'chat', status: 'active', created_at: '2026-09-06T00:00:00Z', updated_at: '2026-09-06T00:00:00Z',
  }
}

function seedSession() {
  const store = useCreationStore()
  store.sessions = [session(1), session(2)]
  store.selectedSessionId = 1
  store.groupId = 1
  store.model = 'gpt-4o'
  return store
}

function savedExchange(sessionId: number, request: CreationExchangeRequest): CreationExchange {
  return {
    user: { id: 10, session_id: sessionId, role: 'user', content: request.user_content, exchange_request_id: request.request_id, created_at: '' },
    assistant: {
      id: 11, session_id: sessionId, role: 'assistant', content: request.assistant_content, model: request.model,
      input_tokens: request.input_tokens, output_tokens: request.output_tokens, created_at: '',
      exchange_request_id: request.request_id,
    },
  }
}

describe('creation workflow regressions', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Object.values(creationApi).forEach((mock) => mock.mockReset())
    groupsApi.getAvailable.mockReset()
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockResolvedValue([])
    creationApi.createSessionExchange.mockImplementation(async (id: number, request: CreationExchangeRequest) => savedExchange(id, request))
  })

  afterEach(() => {
    useCreationStore().reset()
    vi.useRealTimers()
  })

  it('discards a session creation response after logout reset', async () => {
    const store = seedSession()
    const oldRequest = deferred<CreationSession>()
    creationApi.createSession.mockReturnValue(oldRequest.promise)
    const creating = store.createSession('chat').catch(() => undefined)
    await flushPromises()
    expect(creationApi.createSession).toHaveBeenCalledOnce()
    store.reset()
    oldRequest.resolve(session(3))
    await creating
    expect(store.sessions).toEqual([])
    expect(store.groupId).toBeNull()
    expect(store.selectedSessionId).toBeNull()
  })

  it('does not resume initialization with an old account group response', async () => {
    const groups = deferred<Array<{ id: number; platform: string; name: string }>>()
    groupsApi.getAvailable.mockReturnValue(groups.promise)
    const store = useCreationStore()
    const initializing = store.initialize().catch(() => undefined)
    store.reset()
    groups.resolve([{ id: 1, platform: 'openai', name: 'Old account group' }])
    await initializing
    expect(store.groups).toEqual([])
    expect(creationApi.listSessions).not.toHaveBeenCalled()
  })

  it('does not restore failed sends or persist an old stream after reset', async () => {
    const store = seedSession()
    const reply = deferred<string>()
    creationApi.streamCreationChat.mockReturnValue(reply.promise)
    const sending = store.submitText('Old account prompt')
    store.reset()
    reply.resolve('Old account reply')
    await sending
    expect(creationApi.createSessionExchange).not.toHaveBeenCalled()
    expect(store.lastFailedSend).toBeNull()
    expect(store.messages).toEqual([])
  })

  it('does not restart image polling when an accepted task arrives after reset', async () => {
    vi.useFakeTimers()
    const store = seedSession()
    store.sessions[0].mode = 'image'
    const task = deferred<{ task_id: string; status: string }>()
    creationApi.submitImageGenerationAsync.mockReturnValue(task.promise)
    const sending = store.submitText('Old image prompt')
    store.reset()
    task.resolve({ task_id: 'old-task', status: 'processing' })
    await sending
    await vi.advanceTimersByTimeAsync(6000)
    expect(creationApi.getImageTask).not.toHaveBeenCalled()
    expect(store.imageTasks).toEqual([])
    expect(store.lastFailedSend).toBeNull()
  })

  it('keeps the latest group when an earlier create response arrives last', async () => {
    const store = seedSession()
    const earlier = deferred<CreationSession>()
    creationApi.createSession.mockImplementation(({ group_id }: { group_id: number }) =>
      group_id === 2 ? earlier.promise : Promise.resolve(session(30, group_id)),
    )
    const first = store.setGroupId(2)
    await flushPromises()
    expect(creationApi.createSession).toHaveBeenCalledWith(expect.objectContaining({ group_id: 2 }))
    await store.setGroupId(3)
    earlier.resolve(session(20, 2))
    await first
    expect(store.groupId).toBe(3)
    expect(store.selectedSessionId).toBe(30)
    expect(store.sessions.some((item) => item.id === 20)).toBe(true)
    expect(store.sessionLoading).toBe(false)
  })

  it('does not create a session for a superseded group model request', async () => {
    const store = seedSession()
    const earlierModels = deferred<{ data: Array<{ id: string }> }>()
    creationApi.getModels.mockImplementation((id: number) => id === 2 ? earlierModels.promise : Promise.resolve({ data: [{ id: 'gpt-4o' }] }))
    creationApi.createSession.mockImplementation(({ group_id }: { group_id: number }) => Promise.resolve(session(30, group_id)))
    const first = store.setGroupId(2)
    await store.setGroupId(3)
    earlierModels.resolve({ data: [{ id: 'old-model' }] })
    await first
    expect(creationApi.createSession).toHaveBeenCalledOnce()
    expect(store.groupId).toBe(3)
    expect(store.model).toBe('gpt-4o')
  })

  it('retains completed replies across session switches and retries only the save with the same id', async () => {
    const store = seedSession()
    creationApi.streamCreationChat.mockResolvedValue('Paid generated answer')
    creationApi.createSessionExchange.mockRejectedValueOnce(new Error('Transient database failure'))
    await store.submitText('Generate an answer')
    const firstRequest = creationApi.createSessionExchange.mock.calls[0]?.[1] as CreationExchangeRequest
    expect(firstRequest.request_id).toBeTruthy()
    expect(store.messages.map((message) => message.content)).toEqual(['Generate an answer', 'Paid generated answer'])
    expect(store.hasUnsavedExchange).toBe(true)
    await store.submitText('Do not start another generation')
    await store.selectSession(2)
    expect(store.messages).toEqual([])
    await store.selectSession(1)
    expect(store.messages.map((message) => message.content)).toEqual(['Generate an answer', 'Paid generated answer'])
    await store.retryLastFailed()
    expect(creationApi.streamCreationChat).toHaveBeenCalledOnce()
    expect(creationApi.createSessionExchange).toHaveBeenLastCalledWith(1, firstRequest)
    expect(store.messages.map((message) => message.id)).toEqual([10, 11])
    expect(store.hasUnsavedExchange).toBe(false)
    expect(store.lastFailedSend).toBeNull()
  })

  it('reuses the idempotency key when the save committed but its response was lost', async () => {
    const store = seedSession()
    const persisted = new Map<string, CreationExchange>()
    let loseResponse = true
    creationApi.streamCreationChat.mockResolvedValue('An already billed answer')
    creationApi.createSessionExchange.mockImplementation(async (id: number, request: CreationExchangeRequest) => {
      if (!persisted.has(request.request_id)) persisted.set(request.request_id, savedExchange(id, request))
      if (loseResponse) {
        loseResponse = false
        throw new Error('Response connection dropped')
      }
      return persisted.get(request.request_id)
    })
    await store.submitText('Prompt')
    await store.retryLastFailed()
    expect(creationApi.streamCreationChat).toHaveBeenCalledOnce()
    expect(persisted.size).toBe(1)
    expect(store.messages.map((message) => message.role)).toEqual(['user', 'assistant'])
  })

  it('creates a fallback id before generation and reuses it when randomUUID is unavailable', async () => {
    vi.stubGlobal('crypto', { randomUUID: undefined })
    const random = vi.spyOn(Math, 'random').mockReturnValue(0.5)
    try {
      const store = seedSession()
      creationApi.streamCreationChat.mockImplementation(async () => {
        expect(random).toHaveBeenCalledOnce()
        return 'Retained answer'
      })
      creationApi.createSessionExchange.mockRejectedValueOnce(new Error('Save failed'))
      await store.submitText('Prompt over HTTP')
      const request = creationApi.createSessionExchange.mock.calls[0]?.[1] as CreationExchangeRequest
      expect(request.request_id).toMatch(/^\d+-[a-z0-9]+$/)
      expect(store.messages.map((message) => message.content)).toEqual(['Prompt over HTTP', 'Retained answer'])
      await store.retryLastFailed()
      expect(creationApi.streamCreationChat).toHaveBeenCalledOnce()
      expect(creationApi.createSessionExchange).toHaveBeenLastCalledWith(1, request)
      expect(store.hasUnsavedExchange).toBe(false)
    } finally {
      random.mockRestore()
      vi.unstubAllGlobals()
    }
  })

  it('reconciles a lost save response with persisted history without showing a duplicate pair', async () => {
    const store = seedSession()
    let committed: CreationMessage[] = []
    creationApi.streamCreationChat.mockResolvedValue('Committed answer')
    creationApi.createSessionExchange.mockImplementation(async (id: number, request: CreationExchangeRequest) => {
      const result = savedExchange(id, request)
      committed = [result.user, result.assistant]
      throw new Error('Response lost after commit')
    })
    creationApi.listSessionMessages.mockImplementation(async (id: number) => id === 1 ? committed : [])
    await store.submitText('Prompt')
    await store.selectSession(2)
    await store.selectSession(1)
    expect(store.messages.map((message) => message.id)).toEqual([10, 11])
    expect(store.hasUnsavedExchange).toBe(false)
    expect(store.lastFailedSend).toBeNull()
    expect(creationApi.createSessionExchange).toHaveBeenCalledOnce()
    expect(creationApi.streamCreationChat).toHaveBeenCalledOnce()
  })

  it('does not clear another session failure when an older save retry completes', async () => {
    const store = seedSession()
    creationApi.streamCreationChat.mockResolvedValueOnce('Answer 1').mockRejectedValueOnce(new Error('Session 2 failed'))
    creationApi.createSessionExchange.mockRejectedValueOnce(new Error('Save 1 failed'))
    await store.submitText('Prompt 1')
    const request = creationApi.createSessionExchange.mock.calls[0]?.[1] as CreationExchangeRequest
    const saving = deferred<CreationExchange>()
    creationApi.createSessionExchange.mockReturnValueOnce(saving.promise)
    const retrying = store.retryLastFailed()
    await store.selectSession(2)
    await store.submitText('Prompt 2')
    expect(store.lastFailedSend).toEqual({ sessionId: 2, text: 'Prompt 2' })
    saving.resolve(savedExchange(1, request))
    await retrying
    expect(store.selectedSessionId).toBe(2)
    expect(store.lastFailedSend).toEqual({ sessionId: 2, text: 'Prompt 2' })
  })

  it('persists usage snapshots without summing cumulative output values', async () => {
    const store = seedSession()
    let persisted: CreationMessage[] = []
    creationApi.streamCreationChat.mockImplementation(async ({ onUsage }: { onUsage: (value: CreationTokenUsage) => void }) => {
      onUsage({ input_tokens: 25, output_tokens: 1 })
      onUsage({ output_tokens: 10 })
      onUsage({ output_tokens: 12 })
      return 'Answer'
    })
    creationApi.createSessionExchange.mockImplementation(async (id: number, request: CreationExchangeRequest) => {
      const result = savedExchange(id, request)
      persisted = [result.user, result.assistant]
      return result
    })
    creationApi.listSessionMessages.mockImplementation(async () => persisted)
    await store.submitText('Prompt')
    expect(creationApi.createSessionExchange).toHaveBeenCalledWith(1, expect.objectContaining({ input_tokens: 25, output_tokens: 12 }))
    expect(store.messages[1]).toMatchObject({ input_tokens: 25, output_tokens: 12 })
  })

  it('loads owned chat history after a models authorization failure and disables new generation', async () => {
    const store = seedSession()
    creationApi.getModels.mockRejectedValue(new Error('CREATION_GROUP_NOT_ALLOWED'))
    const history = [{ id: 7, session_id: 1, role: 'assistant', content: 'Saved answer', created_at: '' }]
    creationApi.listSessionMessages.mockResolvedValue(history)
    await store.selectSession(1)
    expect(store.messages).toEqual(history)
    expect(store.selectedSessionId).toBe(1)
    expect(store.generationAvailable).toBe(false)
    await store.submitText('Must not generate')
    expect(creationApi.streamCreationChat).not.toHaveBeenCalled()
  })

  it('loads completed image history after its subscription expires', async () => {
    const store = seedSession()
    store.sessions[0].mode = 'image'
    creationApi.getModels.mockRejectedValue(new Error('CREATION_GROUP_NOT_ALLOWED'))
    creationApi.listImages.mockResolvedValue({ items: [{ id: 3, session_id: 1, status: 'completed', media_url: 'https://example.test/saved.png' }], total: 1 })
    await store.selectSession(1)
    expect(store.imageTasks[0]?.media_url).toBe('https://example.test/saved.png')
    expect(store.generationAvailable).toBe(false)
  })

  it('ignores IME Enter but still sends ordinary Enter', async () => {
    const store = seedSession()
    const submit = vi.spyOn(store, 'submitText').mockResolvedValue(undefined)
    const wrapper = mount(ComposerBar, { global: { plugins: [i18n] } })
    try {
      const input = wrapper.get('textarea')
      await input.setValue('An unfinished prompt')
      await input.trigger('compositionstart')
      const event = new KeyboardEvent('keydown', { key: 'Enter', code: 'Enter', isComposing: true, bubbles: true, cancelable: true })
      input.element.dispatchEvent(event)
      await flushPromises()
      expect(submit).not.toHaveBeenCalled()
      expect(event.defaultPrevented).toBe(false)
      await input.trigger('compositionend')
      await input.trigger('keydown', { key: 'Enter', code: 'Enter' })
      expect(submit).toHaveBeenCalledWith('An unfinished prompt')
    } finally {
      wrapper.unmount()
    }
  })
})
