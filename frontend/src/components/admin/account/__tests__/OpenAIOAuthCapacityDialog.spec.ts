import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

const { getOverviewMock, getTimeseriesMock } = vi.hoisted(() => ({
  getOverviewMock: vi.fn(),
  getTimeseriesMock: vi.fn()
}))

vi.mock('@/api/admin/oauthCapacity', () => ({
  getOverview: getOverviewMock,
  getTimeseries: getTimeseriesMock
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      locale: { value: 'en' },
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({ name: 'Line', template: '<div data-testid="oauth-trend-chart" />' })
}))

import OpenAIOAuthCapacityDialog from '../OpenAIOAuthCapacityDialog.vue'

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: { show: { type: Boolean, default: false } },
  template: '<div v-if="show"><slot /></div>'
})

describe('OpenAIOAuthCapacityDialog', () => {
  beforeEach(() => {
    getOverviewMock.mockResolvedValue({
      generated_at: '2026-08-13T12:00:00Z',
      total: {
        group_name: 'all',
        accounts: { plan_type: 'all', total: 4, schedulable: 3, errors: 1, rate_limited: 2 },
        plan_counts: [{ plan_type: 'plus', total: 4, schedulable: 3, errors: 1, rate_limited: 2 }],
        rate_limits: {
          total: 2,
          up_to_10m: 0,
          from_10m_to_30m: 1,
          from_30m_to_1h: 0,
          from_1h_to_3h: 0,
          from_3h_to_5h: 1,
          from_5h_to_1d: 0,
          from_1d_to_3d: 0,
          over_3d: 0
        },
        windows: [
          {
            window: '5h',
            used_percent: 80,
            remaining_percent: 20,
            reset_at: '2026-08-13T14:00:00Z',
            burn_rate: 1.33,
            exhausts_at: '2026-08-13T13:30:00Z',
            measured_accounts: 3,
            alert: 'warning'
          },
          {
            window: '7d',
            used_percent: null,
            remaining_percent: null,
            reset_at: null,
            burn_rate: 0,
            measured_accounts: 0
          }
        ],
        suggest_accounts: 2
      },
      groups: [{ group_id: 9, group_name: 'prod', accounts: { plan_type: 'all', total: 4, schedulable: 3, errors: 1, rate_limited: 2 } }]
    })
    getTimeseriesMock.mockResolvedValue({
      generated_at: '2026-08-13T12:00:00Z',
      now: '2026-08-13T12:00:00Z',
      platform: 'openai',
      group_id: null,
      unit: 'usd',
      range: '24h',
      kpis: {
        current_available_usd: 12.5,
        future_forecast_spend_usd: 8.2,
        first_shortfall_at: null,
        suggest_accounts: 2
      },
      points: [
        {
          bucket_start: '2026-08-13T11:00:00Z',
          segment: 'past',
          spend_usd: 3.1,
          forecast_spend_usd: null,
          available_usd: 15,
          forecast_available_usd: null,
          recovered_usd: 0
        }
      ],
      events: [],
      recommendations: []
    })
  })

  it('renders seat health, dual windows, rate-limit buckets, and trend chart', async () => {
    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: true,
          Select: true
        }
      }
    })
    await flushPromises()

    expect(getOverviewMock).toHaveBeenCalled()
    expect(getTimeseriesMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('20.0%')
    expect(wrapper.text()).toContain('—')
    expect(wrapper.text()).toContain('admin.accounts.oauthCapacity.alertWarning')
    expect(wrapper.text()).toContain('≤10m')
    expect(wrapper.text()).toContain('10-30m')
    expect(wrapper.text()).toContain('plus')
    expect(wrapper.text()).toContain('prod')
    expect(wrapper.find('[data-testid="oauth-trend-chart"]').exists()).toBe(true)
  })

  it('renders a group-scoped overview when groups is null', async () => {
    getOverviewMock.mockResolvedValue({
      generated_at: '2026-08-13T12:00:00Z',
      total: {
        group_name: 'prod',
        accounts: { plan_type: 'all', total: 2, schedulable: 2, errors: 0, rate_limited: 0 },
        plan_counts: [],
        rate_limits: {
          total: 0,
          up_to_10m: 0,
          from_10m_to_30m: 0,
          from_30m_to_1h: 0,
          from_1h_to_3h: 0,
          from_3h_to_5h: 0,
          from_5h_to_1d: 0,
          from_1d_to_3d: 0,
          over_3d: 0
        },
        windows: [
          {
            window: '5h',
            used_percent: 80,
            remaining_percent: 20,
            reset_at: '2026-08-13T14:00:00Z',
            burn_rate: 1.33,
            measured_accounts: 2
          }
        ],
        suggest_accounts: 0
      },
      groups: null
    })

    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: true,
          Select: true
        }
      }
    })
    await flushPromises()

    expect(wrapper.text()).toContain('20.0%')
    expect(wrapper.text()).not.toContain('admin.accounts.oauthCapacity.groups')
  })
})
