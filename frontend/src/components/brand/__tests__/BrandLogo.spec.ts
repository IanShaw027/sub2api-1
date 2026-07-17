import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import BrandLogo from '../BrandLogo.vue'

describe('BrandLogo', () => {
  it('renders wordmark by default', () => {
    const wrapper = mount(BrandLogo)
    expect(wrapper.text()).toContain('Clomio')
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('hides wordmark when iconOnly', () => {
    const wrapper = mount(BrandLogo, { props: { iconOnly: true } })
    expect(wrapper.text()).not.toContain('Clomio')
    expect(wrapper.find('svg').exists()).toBe(true)
  })

  it('accepts custom wordmark', () => {
    const wrapper = mount(BrandLogo, { props: { wordmark: 'Custom' } })
    expect(wrapper.text()).toContain('Custom')
  })

  it('uses unique gradient ids across instances', () => {
    const a = mount(BrandLogo)
    const b = mount(BrandLogo)
    const idA = a.find('linearGradient').attributes('id')
    const idB = b.find('linearGradient').attributes('id')
    expect(idA).toBeTruthy()
    expect(idB).toBeTruthy()
    expect(idA).not.toBe(idB)
  })
})
