import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import MonoCell from '../MonoCell.vue'

describe('MonoCell', () => {
  it('renders the value in a mono span', () => {
    const wrapper = mount(MonoCell, { props: { value: 'sk-abc123' } })
    expect(wrapper.find('.cell-mono').text()).toBe('sk-abc123')
  })

  it('supports a default slot override', () => {
    const wrapper = mount(MonoCell, {
      slots: { default: '<b>custom</b>' }
    })
    expect(wrapper.find('.cell-mono').html()).toContain('<b>custom</b>')
  })
})
