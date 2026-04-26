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
    antigravity: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      refreshAntigravityToken: vi.fn()
    }
  }
}))

import { useAntigravityOAuth } from '@/composables/useAntigravityOAuth'

describe('useAntigravityOAuth.buildCredentials', () => {
  it('omits empty refresh tokens so reauth keeps the existing token', () => {
    const oauth = useAntigravityOAuth()

    const credentials = oauth.buildCredentials({
      access_token: 'access-new',
      refresh_token: null,
      token_type: '',
      expires_at: '2026-04-25T00:00:00Z',
      project_id: 'project-1',
      email: 'user@example.com'
    })

    expect(credentials).toEqual({
      access_token: 'access-new',
      expires_at: '2026-04-25T00:00:00Z',
      project_id: 'project-1',
      email: 'user@example.com'
    })
    expect(credentials).not.toHaveProperty('refresh_token')
  })
})

describe('useAntigravityOAuth.buildAccountName', () => {
  it('uses manual name first and otherwise formats email with project or plan context', () => {
    const oauth = useAntigravityOAuth()

    expect(oauth.buildAccountName({ email: 'user@example.com', project_id: 'project-1' }, ' Manual ')).toBe('Manual')
    expect(oauth.buildAccountName({ email: 'user@example.com', project_id: 'project-1' })).toBe('user@example.com (project-1)')
    expect(oauth.buildAccountName({ email: 'user@example.com', plan_type: 'Pro' })).toBe('user@example.com (Pro)')
    expect(oauth.buildAccountName({ plan_type: 'Pro' })).toBe('Antigravity Pro')
    expect(oauth.buildAccountName({})).toBe('Antigravity OAuth Account')
  })
})
