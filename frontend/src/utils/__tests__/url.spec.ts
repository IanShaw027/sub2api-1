import { describe, expect, it } from 'vitest'

import { buildAppAbsoluteUrl, buildAppPath } from '@/utils/url'

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
})
