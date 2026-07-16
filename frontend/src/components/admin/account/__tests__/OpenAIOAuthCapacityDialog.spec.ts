import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, nextTick } from 'vue'

import OpenAIOAuthCapacityDialog from '../OpenAIOAuthCapacityDialog.vue'

const { getOpenAIOAuthCapacity, getOpenAIOAuthCapacityTimeseries } = vi.hoisted(() => ({
  getOpenAIOAuthCapacity: vi.fn(),
  getOpenAIOAuthCapacityTimeseries: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: { getOpenAIOAuthCapacity, getOpenAIOAuthCapacityTimeseries }
  }
}))

vi.mock('vue-chartjs', () => ({
  Line: defineComponent({
    name: 'LineChartStub',
    props: ['data', 'options'],
    template: '<div class="line-chart-stub" />'
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en-US' }
    })
  }
})

const BaseDialogStub = defineComponent({
  props: ['show', 'title'],
  template: '<div v-if="show"><h1>{{ title }}</h1><slot /></div>'
})

const SelectStub = defineComponent({
  props: ['modelValue', 'options'],
  emits: ['update:modelValue'],
  template: `
    <select
      id="oauth-capacity-scope"
      :value="modelValue"
      @change="$emit('update:modelValue', ($event.target).value)"
    >
      <option v-for="opt in options" :key="String(opt.value)" :value="opt.value">{{ opt.label }}</option>
    </select>
  `
})

const deferred = <T,>() => {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

function timeseries(groupId: number | undefined, groupName: string, accounts: number, spend: number) {
  return {
    generated_at: '2026-07-15T18:42:00Z',
    now: '2026-07-15T18:42:00Z',
    method: {
      currency: 'USD',
      window: '5h',
      recent_weight: 0.5,
      previous_day_weight: 0.3,
      rpm_weight: 0.2,
      rpm_window_minutes: 15,
      minimum_inference_percent: 5,
      curve: 'seasonal_shape_v1',
      group_allocation: 'actual-window-spend-share',
      past_hours: 12,
      future_hours: 12
    },
    group_id: groupId,
    group_name: groupName,
    health: { plan_type: '', total: accounts, schedulable: accounts - 1, errors: 1, rate_limited: 2 },
    plan_counts: [
      { plan_type: 'k12', total: accounts, schedulable: accounts - 1, errors: 1, rate_limited: 2 }
    ],
    rate_limits: {
      total: 2,
      up_to_10m: 1,
      from_10m_to_30m: 1,
      from_30m_to_1h: 0,
      from_1h_to_3h: 0,
      from_3h_to_5h: 0,
      from_5h_to_1d: 0,
      from_1d_to_3d: 0,
      over_3d: 0
    },
    forecast: {
      recent_three_hour_usd: 30,
      previous_day_same_period_usd: 24,
      recent_rpm_window_usd: 2.5,
      blended_hourly_rate_usd: 9.3,
      rpm_hourly_rate_usd: 10
    },
    summary: {
      spent_usd: spend,
      available_usd: 100 - spend,
      capacity_usd: 100,
      used_percent: spend,
      forecast_remaining_usd: 20,
      projected_cycle_spend_usd: spend + 20,
      projected_shortfall_usd: 0,
      confidence: 'high',
      measured_accounts: accounts - 1,
      capacity_accounts: accounts
    } as const,
    windows: [],
    points: [
      {
        bucket_start: '2026-07-15T17:00:00Z',
        segment: 'past',
        spent_usd: spend / 2,
        forecast_usd: null,
        display_spend_usd: spend / 2,
        available_usd: 70,
        capacity_usd: 100,
        used_percent: 30,
        shortfall_risk_usd: null,
        sealed: true,
        source: 'usage_logs'
      },
      {
        bucket_start: '2026-07-15T18:00:00Z',
        segment: 'current',
        spent_usd: spend / 3,
        forecast_usd: 3,
        display_spend_usd: spend / 3 + 3,
        available_usd: 100 - spend,
        capacity_usd: 100,
        used_percent: spend,
        shortfall_risk_usd: null,
        sealed: false,
        source: 'live'
      },
      {
        bucket_start: '2026-07-15T19:00:00Z',
        segment: 'future',
        spent_usd: null,
        forecast_usd: 9,
        display_spend_usd: 9,
        available_usd: 50,
        capacity_usd: 100,
        used_percent: 50,
        shortfall_risk_usd: null,
        sealed: false,
        source: 'forecast'
      },
      {
        // No data hour — UI must not invent zeros.
        bucket_start: '2026-07-15T16:00:00Z',
        segment: 'past',
        spent_usd: null,
        forecast_usd: null,
        display_spend_usd: null,
        available_usd: null,
        capacity_usd: null,
        used_percent: null,
        shortfall_risk_usd: null,
        sealed: false
      }
    ],
    recommendations: [
      {
        plan_type: 'k12',
        mode: 'alternative',
        seven_day_accounts: 2,
        baseline_source: 'current_cohort',
        baseline_samples: 10
      }
    ],
    baselines: [
      { plan_type: 'k12', window: '5h', median_capacity_usd: 16, samples: 10, source: 'current_cohort' },
      { plan_type: 'k12', window: '7d', median_capacity_usd: 96, samples: 10, source: 'current_cohort' }
    ]
  }
}

describe('OpenAIOAuthCapacityDialog', () => {
  beforeEach(() => {
    getOpenAIOAuthCapacity.mockReset()
    getOpenAIOAuthCapacityTimeseries.mockReset()
    getOpenAIOAuthCapacity.mockResolvedValue({
      generated_at: '2026-07-15T18:42:00Z',
      method: {
        currency: 'USD',
        recent_hours: 3,
        recent_weight: 0.65,
        previous_day_weight: 0.35,
        minimum_inference_percent: 5,
        group_allocation: 'actual-window-spend-share'
      },
      total: {
        group_name: 'All groups',
        accounts: { plan_type: '', total: 12, schedulable: 11, errors: 1, rate_limited: 2 },
        plan_counts: [],
        rate_limits: {
          total: 2,
          up_to_10m: 1,
          from_10m_to_30m: 1,
          from_30m_to_1h: 0,
          from_1h_to_3h: 0,
          from_3h_to_5h: 0,
          from_5h_to_1d: 0,
          from_1d_to_3d: 0,
          over_3d: 0
        },
        forecast: { recent_three_hour_usd: 30, previous_day_same_period_usd: 24, blended_hourly_rate_usd: 9.3 },
        windows: [],
        plans: [],
        recommendations: []
      },
      groups: [
        {
          group_id: 14,
          group_name: 'codex(plus)',
          accounts: { plan_type: '', total: 10, schedulable: 9, errors: 1, rate_limited: 2 },
          plan_counts: [],
          rate_limits: {
            total: 2,
            up_to_10m: 1,
            from_10m_to_30m: 1,
            from_30m_to_1h: 0,
            from_1h_to_3h: 0,
            from_3h_to_5h: 0,
            from_5h_to_1d: 0,
            from_1d_to_3d: 0,
            over_3d: 0
          },
          forecast: { recent_three_hour_usd: 20, previous_day_same_period_usd: 18, blended_hourly_rate_usd: 7 },
          windows: [],
          plans: [],
          recommendations: []
        }
      ],
      baselines: []
    })
    getOpenAIOAuthCapacityTimeseries.mockImplementation(async (params: { group_id?: number } = {}) => {
      if (params.group_id === 14) {
        return timeseries(14, 'codex(plus)', 10, 30)
      }
      return timeseries(undefined, 'All groups', 12, 40)
    })
  })

  it('loads timeseries health, KPI, and chart for the total scope', async () => {
    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: true,
          Select: SelectStub
        }
      }
    })
    await flushPromises()

    expect(getOpenAIOAuthCapacityTimeseries).toHaveBeenCalled()
    expect(wrapper.text()).toContain('12')
    expect(wrapper.text()).toContain('$40.00')
    expect(wrapper.text()).toContain('admin.accounts.oauthCapacity.hourlyTrend')
    expect(wrapper.text()).toContain('admin.accounts.oauthCapacity.persistenceNote')
    expect(wrapper.find('.line-chart-stub').exists()).toBe(true)
  })

  it('reloads timeseries when switching group scope', async () => {
    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: true,
          Select: SelectStub
        }
      }
    })
    await flushPromises()
    await nextTick()

    const select = wrapper.get('#oauth-capacity-scope')
    await select.setValue('group:14')
    await flushPromises()

    expect(getOpenAIOAuthCapacityTimeseries).toHaveBeenCalledWith(
      expect.objectContaining({ group_id: 14, range: '24h', window: '5h' })
    )
    expect(wrapper.text()).toContain('10')
    expect(wrapper.text()).toContain('$30.00')
    expect(wrapper.text()).toContain('$16.00')
    expect(wrapper.text()).toContain('+2')
  })

  it('reloads when the dialog is opened again', async () => {
    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: true,
          Select: SelectStub
        }
      }
    })
    await flushPromises()
    const firstCalls = getOpenAIOAuthCapacityTimeseries.mock.calls.length
    expect(firstCalls).toBeGreaterThanOrEqual(1)

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getOpenAIOAuthCapacityTimeseries.mock.calls.length).toBeGreaterThan(firstCalls)
  })

  it('covers the existing chart with a sized loading overlay while refreshing', async () => {
    const wrapper = mount(OpenAIOAuthCapacityDialog, {
      props: { show: true },
      global: {
        stubs: {
          BaseDialog: BaseDialogStub,
          Icon: true,
          LoadingSpinner: { template: '<span data-test="loading-spinner" />' },
          Select: SelectStub
        }
      }
    })
    await flushPromises()
    const pending = deferred<ReturnType<typeof timeseries>>()
    getOpenAIOAuthCapacityTimeseries.mockReturnValueOnce(pending.promise)

    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await nextTick()

    const chartContainer = wrapper.get('.relative.min-h-48')
    expect(chartContainer.find('.absolute.inset-0').exists()).toBe(true)
    expect(chartContainer.find('[data-test="loading-spinner"]').exists()).toBe(true)

    pending.resolve(timeseries(undefined, 'All groups', 12, 40))
    await flushPromises()
  })
})
