import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import DashboardView from '../DashboardView.vue'

const { auth, rpmStatus } = vi.hoisted(() => ({
  auth: {
    isSimpleMode: false,
    user: { balance: 10, email: 'user@example.com' },
    refreshUser: vi.fn().mockResolvedValue(undefined)
  },
  rpmStatus: vi.fn()
}))

vi.mock('@/stores/auth', () => ({ useAuthStore: () => auth }))
vi.mock('vue-router', () => ({ useRouter: () => ({ push: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))
vi.mock('@/api/usage', () => ({
  usageAPI: {
    getDashboardStats: vi.fn().mockResolvedValue({
      total_api_keys: 2, active_api_keys: 1, today_requests: 3,
      average_duration_ms: 100, rpm: 4
    }),
    getDashboardTrend: vi.fn().mockResolvedValue({ trend: [] }),
    getDashboardModels: vi.fn().mockResolvedValue({ models: [] }),
    getByDateRange: vi.fn().mockResolvedValue({ items: [] })
  }
}))
vi.mock('@/api/user', () => ({
  getMyRPMStatus: rpmStatus,
  getMyPlatformQuotas: vi.fn().mockResolvedValue({ platform_quotas: [] })
}))

async function mountDashboard() {
  const wrapper = shallowMount(DashboardView, {
    global: { stubs: { AppLayout: { template: '<main><slot /></main>' } } }
  })
  await flushPromises()
  return wrapper
}

describe('dashboard hero metric migration', () => {
  beforeEach(() => {
    auth.isSimpleMode = false
    rpmStatus.mockResolvedValue({ user_rpm_used: 7, user_rpm_limit: 20, current_concurrency: 3 })
  })

  it('retains live RPM, concurrency and the balance history action in the hero', async () => {
    const wrapper = await mountDashboard()
    const metrics = wrapper.findAll('.dash-hero-mini')
    expect(metrics[0].text()).toContain('$10.00')
    expect(metrics[1].get('.dash-mini-value').text()).toBe('7')
    expect(metrics[1].text()).toContain('RPM 7/20')
    expect(metrics[2].get('.dash-mini-value').text()).toBe('3')
    const history = wrapper.getComponent({ name: 'UserBalanceHistoryModal' })
    expect(history.props('show')).toBe(false)
    await metrics[0].trigger('click')
    expect(history.props('show')).toBe(true)
    history.vm.$emit('close')
    await flushPromises()
    expect(history.props('show')).toBe(false)
    wrapper.unmount()
  })

  it('keeps simple mode free of balance actions and uses zero for missing live metrics', async () => {
    auth.isSimpleMode = true
    rpmStatus.mockResolvedValue({})
    const wrapper = await mountDashboard()
    const metrics = wrapper.findAll('.dash-hero-mini')
    expect(metrics[0].text()).toContain('dashboard.apiKeys')
    expect(metrics[0].text()).not.toContain('dashboard.balance')
    expect(metrics[1].get('.dash-mini-value').text()).toBe('0')
    expect(metrics[1].text()).toContain('4 dashboard.avgRpm')
    expect(metrics[2].get('.dash-mini-value').text()).toBe('0')
    await metrics[0].trigger('click')
    expect(wrapper.getComponent({ name: 'UserBalanceHistoryModal' }).props('show')).toBe(false)
    wrapper.unmount()
  })
})
