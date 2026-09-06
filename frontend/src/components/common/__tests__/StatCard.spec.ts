import { mount } from '@vue/test-utils'
import { markRaw } from 'vue'
import { describe, expect, it } from 'vitest'
import StatCard from '../StatCard.vue'
import UiStatCard from '@/components/ui/StatCard.vue'

describe('legacy StatCard adapter', () => {
  it('preserves formatting, icon, trend and attribute hooks through the shared renderer', () => {
    const wrapper = mount(StatCard, {
      props: { title: 'Balance', value: 1234, formatValue: value => `$${value}`, icon: markRaw({ template: '<svg data-testid="icon" />' }), iconVariant: 'success', change: -12, changeType: 'down' },
      attrs: { 'data-testid': 'balance-card' }
    })
    expect(wrapper.findComponent(UiStatCard).exists()).toBe(true)
    expect(wrapper.attributes('data-testid')).toBe('balance-card')
    expect(wrapper.get('.stat-label').text()).toBe('Balance')
    expect(wrapper.get('.stat-value').text()).toBe('$1234')
    expect(wrapper.get('.stat-value').attributes('title')).toBe('$1234')
    expect(wrapper.get('.stat-icon-success [data-testid="icon"]').exists()).toBe(true)
    expect(wrapper.get('.stat-trend-down').text()).toBe('12%')
    expect(wrapper.get('.rotate-180').exists()).toBe(true)
  })

  it('retains numeric locale formatting and zero changes', () => {
    const wrapper = mount(StatCard, { props: { title: 'Requests', value: 1234, change: 0 } })
    expect(wrapper.get('.stat-value').text()).toBe((1234).toLocaleString())
    expect(wrapper.get('.stat-trend').text()).toBe('0%')
    expect(wrapper.find('.stat-trend svg').exists()).toBe(false)
  })
})
