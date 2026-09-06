import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import type { UserDashboardStats as UserStats } from '@/api/usage'
import StatCard from '@/components/ui/StatCard.vue'
import UserDashboardStats from '../UserDashboardStats.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('UserDashboardStats metric completeness', () => {
  it('keeps standard costs, cache tokens and TPM alongside actual usage', () => {
    const wrapper = mount(UserDashboardStats, {
      props: {
        balance: 10,
        isSimple: false,
        liveRpmUsed: 7,
        liveRpmLimit: 20,
        currentConcurrency: 3,
        stats: {
          today_actual_cost: 1.25,
          today_cost: 2.5,
          total_actual_cost: 12.5,
          total_cost: 25,
          today_input_tokens: 100,
          today_output_tokens: 200,
          today_cache_creation_tokens: 300,
          today_cache_read_tokens: 400,
          total_input_tokens: 200,
          total_output_tokens: 300,
          total_cache_creation_tokens: 400,
          total_cache_read_tokens: 500,
          rpm: 4,
          tpm: 1500
        } as UserStats
      }
    })
    const card = (label: string) => wrapper.findAllComponents(StatCard)
      .find(item => item.props('label') === label)!

    const cost = card('dashboard.todayCost')
    expect(cost.props('value')).toBe('$1.2500')
    expect(cost.text()).toContain('dashboard.standard: $2.5000')
    expect(cost.text()).toContain('common.total (dashboard.actual): $12.5000')
    expect(cost.text()).toContain('common.total (dashboard.standard): $25.0000')
    expect(card('dashboard.todayTokens').text()).toContain('dashboard.cache: 700')
    expect(card('dashboard.totalTokens').text()).toContain('dashboard.cache: 900')
    const performance = card('dashboard.performance')
    expect(performance.props('value')).toBe(7)
    expect(performance.text()).toContain('RPM 7/20')
    expect(performance.text()).toContain('4 dashboard.avgRpm')
    expect(performance.text()).toContain('3 dashboard.currentConcurrency')
    expect(performance.text()).toContain('1.5K TPM')
    expect(performance.find('.ui-stat-card-sparkline').exists()).toBe(false)
    wrapper.unmount()
  })

  it('renders zero for absent metrics and retains the balance history action', async () => {
    const wrapper = mount(UserDashboardStats, {
      props: { balance: 0, isSimple: false, stats: {} as UserStats }
    })
    expect(wrapper.text()).toContain('dashboard.cache: 0')
    expect(wrapper.text()).toContain('0 TPM')
    expect(wrapper.text()).not.toMatch(/NaN|undefined/)
    await wrapper.findAllComponents(StatCard)[0]!.trigger('click')
    expect(wrapper.emitted('balance-history')).toHaveLength(1)
    wrapper.unmount()
  })
})
