import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import AccountStatusIndicator from '../AccountStatusIndicator.vue'
import type { Account } from '@/types'
import { accountStatusI18nKey, paymentOrderTypeI18nKey } from '@/utils/i18n'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/i18n', () => ({
  getLocale: () => 'en',
  i18n: {
    global: {
      t: (key: string, params?: Record<string, unknown>) => {
        if (key === 'common.time.countdown.daysHours') return `${params?.d}d ${params?.h}h`
        if (key === 'common.time.countdown.hoursMinutes') return `${params?.h}h ${params?.m}m`
        if (key === 'common.time.countdown.minutes') return `${params?.m}m`
        if (key === 'common.time.countdown.withSuffix') return `${params?.time} to lift`
        return key
      }
    }
  }
}))

function makeAccount(overrides: Partial<Account>): Account {
  return {
    id: 1,
    name: 'account',
    platform: 'antigravity',
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

describe('AccountStatusIndicator', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('normalizes historical account status aliases to canonical i18n keys', () => {
    expect(accountStatusI18nKey('temp_unschedulable')).toBe('admin.accounts.status.tempUnschedulable')
  })

  it('normalizes payment order type aliases to canonical admin i18n keys', () => {
    expect(paymentOrderTypeI18nKey('BALANCE')).toBe('payment.admin.balanceOrder')
  })

  it('模型限流 + overages 启用 + 无 AICredits key → 显示 ⚡ (credits_active)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 1,
          name: 'ag-1',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('⚡')
    expect(wrapper.text()).toContain('CSon45')
  })

  it('模型限流 + overages 未启用 → 普通限流样式（无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 2,
          name: 'ag-2',
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
  })

  it('AICredits key 生效 → 显示积分已用尽 (credits_exhausted)', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 3,
          name: 'ag-3',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })

  it('temp unschedulable badge expires after the clock advances past until', async () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          temp_unschedulable_until: '2026-03-17T00:00:30Z'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.tempUnschedulable')
    expect(wrapper.find('button').exists()).toBe(true)

    await vi.advanceTimersByTimeAsync(60_000)
    await nextTick()

    expect(wrapper.text()).not.toContain('admin.accounts.status.tempUnschedulable')
    expect(wrapper.text()).toContain('admin.accounts.status.active')
    expect(wrapper.find('button').exists()).toBe(false)
  })

  it('rate limit badge expires after the clock advances past reset time', async () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          rate_limit_reset_at: '2026-03-17T00:00:30Z'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.rateLimited')
    expect(wrapper.text()).toContain('429')

    await vi.advanceTimersByTimeAsync(60_000)
    await nextTick()

    expect(wrapper.text()).not.toContain('admin.accounts.status.rateLimited')
    expect(wrapper.text()).not.toContain('429')
  })

  it('overload badge expires after the clock advances past overload time', async () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          overload_until: '2026-03-17T00:00:30Z'
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.status.overloaded')
    expect(wrapper.text()).toContain('529')

    await vi.advanceTimersByTimeAsync(60_000)
    await nextTick()

    expect(wrapper.text()).not.toContain('admin.accounts.status.overloaded')
    expect(wrapper.text()).not.toContain('529')
  })

  it('model rate limit badge expires after the clock advances past reset time', async () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          extra: {
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-17T00:00:00Z',
                rate_limit_reset_at: '2026-03-17T00:00:30Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).toContain('30s')

    await vi.advanceTimersByTimeAsync(60_000)
    await nextTick()

    expect(wrapper.text()).not.toContain('CSon45')
  })

  it('模型限流 + overages 启用 + AICredits key 生效 → 普通限流样式（积分耗尽，无 ⚡）', () => {
    const wrapper = mount(AccountStatusIndicator, {
      props: {
        account: makeAccount({
          id: 4,
          name: 'ag-4',
          extra: {
            allow_overages: true,
            model_rate_limits: {
              'claude-sonnet-4-5': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              },
              'AICredits': {
                rate_limited_at: '2026-03-15T00:00:00Z',
                rate_limit_reset_at: '2099-03-15T00:00:00Z'
              }
            }
          }
        })
      },
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    // 模型限流 + 积分耗尽 → 不应显示 ⚡
    expect(wrapper.text()).toContain('CSon45')
    expect(wrapper.text()).not.toContain('⚡')
    // AICredits 积分耗尽状态应显示
    expect(wrapper.text()).toContain('admin.accounts.status.creditsExhausted')
  })
})
