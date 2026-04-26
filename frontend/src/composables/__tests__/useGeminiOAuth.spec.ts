import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    gemini: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn()
    }
  }
}))

import { useGeminiOAuth } from '@/composables/useGeminiOAuth'

describe('useGeminiOAuth.buildCredentials', () => {
  it('omits empty refresh tokens so reauth keeps the existing token', () => {
    const oauth = useGeminiOAuth()

    const credentials = oauth.buildCredentials({
      access_token: 'access-new',
      refresh_token: '',
      token_type: undefined,
      expires_at: 1714032000,
      scope: null,
      project_id: 'project-1',
      email: 'user@example.com'
    })

    expect(credentials).toEqual({
      access_token: 'access-new',
      expires_at: '1714032000',
      project_id: 'project-1',
      email: 'user@example.com'
    })
    expect(credentials).not.toHaveProperty('refresh_token')
  })
})

describe('useGeminiOAuth.buildAccountName', () => {
  it('uses manual name first and otherwise formats project with tier or OAuth type context', () => {
    const oauth = useGeminiOAuth()

    expect(oauth.buildAccountName({ project_id: 'project-1', tier_id: 'Pro' }, ' Manual ')).toBe('Manual')
    expect(oauth.buildAccountName({ email: 'user@example.com', project_id: 'project-1' })).toBe(
      'user@example.com (project-1)'
    )
    expect(oauth.buildAccountName({ email: 'user@example.com', tier_id: 'Pro' })).toBe(
      'user@example.com (Pro)'
    )
    expect(oauth.buildAccountName({ project_id: 'project-1', tier_id: 'Pro' })).toBe('project-1 (Pro)')
    expect(oauth.buildAccountName({ project_id: 'project-1', oauth_type: 'code_assist' })).toBe('project-1 (code_assist)')
    expect(oauth.buildAccountName({ tier_id: 'Google One Ultra' })).toBe('Gemini Google One Ultra')
    expect(oauth.buildAccountName({})).toBe('Gemini OAuth Account')
  })
})
