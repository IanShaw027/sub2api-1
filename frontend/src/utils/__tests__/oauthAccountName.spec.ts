import { describe, expect, it } from 'vitest'
import { formatOAuthAccountName } from '@/utils/oauthAccountName'

describe('formatOAuthAccountName', () => {
  it('prefers a trimmed manual name', () => {
    expect(
      formatOAuthAccountName({
        manualName: '  Manual  ',
        primary: 'user@example.com',
        platformLabel: 'OpenAI',
        defaultName: 'OpenAI OAuth Account'
      })
    ).toBe('Manual')
  })

  it('joins primary and the first distinct detail', () => {
    expect(
      formatOAuthAccountName({
        primary: 'user@example.com',
        details: ['Team A', 'ignored'],
        platformLabel: 'OpenAI',
        defaultName: 'OpenAI OAuth Account'
      })
    ).toBe('user@example.com (Team A)')
  })

  it('prefixes fallback detail with the platform label', () => {
    expect(
      formatOAuthAccountName({
        fallbackDetail: 'Pro',
        platformLabel: 'OpenAI',
        defaultName: 'OpenAI OAuth Account'
      })
    ).toBe('OpenAI Pro')
  })

  it('returns the default name when nothing usable is present', () => {
    expect(
      formatOAuthAccountName({
        platformLabel: 'Claude',
        defaultName: 'Claude OAuth Account'
      })
    ).toBe('Claude OAuth Account')
  })
})
