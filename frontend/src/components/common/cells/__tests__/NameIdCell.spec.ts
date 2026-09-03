import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import NameIdCell from '../NameIdCell.vue'

describe('NameIdCell', () => {
  it('renders name, id and meta', () => {
    const wrapper = mount(NameIdCell, { props: { name: 'Acme', id: 42, meta: 'default' } })
    expect(wrapper.find('.cell-name-id-name').text()).toBe('Acme')
    expect(wrapper.find('.cell-name-id-meta').text()).toBe('#42 · default')
  })

  it('renders an email line when provided', () => {
    const wrapper = mount(NameIdCell, { props: { name: 'Acme', id: 1, email: 'a@b.com' } })
    expect(wrapper.find('.cell-name-id-email').text()).toBe('a@b.com')
  })

  it('renders the name as a link when href is set', () => {
    const wrapper = mount(NameIdCell, { props: { name: 'Acme', id: 1, href: 'https://example.com' } })
    const anchor = wrapper.find('a.cell-name-id-name')
    expect(anchor.exists()).toBe(true)
    expect(anchor.attributes('href')).toBe('https://example.com')
  })
})
