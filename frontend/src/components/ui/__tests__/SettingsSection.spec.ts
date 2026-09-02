import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SettingsSection from '../SettingsSection.vue'

describe('SettingsSection', () => {
  it('wraps rows in a glass card', () => {
    const wrapper = mount(SettingsSection, {
      props: { title: 'General', description: 'Site identity' },
      slots: { default: '<div class="row">row</div>' }
    })
    expect(wrapper.classes()).toContain('glass-card')
    expect(wrapper.text()).toContain('General')
    expect(wrapper.text()).toContain('Site identity')
    expect(wrapper.find('.row').exists()).toBe(true)
  })
})
