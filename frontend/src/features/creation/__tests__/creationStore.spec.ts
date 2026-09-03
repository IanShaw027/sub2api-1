import { beforeEach, describe, expect, it, vi } from 'vitest'
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

vi.mock('../api', () => ({
  default: creationApi,
  extractImageUrlFromTask: vi.fn(),
  mapAsyncTaskToImageJob: vi.fn(),
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: groupsApi,
}))

import { useCreationStore } from '../stores/creation'

describe('creation store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    Object.values(creationApi).forEach((mock) => mock.mockReset())
    groupsApi.getAvailable.mockReset()
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
