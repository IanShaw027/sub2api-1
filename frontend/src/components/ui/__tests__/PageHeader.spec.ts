import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PageHeader from '../PageHeader.vue'
import { readUi } from './source'

describe('PageHeader', () => {
  it('renders compact title, description, and actions', () => {
    const wrapper = mount(PageHeader, {
      props: { title: 'API Keys', description: 'Manage keys' },
      slots: { actions: '<button>Create</button>' }
    })
    expect(wrapper.classes()).toContain('ui-page-header-compact')
    expect(wrapper.get('h1').text()).toBe('API Keys')
    expect(wrapper.text()).toContain('Manage keys')
    expect(wrapper.find('.ui-page-header-actions').exists()).toBe(true)
  })

  it('uses 24px compact and 30px hero titles', () => {
    const hero = mount(PageHeader, { props: { title: 'Overview', variant: 'hero' } })
    expect(hero.classes()).toContain('ui-page-header-hero')
    const src = readUi('PageHeader.vue')
    expect(src).toContain('font-size: 24px')
    expect(src).toContain('font-size: 30px')
  })
})
