import { describe, expect, it } from 'vitest'
import { sanitizeHtml, sanitizeRedirectPath } from '@/utils/sanitize'

describe('sanitize utils', () => {
  it('removes scriptable content from custom HTML', () => {
    const sanitized = sanitizeHtml('<h1>Hello</h1><img src=x onerror="alert(1)"><script>alert(2)</script>')

    expect(sanitized).toContain('<h1>Hello</h1>')
    expect(sanitized).not.toContain('onerror')
    expect(sanitized).not.toContain('<script')
  })

  it('allows only same-site redirect paths', () => {
    expect(sanitizeRedirectPath('/dashboard?tab=usage')).toBe('/dashboard?tab=usage')
    expect(sanitizeRedirectPath('https://evil.test')).toBe('/dashboard')
    expect(sanitizeRedirectPath('//evil.test/path')).toBe('/dashboard')
    expect(sanitizeRedirectPath(' /admin/dashboard ')).toBe('/admin/dashboard')
    expect(sanitizeRedirectPath('/safe\nLocation: //evil.test')).toBe('/dashboard')
  })

  it('coerces unknown query-like values without throwing', () => {
    expect(sanitizeRedirectPath(['/admin/dashboard', '/evil'])).toBe('/admin/dashboard')
    expect(sanitizeRedirectPath([null, '/keys'] as unknown[])).toBe('/keys')
    expect(sanitizeRedirectPath([])).toBe('/dashboard')
    expect(sanitizeRedirectPath(undefined)).toBe('/dashboard')
    expect(sanitizeRedirectPath(null)).toBe('/dashboard')
    expect(sanitizeRedirectPath(42)).toBe('/dashboard')
    expect(sanitizeRedirectPath({ path: '/admin' })).toBe('/dashboard')
    expect(sanitizeRedirectPath(['//evil.test'], '/home')).toBe('/home')
  })
})
