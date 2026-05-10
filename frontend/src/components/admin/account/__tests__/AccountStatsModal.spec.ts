import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, beforeEach, vi } from 'vitest'

const getStatsMock = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', () => ({
  createI18n: () => ({
    global: {
      locale: { value: 'en' },
      setLocaleMessage: vi.fn()
    }
  }),
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getStats: getStatsMock
    }
  }
}))

import AccountStatsModal from '../AccountStatsModal.vue'

describe('AccountStatsModal', () => {
  beforeEach(() => {
    getStatsMock.mockReset()
    getStatsMock.mockResolvedValue({
      history: [],
      summary: {
        days: 30,
        actual_days_used: 0,
        total_cost: 0,
        total_user_cost: 0,
        total_standard_cost: 0,
        total_requests: 0,
        total_tokens: 0,
        avg_daily_cost: 0,
        avg_daily_user_cost: 0,
        avg_daily_requests: 0,
        avg_daily_tokens: 0,
        avg_duration_ms: 0,
        today: null,
        highest_cost_day: null,
        highest_request_day: null
      },
      models: [],
      endpoints: [],
      upstream_endpoints: []
    })
  })

  it('loads stats immediately when mounted open', async () => {
    shallowMount(AccountStatsModal, {
      props: {
        show: true,
        account: {
          id: 42,
          name: 'OpenAI Account',
          status: 'active'
        }
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          LoadingSpinner: true,
          ModelDistributionChart: true,
          EndpointDistributionChart: true,
          Icon: true,
          Line: true
        }
      }
    })

    await flushPromises()

    expect(getStatsMock).toHaveBeenCalledTimes(1)
    expect(getStatsMock).toHaveBeenCalledWith(42, 30)
  })
})
