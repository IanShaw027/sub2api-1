import { describe, expect, it } from 'vitest'
import { extractTextDeltaFromSSEData, parseSSEBuffer, parseSSELines, splitSSEBuffer } from '../sse'

describe('creation sse parser', () => {
  it('splits buffer and keeps trailing partial line', () => {
    const { lines, remainder } = splitSSEBuffer('data: {"ok":true}\ndata: [DONE]\npartial')
    expect(lines).toEqual(['data: {"ok":true}', 'data: [DONE]'])
    expect(remainder).toBe('partial')
  })

  it('parses OpenAI chat completion deltas', () => {
    const payload = JSON.stringify({ choices: [{ delta: { content: 'Hello' } }] })
    expect(extractTextDeltaFromSSEData(payload)).toBe('Hello')
  })

  it('parses Anthropic content_block_delta chunks', () => {
    const payload = JSON.stringify({
      type: 'content_block_delta',
      delta: { type: 'text_delta', text: 'Hi there' },
    })
    expect(extractTextDeltaFromSSEData(payload)).toBe('Hi there')
  })

  it('extracts deltas from an SSE buffer', () => {
    const openai = JSON.stringify({ choices: [{ delta: { content: 'A' } }] })
    const anthropic = JSON.stringify({
      type: 'content_block_delta',
      delta: { type: 'text_delta', text: 'B' },
    })
    const buffer = `data: ${openai}\ndata: ${anthropic}\ndata: [DONE]\n`
    const chunks = parseSSELines(buffer.split('\n'))
    expect(chunks.length).toBe(3)
    const { deltas } = parseSSEBuffer(buffer)
    expect(deltas).toEqual(['A', 'B'])
  })
})
