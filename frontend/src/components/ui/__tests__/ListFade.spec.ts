import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ListFade from '../ListFade.vue'
import { readUi } from './source'

describe('ListFade', () => {
  it('renders a 120px bottom gradient', () => {
    const wrapper = mount(ListFade)
    expect(wrapper.classes()).toContain('ui-list-fade')
    expect(wrapper.attributes('aria-hidden')).toBe('true')
    const src = readUi('ListFade.vue')
    expect(src).toContain('height: 120px')
    expect(src).toContain('linear-gradient(180deg, transparent, var(--background) 70%)')
  })
})
