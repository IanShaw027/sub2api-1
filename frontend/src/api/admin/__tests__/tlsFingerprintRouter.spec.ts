import { beforeEach, describe, expect, it, vi } from 'vitest'

const { adapter } = vi.hoisted(() => ({
  adapter: vi.fn()
}))

vi.mock('@/i18n', () => ({
  getLocale: () => 'en-US'
}))

describe('admin TLS fingerprint router api', () => {
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

  it('uses the admin tls fingerprint router endpoints for CRUD and toggle', async () => {
    const { tlsFingerprintRouterAPI } = await import('../tlsFingerprintRouter')
    const payload = {
      name: 'OpenAI UA router',
      description: null,
      enabled: true,
      rules: [
        {
          name: 'Chrome',
          enabled: true,
          match_type: 'contains' as const,
          pattern: 'Chrome',
          case_sensitive: false,
          tls_fingerprint_profile_id: 12,
          upstream_user_agent: '',
          upstream_originator: ''
        }
      ]
    }

    await tlsFingerprintRouterAPI.list()
    await tlsFingerprintRouterAPI.create(payload)
    await tlsFingerprintRouterAPI.update(7, payload)
    await tlsFingerprintRouterAPI.toggle(7, false)
    await tlsFingerprintRouterAPI.delete(7)

    expect(adapter.mock.calls.map(([config]) => [config.method, config.url, config.data && JSON.parse(config.data)])).toEqual([
      ['get', '/admin/tls-fingerprint-routers', undefined],
      ['post', '/admin/tls-fingerprint-routers', payload],
      ['put', '/admin/tls-fingerprint-routers/7', payload],
      ['put', '/admin/tls-fingerprint-routers/7', { enabled: false }],
      ['delete', '/admin/tls-fingerprint-routers/7', undefined]
    ])
  })
})
