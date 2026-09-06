import { describe, expect, it } from 'vitest'
import { totalMessageTokens } from '../tokenUsage'
import type { CreationMessage } from '../types'

function message(id: number, input?: number, output?: number): CreationMessage {
  return { id, session_id: 1, role: 'assistant', content: 'Answer', input_tokens: input, output_tokens: output, created_at: '' }
}

describe('creation token totals', () => {
  it('sums known assistant usage without counting user rows twice', () => {
    expect(totalMessageTokens([message(1, 10, 20), message(2, 30, 40), { ...message(3), role: 'user' }])).toEqual({ input: 40, output: 60 })
  })

  it('keeps each total unknown if any historical reply lacks its usage', () => {
    expect(totalMessageTokens([message(1, 10, 20), message(2, 30)])).toEqual({ input: 40, output: null })
    expect(totalMessageTokens([message(1, 10, 20), message(2)])).toEqual({ input: null, output: null })
    expect(totalMessageTokens([])).toEqual({ input: null, output: null })
  })
})
