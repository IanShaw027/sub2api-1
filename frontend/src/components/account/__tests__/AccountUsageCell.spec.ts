import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import AccountUsageCell from '../AccountUsageCell.vue'
import type { Account } from '@/types'

const { getUsage, getCodexInviteResetStatus, consumeCodexInviteReset } = vi.hoisted(() => ({
  getUsage: vi.fn(),
  getCodexInviteResetStatus: vi.fn(),
  consumeCodexInviteReset: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getUsage,
      getCodexInviteResetStatus,
      consumeCodexInviteReset
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

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

describe('AccountUsageCell', () => {
  beforeEach(() => {
    getUsage.mockReset()
    getCodexInviteResetStatus.mockReset()
    consumeCodexInviteReset.mockReset()
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: true,
        media: '(min-width: 768px)',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }))
    })
  })

  it('Kiro 用量会显示订阅名、overage 标记和邮箱', async () => {
    getUsage.mockResolvedValue({
      kiro_subscription_title: 'Kiro Pro',
      kiro_overage_capability: 'ENABLED',
      kiro_overage_enabled: true,
      kiro_email: 'kiro-user@example.com',
      kiro_quota: {
        utilization: 32,
        resets_at: '2026-07-12T08:00:00Z'
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 5101,
          platform: 'kiro',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Kiro Pro')
    expect(wrapper.text()).toContain('admin.accounts.kiro.overageEnabled')
    expect(wrapper.text()).toContain('kiro-user@example.com')
    expect(wrapper.text()).toContain('$0|32|2026-07-12T08:00:00Z')
  })

  it('Kiro 用量会展示 profile/login/status 诊断标签', async () => {
    getUsage.mockResolvedValue({
      kiro_subscription_title: 'Kiro Pro',
      kiro_quota: {
        utilization: 18,
        resets_at: '2026-07-12T08:00:00Z'
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 5104,
          platform: 'kiro',
          type: 'oauth',
          extra: {
            profile_id: 'PROFILE-123',
            login_provider: 'microsoft',
            kiro_status_reason: 'FEATURE_NOT_SUPPORTED'
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.kiro.profileIdShort')
    expect(wrapper.text()).toContain('PROFILE-123')
    expect(wrapper.text()).toContain('admin.accounts.kiro.loginProviderShort')
    expect(wrapper.text()).toContain('microsoft')
    expect(wrapper.text()).toContain('admin.accounts.kiro.statusReasonShort')
    expect(wrapper.text()).toContain('FEATURE_NOT_SUPPORTED')
  })

  it('Kiro 用量会把不支持超额和未知套餐显示为明确徽标', async () => {
    getUsage.mockResolvedValue({
      kiro_overage_capability: 'NOT_SUPPORTED',
      kiro_overage_enabled: false,
      kiro_quota: {
        utilization: 12,
        resets_at: '2026-07-12T08:00:00Z'
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 5102,
          platform: 'kiro',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.kiro.subscriptionUnknown')
    expect(wrapper.text()).toContain('admin.accounts.kiro.overageUnsupported')
  })

  it('Kiro 用量会把可开未开的超额显示为 capable 徽标', async () => {
    getUsage.mockResolvedValue({
      kiro_subscription_title: 'Kiro Power',
      kiro_overage_capability: 'SUPPORTED',
      kiro_overage_enabled: false,
      kiro_quota: {
        utilization: 55,
        resets_at: '2026-07-12T08:00:00Z'
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 5103,
          platform: 'kiro',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Kiro Power')
    expect(wrapper.text()).toContain('admin.accounts.kiro.overageCapable')
    expect(wrapper.text()).not.toContain('admin.accounts.kiro.overageUnsupported')
  })

  it('Antigravity 图片用量会聚合新旧 image 模型', async () => {
    getUsage.mockResolvedValue({
      antigravity_quota: {
        'gemini-2.5-flash-image': {
          utilization: 45,
          reset_time: '2026-03-01T11:00:00Z'
        },
        'gemini-3.1-flash-image': {
          utilization: 20,
          reset_time: '2026-03-01T10:00:00Z'
        },
        'gemini-3-pro-image': {
          utilization: 70,
          reset_time: '2026-03-01T09:00:00Z'
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 1001,
          platform: 'antigravity',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.usageWindow.gemini3Image|70|2026-03-01T09:00:00Z')
  })

  it('Antigravity 会显示 AI Credits 余额信息', async () => {
    getUsage.mockResolvedValue({
      ai_credits: [
        {
          credit_type: 'GOOGLE_ONE_AI',
          amount: 25,
          minimum_balance: 5
        }
      ]
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 1002,
          platform: 'antigravity',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.aiCreditsBalance')
    expect(wrapper.text()).toContain('25')
  })

  it('Anthropic service_account 会拉取并展示用量窗口', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 31,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 5,
          tokens: 500,
          cost: 0.05,
          standard_cost: 0.05,
          user_cost: 0.05
        }
      },
      seven_day: {
        utilization: 62,
        resets_at: '2026-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 8,
          tokens: 800,
          cost: 0.08,
          standard_cost: 0.08,
          user_cost: 0.08
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 1003,
          platform: 'anthropic',
          type: 'service_account',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(1003, 'passive')
    expect(wrapper.text()).toContain('5h|31|2026-03-08T12:00:00Z')
    expect(wrapper.text()).toContain('7d|62|2026-03-13T12:00:00Z')
  })

  it('Grok OAuth 用量窗只展示 7d 比例和 req/token/账号/用户计费统计', async () => {
    getUsage.mockResolvedValue({
      seven_day: {
        utilization: 42,
        resets_at: '2026-07-16T00:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 7,
          tokens: 700,
          cost: 1.23,
          standard_cost: 1.11,
          user_cost: 2.34
        }
      },
      grok_request_quota: { limit: 100, remaining: 58, reset_at: '2026-07-16T00:00:00Z' },
      grok_token_quota: { limit: 1000, remaining: 580, reset_at: '2026-07-16T00:00:00Z' },
      grok_retry_after_seconds: 60,
      grok_quota_snapshot_state: 'no_headers',
      grok_last_status_code: 200,
      grok_last_quota_probe_at: '2026-07-09T00:00:00Z',
      subscription_tier: 'supergrok',
      subscription_tier_raw: 'supergrok',
      grok_entitlement_status: 'active'
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 7001,
          platform: 'grok',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}|{{ windowStats?.requests }}req|{{ windowStats?.tokens }}tok|A{{ windowStats?.cost }}|U{{ windowStats?.user_cost }}</div>'
          },
          AccountQuotaInfo: true,
          GrokQuotaProbeCell: { template: '<div>PROBE_RESET_SHOULD_NOT_RENDER</div>' }
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('7d|42|2026-07-16T00:00:00Z|7req|700tok|A1.23|U2.34')
    expect(wrapper.text()).not.toContain('supergrok')
    expect(wrapper.text()).not.toContain('active')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.grokRequests')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.grokTokens')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.grokNoHeaders')
    expect(wrapper.text()).not.toContain('admin.accounts.usageWindow.grokLastStatus')
    expect(wrapper.text()).not.toContain('PROBE_RESET_SHOULD_NOT_RENDER')
  })

  it('父级接管 batch usage 时不再自发调用逐个 getUsage', async () => {
    const requestBatchedUsage = vi.fn()

    mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 1999,
          platform: 'openai',
          type: 'oauth',
          extra: {}
        }),
        requestBatchedUsage
      } as any,
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(requestBatchedUsage).toHaveBeenCalledTimes(1)
    expect(requestBatchedUsage).toHaveBeenCalledWith(expect.objectContaining({ id: 1999 }), undefined)
    expect(getUsage).not.toHaveBeenCalled()
  })


  it('OpenAI OAuth 快照已过期时首屏会重新请求 usage', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 15,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 3,
          tokens: 300,
          cost: 0.03,
          standard_cost: 0.03,
          user_cost: 0.03
        }
      },
      seven_day: {
        utilization: 77,
        resets_at: '2026-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 3,
          tokens: 300,
          cost: 0.03,
          standard_cost: 0.03,
          user_cost: 0.03
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2000,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_usage_updated_at: '2026-03-07T00:00:00Z',
            codex_5h_used_percent: 12,
            codex_5h_reset_at: '2026-03-08T12:00:00Z',
            codex_7d_used_percent: 34,
            codex_7d_reset_at: '2026-03-13T12:00:00Z'
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(2000)
    expect(wrapper.text()).toContain('5h|15|300')
    expect(wrapper.text()).toContain('7d|77|300')
  })

  it('OpenAI OAuth 有 codex 快照时仍然使用 /usage API 数据渲染', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 18,
        resets_at: '2099-03-07T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 9,
          tokens: 900,
          cost: 0.09,
          standard_cost: 0.09,
          user_cost: 0.09
        }
      },
      seven_day: {
        utilization: 36,
        resets_at: '2099-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 9,
          tokens: 900,
          cost: 0.09,
          standard_cost: 0.09,
          user_cost: 0.09
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2001,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_usage_updated_at: '2099-03-07T10:00:00Z',
            codex_5h_used_percent: 12,
            codex_5h_reset_at: '2099-03-07T12:00:00Z',
            codex_7d_used_percent: 34,
            codex_7d_reset_at: '2099-03-13T12:00:00Z'
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(2001)
    // 单一数据源：始终使用 /usage API 返回值，忽略 codex 快照
    expect(wrapper.text()).toContain('5h|18|900')
    expect(wrapper.text()).toContain('7d|36|900')
  })

  it('OpenAI OAuth 会从账号 extra 显示已持久化的 Codex 重置次数', async () => {
    getUsage.mockResolvedValue({})

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2007,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_invite_reset_available_count: 3,
            codex_invite_reset_updated_at: '2026-03-07T10:00:00Z',
            codex_invite_reset_credit_ids: ['credit-1', 'credit-2', 'credit-3'],
            codex_invite_reset_credits: [
              { id: 'credit-1', status: 'available' },
              { id: 'credit-2', status: 'available' },
              { id: 'credit-3', status: 'available' }
            ]
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true,
          CodexInviteResetModal: true
        }
      }
    })

    await flushPromises()

    const inviteResetButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.accounts.inviteResetCountShort'))
    expect(inviteResetButton?.text()).toContain('3')
  })

  it('OpenAI OAuth 只显示 personal-dev 的 Codex 邀请重置入口', async () => {
    getUsage.mockResolvedValue({})

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2009,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_invite_reset_available_count: 2,
            codex_invite_reset_updated_at: '2026-03-07T10:00:00Z',
            codex_invite_reset_credits: []
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true,
          CodexInviteResetModal: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('admin.accounts.openaiQuotaReset.count')
    expect(wrapper.text()).toContain('admin.accounts.inviteResetCountShort')
    expect(wrapper.text()).toContain('2')
  })

  it('OpenAI OAuth 只有持久化次数没有 credit 明细时仍允许重置', async () => {
    getUsage.mockResolvedValue({})

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2013,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_invite_reset_available_count: 2,
            codex_invite_reset_updated_at: '2026-03-07T10:00:00Z',
            codex_invite_reset_credit_ids: [],
            codex_invite_reset_credits: []
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true,
          CodexInviteResetModal: true
        }
      }
    })

    await flushPromises()

    const resetButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.accounts.inviteResetReset'))
    expect(resetButton?.attributes('disabled')).toBeUndefined()
  })

  it('OpenAI OAuth 已有持久化 Codex 重置次数 0 时打开弹窗不重新查询', async () => {
    getUsage.mockResolvedValue({})

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2008,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_invite_reset_available_count: 0,
            codex_invite_reset_updated_at: '2026-03-07T10:00:00Z',
            codex_invite_reset_credit_ids: [],
            codex_invite_reset_credits: []
          }
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true,
          CodexInviteResetModal: true
        }
      }
    })

    await flushPromises()

    const inviteResetButton = wrapper
      .findAll('button')
      .find((button) => button.text().includes('admin.accounts.inviteResetCountShort'))
    expect(inviteResetButton?.text()).toContain('0')

    await inviteResetButton!.trigger('click')
    await flushPromises()

    expect(getCodexInviteResetStatus).not.toHaveBeenCalled()
  })

  it('OpenAI OAuth 有现成快照时，手动刷新信号会触发 usage 重拉', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 18,
        resets_at: '2099-03-07T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 9,
          tokens: 900,
          cost: 0.09,
          standard_cost: 0.09,
          user_cost: 0.09
        }
      },
      seven_day: {
        utilization: 36,
        resets_at: '2099-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 9,
          tokens: 900,
          cost: 0.09,
          standard_cost: 0.09,
          user_cost: 0.09
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2010,
          platform: 'openai',
          type: 'oauth',
          extra: {
            codex_usage_updated_at: '2099-03-07T10:00:00Z',
            codex_5h_used_percent: 12,
            codex_5h_reset_at: '2099-03-07T12:00:00Z',
            codex_7d_used_percent: 34,
            codex_7d_reset_at: '2099-03-13T12:00:00Z'
          },
          rate_limit_reset_at: null
        }),
        manualRefreshToken: 0
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()
    // mount 时已经拉取一次
    expect(getUsage).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ manualRefreshToken: 1 })
    await flushPromises()

    // 手动刷新再拉一次
    expect(getUsage).toHaveBeenCalledTimes(2)
    expect(getUsage).toHaveBeenCalledWith(2010)
    // 单一数据源：始终使用 /usage API 值
    expect(wrapper.text()).toContain('5h|18|900')
  })

  it('父级接管 batch usage 时手动刷新改为重新请求 batch', async () => {
    const requestBatchedUsage = vi.fn()

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2012,
          platform: 'openai',
          type: 'oauth',
          extra: {}
        }),
        manualRefreshToken: 0,
        requestBatchedUsage
      } as any,
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()
    expect(requestBatchedUsage).toHaveBeenCalledTimes(1)
    expect(getUsage).not.toHaveBeenCalled()

    await wrapper.setProps({ manualRefreshToken: 1 })
    await flushPromises()

    expect(requestBatchedUsage).toHaveBeenCalledTimes(2)
    expect(requestBatchedUsage).toHaveBeenLastCalledWith(expect.objectContaining({ id: 2012 }), { force: true })
    expect(getUsage).not.toHaveBeenCalled()
  })

  it('OpenAI OAuth usage 接口返回空窗口时不渲染 0% 占位条', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 0,
        resets_at: null,
        remaining_seconds: 0,
        window_stats: {
          requests: 0,
          tokens: 0,
          cost: 0,
          standard_cost: 0,
          user_cost: 0
        }
      },
      seven_day: {
        utilization: 0,
        resets_at: null,
        remaining_seconds: 0,
        window_stats: {
          requests: 0,
          tokens: 0,
          cost: 0,
          standard_cost: 0,
          user_cost: 0
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2011,
          platform: 'openai',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(2011)
    expect(wrapper.findAll('.usage-bar')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('5h|0')
    expect(wrapper.text()).not.toContain('7d|0')
  })

  it('OpenAI OAuth 在无 codex 快照时会回退显示 usage 接口窗口', async () => {
	getUsage.mockResolvedValue({
	  five_hour: {
	    utilization: 0,
	    resets_at: null,
	    remaining_seconds: 0,
	    window_stats: {
	      requests: 2,
	      tokens: 27700,
	      cost: 0.06,
	      standard_cost: 0.06,
	      user_cost: 0.06
	    }
	  },
	  seven_day: {
	    utilization: 0,
	    resets_at: null,
	    remaining_seconds: 0,
	    window_stats: {
	      requests: 2,
	      tokens: 27700,
	      cost: 0.06,
	      standard_cost: 0.06,
	      user_cost: 0.06
	    }
	  }
	})

		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 2002,
		      platform: 'openai',
		      type: 'oauth',
		      extra: {}
		    })
		  },
	  global: {
	    stubs: {
	      UsageProgressBar: {
	        props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
	        template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
	      },
	      AccountQuotaInfo: true
	    }
	  }
	})

	await flushPromises()

	expect(getUsage).toHaveBeenCalledWith(2002)
	expect(wrapper.text()).toContain('5h|0|27700')
	expect(wrapper.text()).toContain('7d|0|27700')
  })

  it('OpenAI OAuth 在行数据刷新但仍无 codex 快照时会重新拉取 usage', async () => {
	getUsage
	  .mockResolvedValueOnce({
	    five_hour: {
	      utilization: 0,
	      resets_at: null,
	      remaining_seconds: 0,
	      window_stats: {
	        requests: 1,
	        tokens: 100,
	        cost: 0.01,
	        standard_cost: 0.01,
	        user_cost: 0.01
	      }
	    },
	    seven_day: null
	  })
	  .mockResolvedValueOnce({
	    five_hour: {
	      utilization: 0,
	      resets_at: null,
	      remaining_seconds: 0,
	      window_stats: {
	        requests: 2,
	        tokens: 200,
	        cost: 0.02,
	        standard_cost: 0.02,
	        user_cost: 0.02
	      }
	    },
	    seven_day: null
	  })

		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 2003,
		      platform: 'openai',
		      type: 'oauth',
		      updated_at: '2026-03-07T10:00:00Z',
		      extra: {}
		    })
		  },
	  global: {
	    stubs: {
	      UsageProgressBar: {
	        props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
	        template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
	      },
	      AccountQuotaInfo: true
	    }
	  }
	})

	await flushPromises()
	expect(wrapper.text()).toContain('5h|0|100')
	expect(getUsage).toHaveBeenCalledTimes(1)

		await wrapper.setProps({
		  account: {
		    id: 2003,
		    platform: 'openai',
		    type: 'oauth',
		    last_used_at: null,
		    rate_limit_reset_at: null,
		    updated_at: '2026-03-07T10:01:00Z',
		    extra: {}
		  }
		})

	await flushPromises()
	expect(getUsage).toHaveBeenCalledTimes(2)
	expect(wrapper.text()).toContain('5h|0|200')
  })

  it('OpenAI OAuth 已限额时显示 /usage API 返回的限额数据', async () => {
	getUsage.mockResolvedValue({
	  five_hour: {
	    utilization: 100,
	    resets_at: '2026-03-07T12:00:00Z',
	    remaining_seconds: 3600,
	    window_stats: {
	      requests: 211,
	      tokens: 106540000,
	      cost: 38.13,
	      standard_cost: 38.13,
	      user_cost: 38.13
	    }
	  },
	  seven_day: {
	    utilization: 100,
	    resets_at: '2026-03-13T12:00:00Z',
	    remaining_seconds: 3600,
	    window_stats: {
	      requests: 211,
	      tokens: 106540000,
	      cost: 38.13,
	      standard_cost: 38.13,
	      user_cost: 38.13
	    }
	  }
	})

		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 2004,
		      platform: 'openai',
		      type: 'oauth',
		      rate_limit_reset_at: '2099-03-07T12:00:00Z',
		      extra: {
		        codex_5h_used_percent: 0,
		        codex_7d_used_percent: 0
		      }
		    })
		  },
	  global: {
	    stubs: {
	      UsageProgressBar: {
	        props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
	        template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
	      },
	      AccountQuotaInfo: true
	    }
	  }
	})

	await flushPromises()

  expect(getUsage).toHaveBeenCalledWith(2004)
  expect(wrapper.text()).toContain('5h|100|106540000')
  expect(wrapper.text()).toContain('7d|100|106540000')
  })

  it('OpenAI 生图路由只显示当前分组启用且账号支持的路线', async () => {
    getUsage.mockResolvedValue({
      openai_image_codex_supported: false,
      openai_image_codex_reason: 'free_plan_not_supported',
      openai_image_codex_five_hour: {
        utilization: 100,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: { requests: 9, tokens: 99, cost: 0.99 }
      },
      openai_image_workspace_name: 'Personal',
      openai_image_plan_type: 'free'
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2005,
          platform: 'openai',
          type: 'oauth',
          extra: {},
          groups: [
            {
              id: 101,
              name: 'OpenAI Codex Unsupported',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: false,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            },
            {
              id: 102,
              name: 'OpenAI Legacy Route',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: false,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        }),
        activeGroupId: 101
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.usage-bar')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('admin.accounts.openaiImageRoutes')
    expect(wrapper.text()).not.toContain('free accounts cannot use codex image generation')
    expect(wrapper.text()).not.toContain('Personal')
    expect(wrapper.text()).not.toContain('available')

    await wrapper.setProps({ activeGroupId: 102 })
    await flushPromises()

    expect(wrapper.findAll('.usage-bar')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('img:')
  })

  it('OpenAI Web2API 生图窗口不再纳入 img 摘要', async () => {
    getUsage.mockResolvedValue({
      openai_image_codex_supported: true,
      openai_image_web2api_five_hour: {
        utilization: 88,
        resets_at: '2026-03-08T13:00:00Z',
        remaining_seconds: 1800,
        window_stats: { requests: 3, tokens: 30, cost: 0.3, user_cost: 0.3 }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2006,
          platform: 'openai',
          type: 'oauth',
          groups: [
            {
              id: 201,
              name: 'OpenAI Image Route',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: false,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'windowStats'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.requests }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.usage-bar')).toHaveLength(0)
    expect(wrapper.text()).not.toContain('img:')
    expect(wrapper.text()).not.toContain('web 3req $0.30')
  })

  it('OpenAI 生图窗口只有限流或重置进度时仍显示 img 摘要', async () => {
    getUsage.mockResolvedValue({
      openai_image_codex_supported: true,
      openai_image_codex_five_hour: {
        utilization: 100,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 1800,
        window_stats: {
          requests: 0,
          tokens: 0,
          cost: 0,
          standard_cost: 0,
          user_cost: 0
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2007,
          platform: 'openai',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('img:')
    expect(wrapper.text()).toContain('5h 0req $0.00')
  })

  it('OpenAI 生图窗口只有 used_requests 时仍显示 img 摘要', async () => {
    getUsage.mockResolvedValue({
      openai_image_codex_supported: true,
      openai_image_codex_five_hour: {
        utilization: 0,
        resets_at: null,
        remaining_seconds: 0,
        used_requests: 4
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2008,
          platform: 'openai',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('img:')
    expect(wrapper.text()).toContain('5h 4req $0.00')
  })

  it('OpenAI codex 生图路线开启时不会补齐 img 5h/img 7d 占位条', async () => {
    getUsage.mockResolvedValue({
      openai_image_codex_supported: true,
      five_hour: {
        utilization: 12,
        resets_at: '2026-03-08T12:00:00Z',
        window_stats: { requests: 10, tokens: 1000, cost: 0.12 }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 20051,
          platform: 'openai',
          type: 'oauth',
          groups: [
            {
              id: 103,
              name: 'OpenAI Image Route',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: false,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        }),
        activeGroupId: 103
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.usage-bar').map((node) => node.text())).toEqual([
      '5h|12'
    ])
    expect(wrapper.text()).not.toContain('img 5h')
    expect(wrapper.text()).not.toContain('img 7d')
    expect(wrapper.text()).not.toContain('codex')
  })

  it('OpenAI 响应用量条不会被生图窗口替换', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 42,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 100,
          tokens: 123456,
          cost: 1.23
        }
      },
      seven_day: {
        utilization: 58,
        resets_at: '2026-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 200,
          tokens: 654321,
          cost: 2.34
        }
      },
      openai_image_codex_five_hour: {
        utilization: 99,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 10,
          tokens: 111,
          cost: 3.45,
          user_cost: 3.45
        }
      },
      openai_image_codex_seven_day: {
        utilization: 100,
        resets_at: '2026-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 20,
          tokens: 222,
          cost: 4.56,
          user_cost: 4.56
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2006,
          platform: 'openai',
          type: 'oauth',
          extra: {},
          groups: [
            {
              id: 201,
              name: 'OpenAI Codex',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: false,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        }),
        activeGroupId: 201
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'windowStats'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('5h|42|123456')
    expect(wrapper.text()).toContain('7d|58|654321')
    expect(wrapper.text()).not.toContain('img 5h')
    expect(wrapper.text()).not.toContain('img 7d')
    expect(wrapper.text()).toContain('img:')
    expect(wrapper.text()).toContain('5h 10req $3.45')
    expect(wrapper.text()).toContain('7d 20req $4.56')
  })

  it('OpenAI OAuth 用短标签展示普通窗口，并用 img 摘要展示生图窗口', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 19,
        resets_at: '2026-03-08T12:00:00Z',
        window_stats: { requests: 76, tokens: 6200000, cost: 3.26, user_cost: 0.98 }
      },
      seven_day: {
        utilization: 31,
        resets_at: '2026-03-13T12:00:00Z',
        window_stats: { requests: 642, tokens: 57400000, cost: 34.55, user_cost: 10.37 }
      },
      openai_image_codex_five_hour: {
        utilization: 0,
        resets_at: '2026-03-08T12:00:00Z',
        window_stats: { requests: 1, tokens: 10, cost: 0.1, user_cost: 0.1 }
      },
      openai_image_codex_seven_day: {
        utilization: 0,
        resets_at: '2026-03-13T12:00:00Z',
        window_stats: { requests: 2, tokens: 20, cost: 0.2, user_cost: 0.2 }
      },
      openai_image_web2api_five_hour: {
        utilization: 88,
        resets_at: '2026-03-08T13:00:00Z',
        window_stats: { requests: 3, tokens: 30, cost: 0.3, user_cost: 0.3 }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2007,
          platform: 'openai',
          type: 'oauth',
          groups: [
            {
              id: 301,
              name: 'OpenAI Images',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: true,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'windowStats'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.requests }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.usage-bar').map((node) => node.text())).toEqual([
      '5h|19|76',
      '7d|31|642'
    ])
    expect(wrapper.text()).toContain('img:')
    expect(wrapper.text()).toContain('5h 1req $0.10')
    expect(wrapper.text()).toContain('7d 2req $0.20')
    expect(wrapper.text()).not.toContain('web 3req $0.30')
    expect(wrapper.text()).not.toContain('img 5h')
    expect(wrapper.text()).not.toContain('img 7d')
    expect(wrapper.text()).not.toContain('codex 5h')
    expect(wrapper.text()).not.toContain('web2api 5h')
    expect(wrapper.text()).not.toContain('web 5h')
  })

  it('OpenAI OAuth 在当前分组为 codex 时忽略历史 web2api 窗口', async () => {
    getUsage.mockResolvedValue({
      five_hour: {
        utilization: 11,
        resets_at: '2026-03-08T12:00:00Z',
        window_stats: { requests: 1, tokens: 11, cost: 0.11 }
      },
      seven_day: {
        utilization: 22,
        resets_at: '2026-03-13T12:00:00Z',
        window_stats: { requests: 2, tokens: 22, cost: 0.22 }
      },
      openai_image_codex_supported: true,
      openai_image_codex_five_hour: {
        utilization: 11,
        resets_at: '2026-03-08T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: { requests: 1, tokens: 11, cost: 0.11, user_cost: 0.11 }
      },
      openai_image_codex_seven_day: {
        utilization: 22,
        resets_at: '2026-03-13T12:00:00Z',
        remaining_seconds: 3600,
        window_stats: { requests: 2, tokens: 22, cost: 0.22, user_cost: 0.22 }
      },
      openai_image_web2api_five_hour: {
        utilization: 33,
        resets_at: '2026-03-08T13:00:00Z',
        remaining_seconds: 1800,
        window_stats: { requests: 3, tokens: 33, cost: 0.33, user_cost: 0.33 }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 2008,
          platform: 'openai',
          type: 'oauth',
          groups: [
            {
              id: 401,
              name: 'OpenAI Images',
              description: '',
              platform: 'openai',
              rate_limit: 0,
              priority: 0,
              rate_multiplier: 1,
              is_exclusive: false,
              status: 'active',
              subscription_type: 'free',
              daily_limit_usd: null,
              weekly_limit_usd: null,
              monthly_limit_usd: null,
              allow_image_generation: true,
              image_generation_route: 'codex',
              image_rate_independent: true,
              image_rate_multiplier: 1,
              image_price_1k: null,
              image_price_2k: null,
              image_price_4k: null,
              claude_code_only: false,
              fallback_group_id: null,
              fallback_group_id_on_invalid_request: null,
              require_oauth_only: false,
              require_privacy_set: false
            }
          ]
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'windowStats'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.requests }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.findAll('.usage-bar').map((node) => node.text())).toEqual([
      '5h|11|1',
      '7d|22|2'
    ])
    expect(wrapper.text()).toContain('img:')
    expect(wrapper.text()).toContain('5h 1req $0.11')
    expect(wrapper.text()).toContain('7d 2req $0.22')
    expect(wrapper.text()).not.toContain('web 3req $0.33')
    expect(wrapper.text()).not.toContain('img 5h')
    expect(wrapper.text()).not.toContain('img 7d')
  })

  it('Key 账号会展示 today stats 徽章并带 A/U 提示', async () => {
		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 3001,
		      platform: 'anthropic',
		      type: 'apikey'
		    }),
		    todayStats: {
		      requests: 1_000_000,
		      tokens: 1_000_000_000,
		      cost: 12.345,
		      standard_cost: 12.345,
		      user_cost: 6.789
		    }
		  },
		  global: {
		    stubs: {
		      UsageProgressBar: true,
		      AccountQuotaInfo: true
		    }
		  }
		})

		await flushPromises()

		expect(wrapper.text()).toContain('1.0M req')
		expect(wrapper.text()).toContain('1.0B')
		expect(wrapper.text()).toContain('A $12.35')
		expect(wrapper.text()).toContain('U $6.79')

		const badges = wrapper.findAll('span[title]')
		expect(badges.some(node => node.attributes('title') === 'usage.accountBilled')).toBe(true)
		expect(badges.some(node => node.attributes('title') === 'usage.userBilled')).toBe(true)
  })

  it('Grok OAuth 会展示本地 user billed 用量并保留超限百分比', async () => {
    getUsage.mockResolvedValue({
      grok_local_usage: {
        requests: 4,
        tokens: 1200,
        cost: 0.12,
        standard_cost: 0.12,
        user_cost: 0.34
      },
      grok_request_quota: {
        limit: 10,
        remaining: -2,
        reset_at: '2026-07-09T16:00:00Z'
      },
      grok_quota_snapshot_state: 'observed'
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 3861,
          platform: 'grok',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ resetsAt }}</div>'
          },
          AccountQuotaInfo: true,
          GrokQuotaProbeCell: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(3861)
    expect(wrapper.text()).toContain('4 req')
    expect(wrapper.text()).toContain('1.2K')
    expect(wrapper.text()).toContain('A $0.12')
    expect(wrapper.text()).toContain('U $0.34')
    expect(wrapper.text()).toContain('admin.accounts.usageWindow.grokRequests|120|2026-07-09T16:00:00Z')

    const badges = wrapper.findAll('span[title]')
    expect(badges.some(node => node.attributes('title') === 'usage.accountBilled')).toBe(true)
    expect(badges.some(node => node.attributes('title') === 'usage.userBilled')).toBe(true)
  })

  it('Key 账号在 today stats loading 时显示骨架屏', async () => {
		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 3002,
		      platform: 'anthropic',
		      type: 'apikey'
		    }),
		    todayStats: null,
		    todayStatsLoading: true
		  },
		  global: {
		    stubs: {
		      UsageProgressBar: true,
		      AccountQuotaInfo: true
		    }
		  }
		})

		await flushPromises()

		expect(wrapper.findAll('.animate-pulse').length).toBeGreaterThan(0)
  })

  it('Key 账号在无 today stats 且无配额时显示兜底短横线', async () => {
		const wrapper = mount(AccountUsageCell, {
		  props: {
		    account: makeAccount({
		      id: 3003,
		      platform: 'anthropic',
		      type: 'apikey',
		      quota_limit: 0,
		      quota_daily_limit: 0,
		      quota_weekly_limit: 0
		    }),
		    todayStats: null,
		    todayStatsLoading: false
		  },
		  global: {
		    stubs: {
		      UsageProgressBar: true,
		      AccountQuotaInfo: true
		    }
		  }
		})

		await flushPromises()

		expect(wrapper.text().trim()).toBe('-')
  })

  it('Gemini 共享池账号按 Pro / Flash 两类展示时间窗口', async () => {
    getUsage.mockResolvedValue({
      gemini_pro_daily: {
        utilization: 25,
        resets_at: '2026-03-08T08:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 5,
          tokens: 5000,
          cost: 0.05
        }
      },
      gemini_flash_daily: {
        utilization: 40,
        resets_at: '2026-03-08T08:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 12,
          tokens: 12000,
          cost: 0.03
        }
      },
      gemini_shared_daily: {
        utilization: 65,
        resets_at: '2026-03-08T08:00:00Z',
        remaining_seconds: 3600
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4001,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'google_one',
            tier_id: 'google_ai_pro'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(4001)
    expect(wrapper.text()).toContain('admin.accounts.usageWindow.geminiProDaily|65|5000')
    expect(wrapper.text()).toContain('admin.accounts.usageWindow.geminiFlashDaily|65|12000')
  })

  it('Gemini 在无日窗口时回退显示分钟窗口', async () => {
    getUsage.mockResolvedValue({
      gemini_pro_minute: {
        utilization: 60,
        resets_at: '2026-03-08T00:01:00Z',
        remaining_seconds: 30,
        window_stats: {
          requests: 3,
          tokens: 300,
          cost: 0.01
        }
      },
      gemini_flash_minute: {
        utilization: 10,
        resets_at: '2026-03-08T00:01:00Z',
        remaining_seconds: 30,
        window_stats: {
          requests: 4,
          tokens: 400,
          cost: 0.01
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4002,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'code_assist',
            tier_id: 'gcp_standard'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledWith(4002)
    expect(wrapper.text()).toContain('admin.accounts.usageWindow.geminiProDaily|60|300')
    expect(wrapper.text()).toContain('admin.accounts.usageWindow.geminiFlashDaily|10|400')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.rateLimit.unlimited')
  })

  it('Gemini 仅返回共享池窗口时仍展示 shared 用量', async () => {
    getUsage.mockResolvedValue({
      gemini_shared_daily: {
        utilization: 73,
        resets_at: '2026-03-08T08:00:00Z',
        remaining_seconds: 3600,
        window_stats: {
          requests: 17,
          tokens: 17000,
          cost: 0.08
        }
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4003,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'google_one',
            tier_id: 'google_ai_pro'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('1d|73|17000')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.rateLimit.ok')
  })

  it('Gemini service account 不再额外展示 today stats 徽章', async () => {
    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4004,
          platform: 'gemini',
          type: 'service_account',
          credentials: {
            tier_id: 'vertex',
            project_id: 'vertex-proj',
            client_email: 'svc@vertex-proj.iam.gserviceaccount.com',
            location: 'global'
          },
          extra: {}
        }),
        todayStats: {
          requests: 0,
          tokens: 0,
          cost: 0,
          standard_cost: 0,
          user_cost: 0
        }
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).not.toContain('0 req')
    expect(wrapper.text()).not.toContain('A $0.00')
    expect(wrapper.text()).not.toContain('U $0.00')
    expect(wrapper.text().trim()).toBe('-')
  })

  it('Gemini forbidden 状态优先展示封禁徽章而不是 unlimited', async () => {
    getUsage.mockResolvedValue({
      is_forbidden: true,
      forbidden_type: 'validation',
      validation_url: 'https://example.com/verify'
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4005,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'google_one'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.forbiddenValidation')
    expect(wrapper.text()).toContain('admin.accounts.openVerification')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.rateLimit.ok')
  })

  it('Gemini needs reauth 状态优先展示重新授权徽章', async () => {
    getUsage.mockResolvedValue({
      needs_reauth: true
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4006,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'code_assist'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.needsReauth')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.rateLimit.ok')
  })

  it('Gemini 配额查询降级时展示错误徽章', async () => {
    getUsage.mockResolvedValue({
      error: 'quota snapshot fetch failed',
      error_code: 'rate_limited'
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4007,
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'google_one'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: true,
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.rateLimited')
    expect(wrapper.text()).not.toContain('admin.accounts.gemini.rateLimit.ok')
  })

  it('Gemini 行数据中的 usage 快照变化时会重新拉取 usage', async () => {
    getUsage
      .mockResolvedValueOnce({
        gemini_shared_daily: {
          utilization: 15,
          resets_at: '2026-03-08T08:00:00Z',
          remaining_seconds: 3600,
          window_stats: {
            requests: 2,
            tokens: 200,
            cost: 0.01
          }
        }
      })
      .mockResolvedValueOnce({
        gemini_shared_daily: {
          utilization: 55,
          resets_at: '2026-03-08T10:00:00Z',
          remaining_seconds: 3600,
          window_stats: {
            requests: 5,
            tokens: 500,
            cost: 0.03
          }
        }
      })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 4008,
          platform: 'gemini',
          type: 'oauth',
          updated_at: '2026-03-07T10:00:00Z',
          credentials: {
            oauth_type: 'google_one',
            tier_id: 'google_ai_pro',
            usage_updated_at: '2026-03-07T10:00:00Z'
          },
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'windowStats', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}|{{ windowStats?.tokens }}</div>'
          },
          AccountQuotaInfo: true
        }
      }
    })

    await flushPromises()
    expect(getUsage).toHaveBeenCalledTimes(1)
    expect(wrapper.text()).toContain('1d|15|200')

    await wrapper.setProps({
      account: makeAccount({
        id: 4008,
        platform: 'gemini',
        type: 'oauth',
        updated_at: '2026-03-07T10:01:00Z',
        credentials: {
          oauth_type: 'google_one',
          tier_id: 'google_ai_pro',
          usage_updated_at: '2026-03-07T10:01:00Z'
        },
        extra: {}
      })
    })

    await flushPromises()

    expect(getUsage).toHaveBeenCalledTimes(2)
    expect(wrapper.text()).toContain('1d|55|500')
  })

  it('Anthropic OAuth 会渲染 7d F (Fable) 进度条，且 7d S 逻辑保留', async () => {
    getUsage.mockResolvedValue({
      source: 'passive',
      five_hour: {
        utilization: 41,
        resets_at: '2026-07-03T10:00:00Z',
        remaining_seconds: 3600
      },
      seven_day: {
        utilization: 56,
        resets_at: '2026-07-06T22:00:00Z',
        remaining_seconds: 300000
      },
      seven_day_sonnet: {
        utilization: 30,
        resets_at: '2026-07-06T22:00:00Z',
        remaining_seconds: 300000
      },
      seven_day_fable: {
        utilization: 100,
        resets_at: '2026-07-06T22:00:00Z',
        remaining_seconds: 300000
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 3001,
          platform: 'anthropic',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}</div>'
          },
          AccountQuotaInfo: true,
          GrokQuotaProbeCell: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('5h|41')
    expect(wrapper.text()).toContain('7d|56')
    expect(wrapper.text()).toContain('7d S|30')
    expect(wrapper.text()).toContain('7d F|100')
  })

  it('Anthropic OAuth 无 Fable 数据时不渲染 7d F 进度条', async () => {
    getUsage.mockResolvedValue({
      source: 'passive',
      five_hour: {
        utilization: 41,
        resets_at: '2026-07-03T10:00:00Z',
        remaining_seconds: 3600
      },
      seven_day: {
        utilization: 56,
        resets_at: '2026-07-06T22:00:00Z',
        remaining_seconds: 300000
      }
    })

    const wrapper = mount(AccountUsageCell, {
      props: {
        account: makeAccount({
          id: 3002,
          platform: 'anthropic',
          type: 'oauth',
          extra: {}
        })
      },
      global: {
        stubs: {
          UsageProgressBar: {
            props: ['label', 'utilization', 'resetsAt', 'color'],
            template: '<div class="usage-bar">{{ label }}|{{ utilization }}</div>'
          },
          AccountQuotaInfo: true,
          GrokQuotaProbeCell: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.text()).toContain('5h|41')
    expect(wrapper.text()).toContain('7d|56')
    expect(wrapper.text()).not.toContain('7d S')
    expect(wrapper.text()).not.toContain('7d F')
  })
})
