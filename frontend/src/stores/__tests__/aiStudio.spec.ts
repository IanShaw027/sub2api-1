import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    get,
    post,
  },
}))

vi.mock('@/api/ai', () => ({
  getRuntimeInfo: vi.fn().mockResolvedValue({ lines: [] }),
}))

import { useAiStudioStore } from '@/stores/aiStudio'

describe('useAiStudioStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    get.mockReset()
    post.mockReset()
    localStorage.clear()
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
})
