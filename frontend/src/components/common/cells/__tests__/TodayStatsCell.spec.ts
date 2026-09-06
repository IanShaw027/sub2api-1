import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TodayStatsCell from '../TodayStatsCell.vue'

describe('TodayStatsCell', () => {
  it('formats count, tokens and cost', () => {
    const wrapper = mount(TodayStatsCell, {
      props: { count: 1284, unit: '次', tokens: 18_200_000, cost: 12.4 }
    })
    expect(wrapper.find('.cell-today-stats-primary').text()).toBe('1,284 次')
    expect(wrapper.find('.cell-today-stats-secondary').text()).toContain('18.2M')
    expect(wrapper.find('.cell-today-stats-secondary').text()).toContain('$12.40')
  })

  it('defaults to zero when no count is given', () => {
    const wrapper = mount(TodayStatsCell, { props: {} })
    expect(wrapper.find('.cell-today-stats-primary').text()).toBe('0')
  })
})
