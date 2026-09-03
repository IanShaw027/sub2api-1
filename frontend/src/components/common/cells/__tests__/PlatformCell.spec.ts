import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PlatformCell from '../PlatformCell.vue'

describe('PlatformCell', () => {
  it('renders the resolved label and a brand tile', () => {
    const wrapper = mount(PlatformCell, { props: { platform: 'anthropic' } })
    expect(wrapper.text()).toContain('Claude')
    expect(wrapper.find('.cell-platform-tile').exists()).toBe(true)
  })

  it('accepts a label override', () => {
    const wrapper = mount(PlatformCell, { props: { platform: 'anthropic', label: 'Custom' } })
    expect(wrapper.text()).toContain('Custom')
  })

  it('falls back to a neutral tile for unknown platforms', () => {
    const wrapper = mount(PlatformCell, { props: { platform: 'unknown-platform' } })
    expect(wrapper.find('.cell-platform-tile').attributes('style')).toContain('surface-tertiary')
  })
})
