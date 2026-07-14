import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      cookieAuth: vi.fn()
    }
  }
}))

import { useAccountOAuth } from '../useAccountOAuth'
import { adminAPI } from '@/api/admin'

beforeEach(() => {
  vi.clearAllMocks()
})

describe('useAccountOAuth state binding', () => {
  it('stores the generated state and sends it during code exchange', async () => {
    vi.mocked(adminAPI.accounts.generateAuthUrl).mockResolvedValue({
      auth_url: 'https://claude.example/authorize',
      session_id: 'session-id',
      state: 'generated-state'
    })
    vi.mocked(adminAPI.accounts.exchangeCode).mockResolvedValue({ access_token: 'access-token' })
    const oauth = useAccountOAuth()

    await expect(oauth.generateAuthUrl('oauth')).resolves.toBe(true)
    expect(oauth.oauthState.value).toBe('generated-state')

    oauth.authCode.value = 'authorization-code'
    await expect(oauth.exchangeAuthCode('oauth')).resolves.toEqual({ access_token: 'access-token' })
    expect(adminAPI.accounts.exchangeCode).toHaveBeenCalledWith('/admin/accounts/exchange-code', {
      session_id: 'session-id',
      code: 'authorization-code',
      state: 'generated-state'
    })
  })

  it('clears state together with the rest of the authorization session', async () => {
    vi.mocked(adminAPI.accounts.generateAuthUrl).mockResolvedValue({
      auth_url: 'https://claude.example/authorize',
      session_id: 'session-id',
      state: 'generated-state'
    })
    const oauth = useAccountOAuth()
    await oauth.generateAuthUrl('oauth')

    oauth.resetState()

    expect(oauth.oauthState.value).toBe('')
    expect(oauth.sessionId.value).toBe('')
  })
})

describe('useAccountOAuth.buildAccountName', () => {
  it('uses manual name first and otherwise keeps Claude oauth names as plain email', () => {
    const oauth = useAccountOAuth()

    expect(oauth.buildAccountName({ email_address: 'user@example.com', account_uuid: 'acc-1' }, ' Manual ')).toBe('Manual')
    expect(oauth.buildAccountName({ email_address: 'user@example.com', account_uuid: 'acc-1' })).toBe('user@example.com')
    expect(oauth.buildAccountName({ email_address: 'user@example.com', org_uuid: 'org-1' })).toBe('user@example.com')
    expect(oauth.buildAccountName({})).toBe('Claude OAuth Account')
  })
})
