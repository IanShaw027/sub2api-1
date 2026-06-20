import { describe, expect, it } from 'vitest'

import { supportsTextEndpointAutoRoute } from '@/components/account/textEndpointAutoRoute'

describe('supportsTextEndpointAutoRoute', () => {
  it('supports only OpenAI and Anthropic API key accounts', () => {
    expect(supportsTextEndpointAutoRoute('openai', 'apikey')).toBe(true)
    expect(supportsTextEndpointAutoRoute('anthropic', 'apikey')).toBe(true)

    expect(supportsTextEndpointAutoRoute('openai', 'oauth')).toBe(false)
    expect(supportsTextEndpointAutoRoute('anthropic', 'oauth')).toBe(false)
    expect(supportsTextEndpointAutoRoute('anthropic', 'upstream')).toBe(false)
    expect(supportsTextEndpointAutoRoute('anthropic', 'bedrock')).toBe(false)
    expect(supportsTextEndpointAutoRoute('gemini', 'apikey')).toBe(false)
    expect(supportsTextEndpointAutoRoute('kiro', 'apikey')).toBe(false)
  })

  it('requires Chat Completions capability for OpenAI API key auto-route', () => {
    expect(supportsTextEndpointAutoRoute('openai', 'apikey', {
      openAIEndpointCapabilities: ['chat_completions']
    })).toBe(true)
    expect(supportsTextEndpointAutoRoute('openai', 'apikey', {
      openAIEndpointCapabilities: ['embeddings']
    })).toBe(false)
  })
})
