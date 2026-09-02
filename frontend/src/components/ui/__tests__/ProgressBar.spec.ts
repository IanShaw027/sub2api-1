import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ProgressBar from '../ProgressBar.vue'
import { readUi } from './source'

describe('ProgressBar', () => {
  it('uses warning at 70% and danger at 90%', () => {
    expect(mount(ProgressBar, { props: { value: 69 } }).find('.ui-progress-accent').exists()).toBe(true)
    expect(mount(ProgressBar, { props: { value: 70 } }).find('.ui-progress-warning').exists()).toBe(true)
    expect(mount(ProgressBar, { props: { value: 90 } }).find('.ui-progress-danger').exists()).toBe(true)
  })

  it('clamps width and uses a 6px track', () => {
    const wrapper = mount(ProgressBar, { props: { value: 140, showLabel: true } })
    expect(wrapper.find('.ui-progress-fill').attributes('style')).toContain('100%')
    expect(wrapper.text()).toContain('100%')
    expect(readUi('ProgressBar.vue')).toContain('height: 6px')
  })
})
