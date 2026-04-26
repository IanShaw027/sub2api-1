import { describe, expect, it, vi } from 'vitest'

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      generateAuthUrl: vi.fn(),
      exchangeCode: vi.fn(),
      cookieAuth: vi.fn()
    }
  }
}))

import { useAccountOAuth } from '../useAccountOAuth'

describe('useAccountOAuth.buildAccountName', () => {
  it('uses manual name first and otherwise formats email with account or org context', () => {
    const oauth = useAccountOAuth()

    expect(oauth.buildAccountName({ email_address: 'user@example.com', account_uuid: 'acc-1' }, ' Manual ')).toBe('Manual')
    expect(oauth.buildAccountName({ email_address: 'user@example.com', account_uuid: 'acc-1' })).toBe('user@example.com (acc-1)')
    expect(oauth.buildAccountName({ email_address: 'user@example.com', org_uuid: 'org-1' })).toBe('user@example.com (org-1)')
    expect(oauth.buildAccountName({})).toBe('Claude OAuth Account')
  })
})

