import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import UsageWindowCell from '../UsageWindowCell.vue'

describe('UsageWindowCell', () => {
  it('renders one row per window with label and percent', () => {
    const wrapper = mount(UsageWindowCell, {
      props: { windows: [{ label: '5h', percent: 42 }, { label: '7d', percent: 91 }] }
    })
    const rows = wrapper.findAll('.cell-usage-window-row')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('5h')
    expect(rows[0].text()).toContain('42%')
  })

  it('applies the danger tone at/above the danger threshold', () => {
    const wrapper = mount(UsageWindowCell, { props: { windows: [{ label: '7d', percent: 92 }] } })
    expect(wrapper.find('.progress-bar').classes()).toContain('progress-bar-danger')
  })

  it('applies the warning tone at/above the warning threshold', () => {
    const wrapper = mount(UsageWindowCell, { props: { windows: [{ label: '7d', percent: 75 }] } })
    expect(wrapper.find('.progress-bar').classes()).toContain('progress-bar-warning')
  })

  it('renders a dash for a missing percent', () => {
    const wrapper = mount(UsageWindowCell, { props: { windows: [{ label: '5h', percent: null }] } })
    expect(wrapper.find('.cell-usage-window-value').text()).toBe('-')
  })
})
