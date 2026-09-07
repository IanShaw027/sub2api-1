import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import AdminPaymentDashboardView from '../orders/AdminPaymentDashboardView.vue'

const { getDashboard, showError } = vi.hoisted(() => ({ getDashboard: vi.fn(), showError: vi.fn() }))
vi.mock('@/api/admin/payment', () => ({ adminPaymentAPI: { getDashboard }, default: { getDashboard } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError }) }))
vi.mock('vue-i18n', async () => ({
  ...await vi.importActual<typeof import('vue-i18n')>('vue-i18n'),
  useI18n: () => ({ t: (key: string) => key })
}))

describe('payment dashboard loading', () => {
  beforeEach(() => {
    getDashboard.mockReset()
    showError.mockReset()
  })

  it('replaces the initial skeleton with data and retains that data during refresh', async () => {
    let resolveRequest!: (value: unknown) => void
    getDashboard.mockImplementation(() => new Promise(resolve => { resolveRequest = resolve }))
    const wrapper = shallowMount(AdminPaymentDashboardView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          PageHeader: { template: '<header><slot name="actions" /></header>' }
        }
      }
    })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(true)
    resolveRequest({ data: { daily_series: [], payment_methods: [], top_users: {} } })
    await flushPromises()
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'OrderStatsCards' }).exists()).toBe(true)
    wrapper.findComponent({ name: 'Button' }).vm.$emit('click')
    await flushPromises()
    expect(getDashboard).toHaveBeenCalledTimes(2)
    expect(wrapper.findComponent({ name: 'DashboardSkeleton' }).exists()).toBe(false)
    expect(wrapper.findComponent({ name: 'OrderStatsCards' }).exists()).toBe(true)
    resolveRequest({ data: { daily_series: [], payment_methods: [], top_users: {} } })
    await flushPromises()
    wrapper.unmount()
  })
})
