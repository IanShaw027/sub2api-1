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
      project_id: 'project-1'
    })

    expect(credentials).toEqual({
      access_token: 'access-new',
      expires_at: '1714032000',
      project_id: 'project-1'
    })
    expect(credentials).not.toHaveProperty('refresh_token')
  })
})
