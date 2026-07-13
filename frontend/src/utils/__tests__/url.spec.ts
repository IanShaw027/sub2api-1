import { describe, expect, it } from 'vitest'

import { buildAppAbsoluteUrl, buildAppPath, sanitizeUrl } from '@/utils/url'

describe('url utils', () => {
  it('builds app path under root base url', () => {
    expect(buildAppPath('/register?aff=abc', '/')).toBe('/register?aff=abc')
    expect(buildAppPath('admin/usage', '/')).toBe('/admin/usage')
  })

  it('builds app path under nested base url', () => {
    expect(buildAppPath('/register?aff=abc', '/console/')).toBe('/console/register?aff=abc')
    expect(buildAppPath('admin/usage', '/console/')).toBe('/console/admin/usage')
  })

  it('builds absolute app url under nested base url', () => {
    expect(buildAppAbsoluteUrl('/register?aff=abc', 'https://app.example.com', '/console/')).toBe(
      'https://app.example.com/console/register?aff=abc'
    )
    expect(buildAppAbsoluteUrl('admin/usage', 'https://app.example.com', '/console/')).toBe(
      'https://app.example.com/console/admin/usage'
    )
  })

  it('allows only padded raster image data URLs when requested', () => {
    const valid = 'data:image/png;base64,QUJDRA=='
    expect(sanitizeUrl(valid, { allowDataUrl: true })).toBe(valid)
    expect(sanitizeUrl('data:image/svg+xml;base64,PHN2Zz48L3N2Zz4=', { allowDataUrl: true })).toBe('')
    expect(sanitizeUrl('data:image/png;base64,QUJD=', { allowDataUrl: true })).toBe('')
    expect(sanitizeUrl('data:image/png;base64,QUJD===', { allowDataUrl: true })).toBe('')
    expect(sanitizeUrl('data:image/png;base64,not valid base64', { allowDataUrl: true })).toBe('')
  })
})
