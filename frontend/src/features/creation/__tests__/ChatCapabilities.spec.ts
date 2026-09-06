import { describe, expect, it } from 'vitest'
import { getChatReasoningEfforts, supportsChatTemperature } from '../chatCapabilities'

describe('chat capability contracts', () => {
  it.each([
    ['gemini-3-flash-preview', 'gemini', ['minimal', 'low', 'medium', 'high']],
    ['gemini-3-pro-preview', 'antigravity', ['low', 'high']],
    ['gemini-3.1-pro-preview', 'gemini', ['low', 'medium', 'high']],
    ['gemini-2.5-flash', 'gemini', ['none', 'low', 'medium', 'high']],
    ['gemini-2.5-pro', 'antigravity', ['low', 'medium', 'high']],
    ['claude-opus-4-6', 'anthropic', ['low', 'medium', 'high', 'max']],
    ['claude-opus-4-7', 'anthropic', ['low', 'medium', 'high', 'xhigh', 'max']],
    ['claude-opus-4-6-thinking', 'antigravity', ['low', 'medium', 'high']],
    ['anthropic.claude-opus-4-5-20251101-v1:0', 'kiro', ['low', 'medium', 'high']],
    ['gpt-5-pro', 'openai', ['high']],
    ['custom-model', 'openai', []],
  ])('derives %s capabilities from the actual gateway model id on %s', (model, platform, efforts) => {
    expect(getChatReasoningEfforts(model as string, platform as string)).toEqual(efforts)
  })

  it('consumes the Grok gateway camelCase fields and validates every option', () => {
    expect(getChatReasoningEfforts('grok-4.6', 'grok', {
      supportsReasoningEffort: true,
      reasoningEfforts: [{ value: 'low', label: 'Low' }, { value: 'high' }, { value: 'xhigh' }, { value: 'unsupported' }, null, true],
    })).toEqual(['low', 'high', 'xhigh'])
    expect(getChatReasoningEfforts('grok-4.6', 'grok', { supportsReasoningEffort: false })).toEqual([])
    expect(getChatReasoningEfforts('grok-4.6', 'grok')).toEqual([])
  })

  it('never treats boolean reasoning metadata as a structured capability object', () => {
    expect(getChatReasoningEfforts('unknown-model', 'openai', { reasoning: true })).toEqual([])
    expect(getChatReasoningEfforts('gemini-3-pro', 'gemini', true)).toEqual(['low', 'high'])
    expect(getChatReasoningEfforts('unknown-model', 'anthropic', { reasoning: { supported: true, efforts: [{ value: 'max' }] } })).toEqual([])
  })

  it('uses the same temperature guard for standard and provider-qualified model ids', () => {
    expect(supportsChatTemperature('openai/gpt-5.4')).toBe(false)
    expect(supportsChatTemperature('o3-pro')).toBe(false)
    expect(supportsChatTemperature('gpt-4o')).toBe(true)
  })
})
