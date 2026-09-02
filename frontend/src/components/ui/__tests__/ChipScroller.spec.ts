import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ChipScroller from '../ChipScroller.vue'
import { readUi } from './source'

describe('ChipScroller', () => {
  it('renders 30px chips with radius 10 and active inversion', async () => {
    const wrapper = mount(ChipScroller, {
      props: {
        modelValue: 'all',
        chips: [
          { value: 'all', label: 'All 5' },
          { value: 'live', label: 'Active 3' }
        ]
      }
    })
    expect(wrapper.find('.ui-chip-active').text()).toBe('All 5')
    await wrapper.findAll('.ui-chip')[1].trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['live'])
    const src = readUi('ChipScroller.vue')
    expect(src).toContain('height: 30px')
    expect(src).toContain('border-radius: 10px')
  })
})
