import { afterEach, describe, expect, it, vi } from 'vitest'
import { mapCreationImageJob, streamMessages } from '../api'

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('mapCreationImageJob', () => {
  it('maps media_url from list payloads', () => {
    const job = mapCreationImageJob({
      id: 42,
      session_id: 5,
      user_id: 1,
      group_id: 2,
      status: 'completed',
      model: 'dall-e-3',
      prompt: 'a red balloon',
      provider_task_id: 'task_abc',
      media_url: 'https://cdn.example/img.png',
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    })

    expect(job.media_url).toBe('https://cdn.example/img.png')
    expect(job.provider_task_id).toBe('task_abc')
    expect(job.status).toBe('completed')
  })

  it('falls back to mediaUrl when media_url is missing', () => {
    const job = mapCreationImageJob({
      id: 7,
      status: 'completed',
      mediaUrl: 'https://cdn.example/alt.png',
    })

    expect(job.media_url).toBe('https://cdn.example/alt.png')
  })
})

describe('streamMessages', () => {
  it('includes the Anthropic max_tokens requirement', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    await streamMessages({
      groupId: 2,
      sessionId: 5,
      platform: 'anthropic',
      model: 'claude-sonnet-4',
      messages: [],
      userText: 'hello',
      onDelta: vi.fn(),
    })

    const init = fetchMock.mock.calls[0]?.[1] as RequestInit
    expect(JSON.parse(String(init.body))).toMatchObject({
      model: 'claude-sonnet-4',
      max_tokens: 4096,
      stream: true,
    })
  })
})
