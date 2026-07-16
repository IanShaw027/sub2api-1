import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

const { getRuntimeInfo } = vi.hoisted(() => ({
  getRuntimeInfo: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

vi.mock('@/api/ai', () => ({
  getRuntimeInfo,
}))

import { useAiStudioStore } from '@/stores/aiStudio'
import { setSessionUser } from '@/utils/authSession'

describe('useAiStudioStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    get.mockReset()
    post.mockReset()
    getRuntimeInfo.mockReset()
    getRuntimeInfo.mockResolvedValue({ lines: [] })
    localStorage.clear()
    setSessionUser(null)
  })

  it('creates a chat session and restores session messages from backend', async () => {
    post.mockResolvedValueOnce({
      data: {
        id: 99,
        title: 'Session 99',
        status: 'active',
        trace: { group_id: 12 },
        created_at: '2026-05-01T00:00:00Z',
        updated_at: '2026-05-01T00:00:00Z',
      },
    })
    get.mockResolvedValueOnce({
      data: {
        id: 99,
        title: 'Session 99',
        status: 'active',
        trace: { group_id: 12 },
        created_at: '2026-05-01T00:00:00Z',
        updated_at: '2026-05-01T00:05:00Z',
      },
    })
    get.mockResolvedValueOnce({
      data: {
        items: [
          {
            id: 1,
            role: 'user',
            content: 'hello',
            created_at: '2026-05-01T00:00:00Z',
          },
          {
            id: 2,
            role: 'assistant',
            content: 'world',
            created_at: '2026-05-01T00:01:00Z',
          },
        ],
        total: 2,
        page: 1,
        page_size: 200,
        pages: 1,
      },
    })

    const store = useAiStudioStore()
    const created = await store.createChatSession('Session 99')
    const messages = await store.loadChatSession(created.id)

    expect(created.id).toBe(99)
    expect(store.activeSessionId).toBe(99)
    expect(store.chatSessions.items[0].id).toBe(99)
    expect(messages).toHaveLength(2)
    expect(store.sessionMessages.map((item) => item.content)).toEqual(['hello', 'world'])
    expect(get).toHaveBeenNthCalledWith(1, '/user/ai/sessions/99')
    expect(get).toHaveBeenNthCalledWith(2, '/user/ai/sessions/99/messages', {
      params: {
        page: 1,
        page_size: 200,
        sort_by: 'created_at',
        sort_order: 'asc',
      },
    })
  })

  it('resets chat session state', async () => {
    post.mockResolvedValueOnce({
      data: {
        id: 100,
        title: 'Session 100',
        status: 'active',
        created_at: '2026-05-01T00:00:00Z',
        updated_at: '2026-05-01T00:00:00Z',
      },
    })

    const store = useAiStudioStore()
    await store.createChatSession('Session 100')
    store.reset()

    expect(store.activeSessionId).toBeNull()
    expect(store.sessionMessages).toEqual([])
    expect(store.chatSessions.items).toEqual([])
  })

  it('scopes persisted line and key selections per authenticated user', async () => {
    setSessionUser({ id: 1 } as any)

    let store = useAiStudioStore()
    store.lines = [
      {
        group_id: 11,
        label: 'User 1 Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 111, name: 'User 1 Key' }],
        key_ids: [111],
        key_count: 1,
        default_key_id: 111,
      },
    ]
    store.setSelectedLine(11)
    store.setSelectedKey(111)

    setActivePinia(createPinia())
    setSessionUser({ id: 2 } as any)
    store = useAiStudioStore()
    store.runtimeInfo = {
      default_line: {
        group_id: 22,
        label: 'User 2 Default',
        platform: 'openai',
        description: null,
        keys: [{ id: 222, name: 'User 2 Key' }],
        key_ids: [222],
        key_count: 1,
        default_key_id: 222,
      },
      lines: [],
    }
    store.lines = [
      {
        group_id: 11,
        label: 'Shared Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 111, name: 'User 1 Key' }],
        key_ids: [111],
        key_count: 1,
        default_key_id: 111,
      },
      {
        group_id: 22,
        label: 'User 2 Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 222, name: 'User 2 Key' }],
        key_ids: [222],
        key_count: 1,
        default_key_id: 222,
      },
    ]
    store.ensureSelection()

    expect(store.selectedLineId).toBe(22)
    expect(store.selectedKeyId).toBe(222)

    store.setSelectedLine(22)
    store.setSelectedKey(222)

    setActivePinia(createPinia())
    setSessionUser({ id: 1 } as any)
    store = useAiStudioStore()
    store.lines = [
      {
        group_id: 11,
        label: 'User 1 Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 111, name: 'User 1 Key' }],
        key_ids: [111],
        key_count: 1,
        default_key_id: 111,
      },
    ]
    store.ensureSelection()

    expect(store.selectedLineId).toBe(11)
    expect(store.selectedKeyId).toBe(111)
  })

  it('clears in-memory session state when auth scope changes before loading the next user runtime', async () => {
    setSessionUser({ id: 1 } as any)
    const store = useAiStudioStore()

    store.lines = [
      {
        group_id: 11,
        label: 'User 1 Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 111, name: 'User 1 Key' }],
        key_ids: [111],
        key_count: 1,
        default_key_id: 111,
      },
    ]
    store.chatSessions.items = [
      {
        id: 99,
        title: 'User 1 Session',
        status: 'active',
        line_id: 11,
        last_message_at: null,
        created_at: '2026-05-01T00:00:00Z',
        updated_at: '2026-05-01T00:00:00Z',
      },
    ]
    store.activeSessionId = 99
    store.sessionMessages = [
      {
        id: 'msg-1',
        role: 'user',
        content: 'hello',
        created_at: '2026-05-01T00:00:00Z',
        line_id: 11,
        line_name: 'User 1 Line',
        model: null,
      },
    ]

    setSessionUser({ id: 2 } as any)
    localStorage.setItem('sub2api_ai_selected_line_v1:user_2', '22')
    localStorage.setItem('sub2api_ai_selected_key_by_line_v1:user_2', JSON.stringify({ 22: 222 }))

    getRuntimeInfo.mockResolvedValueOnce({
      lines: [
        {
          group_id: 22,
          label: 'User 2 Line',
          platform: 'openai',
          description: null,
          keys: [{ id: 222, name: 'User 2 Key' }],
          key_ids: [222],
          key_count: 1,
          default_key_id: 222,
        },
      ],
      default_line: {
        group_id: 22,
        label: 'User 2 Line',
        platform: 'openai',
        description: null,
        keys: [{ id: 222, name: 'User 2 Key' }],
        key_ids: [222],
        key_count: 1,
        default_key_id: 222,
      },
    })

    await store.loadRuntimeLines()

    expect(store.activeSessionId).toBeNull()
    expect(store.sessionMessages).toEqual([])
    expect(store.chatSessions.items).toEqual([])
    expect(store.selectedLineId).toBe(22)
    expect(store.selectedKeyId).toBe(222)
  })
})
