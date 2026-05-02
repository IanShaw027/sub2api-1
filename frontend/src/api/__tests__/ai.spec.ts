import { beforeEach, describe, expect, it, vi } from 'vitest'

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

import { chat, editArtwork, listArtworks, loadRuntimeLines } from '@/api/ai'
import type { CreateAiArtworkRequest } from '@/types'

describe('ai api', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
  })

  it('loads runtime lines from /user/ai/runtime', async () => {
    get.mockResolvedValue({
      data: {
        lines: [
          {
            group_id: 12,
            label: 'Codex',
            platform: 'openai',
            description: 'codex line',
            keys: [
              { id: 101, name: 'Primary' },
              { id: 102, name: 'Backup' },
            ],
            key_ids: [101, 102],
            key_count: 2,
            default_key_id: 101,
          },
        ],
      },
    })

    const lines = await loadRuntimeLines()

    expect(get).toHaveBeenCalledWith('/user/ai/runtime')
    expect(lines).toEqual([
      {
        group_id: 12,
        label: 'Codex',
        platform: 'openai',
        description: 'codex line',
        keys: [
          { id: 101, name: 'Primary' },
          { id: 102, name: 'Backup' },
        ],
        key_ids: [101, 102],
        key_count: 2,
        default_key_id: 101,
      },
    ])
  })

  it('loads gallery items from /user/ai/gallery with normalized paging filters', async () => {
    const signal = new AbortController().signal

    get.mockResolvedValueOnce({
      data: {
        items: [],
        total: 0,
        page: 3,
        page_size: 24,
        pages: 1,
      },
    })

    await expect(
      listArtworks(
        3,
        24,
        {
          search: 'glass',
          visibility: 'public',
          status: 'succeeded',
          line_id: 88,
          style: 'editorial',
          owner_id: 7,
          featured: true,
        },
        { signal },
      ),
    ).resolves.toMatchObject({
      page: 3,
      page_size: 24,
      total: 0,
      items: [],
    })

    expect(get).toHaveBeenCalledWith(
      '/user/ai/gallery',
      expect.objectContaining({
        params: {
          page: 3,
          page_size: 24,
          search: 'glass',
          visibility: 'public',
          status: 'ready',
          group_id: 88,
        },
        signal,
      }),
    )
  })

  it('posts artwork edits to /user/ai/artworks/edit', async () => {
    const payload: CreateAiArtworkRequest = {
      prompt: 'draw a poster',
      line_id: 12,
      mode: 'edit',
      key_id: 101,
      prompt_template_id: 9,
      source_image: 'https://source.qazwc.com/ai/source.png',
      mask_image: 'https://source.qazwc.com/ai/mask.png',
    }

    post.mockResolvedValueOnce({
      data: {
        id: 88,
        image_url: 'https://source.qazwc.com/ai/88.png',
        visibility: 'public',
        status: 'succeeded',
        created_at: '2026-04-30T00:00:00Z',
        updated_at: '2026-04-30T00:00:00Z',
      },
    })

    await expect(editArtwork(payload)).resolves.toMatchObject({
      id: 88,
      image_url: 'https://source.qazwc.com/ai/88.png',
    })

    expect(post).toHaveBeenCalledTimes(1)
    expect(post).toHaveBeenCalledWith('/user/ai/artworks/edit', payload)
  })

  it('forwards use_responses on /user/ai/chat', async () => {
    const payload = {
      prompt: 'write a caption',
      line_id: 12,
      key_id: 101,
      prompt_template_id: 9,
      history: [
        {
          id: 'msg-1',
          role: 'assistant',
          content: 'ready',
          created_at: '2026-04-30T00:00:00Z',
        },
      ],
      use_responses: false,
    }

    post.mockResolvedValueOnce({
      data: {
        message: {
          id: 'reply-1',
          role: 'assistant',
          content: 'ok',
          created_at: '2026-04-30T00:00:00Z',
        },
      },
    })

    await expect(chat(payload)).resolves.toMatchObject({
      message: {
        id: 'reply-1',
        role: 'assistant',
        content: 'ok',
      },
    })

    expect(post).toHaveBeenCalledWith('/user/ai/chat', payload)
  })
})
