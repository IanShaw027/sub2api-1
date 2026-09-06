import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { i18n } from '@/i18n'
import enStudio from '@/i18n/locales/en/studio'
import SessionList from '../components/SessionList.vue'
import ComposerBar from '../components/ComposerBar.vue'

const creationApi = vi.hoisted(() => ({
  listSessions: vi.fn(),
  createSession: vi.fn(),
  updateSession: vi.fn(),
  deleteSession: vi.fn(),
  listSessionMessages: vi.fn(),
  createSessionMessage: vi.fn(),
  listImages: vi.fn(),
  getModels: vi.fn(),
  submitImageGenerationAsync: vi.fn(),
  getImageTask: vi.fn(),
  streamCreationChat: vi.fn(),
}))

const groupsApi = vi.hoisted(() => ({
  getAvailable: vi.fn(),
}))

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    default: creationApi,
  }
})

vi.mock('@/api/groups', () => ({
  userGroupsAPI: groupsApi,
}))

import { useCreationStore } from '../stores/creation'
import type { CreationImageJob, CreationSession } from '../types'

const IMAGE_POLL_INTERVAL_MS = 3000

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function chatSession(id: number): CreationSession {
  return imageSession({ id, mode: 'chat', model: 'gpt-4o' })
}

function imageSession(overrides: Partial<CreationSession> = {}): CreationSession {
  return {
    id: 5,
    user_id: 1,
    group_id: 2,
    title: 'Images',
    model: 'dall-e-3',
    mode: 'image',
    status: 'active',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

function imageJob(overrides: Partial<CreationImageJob> = {}): CreationImageJob {
  return {
    id: 1,
    session_id: 5,
    user_id: 1,
    group_id: 2,
    status: 'processing',
    model: 'dall-e-3',
    prompt: 'a red balloon',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
    ...overrides,
  }
}

describe('creation store', () => {
  beforeEach(() => {
    i18n.global.mergeLocaleMessage('en', enStudio)
    setActivePinia(createPinia())
    Object.values(creationApi).forEach((mock) => mock.mockReset())
    groupsApi.getAvailable.mockReset()
  })

  afterEach(() => {
    useCreationStore().reset()
    vi.useRealTimers()
  })

  it('selects a session and loads messages', async () => {
    groupsApi.getAvailable.mockResolvedValue([{ id: 7, name: 'OpenAI', platform: 'openai' }])
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockResolvedValue([
      {
        id: 1,
        session_id: 10,
        role: 'user',
        content: 'hello',
        created_at: '2026-01-01T00:00:00Z',
      },
    ])

    const store = useCreationStore()
    store.sessions = [
      {
        id: 10,
        user_id: 1,
        group_id: 7,
        title: 'Chat',
        model: 'gpt-4o',
        mode: 'chat',
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]

    await store.selectSession(10)

    expect(store.selectedSessionId).toBe(10)
    expect(store.groupId).toBe(7)
    expect(store.messages.length).toBe(1)
    expect(creationApi.listSessionMessages).toHaveBeenCalledWith(10)
  })

  it('queues send requests while streaming', async () => {
    const store = useCreationStore()
    store.selectedSessionId = 1
    store.groupId = 2
    store.model = 'gpt-4o'
    store.sessions = [
      {
        id: 1,
        user_id: 1,
        group_id: 2,
        title: 'Chat',
        model: 'gpt-4o',
        mode: 'chat',
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]
    store.streaming = true

    store.enqueueSend(1, 'queued message')
    expect(store.pendingQueue).toHaveLength(1)
    expect(store.pendingQueue[0].text).toBe('queued message')
  })

  it('retryLastFailed resends the last failed message', async () => {
    creationApi.streamCreationChat.mockRejectedValue(new Error('stream failed'))
    creationApi.listSessionMessages.mockResolvedValue([])

    const store = useCreationStore()
    store.sessions = [
      {
        id: 1,
        user_id: 1,
        group_id: 2,
        title: 'Chat',
        model: 'gpt-4o',
        mode: 'chat',
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]
    store.selectedSessionId = 1
    store.groupId = 2
    store.model = 'gpt-4o'

    await store.submitText('hello again')
    expect(creationApi.streamCreationChat).toHaveBeenCalledTimes(1)
    expect(store.lastFailedSend).toEqual({ sessionId: 1, text: 'hello again' })

    creationApi.streamCreationChat.mockResolvedValue('world')
    creationApi.createSessionMessage.mockResolvedValue({
      id: 99,
      session_id: 1,
      role: 'assistant',
      content: 'world',
      created_at: '2026-01-01T00:00:00Z',
    })

    await store.retryLastFailed()

    expect(creationApi.streamCreationChat).toHaveBeenCalled()
    expect(store.lastFailedSend).toBeNull()
  })

  it('creates a new session when switching groups', async () => {
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.createSession.mockResolvedValue({
      id: 12,
      user_id: 1,
      group_id: 8,
      title: 'New chat',
      model: 'gpt-4o',
      mode: 'chat',
      status: 'active',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })
    creationApi.listSessionMessages.mockResolvedValue([])

    const store = useCreationStore()
    store.groupId = 7
    store.model = 'old-model'
    store.selectedSessionId = 10
    store.sessions = [{
      id: 10,
      user_id: 1,
      group_id: 7,
      title: 'Existing',
      model: 'old-model',
      mode: 'chat',
      status: 'active',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    }]

    await store.setGroupId(8)

    expect(creationApi.createSession).toHaveBeenCalledWith({
      group_id: 8,
      mode: 'chat',
      model: 'gpt-4o',
      title: 'New chat',
    })
    expect(store.selectedSessionId).toBe(12)
    expect(store.groupId).toBe(8)
  })

  it('marks image placeholder failed when async submit errors', async () => {
    creationApi.submitImageGenerationAsync.mockRejectedValue(new Error('quota exceeded'))

    const store = useCreationStore()
    store.groupId = 2
    store.model = 'dall-e-3'
    store.imageModels = ['dall-e-3']
    store.sessions = [imageSession()]
    store.selectedSessionId = 5

    await expect(store.sendMessage('a red balloon', 5)).rejects.toThrow('quota exceeded')
    expect(store.imageTasks[0]?.status).toBe('failed')
    expect(store.imageTasks[0]?.error).toBe('quota exceeded')
  })

  it('reloads image list when poll completes', async () => {
    vi.useFakeTimers()
    creationApi.submitImageGenerationAsync.mockResolvedValue({
      task_id: 'task_abc',
      status: 'processing',
    })
    creationApi.getImageTask.mockResolvedValue({
      task_id: 'task_abc',
      status: 'completed',
      image_url: 'https://cdn.example/img.png',
    })
    creationApi.listImages.mockResolvedValue({
      items: [
        imageJob({
          id: 42,
          status: 'completed',
          provider_task_id: 'task_abc',
          media_url: 'https://cdn.example/img.png',
        }),
      ],
      total: 1,
      page: 1,
      page_size: 100,
    })

    const store = useCreationStore()
    store.groupId = 2
    store.model = 'dall-e-3'
    store.imageModels = ['dall-e-3']
    store.sessions = [imageSession()]
    store.selectedSessionId = 5

    await store.sendMessage('a red balloon', 5)

    expect(creationApi.submitImageGenerationAsync).toHaveBeenCalledWith(2, 5, {
      model: 'dall-e-3',
      prompt: 'a red balloon',
    })
    expect(store.imageTasks[0]?.provider_task_id).toBe('task_abc')
    expect(creationApi.listImages).not.toHaveBeenCalled()

    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)

    expect(creationApi.getImageTask).toHaveBeenCalledWith(2, 'task_abc')
    expect(creationApi.listImages).toHaveBeenCalledWith({ session_id: 5, page: 1, page_size: 100 })
    expect(store.imageTasks).toHaveLength(1)
    expect(store.imageTasks[0]?.id).toBe(42)
    expect(store.imageTasks[0]?.status).toBe('completed')
    expect(store.imageTasks[0]?.media_url).toBe('https://cdn.example/img.png')
  })

  it('does not clobber other in-flight jobs when loadImageTasks merges server rows', async () => {
    creationApi.listImages.mockResolvedValue({
      items: [
        imageJob({
          id: 99,
          status: 'completed',
          provider_task_id: 'task_a',
          media_url: 'https://cdn.example/a.png',
          prompt: 'first',
        }),
      ],
      total: 1,
      page: 1,
      page_size: 100,
    })

    const store = useCreationStore()
    store.selectedSessionId = 5
    store.imageTasks = [
      imageJob({
        id: 11,
        status: 'processing',
        provider_task_id: 'task_a',
        prompt: 'first',
      }),
      imageJob({
        id: 12,
        status: 'processing',
        provider_task_id: 'task_b',
        prompt: 'second',
        created_at: '2026-01-01T00:01:00Z',
      }),
    ]

    await store.loadImageTasks(5)

    expect(store.imageTasks).toHaveLength(2)
    const completed = store.imageTasks.find((task) => task.provider_task_id === 'task_a')
    const inFlight = store.imageTasks.find((task) => task.provider_task_id === 'task_b')
    expect(completed?.id).toBe(99)
    expect(completed?.status).toBe('completed')
    expect(completed?.media_url).toBe('https://cdn.example/a.png')
    expect(inFlight?.id).toBe(12)
    expect(inFlight?.status).toBe('processing')
  })

  it('resumes polling persisted image tasks after reload', async () => {
    vi.useFakeTimers()
    creationApi.listImages.mockResolvedValue({
      items: [imageJob({ id: 42, provider_task_id: 'task_reload' })],
      total: 1,
      page: 1,
      page_size: 100,
    })
    creationApi.getImageTask.mockResolvedValue({
      task_id: 'task_reload',
      status: 'processing',
    })

    const store = useCreationStore()
    store.selectedSessionId = 5
    await store.loadImageTasks(5)
    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)

    expect(creationApi.getImageTask).toHaveBeenCalledWith(2, 'task_reload')
  })

  it('does not expose image models for unsupported platforms', async () => {
    groupsApi.getAvailable.mockResolvedValue([{ id: 3, name: 'Gemini', platform: 'gemini' }])
    creationApi.listSessions.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100 })
    creationApi.getModels.mockResolvedValue({
      data: [{ id: 'gemini-3-pro-image-preview' }, { id: 'gemini-3-pro' }],
    })
    creationApi.createSession.mockResolvedValue({
      id: 10,
      user_id: 1,
      group_id: 3,
      title: 'New chat',
      model: 'gemini-3-pro',
      mode: 'chat',
      status: 'active',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })
    creationApi.listSessionMessages.mockResolvedValue([])

    const store = useCreationStore()
    await store.initialize()

    expect(store.imageModels).toEqual([])
    expect(store.chatModels).toContain('gemini-3-pro')
  })

  it('allows initialize to retry after a failure', async () => {
    groupsApi.getAvailable
      .mockRejectedValueOnce(new Error('temporary failure'))
      .mockResolvedValueOnce([])
    creationApi.listSessions.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 100 })

    const store = useCreationStore()
    await expect(store.initialize()).rejects.toThrow('temporary failure')
    await store.initialize()
    await store.initialize()

    expect(groupsApi.getAvailable).toHaveBeenCalledTimes(2)
    expect(creationApi.listSessions).toHaveBeenCalledTimes(1)
  })

  it('keeps the latest session when model requests finish out of order', async () => {
    const modelResolvers = new Map<number, (value: { data: Array<{ id: string }> }) => void>()
    creationApi.getModels.mockImplementation((groupId: number) => new Promise((resolve) => {
      modelResolvers.set(groupId, resolve)
    }))
    creationApi.listSessionMessages.mockResolvedValue([])

    const store = useCreationStore()
    store.sessions = [
      {
        id: 1, user_id: 1, group_id: 10, title: 'Old', model: 'old-model', mode: 'chat',
        status: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
      },
      {
        id: 2, user_id: 1, group_id: 20, title: 'New', model: 'new-model', mode: 'chat',
        status: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
      },
    ]

    const oldSelection = store.selectSession(1)
    const newSelection = store.selectSession(2)
    modelResolvers.get(20)?.({ data: [{ id: 'new-model' }] })
    await newSelection
    modelResolvers.get(10)?.({ data: [{ id: 'old-model' }] })
    await oldSelection

    expect(store.selectedSessionId).toBe(2)
    expect(store.groupId).toBe(20)
    expect(store.chatModels).toEqual(['new-model'])
    expect(creationApi.listSessionMessages).toHaveBeenCalledTimes(1)
    expect(creationApi.listSessionMessages).toHaveBeenCalledWith(2)
  })

  it('keeps polling after a transient getImageTask failure', async () => {
    vi.useFakeTimers()
    creationApi.listImages.mockResolvedValue({
      items: [imageJob({ id: 42, provider_task_id: 'task_retry' })],
      total: 1,
      page: 1,
      page_size: 100,
    })
    creationApi.getImageTask
      .mockRejectedValueOnce(new Error('temporary'))
      .mockResolvedValueOnce({
        task_id: 'task_retry',
        status: 'processing',
      })

    const store = useCreationStore()
    store.selectedSessionId = 5
    await store.loadImageTasks(5)
    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)
    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)

    expect(creationApi.getImageTask).toHaveBeenCalledTimes(2)
  })

  it('does not stop the selected session poller when deleting another session', async () => {
    vi.useFakeTimers()
    creationApi.listImages.mockResolvedValue({
      items: [imageJob({ id: 42, provider_task_id: 'task_keep' })],
      total: 1,
      page: 1,
      page_size: 100,
    })
    creationApi.getImageTask.mockResolvedValue({
      task_id: 'task_keep',
      status: 'processing',
    })
    creationApi.deleteSession.mockResolvedValue(undefined)

    const store = useCreationStore()
    store.sessions = [imageSession(), imageSession({ id: 9, title: 'Other' })]
    store.selectedSessionId = 5
    await store.loadImageTasks(5)
    await store.deleteSession(9)
    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)

    expect(creationApi.getImageTask).toHaveBeenCalledWith(2, 'task_keep')
  })

  it('treats stream abort as a failed send instead of success', async () => {
    let started = false
    creationApi.streamCreationChat.mockImplementation(async (input: { signal?: AbortSignal }) => {
      started = true
      await new Promise<void>((_resolve, reject) => {
        input.signal?.addEventListener('abort', () => {
          reject(new DOMException('Aborted', 'AbortError'))
        })
      })
    })
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockResolvedValue([])

    const store = useCreationStore()
    store.sessions = [{
      id: 1, user_id: 1, group_id: 2, title: 'Chat', model: 'gpt-4o', mode: 'chat',
      status: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
    }, {
      id: 2, user_id: 1, group_id: 2, title: 'Other', model: 'gpt-4o', mode: 'chat',
      status: 'active', created_at: '2026-01-01T00:00:00Z', updated_at: '2026-01-01T00:00:00Z',
    }]
    store.selectedSessionId = 1
    store.groupId = 2
    store.model = 'gpt-4o'

    const send = store.submitText('keep me')
    await vi.waitFor(() => expect(started).toBe(true))
    await store.selectSession(2)
    await send

    expect(store.lastFailedSend).toEqual({ sessionId: 1, text: 'keep me' })
    expect(creationApi.createSessionMessage).not.toHaveBeenCalled()
  })

  it('reset clears state', () => {
    const store = useCreationStore()
    store.sessions = [
      {
        id: 1,
        user_id: 1,
        group_id: 2,
        title: 'Chat',
        model: 'gpt-4o',
        mode: 'chat',
        status: 'active',
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ]
    store.messages = [
      {
        id: 2,
        session_id: 1,
        role: 'assistant',
        content: 'hi',
        created_at: '2026-01-01T00:00:00Z',
      },
    ]
    store.groupId = 2
    store.model = 'gpt-4o'

    store.reset()

    expect(store.sessions).toEqual([])
    expect(store.messages).toEqual([])
    expect(store.groupId).toBeNull()
    expect(store.model).toBe('')
  })

  it('restores the failed session before retrying instead of sending the visible session history', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1), chatSession(2)]
    store.selectedSessionId = 1
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockImplementation(async (id: number) => [{
      id: id * 10, session_id: id, role: 'assistant', content: `secret-${id}`, created_at: '',
    }])
    creationApi.streamCreationChat.mockRejectedValueOnce(new Error('failed')).mockResolvedValueOnce('answer')
    creationApi.createSessionMessage.mockResolvedValue({})
    await store.submitText('retry this')
    await store.selectSession(2)
    await store.retryLastFailed()

    expect(store.selectedSessionId).toBe(1)
    expect(creationApi.streamCreationChat.mock.lastCall?.[0]).toMatchObject({
      sessionId: 1,
      messages: [{ session_id: 1, content: 'secret-1' }],
    })
    expect(store.messages.every((message) => message.session_id === 1)).toBe(true)
  })

  it('keeps empty replies retryable without persisting a successful conversation turn', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1)]
    store.selectedSessionId = 1
    creationApi.streamCreationChat.mockResolvedValue('')
    await store.submitText('please answer')
    expect(store.lastFailedSend).toEqual({ sessionId: 1, text: 'please answer' })
    expect(creationApi.createSessionMessage).not.toHaveBeenCalled()
  })

  it('keeps the composer and sending locked until both models and session history load', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1), chatSession(2)]
    store.selectedSessionId = 1
    store.messages = [{ id: 10, session_id: 1, role: 'assistant', content: 'secret-1', created_at: '' }]
    const models = deferred<{ data: Array<{ id: string }> }>()
    const history = deferred<Array<{ id: number; session_id: number; role: string; content: string; created_at: string }>>()
    creationApi.getModels.mockReturnValue(models.promise)
    creationApi.listSessionMessages.mockReturnValue(history.promise)
    const wrapper = mount(ComposerBar, { global: { plugins: [i18n] } })
    await wrapper.get('textarea').setValue('question-2')
    const selection = store.selectSession(2)
    await flushPromises()
    expect(store.messages).toEqual([])
    expect(wrapper.get('textarea').attributes('disabled')).toBeDefined()
    await store.submitText('question-2')
    expect(creationApi.streamCreationChat).not.toHaveBeenCalled()
    models.resolve({ data: [{ id: 'gpt-4o' }] })
    await flushPromises()
    await store.submitText('question-2')
    expect(creationApi.streamCreationChat).not.toHaveBeenCalled()
    history.resolve([{ id: 20, session_id: 2, role: 'assistant', content: 'secret-2', created_at: '' }])
    await selection
    creationApi.streamCreationChat.mockResolvedValue('answer')
    creationApi.createSessionMessage.mockResolvedValue({})
    await store.submitText('question-2')
    expect(creationApi.streamCreationChat.mock.lastCall?.[0].messages).toEqual([
      expect.objectContaining({ session_id: 2, content: 'secret-2' }),
    ])
    wrapper.unmount()
  })

  it('does not permit sending after a session history load failed', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1)]
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockRejectedValue(new Error('offline'))
    await expect(store.selectSession(1)).rejects.toThrow('offline')
    await store.submitText('must not send without history')
    expect(store.selectedSessionId).toBeNull()
    expect(creationApi.streamCreationChat).not.toHaveBeenCalled()
  })

  it('applies a completed model update only to its original session', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1), chatSession(2)]
    store.selectedSessionId = 1
    store.model = 'gpt-4o'
    const update = deferred<unknown>()
    creationApi.updateSession.mockReturnValue(update.promise)
    creationApi.getModels.mockResolvedValue({ data: [{ id: 'gpt-4o' }] })
    creationApi.listSessionMessages.mockResolvedValue([])
    const updating = store.setModel('gpt-5')
    expect(store.modelUpdating).toBe(true)
    await store.submitText('wait for the selected model')
    expect(creationApi.streamCreationChat).not.toHaveBeenCalled()
    await store.selectSession(2)
    update.resolve({})
    await updating
    expect(creationApi.updateSession).toHaveBeenCalledWith(1, { model: 'gpt-5' })
    expect(store.sessions[0].model).toBe('gpt-5')
    expect(store.sessions[1].model).toBe('gpt-4o')
    expect(store.model).toBe('gpt-4o')
  })

  it('uses the saved model after updating and restores the old selection on failure', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1)]
    store.selectedSessionId = 1
    store.model = 'gpt-4o'
    creationApi.updateSession.mockRejectedValueOnce(new Error('save failed')).mockResolvedValueOnce({})
    await expect(store.setModel('bad-model')).rejects.toThrow('save failed')
    expect(store.model).toBe('gpt-4o')
    expect(store.modelUpdating).toBe(false)
    await store.setModel('gpt-5')
    creationApi.streamCreationChat.mockResolvedValue('answer')
    creationApi.createSessionMessage.mockResolvedValue({})
    creationApi.listSessionMessages.mockResolvedValue([])
    await store.submitText('hello')
    expect(creationApi.streamCreationChat.mock.lastCall?.[0].model).toBe('gpt-5')
  })

  it('deleting another session through the component preserves the current stream', async () => {
    const store = useCreationStore()
    store.sessions = [chatSession(1), chatSession(2)]
    store.selectedSessionId = 2
    const response = deferred<string>()
    creationApi.streamCreationChat.mockReturnValue(response.promise)
    creationApi.deleteSession.mockResolvedValue(undefined)
    creationApi.createSessionMessage.mockResolvedValue({})
    creationApi.listSessionMessages.mockResolvedValue([])
    const send = store.submitText('keep generating')
    const signal = creationApi.streamCreationChat.mock.lastCall?.[0].signal as AbortSignal
    const wrapper = mount(SessionList, { global: { plugins: [i18n] } })
    await wrapper.findAll('.studio-session-delete')[0].trigger('click')
    await flushPromises()
    expect(store.selectedSessionId).toBe(2)
    expect(store.streaming).toBe(true)
    expect(signal.aborted).toBe(false)
    response.resolve('complete answer')
    await send
    expect(creationApi.createSessionMessage).toHaveBeenCalledWith(2, expect.objectContaining({ content: 'complete answer' }))
    wrapper.unmount()
  })

  it('loads sessions beyond the first page including another mode', async () => {
    creationApi.listSessions
      .mockResolvedValueOnce({ items: Array.from({ length: 100 }, (_, index) => chatSession(index + 1)), total: 101 })
      .mockResolvedValueOnce({ items: [imageSession({ id: 101 })], total: 101 })
    const store = useCreationStore()
    await store.loadSessions()
    store.sessionModeFilter = 'image'
    expect(creationApi.listSessions).toHaveBeenLastCalledWith({ page: 2, page_size: 100 })
    expect(store.visibleSessions.map((session) => session.id)).toEqual([101])
  })

  it('loads older image pages and resumes polling a task outside the first page', async () => {
    vi.useFakeTimers()
    creationApi.listImages
      .mockResolvedValueOnce({ items: Array.from({ length: 100 }, (_, index) => imageJob({ id: index + 1, status: 'completed' })), total: 101 })
      .mockResolvedValueOnce({ items: [imageJob({ id: 101, provider_task_id: 'old-pending' })], total: 101 })
    creationApi.getImageTask.mockResolvedValue({ task_id: 'old-pending', status: 'processing' })
    const store = useCreationStore()
    store.selectedSessionId = 5
    await store.loadImageTasks(5)
    expect(store.imageTasks).toHaveLength(101)
    expect(creationApi.listImages).toHaveBeenLastCalledWith({ session_id: 5, page: 2, page_size: 100 })
    await vi.advanceTimersByTimeAsync(IMAGE_POLL_INTERVAL_MS)
    expect(creationApi.getImageTask).toHaveBeenCalledWith(2, 'old-pending')
  })
})
