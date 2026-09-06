import type { CreationTokenUsage } from './types'

export interface ParsedSSEChunk {
  data: string
  done: boolean
}

/**
 * Split an SSE text buffer into complete lines, keeping the trailing partial line.
 */
export function splitSSEBuffer(buffer: string): { lines: string[]; remainder: string } {
  const parts = buffer.split('\n')
  const remainder = parts.pop() ?? ''
  return { lines: parts, remainder }
}

/**
 * Parse `data:` lines from SSE chunks and return JSON payload strings.
 */
export function parseSSELines(lines: string[]): ParsedSSEChunk[] {
  const chunks: ParsedSSEChunk[] = []
  for (const line of lines) {
    if (!line.startsWith('data:')) continue
    const payload = line.slice(5).trim()
    if (!payload) continue
    if (payload === '[DONE]') {
      chunks.push({ data: '', done: true })
      continue
    }
    chunks.push({ data: payload, done: false })
  }
  return chunks
}

/**
 * Extract assistant text deltas from OpenAI or Anthropic-style SSE JSON payloads.
 */
export function extractTextDeltaFromSSEData(data: string): string | null {
  if (!data) return null
  try {
    const parsed = JSON.parse(data) as Record<string, unknown>

    const choices = parsed.choices as Array<{ delta?: { content?: string } }> | undefined
    if (choices?.[0]?.delta?.content) {
      return String(choices[0].delta.content)
    }

    if (parsed.type === 'content_block_delta') {
      const delta = parsed.delta as { type?: string; text?: string } | undefined
      if (delta?.type === 'text_delta' && delta.text) {
        return delta.text
      }
    }

    if (parsed.type === 'message_delta') {
      const delta = parsed.delta as { type?: string; text?: string } | undefined
      if (delta?.text) return delta.text
    }

    const delta = parsed.delta as { text?: string; content?: string } | undefined
    if (delta?.text) return delta.text
    if (delta?.content) return String(delta.content)
  } catch {
    return null
  }
  return null
}

/**
 * Consume an SSE buffer incrementally and return extracted text deltas.
 */
export function parseSSEBuffer(buffer: string): { deltas: string[]; remainder: string; done: boolean; usage: CreationTokenUsage } {
  const { lines, remainder } = splitSSEBuffer(buffer)
  const chunks = parseSSELines(lines)
  const deltas: string[] = []
  let done = false
  const usage: CreationTokenUsage = {}
  for (const chunk of chunks) {
    if (chunk.done) {
      done = true
      break
    }
    const payload = JSON.parse(chunk.data) as Record<string, unknown>
    if (payload.type === 'error' || payload.error) {
      const error = payload.error as { message?: string } | string | undefined
      throw new Error(typeof error === 'string' ? error : error?.message || 'Chat stream failed')
    }
    const message = payload.message as Record<string, unknown> | undefined
    const rawUsage = (payload.type === 'message_start' ? message?.usage : payload.usage) as Record<string, unknown> | undefined
    if (rawUsage && typeof rawUsage === 'object') {
      const input = rawUsage.input_tokens ?? rawUsage.prompt_tokens
      const output = rawUsage.output_tokens ?? rawUsage.completion_tokens
      if (typeof input === 'number' && Number.isSafeInteger(input) && input >= 0) usage.input_tokens = input
      if (typeof output === 'number' && Number.isSafeInteger(output) && output >= 0) usage.output_tokens = output
    }
    const delta = extractTextDeltaFromSSEData(chunk.data)
    if (delta) deltas.push(delta)
    if (payload.type === 'message_stop') {
      done = true
      break
    }
  }
  return { deltas, remainder, done, usage }
}
