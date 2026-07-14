import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => {
      const messages: Record<string, string> = {
        'admin.accounts.oauth.grok.failedToExchangeCode': 'Grok 授权码兑换失败',
        'admin.accounts.oauth.grok.errors.GROK_OAUTH_INVALID_STATE':
          'Grok OAuth state 与当前会话不匹配。请粘贴同一次生成的授权链接返回的回调 URL。'
      }
      return messages[key] ?? key
    }
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    grok: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      refreshGrokToken: vi.fn(),
      validateSSOToken: vi.fn(),
      authorizePassword: vi.fn()
    }
  }
}))

import { useGrokOAuth } from '@/composables/useGrokOAuth'
import { adminAPI } from '@/api/admin'

describe('useGrokOAuth.exchangeAuthCode', () => {
  it('shows a state mismatch recovery hint from structured backend errors', async () => {
    vi.mocked(adminAPI.grok.exchangeCode).mockRejectedValueOnce({
      status: 400,
      reason: 'GROK_OAUTH_INVALID_STATE',
      message: 'invalid oauth state'
    })
    const oauth = useGrokOAuth()

    const tokenInfo = await oauth.exchangeAuthCode({
      code: 'code',
      sessionId: 'session-id',
      state: 'wrong-state'
    })

    expect(tokenInfo).toBeNull()
    expect(oauth.error.value).toBe(
      'Grok OAuth state 与当前会话不匹配。请粘贴同一次生成的授权链接返回的回调 URL。'
    )
  })

  it('validates SSO token through admin API', async () => {
    vi.mocked(adminAPI.grok.validateSSOToken).mockResolvedValueOnce({
      access_token: 'access',
      sso_token: 'sso-token'
    } as any)
    const oauth = useGrokOAuth()

    const tokenInfo = await oauth.validateSSOToken('  sso-token  ', 9)

    expect(adminAPI.grok.validateSSOToken).toHaveBeenCalledWith('sso-token', 9)
    expect(tokenInfo).toEqual({
      access_token: 'access',
      sso_token: 'sso-token'
    })
  })

  it('authorizes email/password through admin API', async () => {
    vi.mocked(adminAPI.grok.authorizePassword).mockResolvedValueOnce({
      access_token: 'access',
      email: 'user@example.com'
    } as any)
    const oauth = useGrokOAuth()

    const tokenInfo = await oauth.authorizePassword('  user@example.com---- secret  ', 7)

    expect(adminAPI.grok.authorizePassword).toHaveBeenCalledWith('  user@example.com---- secret  ', 7)
    expect(tokenInfo).toEqual({
      access_token: 'access',
      email: 'user@example.com'
    })
  })

  it('drops the raw SSO token returned by legacy conversion responses', () => {
    const oauth = useGrokOAuth()

    expect(oauth.buildCredentials({
      access_token: 'access',
      refresh_token: 'refresh',
      sso_token: 'sso-token'
    })).toEqual({
      access_token: 'access',
      refresh_token: 'refresh'
    })
  })
})

describe('useGrokOAuth.buildCredentials', () => {
  it('leaves the OAuth upstream unpinned so the system mode controls inference', () => {
    const oauth = useGrokOAuth()

    const credentials = oauth.buildCredentials({
      access_token: 'access-token',
      token_type: 'Bearer',
      expires_at: 1_900_000_000,
      client_id: 'client-id',
      scope: 'openid grok-cli:access',
      email: 'grok@example.com'
    })

    expect(credentials.base_url).toBeUndefined()
  })
})
