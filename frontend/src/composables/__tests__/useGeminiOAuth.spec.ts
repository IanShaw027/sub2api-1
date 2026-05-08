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
import { adminAPI } from '@/api/admin'

describe('useGeminiOAuth.buildCredentials', () => {
  it('omits empty refresh tokens so reauth keeps the existing token and keeps aligned Gemini fields', () => {
    const oauth = useGeminiOAuth()

    const credentials = oauth.buildCredentials({
      access_token: 'access-new',
      refresh_token: '',
      id_token: 'id-token',
      token_type: undefined,
      expires_at: 1714032000,
      scope: null,
      project_id: 'project-1',
      email: 'user@example.com',
      auth_id: 'subject-123',
      name: 'Example User',
      plan_name: 'Gemini Code Assist in Google One AI Pro'
    })

    expect(credentials).toEqual({
      access_token: 'access-new',
      id_token: 'id-token',
      expires_at: '1714032000',
      project_id: 'project-1',
      email: 'user@example.com',
      auth_id: 'subject-123',
      subject: 'subject-123',
      name: 'Example User',
      plan_name: 'Gemini Code Assist in Google One AI Pro'
    })
    expect(credentials).not.toHaveProperty('refresh_token')
  })
})

describe('useGeminiOAuth.buildExtraInfo', () => {
  it('stores display metadata and prefers plan_name as subscription label', () => {
    const oauth = useGeminiOAuth()

    expect(oauth.buildExtraInfo({
      email: 'user@example.com',
      auth_id: 'subject-123',
      name: 'Example User',
      plan_name: 'Gemini Code Assist in Google One AI Pro',
      tier_id: 'google_ai_pro',
      oauth_type: 'google_one'
    })).toEqual({
      email: 'user@example.com',
      auth_id: 'subject-123',
      name: 'Example User',
      plan_name: 'Gemini Code Assist in Google One AI Pro',
      subscription_type: 'Gemini Code Assist in Google One AI Pro',
      oauth_type: 'google_one'
    })
  })
})

describe('useGeminiOAuth.buildAccountName', () => {
  it('uses manual name first and otherwise formats Gemini names around email(project_id)', () => {
    const oauth = useGeminiOAuth()

    expect(oauth.buildAccountName({ project_id: 'project-1', tier_id: 'Pro' }, ' Manual ')).toBe('Manual')
    expect(oauth.buildAccountName({ email: 'user@example.com', project_id: 'project-1' })).toBe(
      'user@example.com(project-1)'
    )
    expect(oauth.buildAccountName({ email: 'user@example.com', tier_id: 'Pro' })).toBe('user@example.com')
    expect(oauth.buildAccountName({ project_id: 'project-1', tier_id: 'Pro' })).toBe('project-1 (Pro)')
    expect(oauth.buildAccountName({ project_id: 'project-1', oauth_type: 'code_assist' })).toBe('project-1 (code_assist)')
    expect(oauth.buildAccountName({ name: 'Example User', plan_name: 'Google One Pro' })).toBe(
      'Example User (Google One Pro)'
    )
    expect(oauth.buildAccountName({ name: 'Example User', project_id: 'project-1' })).toBe(
      'Example User(project-1)'
    )
    expect(oauth.buildAccountName({ tier_id: 'Google One Ultra' })).toBe('Gemini Google One Ultra')
    expect(oauth.buildAccountName({})).toBe('Gemini OAuth Account')
  })
})

describe('useGeminiOAuth.exchangeAuthCode', () => {
  it('maps real project auto-detect failures to the recovery error key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce(new Error('failed to auto-detect project_id: empty result'))
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'code_assist'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.missingProjectId')
  })

  it('prefers backend response message over generic axios error text', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce({
      message: 'Request failed with status code 400',
      response: {
        data: {
          message: 'missing project_id for Code Assist OAuth'
        }
      }
    })
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'code_assist'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.missingProjectId')
  })

  it('maps google one companion-project detection failures to a dedicated error key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce(
      new Error('google One accounts require a project_id, failed to auto-detect: empty result')
    )
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'google_one'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.googleOneProjectDetectionFailed')
  })

  it('maps code assist age verification ineligible errors to a dedicated error key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce({
      response: {
        data: {
          detail:
            'Failed to exchange code: ineligible_tier[RESTRICTED_AGE]: Your current account is not eligible for Gemini Code Assist for individuals. To use Gemini Code Assist for individuals you must be 18 years old or older. If you think you are receiving this message in error, please ensure you have verified your age and try to log in again.'
        }
      }
    })
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'code_assist'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.codeAssistAgeVerificationRequired')
  })

  it('maps reason-code-only restricted age errors to the age eligibility key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce({
      response: {
        data: {
          detail: 'Failed to exchange code: ineligible_tier[RESTRICTED_AGE]: upstream eligibility rejected this account'
        }
      }
    })
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'google_one'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.codeAssistAgeVerificationRequired')
  })

  it('maps generic ineligible tier errors to a dedicated error key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce({
      response: {
        data: {
          detail: 'Failed to exchange code: ineligible_tier: account is not eligible for this tier'
        }
      }
    })
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'code_assist'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.ineligibleTier')
  })

  it('maps reason-coded non-age ineligible tier errors to the generic ineligible key', async () => {
    vi.mocked(adminAPI.gemini.exchangeCode).mockRejectedValueOnce({
      response: {
        data: {
          detail: 'Failed to exchange code: ineligible_tier[INELIGIBLE_ACCOUNT]: upstream eligibility rejected this account'
        }
      }
    })
    const oauth = useGeminiOAuth()

    const result = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session',
      state: 'state',
      oauthType: 'google_one'
    })

    expect(result).toBeNull()
    expect(oauth.error.value).toBe('admin.accounts.oauth.gemini.ineligibleTier')
  })
})

describe('useGeminiOAuth.generateAuthUrl', () => {
  it('sends project_id_hint for code assist bootstrap', async () => {
    vi.mocked(adminAPI.gemini.generateAuthUrl).mockResolvedValueOnce({
      auth_url: 'https://example.com/oauth',
      session_id: 'session-1',
      state: 'state-1'
    })
    const oauth = useGeminiOAuth()

    const ok = await oauth.generateAuthUrl(123, ' my-gcp-project ', 'code_assist')

    expect(ok).toBe(true)
    expect(adminAPI.gemini.generateAuthUrl).toHaveBeenCalledWith({
      proxy_id: 123,
      project_id_hint: 'my-gcp-project',
      oauth_type: 'code_assist'
    })
  })
})
