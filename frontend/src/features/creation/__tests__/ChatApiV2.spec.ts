import { beforeEach, describe, expect, it, vi } from 'vitest'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import { chatAPI, getChatAttachmentCapabilities, mapChatMessages, validateChatAttachments, type ChatStreamOptions } from '../chatApi'
import type { LocalChatMessage } from '../localChat'

vi.mock('@/api/client', () => ({ apiClient: { get: vi.fn() }, buildApiUrl: (path: string) => `/api/v1${path}` }))
vi.mock('@/api/authenticatedFetch', () => ({ authenticatedFetch: vi.fn() }))

const message = (files: File[] = []): LocalChatMessage => ({ id: 'user', role: 'user', content: 'Read this', files, status: 'completed', createdAt: '' })
const options = (extra: Partial<ChatStreamOptions> = {}): ChatStreamOptions => ({ groupId: 2, platform: 'openai', model: 'gpt-5', messages: [message()], signal: new AbortController().signal, onDelta: vi.fn(), ...extra })
const events = (...payloads: unknown[]) => payloads.map(payload => `data: ${typeof payload === 'string' ? payload : JSON.stringify(payload)}\n\n`).join('')

beforeEach(() => { vi.clearAllMocks(); localStorage.setItem('auth_token', 'local-test') })

describe('local chat multimodal transport', () => {
  it('preserves image, PDF and text attachments in OpenAI content blocks', async () => {
    const files = [new File(['image'], 'image.png', { type: 'image/png' }), new File(['%PDF-1.7'], 'paper.pdf', { type: 'application/pdf' }), new File(['a,b\n1,2'], 'data.csv', { type: 'text/csv' })]
    const mapped = await mapChatMessages([message(files)], 'openai', 'gpt-5')
    expect(mapped.messages[0]?.content).toEqual([
      { type: 'text', text: 'Read this' },
      { type: 'image_url', image_url: { url: 'data:image/png;base64,aW1hZ2U=' } },
      { type: 'file', file: { filename: 'paper.pdf', file_data: 'data:application/pdf;base64,JVBERi0xLjc=' } },
      { type: 'text', text: 'Attached file: data.csv\na,b\n1,2' },
    ])
  })

  it('uses native base64 image/document blocks for Anthropic and Gemini bridge requests', async () => {
    const files = [new File(['image'], 'image.png', { type: 'image/png' }), new File(['%PDF-1.7'], 'paper.pdf', { type: 'application/pdf' })]
    const mapped = await mapChatMessages([message(files)], 'gemini', 'gemini-2.5-pro')
    expect(mapped.messages[0]?.content).toEqual([
      { type: 'text', text: 'Read this' },
      { type: 'image', source: { type: 'base64', media_type: 'image/png', data: 'aW1hZ2U=' } },
      { type: 'document', source: { type: 'base64', media_type: 'application/pdf', data: 'JVBERi0xLjc=' } },
    ])
  })

  it.each(['anthropic', 'gemini', 'antigravity'] as const)('routes %s through the JWT messages bridge without a cloud-session header', async platform => {
    vi.mocked(authenticatedFetch).mockResolvedValueOnce(new Response(events({ type: 'message_stop' })))
    const model = platform === 'gemini' ? 'gemini-3.1-pro' : 'claude-sonnet-4-6'
    await chatAPI.stream(options({ platform, model, settings: { reasoningEffort: 'high' } }))
    const [url, init] = vi.mocked(authenticatedFetch).mock.calls[0]!
    expect(url).toBe('/api/v1/creation/messages?group_id=2')
    expect(init?.headers).toMatchObject({ Authorization: 'Bearer local-test', 'X-Group-Id': '2' })
    expect(init?.headers).not.toHaveProperty('X-Session-Id')
    expect(JSON.parse(init?.body as string)).toMatchObject(platform === 'antigravity'
      ? { thinking: { type: 'enabled', budget_tokens: 24576 }, max_tokens: 25600 }
      : { output_config: { effort: 'high' }, max_tokens: 4096, ...(platform === 'anthropic' ? { thinking: { type: 'adaptive' } } : {}) })
  })

  it('normalizes supported OpenAI max effort and omits unverified Grok effort', async () => {
    vi.mocked(authenticatedFetch).mockImplementation(async () => new Response(events('[DONE]')))
    await chatAPI.stream(options({ settings: { reasoningEffort: 'max', temperature: 0.5 }, modelMetadata: { supportsReasoningEffort: true, reasoningEfforts: [{ value: 'xhigh' }] } }))
    const first = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)
    expect(first.reasoning_effort).toBe('xhigh')
    expect(first.temperature).toBeUndefined()
    await chatAPI.stream(options({ platform: 'composite', model: 'grok-4', settings: { reasoningEffort: 'high' } }))
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)).not.toHaveProperty('reasoning_effort')
  })

  it('sends only supported real Grok metadata effort values', async () => {
    vi.mocked(authenticatedFetch).mockImplementation(async () => new Response(events('[DONE]')))
    const modelMetadata = { supportsReasoningEffort: true, reasoningEfforts: [{ value: 'low' }, { value: 'high' }] }
    await chatAPI.stream(options({ platform: 'grok', model: 'grok-4.6', modelMetadata, settings: { reasoningEffort: 'low' } }))
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)).toHaveProperty('reasoning_effort', 'low')
    await chatAPI.stream(options({ platform: 'grok', model: 'grok-4.6', modelMetadata, settings: { reasoningEffort: 'medium' } }))
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)).not.toHaveProperty('reasoning_effort')
  })

  it.each([['low', 1024], ['medium', 8192], ['high', 24576]] as const)('applies Antigravity Claude %s effort as a real thinking budget', async (reasoningEffort, budget) => {
    vi.mocked(authenticatedFetch).mockResolvedValueOnce(new Response(events({ type: 'message_stop' })))
    await chatAPI.stream(options({ platform: 'antigravity', model: 'claude-opus-4-6', settings: { reasoningEffort, maxTokens: 4096 } }))
    const body = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)
    expect(body.thinking).toEqual({ type: 'enabled', budget_tokens: budget })
    expect(body.max_tokens).toBeGreaterThan(budget)
    expect(body).not.toHaveProperty('output_config')
  })

  it.each([
    ['anthropic', 'claude-opus-4-7', 'max', { output_config: { effort: 'max' } }],
    ['anthropic', 'claude-opus-4-7', 'xhigh', { output_config: { effort: 'xhigh' } }],
    ['gemini', 'gemini-3-flash', 'minimal', { output_config: { effort: 'minimal' } }],
    ['antigravity', 'gemini-2.5-flash', 'none', { thinking: { type: 'disabled' } }],
  ] as const)('preserves supported %s %s %s effort', async (platform, model, reasoningEffort, expected) => {
    vi.mocked(authenticatedFetch).mockResolvedValueOnce(new Response(events({ type: 'message_stop' })))
    await chatAPI.stream(options({ platform, model, settings: { reasoningEffort } }))
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)).toMatchObject(expected)
  })

  it('uses the Kiro converter thinking_effort contract and the non-adaptive Opus 4.5 budget contract', async () => {
    vi.mocked(authenticatedFetch).mockImplementation(async () => new Response(events({ type: 'message_stop' })))
    await chatAPI.stream(options({ platform: 'kiro', model: 'claude-opus-4-6', settings: { reasoningEffort: 'medium' } }))
    expect(JSON.parse(vi.mocked(authenticatedFetch).mock.calls[0]![1]?.body as string)).toMatchObject({ thinking: { type: 'adaptive', thinking_effort: 'medium' } })
    await chatAPI.stream(options({ platform: 'anthropic', model: 'claude-opus-4-5', settings: { reasoningEffort: 'medium', temperature: 0.5 } }))
    const native = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)
    expect(native).toMatchObject({ thinking: { type: 'enabled', budget_tokens: 8192 }, output_config: { effort: 'medium' }, max_tokens: 9216 })
    expect(native).not.toHaveProperty('temperature')
  })

  it('retains signed and redacted thinking across native tool continuation and local history replay', async () => {
    vi.mocked(authenticatedFetch)
      .mockResolvedValueOnce(new Response(events(
        { type: 'content_block_start', index: 0, content_block: { type: 'thinking', thinking: '' } },
        { type: 'content_block_delta', index: 0, delta: { type: 'thinking_delta', thinking: 'Use a chart' } },
        { type: 'content_block_delta', index: 0, delta: { type: 'signature_delta', signature: 'signed-' } },
        { type: 'content_block_delta', index: 0, delta: { type: 'signature_delta', signature: 'thought' } },
        { type: 'content_block_start', index: 1, content_block: { type: 'redacted_thinking', data: 'opaque-data' } },
        { type: 'content_block_start', index: 2, content_block: { type: 'tool_use', id: 'native-1', name: 'chart', input: { x: 1 } } },
        { type: 'message_stop' },
      )))
      .mockResolvedValueOnce(new Response(events({ type: 'content_block_delta', delta: { type: 'text_delta', text: 'Done' } }, { type: 'message_stop' })))
    const onTurn = vi.fn()
    const onReasoningDelta = vi.fn()
    await chatAPI.stream(options({ platform: 'anthropic', model: 'claude-sonnet-4-6', settings: { reasoningEffort: 'high' }, tools: [{ name: 'chart', description: 'Chart', inputSchema: {} }], executeTool: async call => ({ toolCallId: call.id, name: call.name, content: 'Created' }), onTurn, onReasoningDelta }))
    const blocks = [{ type: 'thinking', thinking: 'Use a chart', signature: 'signed-thought' }, { type: 'redacted_thinking', data: 'opaque-data' }, { type: 'tool_use', id: 'native-1', name: 'chart', input: { x: 1 } }]
    const body = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)
    expect(body.messages).toContainEqual({ role: 'assistant', content: blocks })
    expect(onReasoningDelta.mock.calls.flat().join('')).toBe('Use a chart')
    const mapped = await mapChatMessages([{ ...message(), role: 'assistant', content: 'Done', turns: onTurn.mock.calls.map(([turn]) => turn) }], 'anthropic')
    expect(mapped.messages[0]).toEqual({ role: 'assistant', content: blocks })
  })

  it('separates split think tags and native reasoning without losing answer text', async () => {
    const onDelta = vi.fn()
    const onReasoningDelta = vi.fn()
    vi.mocked(authenticatedFetch).mockResolvedValueOnce(new Response(events(
      { choices: [{ delta: { reasoning_content: 'Native. ' } }] },
      ...['<thi', 'nk>Plan', '</th', 'ink>Answer'].map(content => ({ choices: [{ delta: { content } }] })), '[DONE]',
    )))
    expect(await chatAPI.stream(options({ onDelta, onReasoningDelta }))).toBe('Answer')
    expect(onDelta.mock.calls.flat().join('')).toBe('Answer')
    expect(onReasoningDelta.mock.calls.flat().join('')).toBe('Native. Plan')
  })

  it('executes a structured chart call and returns the result in the next model round', async () => {
    vi.mocked(authenticatedFetch)
      .mockResolvedValueOnce(new Response(events(
        { choices: [{ delta: { tool_calls: [{ index: 0, id: 'tool-1', function: { name: 'chart', arguments: '{"values":' } }] } }] },
        { choices: [{ delta: { tool_calls: [{ index: 0, function: { arguments: '[1,2]}' } }] } }] }, '[DONE]',
      )))
      .mockResolvedValueOnce(new Response(events({ choices: [{ delta: { content: 'Chart created' } }] }, { usage: { prompt_tokens: 10, completion_tokens: 2 } }, '[DONE]')))
    const executeTool = vi.fn(async () => ({ toolCallId: 'tool-1', name: 'chart', content: 'Created', data: { type: 'chart' } }))
    const onToolResult = vi.fn()
    const onUsage = vi.fn()
    const result = await chatAPI.stream(options({ tools: [{ name: 'chart', description: 'Chart', inputSchema: { type: 'object' } }], executeTool, onToolResult, onUsage }))
    expect(result).toBe('Chart created')
    expect(executeTool).toHaveBeenCalledWith({ id: 'tool-1', name: 'chart', arguments: { values: [1, 2] } })
    expect(onToolResult).toHaveBeenCalledWith(expect.objectContaining({ data: { type: 'chart' } }))
    const body = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)
    expect(body.messages).toContainEqual({ role: 'tool', tool_call_id: 'tool-1', content: 'Created' })
    expect(onUsage).toHaveBeenLastCalledWith({ input_tokens: undefined, output_tokens: undefined })
  })

  it('parses Anthropic tool JSON fragments and preserves tool-use/result history', async () => {
    vi.mocked(authenticatedFetch)
      .mockResolvedValueOnce(new Response(events(
        { type: 'content_block_start', index: 0, content_block: { type: 'tool_use', id: 'native-1', name: 'chart', input: {} } },
        { type: 'content_block_delta', index: 0, delta: { type: 'input_json_delta', partial_json: '{"x":1}' } }, { type: 'message_stop' },
      )))
      .mockResolvedValueOnce(new Response(events({ type: 'content_block_delta', delta: { type: 'text_delta', text: 'Done' } }, { type: 'message_stop' })))
    await chatAPI.stream(options({ platform: 'anthropic', model: 'claude-sonnet-4', tools: [{ name: 'chart', description: 'Chart', inputSchema: {} }], executeTool: async call => ({ toolCallId: call.id, name: call.name, content: 'Created' }) }))
    const body = JSON.parse(vi.mocked(authenticatedFetch).mock.calls[1]![1]?.body as string)
    expect(body.messages).toContainEqual({ role: 'assistant', content: [{ type: 'tool_use', id: 'native-1', name: 'chart', input: { x: 1 } }] })
    expect(body.messages).toContainEqual({ role: 'user', content: [{ type: 'tool_result', tool_use_id: 'native-1', content: 'Created' }] })
  })

  it('limits tool continuation to five provider calls and never executes unknown tools', async () => {
    vi.mocked(authenticatedFetch).mockImplementation(async () => new Response(events({ choices: [{ delta: { tool_calls: [{ index: 0, id: 'unknown', function: { name: 'web_fetch', arguments: '{}' } }] } }] }, '[DONE]')))
    const executeTool = vi.fn()
    await chatAPI.stream(options({ tools: [{ name: 'chart', description: 'Chart', inputSchema: {} }], executeTool }))
    expect(authenticatedFetch).toHaveBeenCalledTimes(5)
    expect(executeTool).not.toHaveBeenCalled()
  })

  it('retains partial text on unexpected EOF and does not repeat the upstream request', async () => {
    vi.mocked(authenticatedFetch).mockResolvedValueOnce(new Response(events({ choices: [{ delta: { content: 'Partial' } }] })))
    const onDelta = vi.fn()
    await expect(chatAPI.stream(options({ onDelta }))).rejects.toThrow('partial reply was retained')
    expect(onDelta).toHaveBeenCalledWith('Partial')
    expect(authenticatedFetch).toHaveBeenCalledTimes(1)
  })

  it('rejects incompatible models and oversized attachments, but permits text files for text-only models', () => {
    const image = new File(['image'], 'image.png', { type: 'image/png' })
    expect(() => validateChatAttachments([image], 'openai', 'gpt-3.5-turbo')).toThrow('vision model')
    expect(() => validateChatAttachments([new File(['notes'], 'notes.txt')], 'openai', 'gpt-3.5-turbo')).not.toThrow()
    expect(() => validateChatAttachments(Array(9).fill(image))).toThrow('at most 8')
  })

  it.each(['grok-4', 'grok-4.1-fast-reasoning', 'grok-4.6', 'x-ai/grok-4.1', 'grok-2-vision-1212'])('preserves image input for %s without enabling PDF', async model => {
    const image = new File(['image'], 'image.png', { type: 'image/png' })
    const pdf = new File(['pdf'], 'paper.pdf', { type: 'application/pdf' })
    expect(() => validateChatAttachments([image], 'grok', model)).not.toThrow()
    const mapped = await mapChatMessages([message([image])], 'grok', model)
    expect(mapped.messages[0]?.content).toContainEqual({ type: 'image_url', image_url: { url: 'data:image/png;base64,aW1hZ2U=' } })
    expect(() => validateChatAttachments([pdf], 'grok', model)).toThrow('PDF attachments')
  })

  it('honors explicit attachment metadata and restricts PDF aliases by the actual group platform', () => {
    expect(getChatAttachmentCapabilities('custom-model', { image: true, pdf: true }, 'openai')).toEqual({ image: true, pdf: true })
    expect(getChatAttachmentCapabilities('gpt-4o', { image: false }, 'openai')).toEqual({ image: false, pdf: true })
    expect(getChatAttachmentCapabilities('gpt-4o', { pdf: true }, 'grok')).toEqual({ image: true, pdf: false })
    expect(getChatAttachmentCapabilities('grok-3', undefined, 'grok')).toEqual({ image: false, pdf: false })
  })

  it('does not misclassify legacy reasoning and tool blocks as attachments', async () => {
    const assistant: LocalChatMessage = { ...message(), role: 'assistant', content: '', rawContent: [{ type: 'thinking', thinking: 'Private reasoning', signature: 'sig' }, { type: 'tool_use', id: 'old-tool', name: 'chart', input: { x: 1 } }] }
    const mapped = await mapChatMessages([assistant], 'anthropic')
    expect(mapped.messages[0]?.content).toEqual([{ type: 'thinking', thinking: 'Private reasoning', signature: 'sig' }, { type: 'tool_use', id: 'old-tool', name: 'chart', input: { x: 1 } }])
  })
})
