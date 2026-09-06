import type { CreationMessage } from './types'

export function totalMessageTokens(messages: CreationMessage[]): { input: number | null; output: number | null } {
  const replies = messages.filter((message) => message.role === 'assistant')
  function total(field: 'input_tokens' | 'output_tokens'): number | null {
    if (replies.length === 0 || replies.some((message) => typeof message[field] !== 'number')) return null
    return replies.reduce((sum, message) => sum + (message[field] ?? 0), 0)
  }
  return { input: total('input_tokens'), output: total('output_tokens') }
}
