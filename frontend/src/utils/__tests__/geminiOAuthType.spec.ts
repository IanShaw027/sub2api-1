import { describe, expect, it } from 'vitest'

import { inferGeminiOAuthType } from '../geminiOAuthType'

describe('inferGeminiOAuthType', () => {
  it('ignores extra.subscription_type when inferring Gemini OAuth type', () => {
    expect(
      inferGeminiOAuthType(
        {},
        {
          subscription_type: 'Gemini Code Assist in Google One AI Pro'
        },
        ''
      )
    ).toBe('')
  })

  it('still honors explicit extra.oauth_type when present', () => {
    expect(
      inferGeminiOAuthType(
        {},
        {
          oauth_type: 'google_one',
          subscription_type: 'Gemini Code Assist in Google One AI Pro'
        },
        ''
      )
    ).toBe('google_one')
  })
})
