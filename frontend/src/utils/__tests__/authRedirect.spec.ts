import { describe, expect, it } from 'vitest'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

describe('sanitizeAuthRedirect', () => {
  it('preserves ordinary internal destinations', () => {
    expect(sanitizeAuthRedirect('/usage?range=day#errors')).toBe('/usage?range=day#errors')
  })

  it.each([
    'https://evil.example/path',
    '//evil.example/path',
    '/\\evil.example/path',
    '/%5cevil.example/path',
    '/usage%0aheader',
    '/auth/oauth/callback#access_token=attacker',
    '/%61uth/oauth/callback#access_token=attacker',
    '/AUTH/OIDC/CALLBACK',
    '/login',
    '/reset-password?token=secret',
  ])('rejects unsafe authentication redirect %s', (value) => {
    expect(sanitizeAuthRedirect(value)).toBe('/dashboard')
  })

  it('supports a caller-specific fallback', () => {
    expect(sanitizeAuthRedirect('/auth/wechat/callback', '/profile')).toBe('/profile')
  })
})
