import { beforeEach, describe, expect, it, vi } from 'vitest'

const refreshAuthTokens = vi.hoisted(() => vi.fn())

vi.mock('@/api/tokenRefresh', () => ({
  refreshAuthTokens,
}))

import { authenticatedFetch } from '@/api/authenticatedFetch'

describe('authenticatedFetch', () => {
  beforeEach(() => {
    refreshAuthTokens.mockReset()
    localStorage.clear()
    localStorage.setItem('refresh_token', 'refresh-1')
    localStorage.setItem('auth_user', JSON.stringify({ id: 1 }))
    localStorage.setItem('auth_token', 'old-access')
  })

  it('returns the response when status is not 401', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response('ok', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const response = await authenticatedFetch('/api/test', {
      headers: { Authorization: 'Bearer old-access' },
    })

    expect(response.status).toBe(200)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(refreshAuthTokens).not.toHaveBeenCalled()
  })

  it('refreshes the token and retries once on 401', async () => {
    const fetchMock = vi
      .fn()
      .mockResolvedValueOnce(new Response('expired', { status: 401 }))
      .mockResolvedValueOnce(new Response('ok', { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    refreshAuthTokens.mockResolvedValue({
      access_token: 'new-access',
      refresh_token: 'refresh-2',
      expires_in: 3600,
      token_type: 'Bearer',
    })

    const response = await authenticatedFetch('/api/test', {
      headers: { Authorization: 'Bearer old-access' },
    })

    expect(response.status).toBe(200)
    expect(refreshAuthTokens).toHaveBeenCalledWith({ failedAccessToken: 'old-access' })
    expect(fetchMock).toHaveBeenCalledTimes(2)
    expect(fetchMock.mock.calls[1]?.[1]?.headers?.get('Authorization')).toBe('Bearer new-access')
  })

  it('does not retry when refresh token is missing', async () => {
    localStorage.removeItem('refresh_token')
    const fetchMock = vi.fn().mockResolvedValue(new Response('expired', { status: 401 }))
    vi.stubGlobal('fetch', fetchMock)

    const response = await authenticatedFetch('/api/test')

    expect(response.status).toBe(401)
    expect(fetchMock).toHaveBeenCalledTimes(1)
    expect(refreshAuthTokens).not.toHaveBeenCalled()
  })
})
