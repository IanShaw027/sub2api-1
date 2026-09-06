import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TimeCell from '../TimeCell.vue'

describe('TimeCell', () => {
  it('renders a relative time with the absolute time as the title', () => {
    const past = new Date(Date.now() - 2 * 60 * 60 * 1000).toISOString()
    const wrapper = mount(TimeCell, { props: { value: past } })
    expect(wrapper.find('.cell-time').attributes('title')).toBeTruthy()
    expect(wrapper.text().length).toBeGreaterThan(0)
  })

  it('renders the absolute time when mode is absolute', () => {
    const value = '2026-01-02T03:04:00Z'
    const wrapper = mount(TimeCell, { props: { value, mode: 'absolute' } })
    expect(wrapper.text()).toBe(wrapper.attributes('title'))
  })
})
