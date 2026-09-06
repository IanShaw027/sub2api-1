import { apiClient, buildApiUrl } from '@/api/client'
import { authenticatedFetch } from '@/api/authenticatedFetch'
import type { GroupPlatform } from '@/types'
import { ANTHROPIC_STYLE_PLATFORMS, type CreationTokenUsage, type GatewayModelList } from './types'
import { extractTextDeltaFromSSEData, parseSSEBuffer, parseSSELines, splitSSEBuffer } from './sse'
import type { ChatSettings, ChatSource, ChatThinkingBlock, ChatToolCall, ChatToolDefinition, ChatToolResult, ChatTurn, LocalChatMessage } from './localChat'
import { getChatReasoningEfforts, supportsChatTemperature } from './chatCapabilities'

export const CHAT_ATTACHMENT_ACCEPT = '.txt,.md,.markdown,.json,.csv,.pdf,image/png,image/jpeg,image/webp,image/gif'
const IMAGE_TYPES = new Set(['image/png', 'image/jpeg', 'image/webp', 'image/gif'])
export interface ChatModelMetadata {
  image?: boolean
  pdf?: boolean
  supportsReasoningEffort?: boolean
  reasoningEfforts?: Array<{ value: string; label?: string; default?: boolean }>
}
export function getChatAttachmentCapabilities(model: string, metadata?: ChatModelMetadata, platform?: GroupPlatform): { image: boolean; pdf: boolean } {
  const grok = platform === 'grok' || /(?:^|\/)grok(?:-|$)/i.test(model)
  const documents = /gpt-4\.1|gpt-4o|gpt-5|claude|gemini/i.test(model)
  const grokImages = /(?:^|\/)grok-(?:4(?:[.-]|$)|2-vision)/i.test(model)
  return {
    image: metadata?.image ?? (grokImages || documents || /qwen.*vl|vision/i.test(model)),
    // Grok's gateway does not translate Chat Completions file blocks, including aliases.
    pdf: !grok && (metadata?.pdf ?? documents),
  }
}
export function validateChatAttachments(files: File[], platform?: GroupPlatform, model?: string, metadata?: ChatModelMetadata): void {
  if (files.length > 8) throw new Error('Attach at most 8 files per message.')
  if (files.reduce((sum, file) => sum + file.size, 0) > 25 * 1024 * 1024) throw new Error('Attachments must total 25 MiB or less.')
  for (const file of files) {
    if (file.size > 10 * 1024 * 1024) throw new Error(`${file.name}: maximum attachment size is 10 MiB.`)
    if (!IMAGE_TYPES.has(file.type) && file.type !== 'application/pdf' && !/\.(txt|md|markdown|json|csv|pdf|png|jpe?g|webp|gif)$/i.test(file.name)) {
      throw new Error(`${file.name}: attach an image, PDF, TXT, Markdown, JSON or CSV file.`)
    }
    if (model) {
      const capability = getChatAttachmentCapabilities(model, metadata, platform)
      const mime = fileMime(file)
      if (IMAGE_TYPES.has(mime) && !capability.image) throw new Error(`${model} does not support image attachments. Select a vision model.`)
      if (mime === 'application/pdf' && !capability.pdf) throw new Error(`${model} does not support PDF attachments. Use a document-capable model or a text file.`)
    }
  }
}
export const validateChatFiles = validateChatAttachments

type Part = { kind: 'text'; text: string } | { kind: 'image' | 'pdf'; url: string; mime: string; name?: string }
type OpenAIContent = { type: 'text'; text: string } | { type: 'image_url'; image_url: { url: string } } | { type: 'file'; file: { filename: string; file_data: string } }
type AnthropicContent = ChatThinkingBlock | { type: 'text'; text: string } | { type: 'image' | 'document'; source: { type: 'base64'; media_type: string; data: string } | { type: 'url'; url: string } } | { type: 'tool_use'; id: string; name: string; input: Record<string, unknown> } | { type: 'tool_result'; tool_use_id: string; content: string; is_error?: boolean }
type OpenAIMessage = { role: 'system' | 'user' | 'assistant' | 'tool'; content: string | OpenAIContent[]; tool_call_id?: string; tool_calls?: Array<{ id: string; type: 'function'; function: { name: string; arguments: string } }> }
type AnthropicMessage = { role: 'user' | 'assistant'; content: AnthropicContent[] }

function readFile(file: Blob, asURL: boolean): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(reader.error ?? new Error('Could not read the attachment.'))
    if (asURL) reader.readAsDataURL(file)
    else reader.readAsText(file)
  })
}
function fileMime(file: File): string {
  if (IMAGE_TYPES.has(file.type) || file.type === 'application/pdf') return file.type
  const ext = file.name.split('.').pop()?.toLowerCase()
  return ({ png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', webp: 'image/webp', gif: 'image/gif', pdf: 'application/pdf' } as Record<string, string>)[ext ?? ''] ?? 'text/plain'
}
function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null
}
function validMediaURL(url: string): boolean {
  return /^https?:\/\//i.test(url) || /^data:(image\/(png|jpeg|webp|gif)|application\/pdf);base64,/i.test(url)
}

function legacyBlocks(raw: unknown): unknown[] {
  if (raw == null || typeof raw === 'string' && !raw.trim().startsWith('[') && !raw.trim().startsWith('{')) return []
  if (typeof raw === 'string') {
    try { raw = JSON.parse(raw) } catch { return [] }
  }
  return Array.isArray(raw) ? raw : [raw]
}
function legacyMediaParts(raw: unknown): Part[] {
  const blocks = legacyBlocks(raw)
  const parts: Part[] = []
  for (const block of blocks) {
    if (typeof block === 'string') continue
    const item = record(block)
    if (!item) continue
    if (item.type === 'text' || item.type === 'input_text' || !item.type && typeof item.text === 'string') continue
    if (['thinking', 'redacted_thinking', 'reasoning', 'tool_use', 'tool_result'].includes(String(item.type))) continue
    const image = record(item.image_url)
    const source = record(item.source)
    const file = record(item.file)
    const url = typeof image?.url === 'string' ? image.url
      : typeof item.image_url === 'string' ? item.image_url
        : typeof file?.file_data === 'string' ? file.file_data
          : source?.type === 'base64' && typeof source.data === 'string' ? `data:${source.media_type};base64,${source.data}`
            : source?.type === 'url' && typeof source.url === 'string' ? source.url : null
    if (!url || !validMediaURL(url)) throw new Error('This imported message contains an unsupported attachment. Export the original conversation before editing it.')
    const mime = /^data:([^;]+);/.exec(url)?.[1] ?? (item.type === 'document' || item.type === 'file' ? 'application/pdf' : 'image/png')
    parts.push({ kind: mime === 'application/pdf' ? 'pdf' : 'image', url, mime, name: typeof file?.filename === 'string' ? file.filename : undefined })
  }
  return parts
}

async function messageParts(message: LocalChatMessage, platform: GroupPlatform, model?: string, metadata?: ChatModelMetadata): Promise<Part[]> {
  validateChatAttachments(message.files, platform, model, metadata)
  const parts: Part[] = message.content ? [{ kind: 'text', text: message.content }] : []
  const legacy = legacyMediaParts(message.rawContent)
  if (model) {
    const capability = getChatAttachmentCapabilities(model, metadata, platform)
    if (legacy.some(part => part.kind === 'image' && !capability.image || part.kind === 'pdf' && !capability.pdf)) throw new Error('The selected model does not support attachments in the imported conversation.')
  }
  parts.push(...legacy)
  for (const file of message.files) {
    const mime = fileMime(file)
    if (mime === 'text/plain') {
      parts.push({ kind: 'text', text: `Attached file: ${file.name}\n${await readFile(file, false)}` })
    } else {
      const blob = file.type === mime ? file : new Blob([file], { type: mime })
      parts.push({ kind: mime === 'application/pdf' ? 'pdf' : 'image', mime, name: file.name, url: await readFile(blob, true) })
    }
  }
  return parts
}
function openAIParts(parts: Part[]): OpenAIContent[] {
  return parts.map(part => part.kind === 'text' ? { type: 'text', text: part.text }
    : part.kind === 'image' ? { type: 'image_url', image_url: { url: part.url } }
      : { type: 'file', file: { filename: part.name ?? 'attachment.pdf', file_data: part.url } })
}
function anthropicParts(parts: Part[]): AnthropicContent[] {
  return parts.map(part => {
    if (part.kind === 'text') return { type: 'text', text: part.text }
    const base64 = /^data:[^;]+;base64,(.*)$/s.exec(part.url)
    return { type: part.kind === 'image' ? 'image' : 'document', source: base64
      ? { type: 'base64', media_type: part.mime, data: base64[1]! }
      : { type: 'url', url: part.url } }
  })
}

export async function mapChatMessages(messages: LocalChatMessage[], platform: GroupPlatform, model?: string, metadata?: ChatModelMetadata): Promise<{ messages: OpenAIMessage[] | AnthropicMessage[]; system?: string }> {
  const anthropic = ANTHROPIC_STYLE_PLATFORMS.has(platform)
  const openAI: OpenAIMessage[] = []
  const native: AnthropicMessage[] = []
  const system: string[] = []
  for (const message of messages) {
    if (message.role === 'system') { system.push(message.content); openAI.push({ role: 'system', content: message.content }); continue }
    if (message.role === 'assistant' && message.turns?.length) {
      for (const turn of message.turns) {
        appendTurn(turn, openAI, native)
      }
      const streamedContent = message.turns.map(turn => turn.content).join('')
      const partial = message.content.slice(streamedContent.length)
      if (partial) appendTurn({ content: partial, toolCalls: [], toolResults: [] }, openAI, native)
      continue
    }
    if (message.role === 'assistant' && !message.content && !message.files.length && !message.rawContent) continue
    const parts = await messageParts(message, platform, model, metadata)
    const legacy = legacyBlocks(message.rawContent).map(record).filter((item): item is Record<string, unknown> => item != null)
    const calls = legacy.filter(item => item.type === 'tool_use').map(item => ({ id: String(item.id ?? ''), type: 'function' as const, function: { name: String(item.name ?? ''), arguments: JSON.stringify(item.input ?? {}) } }))
    const results = legacy.filter(item => item.type === 'tool_result')
    for (const result of results) openAI.push({ role: 'tool', tool_call_id: String(result.tool_use_id ?? ''), content: typeof result.content === 'string' ? result.content : JSON.stringify(result.content ?? '') })
    if (parts.length || calls.length) openAI.push({ role: message.role, content: openAIParts(parts), ...(calls.length ? { tool_calls: calls } : {}) })
    const nativeTools: AnthropicContent[] = [
      ...calls.map(call => ({ type: 'tool_use' as const, id: call.id, name: call.function.name, input: JSON.parse(call.function.arguments) as Record<string, unknown> })),
      ...results.map(result => ({ type: 'tool_result' as const, tool_use_id: String(result.tool_use_id ?? ''), content: typeof result.content === 'string' ? result.content : JSON.stringify(result.content ?? ''), is_error: result.is_error === true })),
    ]
    const thinking = legacy.flatMap((item): ChatThinkingBlock[] => item.type === 'thinking' && typeof item.thinking === 'string' && typeof item.signature === 'string'
      ? [{ type: 'thinking', thinking: item.thinking, signature: item.signature }]
      : item.type === 'redacted_thinking' && typeof item.data === 'string' ? [{ type: 'redacted_thinking', data: item.data }] : [])
    if (parts.length || nativeTools.length) native.push({ role: message.role, content: [...thinking, ...anthropicParts(parts), ...nativeTools] })
  }
  return anthropic ? { messages: native, system: system.length ? system.join('\n\n') : undefined } : { messages: openAI }
}
function appendTurn(turn: ChatTurn, openAI: OpenAIMessage[], native: AnthropicMessage[]): void {
  const calls = turn.toolCalls.filter(call => turn.toolResults.some(result => result.toolCallId === call.id))
  if (!turn.content && !calls.length) return
  openAI.push({ role: 'assistant', content: turn.content, ...(calls.length ? { tool_calls: calls.map(call => ({ id: call.id, type: 'function' as const, function: { name: call.name, arguments: JSON.stringify(call.arguments) } })) } : {}) })
  native.push({ role: 'assistant', content: [...(turn.thinkingBlocks ?? []), ...(turn.content ? [{ type: 'text' as const, text: turn.content }] : []), ...calls.map(call => ({ type: 'tool_use' as const, id: call.id, name: call.name, input: call.arguments }))] })
  for (const result of turn.toolResults.filter(result => calls.some(call => call.id === result.toolCallId))) {
    openAI.push({ role: 'tool', tool_call_id: result.toolCallId, content: result.content })
    native.push({ role: 'user', content: [{ type: 'tool_result', tool_use_id: result.toolCallId, content: result.content, is_error: result.isError }] })
  }
}

export interface ChatStreamOptions {
  groupId: number
  model: string
  platform: GroupPlatform
  modelMetadata?: ChatModelMetadata
  messages: LocalChatMessage[]
  settings?: ChatSettings
  signal: AbortSignal
  tools?: ChatToolDefinition[]
  executeTool?: (call: ChatToolCall) => Promise<ChatToolResult>
  onDelta: (text: string) => void
  onReasoningDelta?: (text: string) => void
  onUsage?: (usage: CreationTokenUsage) => void
  onSource?: (source: ChatSource) => void
  onToolCall?: (call: ChatToolCall) => void
  onToolResult?: (result: ChatToolResult) => void
  onTurn?: (turn: ChatTurn) => void
}

function assertRunning(signal: AbortSignal): void {
  if (signal.aborted) throw new DOMException('Stopped', 'AbortError')
}
async function streamRound(response: Response, options: ChatStreamOptions): Promise<{ content: string; calls: ChatToolCall[]; usage: CreationTokenUsage; thinkingBlocks: ChatThinkingBlock[] }> {
  const reader = response.body?.getReader()
  if (!reader) throw new Error('No response body')
  const decoder = new TextDecoder()
  let buffer = ''
  let completed = false
  let content = ''
  let thinkBuffer = ''
  let inThink = false
  function emitText(text: string, final = false): void {
    thinkBuffer += text
    while (thinkBuffer) {
      const tag = inThink ? '</think>' : '<think>'
      const index = thinkBuffer.indexOf(tag)
      if (index >= 0) {
        const value = thinkBuffer.slice(0, index)
        if (inThink) options.onReasoningDelta?.(value)
        else { content += value; if (value) options.onDelta(value) }
        thinkBuffer = thinkBuffer.slice(index + tag.length)
        inThink = !inThink
        continue
      }
      let keep = 0
      if (!final) for (let length = 1; length < tag.length; length += 1) if (thinkBuffer.endsWith(tag.slice(0, length))) keep = length
      const value = thinkBuffer.slice(0, thinkBuffer.length - keep)
      if (inThink) options.onReasoningDelta?.(value)
      else { content += value; if (value) options.onDelta(value) }
      thinkBuffer = keep ? thinkBuffer.slice(-keep) : ''
      break
    }
  }
  const usage: CreationTokenUsage = {}
  const calls = new Map<number, { id: string; name: string; json: string; input?: Record<string, unknown> }>()
  const thinkingBlocks = new Map<number, ChatThinkingBlock>()
  function consume(): void {
    const split = splitSSEBuffer(buffer)
    buffer = split.remainder
    for (const chunk of parseSSELines(split.lines)) {
      if (chunk.done) { completed = true; break }
      const payload = JSON.parse(chunk.data) as Record<string, unknown>
      const parsed = parseSSEBuffer(`data: ${chunk.data}\n`)
      Object.assign(usage, parsed.usage)
      if (Object.keys(parsed.usage).length) options.onUsage?.({ ...usage })
      const text = extractTextDeltaFromSSEData(chunk.data)
      if (text) emitText(text)
      const choices = payload.choices as Array<{ delta?: Record<string, unknown> }> | undefined
      const delta = choices?.[0]?.delta ?? record(payload.delta)
      const reasoning = delta?.reasoning_content ?? delta?.reasoning ?? delta?.thinking
      if (typeof reasoning === 'string') options.onReasoningDelta?.(reasoning)
      const toolDeltas = delta?.tool_calls as Array<{ index: number; id?: string; function?: { name?: string; arguments?: string } }> | undefined
      for (const item of toolDeltas ?? []) {
        const call = calls.get(item.index) ?? { id: '', name: '', json: '' }
        if (item.id) call.id = item.id
        if (item.function?.name) call.name += item.function.name
        if (item.function?.arguments) call.json += item.function.arguments
        calls.set(item.index, call)
      }
      if (payload.type === 'content_block_start') {
        const block = record(payload.content_block)
        if (block?.type === 'tool_use') calls.set(Number(payload.index), { id: String(block.id ?? ''), name: String(block.name ?? ''), json: '', input: record(block.input) ?? {} })
        if (block?.type === 'text' && typeof block.text === 'string' && block.text) emitText(block.text)
        if (block?.type === 'thinking') {
          const thinking = typeof block.thinking === 'string' ? block.thinking : ''
          thinkingBlocks.set(Number(payload.index), { type: 'thinking', thinking, signature: typeof block.signature === 'string' ? block.signature : '' })
          if (thinking) options.onReasoningDelta?.(thinking)
        }
        if (block?.type === 'redacted_thinking' && typeof block.data === 'string') thinkingBlocks.set(Number(payload.index), { type: 'redacted_thinking', data: block.data })
      }
      if (payload.type === 'content_block_delta') {
        const block = thinkingBlocks.get(Number(payload.index))
        if (block?.type === 'thinking' && delta?.type === 'thinking_delta' && typeof delta.thinking === 'string') block.thinking += delta.thinking
        if (block?.type === 'thinking' && delta?.type === 'signature_delta' && typeof delta.signature === 'string') block.signature += delta.signature
      }
      if (payload.type === 'content_block_delta' && delta?.type === 'input_json_delta') {
        const call = calls.get(Number(payload.index))
        if (call && typeof delta.partial_json === 'string') call.json += delta.partial_json
      }
      const annotations = Array.isArray(delta?.annotations) ? delta.annotations : []
      if (delta?.citation) annotations.push(delta.citation)
      for (const item of annotations) {
        const annotation = record(item)
        const citation = record(annotation?.url_citation) ?? annotation
        if (typeof citation?.url === 'string' && /^https?:\/\//i.test(citation.url)) options.onSource?.({ url: citation.url, title: typeof citation.title === 'string' ? citation.title : undefined })
      }
      if (parsed.done) { completed = true; break }
    }
  }
  const cancel = () => { void reader.cancel().catch(() => undefined) }
  options.signal.addEventListener('abort', cancel, { once: true })
  try {
    while (!completed) {
      assertRunning(options.signal)
      const { done, value } = await reader.read()
      assertRunning(options.signal)
      if (done) {
        buffer += decoder.decode()
        if (buffer.trim()) { buffer += '\n'; consume() }
        break
      }
      buffer += decoder.decode(value, { stream: true })
      consume()
    }
    emitText('', true)
    if (!completed) throw new Error('Chat stream ended before completion. The partial reply was retained; it was not resent.')
    return { content, usage, thinkingBlocks: [...thinkingBlocks.values()].filter(block => block.type === 'redacted_thinking' || !!block.signature), calls: [...calls.values()].map(call => {
      const input = call.json ? record(JSON.parse(call.json)) : call.input ?? {}
      if (!call.id || !call.name || !input) throw new Error('The model returned an invalid tool call.')
      return { id: call.id, name: call.name, arguments: input }
    }) }
  } finally {
    options.signal.removeEventListener('abort', cancel)
    await reader.cancel().catch(() => undefined)
    reader.releaseLock()
  }
}

export const chatAPI = {
  async getModels(groupId: number, signal?: AbortSignal): Promise<GatewayModelList> {
    const { data } = await apiClient.get<GatewayModelList>('/creation/models', { params: { group_id: groupId }, signal })
    return data
  },
  async stream(options: ChatStreamOptions): Promise<string> {
    const messages = [...options.messages]
    if (options.settings?.systemPrompt) messages.unshift({ id: 'system', role: 'system', content: options.settings.systemPrompt, files: [], status: 'completed', createdAt: '' })
    const mapped = await mapChatMessages(messages, options.platform, options.model, options.modelMetadata)
    const anthropic = ANTHROPIC_STYLE_PLATFORMS.has(options.platform)
    const roundUsages: CreationTokenUsage[] = []
    let fullText = ''
    for (let round = 0; round < 5; round += 1) {
      assertRunning(options.signal)
      const token = localStorage.getItem('auth_token')
      if (!token) throw new Error('Sign in before sending a message.')
      const body: Record<string, unknown> = { model: options.model, stream: true, messages: mapped.messages }
      if (anthropic) {
        body.max_tokens = options.settings?.maxTokens ?? 4096
        if (mapped.system) body.system = mapped.system
        const effort = options.settings?.reasoningEffort
        if (effort && getChatReasoningEfforts(options.model, options.platform, options.modelMetadata).includes(effort)) {
          if (effort === 'none') body.thinking = { type: 'disabled' }
          else if (options.platform === 'antigravity' && /claude/i.test(options.model)) {
            const budget = effort === 'low' ? 1024 : effort === 'medium' ? 8192 : 24576
            body.thinking = { type: 'enabled', budget_tokens: budget }
            body.max_tokens = Math.max(Number(body.max_tokens), budget + 1024)
          } else if (options.platform === 'kiro') body.thinking = { type: 'adaptive', thinking_effort: effort }
          else {
            body.output_config = { effort }
            if (/claude/i.test(options.model)) {
              if (/claude-opus-4[.-]5(?:-|$)/i.test(options.model)) {
                const budget = effort === 'low' ? 1024 : effort === 'medium' ? 8192 : 24576
                body.thinking = { type: 'enabled', budget_tokens: budget }
                body.max_tokens = Math.max(Number(body.max_tokens), budget + 1024)
              } else body.thinking = { type: 'adaptive' }
            }
          }
        }
      } else {
        body.stream_options = { include_usage: true }
        if (options.settings?.maxTokens) body.max_completion_tokens = options.settings.maxTokens
        const effort = options.settings?.reasoningEffort === 'max' ? 'xhigh' : options.settings?.reasoningEffort
        if (effort && getChatReasoningEfforts(options.model, options.platform, options.modelMetadata).includes(effort)) body.reasoning_effort = effort
      }
      if (options.settings?.temperature != null && supportsChatTemperature(options.model) && !body.thinking) body.temperature = options.settings.temperature
      if (options.tools?.length && options.executeTool) body.tools = anthropic
        ? options.tools.map(tool => ({ name: tool.name, description: tool.description, input_schema: tool.inputSchema }))
        : options.tools.map(tool => ({ type: 'function', function: { name: tool.name, description: tool.description, parameters: tool.inputSchema } }))
      const response = await authenticatedFetch(buildApiUrl(`/creation/${anthropic ? 'messages' : 'chat/completions'}?group_id=${options.groupId}`), {
        method: 'POST', headers: { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json', 'X-Group-Id': String(options.groupId) },
        body: JSON.stringify(body), signal: options.signal,
      })
      if (!response.ok) throw new Error(await response.text() || `Chat request failed (${response.status})`)
      const emitUsage = (current?: CreationTokenUsage) => {
        const usages = current ? [...roundUsages, current] : roundUsages
        const sum = (key: 'input_tokens' | 'output_tokens') => usages.every(usage => usage[key] != null) ? usages.reduce((total, usage) => total + usage[key]!, 0) : undefined
        options.onUsage?.({ input_tokens: sum('input_tokens'), output_tokens: sum('output_tokens') })
      }
      const result = await streamRound(response, { ...options, onUsage: emitUsage })
      assertRunning(options.signal)
      fullText += result.content
      roundUsages.push(result.usage)
      emitUsage()
      const turn: ChatTurn = { content: result.content, toolCalls: result.calls, toolResults: [], ...(result.thinkingBlocks.length ? { thinkingBlocks: result.thinkingBlocks } : {}) }
      for (const call of result.calls) {
        assertRunning(options.signal)
        options.onToolCall?.(call)
        let output: ChatToolResult
        try {
          if (!options.executeTool || !options.tools?.some(tool => tool.name === call.name)) throw new Error('This tool is not available in the local workspace.')
          output = await options.executeTool(call)
        } catch (failure) {
          output = { toolCallId: call.id, name: call.name, isError: true, content: failure instanceof Error ? failure.message : 'Tool execution failed' }
        }
        assertRunning(options.signal)
        turn.toolResults.push(output)
        options.onToolResult?.(output)
      }
      options.onTurn?.(turn)
      if (!result.calls.length) return fullText
      if (anthropic) appendTurn(turn, [], mapped.messages as AnthropicMessage[])
      else appendTurn(turn, mapped.messages as OpenAIMessage[], [])
    }
    return fullText
  },
}
