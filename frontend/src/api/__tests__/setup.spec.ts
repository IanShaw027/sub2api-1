import { beforeEach, describe, expect, it, vi } from 'vitest'

const requestUse = vi.fn()
const post = vi.fn()

vi.mock('axios', () => ({
  default: {
    create: vi.fn(() => ({
      interceptors: { request: { use: requestUse } },
      get: vi.fn(),
      post
    }))
  }
}))

describe('setup bootstrap secret', () => {
  beforeEach(() => {
    sessionStorage.clear()
    history.replaceState(null, '', '/setup')
  })

  it('captures a fragment secret for this browser session and removes it from the URL', async () => {
    const { configureSetupBootstrapSecretFromLocation } = await import('../setup')
    history.replaceState(null, '', '/setup#setup-secret=0123456789abcdef0123456789abcdef')

    configureSetupBootstrapSecretFromLocation()

    expect(sessionStorage.getItem('setup_bootstrap_secret')).toBe(
      '0123456789abcdef0123456789abcdef'
    )
    expect(location.hash).toBe('')
    expect(location.pathname).toBe('/setup')
  })

  it('adds the captured secret only to setup client requests', async () => {
    await import('../setup')
    sessionStorage.setItem('setup_bootstrap_secret', '0123456789abcdef0123456789abcdef')
    const interceptor = requestUse.mock.calls[0][0]
    const set = vi.fn()

    interceptor({ headers: { set } })

    expect(set).toHaveBeenCalledWith(
      'X-Setup-Bootstrap-Secret',
      '0123456789abcdef0123456789abcdef'
    )
  })

  it('fails closed on a malformed fragment', async () => {
    const { configureSetupBootstrapSecretFromLocation } = await import('../setup')
    sessionStorage.setItem('setup_bootstrap_secret', 'stale-secret')
    history.replaceState(null, '', '/setup#setup-secret=%zz')

    expect(() => configureSetupBootstrapSecretFromLocation()).not.toThrow()
    expect(sessionStorage.getItem('setup_bootstrap_secret')).toBeNull()
    expect(location.hash).toBe('')
  })

  it('clears the secret only after installation succeeds', async () => {
    const { install } = await import('../setup')
    sessionStorage.setItem('setup_bootstrap_secret', '0123456789abcdef0123456789abcdef')
    post.mockResolvedValueOnce({ data: { data: { message: 'ok', restart: true } } })

    await install({} as never)

    expect(sessionStorage.getItem('setup_bootstrap_secret')).toBeNull()
  })
})
