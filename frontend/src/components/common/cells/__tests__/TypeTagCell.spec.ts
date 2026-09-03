import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TypeTagCell from '../TypeTagCell.vue'

describe('TypeTagCell', () => {
  it('renders the base .tag class by default', () => {
    const wrapper = mount(TypeTagCell, { props: { label: 'OAuth' } })
    expect(wrapper.classes()).toContain('tag')
    expect(wrapper.classes()).not.toContain('tag-accent')
    expect(wrapper.text()).toBe('OAuth')
  })

  it('applies a tone modifier class', () => {
    const wrapper = mount(TypeTagCell, { props: { label: 'API Key', tone: 'accent' } })
    expect(wrapper.classes()).toContain('tag-accent')
  })
})
