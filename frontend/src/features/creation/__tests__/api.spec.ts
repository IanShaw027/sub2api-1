import { afterEach, describe, expect, it, vi } from 'vitest'
import { mapCreationImageJob, streamCreationChat, streamMessages } from '../api'

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
    const fetchMock = vi.fn().mockResolvedValue(new Response('data: {"type":"message_stop"}\n\n', { status: 200 }))
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

  const options = {
    groupId: 2, sessionId: 5, platform: 'anthropic' as const, model: 'claude-sonnet-4',
    messages: [], userText: 'hello', onDelta: vi.fn(),
  }

  it.each([
    ['Anthropic', 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"partial"}}\n\nevent: error\ndata: {"type":"error","error":{"message":"upstream disconnected"}}\n\n'],
    ['OpenAI', 'data: {"choices":[{"delta":{"content":"partial"}}]}\n\ndata: {"error":{"message":"upstream disconnected"}}\n\n'],
  ])('rejects %s error events after partial text', async (_protocol, payload) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(payload)))
    await expect(streamCreationChat(options)).rejects.toThrow('upstream disconnected')
  })

  it.each(['', 'data: {"choices":[{"delta":{"content":"partial"}}]}\n\n'])('rejects EOF without a terminal event: %s', async (payload) => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(payload)))
    await expect(streamCreationChat(options)).rejects.toThrow('ended before completion')
  })

  it.each(['data: [DONE]\n\n', 'data: {"type":"message_stop"}\n\n'])('finishes and cancels the body at the terminal event: %s', async (terminal) => {
    const cancel = vi.fn()
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new TextEncoder().encode('data: {"choices":[{"delta":{"content":"answer"}}]}\n\n' + terminal))
      },
      cancel,
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body)))
    await expect(streamCreationChat(options)).resolves.toBe('answer')
    expect(cancel).toHaveBeenCalledOnce()
    expect(body.locked).toBe(false)
  })

  it('handles a terminal event split between network chunks', async () => {
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of ['data: {"choices":[{"delta":{"content":"answer"}}]}\n\ndata: [DO', 'NE]\r\n\r\n']) {
          controller.enqueue(new TextEncoder().encode(chunk))
        }
        controller.close()
      },
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body)))
    await expect(streamCreationChat(options)).resolves.toBe('answer')
  })

  it('requests and reads the final OpenAI empty-choices usage frame after finish_reason', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response([
      'data: {"choices":[{"delta":{"content":"answer"},"finish_reason":null}],"usage":null}',
      'data: {"choices":[{"delta":{},"finish_reason":"stop"}],"usage":null}',
      'data: {"choices":[],"usage":{"prompt_tokens":17,"completion_tokens":9,"total_tokens":26}}',
      'data: [DONE]',
      '',
    ].join('\n\n')))
    vi.stubGlobal('fetch', fetchMock)
    const onUsage = vi.fn()
    await expect(streamCreationChat({ ...options, platform: 'openai', onUsage })).resolves.toBe('answer')
    expect(onUsage).toHaveBeenCalledOnce()
    expect(onUsage).toHaveBeenCalledWith({ input_tokens: 17, output_tokens: 9 })
    expect(JSON.parse(String(fetchMock.mock.calls[0]?.[1].body))).toMatchObject({ stream_options: { include_usage: true } })
  })

  it('reports Anthropic input and cumulative output usage across network chunks', async () => {
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        for (const chunk of [
          'data: {"type":"message_start","message":{"usage":{"input_tokens":14,"output_tokens":1}}}\n\n',
          'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"answer"}}\n\n',
          'data: {"type":"message_delta","usage":{"output_tokens":8}}\n\n',
          'data: {"type":"message_stop"}\n\n',
        ]) controller.enqueue(new TextEncoder().encode(chunk))
        controller.close()
      },
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(body)))
    const onUsage = vi.fn()
    await expect(streamCreationChat({ ...options, onUsage })).resolves.toBe('answer')
    expect(onUsage.mock.calls).toEqual([[{ input_tokens: 14, output_tokens: 1 }], [{ output_tokens: 8 }]])
  })

  it('leaves usage absent when the provider does not send it', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('data: {"choices":[{"delta":{"content":"answer"}}],"usage":null}\n\ndata: [DONE]\n\n')))
    const onUsage = vi.fn()
    await streamCreationChat({ ...options, onUsage })
    expect(onUsage).not.toHaveBeenCalled()
  })

  it('ignores invalid negative and fractional token counts', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('data: {"choices":[],"usage":{"prompt_tokens":-1,"completion_tokens":2.5}}\n\ndata: [DONE]\n\n')))
    const onUsage = vi.fn()
    await streamCreationChat({ ...options, onUsage })
    expect(onUsage).not.toHaveBeenCalled()
  })
})
