import { beforeEach, describe, expect, it, vi } from 'vitest'

const { refreshSessionMock } = vi.hoisted(() => ({
  refreshSessionMock: vi.fn(),
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post: vi.fn(),
    get: vi.fn(),
  },
  refreshSession: (...args: unknown[]) => refreshSessionMock(...args),
}))

import { persistOAuthTokenContext } from '@/api/auth'
import {
  clearAccessToken,
  getAccessToken,
  getAccessTokenExpiresAt,
} from '@/utils/authSession'

describe('auth cookie session migration', () => {
  beforeEach(() => {
    localStorage.clear()
    clearAccessToken()
    refreshSessionMock.mockReset()
  })

  it('migrates a legacy OAuth fragment refresh token through the compatibility body', async () => {
    refreshSessionMock.mockResolvedValue({
      access_token: 'rotated-access',
      expires_in: 1800,
      token_type: 'Bearer',
    })

    const effective = await persistOAuthTokenContext({
      access_token: 'legacy-access',
      refresh_token: 'legacy-refresh',
      expires_in: 3600,
    })

    expect(refreshSessionMock).toHaveBeenCalledWith('legacy-refresh')
    expect(effective.access_token).toBe('rotated-access')
    expect(getAccessToken()).toBe('rotated-access')
    expect(getAccessTokenExpiresAt()).toBeGreaterThan(Date.now())
    expect(localStorage.getItem('refresh_token')).toBeNull()
    expect(localStorage.getItem('auth_token')).toBeNull()
  })
})
