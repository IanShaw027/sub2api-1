import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import AccountActionMenu from '@/components/admin/account/AccountActionMenu.vue'
import type { Account } from '@/types'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/components/icons', () => ({
  Icon: { template: '<span />' }
}))

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'openai',
    type: 'oauth',
    proxy_id: null,
    concurrency: 1,
    priority: 1,
    status: 'active',
    error_message: null,
    last_used_at: null,
    expires_at: null,
    auto_pause_on_expired: true,
    created_at: '2026-03-15T00:00:00Z',
    updated_at: '2026-03-15T00:00:00Z',
    schedulable: true,
    rate_limited_at: null,
    rate_limit_reset_at: null,
    overload_until: null,
    temp_unschedulable_until: null,
    temp_unschedulable_reason: null,
    session_window_start: null,
    session_window_end: null,
    session_window_status: null,
    ...overrides,
  }
}

describe('AccountActionMenu', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('temp unschedulable recover action expires with the clock', async () => {
    const wrapper = mount(AccountActionMenu, {
      props: {
        show: true,
        position: { top: 0, left: 0 },
        account: makeAccount({ temp_unschedulable_until: '2026-03-17T00:00:30Z' })
      },
      attachTo: document.body,
      global: {
        stubs: {
          Teleport: true
        }
      }
    })

    expect(document.body.textContent).toContain('admin.accounts.recoverState')

    await vi.advanceTimersByTimeAsync(60_000)
    await Promise.resolve()

    expect(document.body.textContent).not.toContain('admin.accounts.recoverState')

    wrapper.unmount()
  })
})
