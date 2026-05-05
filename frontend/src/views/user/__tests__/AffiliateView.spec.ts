import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AffiliateView from '@/views/user/AffiliateView.vue'

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayout',
    template: '<div><slot /></div>'
  }
}))

const {
  getAffiliateDetailMock,
  getAffiliateInviteeLedgerMock,
  transferAffiliateQuotaMock,
  showErrorMock,
  showSuccessMock,
  refreshUserMock,
  copyToClipboardMock
} = vi.hoisted(() => ({
  getAffiliateDetailMock: vi.fn(),
  getAffiliateInviteeLedgerMock: vi.fn(),
  transferAffiliateQuotaMock: vi.fn(),
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  refreshUserMock: vi.fn(),
  copyToClipboardMock: vi.fn()
}))

vi.mock('@/api/user', () => ({
  userAPI: {
    getAffiliateDetail: getAffiliateDetailMock,
    getAffiliateInviteeLedger: getAffiliateInviteeLedgerMock,
    transferAffiliateQuota: transferAffiliateQuotaMock
  },
  default: {
    getAffiliateDetail: getAffiliateDetailMock,
    getAffiliateInviteeLedger: getAffiliateInviteeLedgerMock,
    transferAffiliateQuota: transferAffiliateQuotaMock
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: refreshUserMock
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: copyToClipboardMock
  })
}))

vi.mock('@/utils/format', () => ({
  formatCurrency: (value: number) => `currency:${value}`,
  formatDateTime: (value?: string | null) => value ? `date:${value}` : ''
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (_error: unknown, fallback: string) => fallback
}))

vi.mock('@/utils/url', () => ({
  buildAppAbsoluteUrl: (path: string, origin: string) => `${origin}${path}`,
  buildAppPath: (path: string) => path
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('AffiliateView', () => {
  beforeEach(() => {
    getAffiliateDetailMock.mockReset()
    getAffiliateInviteeLedgerMock.mockReset()
    transferAffiliateQuotaMock.mockReset()
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    refreshUserMock.mockReset()
    copyToClipboardMock.mockReset()

    getAffiliateDetailMock.mockResolvedValue({
      user_id: 1,
      aff_code: 'AFF123',
      aff_count: 1,
      aff_quota: 10,
      aff_frozen_quota: 0,
      aff_history_quota: 20,
      invited_count: 1,
      rebated_invitee_count: 1,
      remaining_rebate_slots: 5,
      effective_rebate_rate_percent: 20,
      policy: {
        enabled: true,
        rebate_rate: 20,
        rebate_cap: 0,
        invitee_limit: 0,
        signup_bonus: 0,
        policy_text: ''
      },
      invitees: [
        {
          user_id: 2,
          email: 'invitee@example.com',
          username: 'invitee',
          created_at: '2026-05-05T00:00:00Z',
          total_consumed: 12.5,
          total_rebate: 2.5,
          rebate_slot_claimed: true
        }
      ]
    })
    getAffiliateInviteeLedgerMock.mockResolvedValue([])
    transferAffiliateQuotaMock.mockResolvedValue({ transferred_quota: 0, balance: 0 })
    refreshUserMock.mockResolvedValue(undefined)
    copyToClipboardMock.mockResolvedValue(undefined)
  })

  it('invitees table only renders one rebate column', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true
        }
      }
    })

    await flushPromises()

    const rebateHeaders = wrapper
      .findAll('th')
      .filter((node) => node.text() === 'affiliate.invitees.columns.rebate')

    expect(rebateHeaders).toHaveLength(1)
    expect(wrapper.text()).toContain('currency:2.5')
  })
})
