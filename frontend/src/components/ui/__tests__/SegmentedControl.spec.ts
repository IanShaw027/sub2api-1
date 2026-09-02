import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SegmentedControl from '../SegmentedControl.vue'
import { styleCss } from './source'

describe('SegmentedControl', () => {
  const options = [
    { value: 'all', label: 'All' },
    { value: 'on', label: 'On' },
    { value: 'off', label: 'Off', disabled: true }
  ]

  it('marks the active item and emits changes', async () => {
    const wrapper = mount(SegmentedControl, {
      props: { modelValue: 'all', options }
    })
    expect(wrapper.classes()).toContain('segmented')
    expect(wrapper.find('.segmented-item-active').text()).toBe('All')
    await wrapper.findAll('.segmented-item')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['on'])
  })

  it('ignores disabled options and uses 36px track', async () => {
    const wrapper = mount(SegmentedControl, {
      props: { modelValue: 'all', options }
    })
    await wrapper.findAll('.segmented-item')[2].trigger('click')
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
    expect(styleCss).toMatch(/\.segmented \{[\s\S]*?height:\s*36px/)
  })
})
