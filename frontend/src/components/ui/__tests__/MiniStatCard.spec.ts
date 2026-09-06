import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MiniStatCard from '../MiniStatCard.vue'
import { readUi } from './source'

describe('MiniStatCard', () => {
  it('renders a single mini stat', () => {
    const wrapper = mount(MiniStatCard, { props: { label: 'Balance', value: '$142.6' } })
    expect(wrapper.text()).toContain('Balance')
    expect(wrapper.text()).toContain('$142.6')
    expect(wrapper.classes()).toContain('glass-card')
  })

  it('renders a 3-column group with 22px values', () => {
    const wrapper = mount(MiniStatCard, {
      props: {
        items: [
          { label: 'Keys', value: 12 },
          { label: 'Active', value: 10 },
          { label: 'Today', value: '$10.52' }
        ]
      }
    })
    expect(wrapper.find('.ui-mini-stat-grid').exists()).toBe(true)
    expect(wrapper.text()).toContain('Keys')
    expect(wrapper.text()).toContain('10')
    const src = readUi('MiniStatCard.vue')
    expect(src).toContain('repeat(3, minmax(0, 1fr))')
    expect(src).toContain('font-size: 22px')
    expect(src).toContain('padding: 10px 12px')
  })
})
