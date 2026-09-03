import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

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
})
