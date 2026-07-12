import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    kiro: {
      generateAuthUrl: vi.fn(),
      exchangeCallback: vi.fn()
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import { adminAPI } from '@/api/admin'
import { stripKiroRuntimeExtra, useKiroOAuth } from '../useKiroOAuth'

describe('useKiroOAuth', () => {
  it('strips account-level Kiro runtime extra fields', () => {
    expect(stripKiroRuntimeExtra({
      keep_flag: true,
      kiro_version: '0.9.0',
      kiro_commit: 'abc123',
      system_version: 'darwin#24.5.0',
      node_version: '22.20.0'
    })).toEqual({
      keep_flag: true
    })
  })

  it('does not build Kiro runtime overrides for OAuth extra info', () => {
    const kiroOAuth = useKiroOAuth()

    expect(kiroOAuth.buildExtraInfo(
      {
        email: 'kiro@example.com',
        profile_id: 'EHGA3GRVQMUK',
        name: 'Kiro User',
        login_provider: 'builder-id',
        plan_name: 'Kiro Pro',
        subscription_type: 'pro',
        plan_tier: 'paid',
        usage_reset_at: '2026-05-01T00:00:00Z',
        status: 'active',
        status_reason: 'ok'
      } as any,
      {
        keep_flag: true,
        kiro_version: '0.9.0',
        kiro_commit: 'abc123',
        system_version: 'darwin#24.5.0',
        node_version: '22.20.0'
      }
    )).toEqual({
      keep_flag: true,
      email: 'kiro@example.com',
      profile_id: 'EHGA3GRVQMUK',
      name: 'Kiro User',
      login_provider: 'builder-id',
      subscription_type: 'pro',
      subscription_tier: 'paid',
      usage_reset_at: '2026-05-01T00:00:00Z',
      kiro_status: 'active',
      kiro_status_reason: 'ok'
    })
  })

  it('builds account names from explicit or identity fields and avoids generic defaults', () => {
    const kiroOAuth = useKiroOAuth()

    expect(kiroOAuth.buildAccountName({ email: 'user@example.com' } as any, '  Manual Name  ')).toBe('Manual Name')
    expect(kiroOAuth.buildAccountName({ email: 'user@example.com', profile_id: 'EHGA3GRVQMUK' } as any)).toBe('user@example.com')
    expect(kiroOAuth.buildAccountName({ name: 'Kiro User' } as any)).toBe('Kiro User')
    expect(kiroOAuth.buildAccountName({ name: 'Kiro User', profile_id: 'EHGA3GRVQMUK' } as any)).toBe('Kiro User')
    expect(kiroOAuth.buildAccountName({ email: 'user@example.com' } as any)).toBe('user@example.com')
    expect(kiroOAuth.buildAccountName({ profile_id: 'EHGA3GRVQMUK' } as any)).toBe('EHGA3GRVQMUK')
    expect(kiroOAuth.buildAccountName({ user_id: 'arn:aws:codewhisperer:us-east-1:699475941385:profile/EHGA3GRVQMUK' } as any)).toBe('EHGA3GRVQMUK')
    expect(kiroOAuth.buildAccountName({ plan_name: 'Pro' } as any)).toBe('Kiro Pro')
    expect(kiroOAuth.buildAccountName({} as any)).toBe('Kiro OAuth Account')
  })

  it('stores External IdP authorization progress without setting an error', async () => {
    const openMock = vi.spyOn(window, 'open').mockReturnValue(null)
    vi.mocked(adminAPI.kiro.exchangeCallback).mockResolvedValueOnce({
      external_idp: {
        session_id: 'session-1',
        status: 'authorization_required',
        auth_method: 'external_idp',
        auth_url: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize?client_id=client-1',
        client_id: 'client-1',
        redirect_uri: 'http://localhost:3128/signin/callback'
      }
    } as any)

    const kiroOAuth = useKiroOAuth()
    kiroOAuth.sessionId.value = 'session-1'

    const result = await kiroOAuth.exchangeCallback('http://localhost:3128/signin/callback')

    expect(result).toBeNull()
    expect(kiroOAuth.error.value).toBe('')
    expect(kiroOAuth.externalIDPAuthorization.value?.auth_url).toContain('login.microsoftonline.com')
    expect(openMock).toHaveBeenCalledWith(
      'https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize?client_id=client-1',
      '_blank',
      'noopener'
    )

    openMock.mockRestore()
  })
})
