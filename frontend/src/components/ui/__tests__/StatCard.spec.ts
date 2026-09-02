import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatCard from '../StatCard.vue'
import { readUi } from './source'

describe('StatCard', () => {
  it('renders label, value, sub, delta, and sparkline', () => {
    const wrapper = mount(StatCard, {
      props: { label: 'RPM', value: 842, sub: 'vs yesterday', delta: '+12%', deltaTone: 'up' },
      slots: { sparkline: '<svg class="spark" />' }
    })
    expect(wrapper.text()).toContain('RPM')
    expect(wrapper.text()).toContain('842')
    expect(wrapper.text()).toContain('vs yesterday')
    expect(wrapper.text()).toContain('+12%')
    expect(wrapper.find('.ui-stat-card-delta-up').exists()).toBe(true)
    expect(wrapper.find('.spark').exists()).toBe(true)
    expect(wrapper.classes()).toContain('glass-card')
  })

  it('uses 24px tabular numbers', () => {
    const src = readUi('StatCard.vue')
    expect(src).toContain('font-size: 24px')
    expect(src).toContain('font-variant-numeric: tabular-nums')
  })
})
