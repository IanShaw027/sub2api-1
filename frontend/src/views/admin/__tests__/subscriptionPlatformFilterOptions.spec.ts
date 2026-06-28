import { describe, expect, it } from 'vitest'
import { buildSubscriptionPlatformFilterOptions } from '../subscriptionPlatformFilterOptions'

describe('buildSubscriptionPlatformFilterOptions', () => {
  it('includes every subscription-capable platform filter option', () => {
    const options = buildSubscriptionPlatformFilterOptions('All platforms')

    expect(options).toEqual([
      { value: '', label: 'All platforms' },
      { value: 'anthropic', label: 'Anthropic' },
      { value: 'openai', label: 'OpenAI' },
      { value: 'gemini', label: 'Gemini' },
      { value: 'antigravity', label: 'Antigravity' },
      { value: 'kiro', label: 'Kiro' },
      { value: 'grok', label: 'Grok' },
    ])
  })
})
