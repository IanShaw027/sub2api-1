import { beforeEach, describe, expect, it, vi } from 'vitest'

const { adapter } = vi.hoisted(() => ({
  adapter: vi.fn()
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'en-US'
}))

describe('admin Grok API', () => {
  beforeEach(async () => {
    vi.resetModules()
    adapter.mockReset()
    adapter.mockResolvedValue({
      status: 200,
      data: { code: 0, data: {} },
      headers: {},
      config: {},
      statusText: 'OK'
    })
    const { apiClient } = await import('@/api/client')
    apiClient.defaults.adapter = adapter
  })

  it('trims only the email and preserves password boundary spaces', async () => {
    const { authorizePassword } = await import('../grok')

    await authorizePassword('  owner@example.com  ---- secret----part  ', 7)

    const config = adapter.mock.calls[0][0]
    expect(config.url).toBe('/admin/grok/oauth/password')
    expect(JSON.parse(config.data)).toEqual({
      email: 'owner@example.com',
      password: ' secret----part  ',
      proxy_id: 7
    })
  })

  it('posts SSO token to the sso-token authorize endpoint', async () => {
    const { validateSSOToken } = await import('../grok')

    await validateSSOToken('sso-cookie-value', 3)

    const config = adapter.mock.calls[0][0]
    expect(config.url).toBe('/admin/grok/oauth/sso-token')
    expect(JSON.parse(config.data)).toEqual({
      sso_token: 'sso-cookie-value',
      proxy_id: 3
    })
  })
})
