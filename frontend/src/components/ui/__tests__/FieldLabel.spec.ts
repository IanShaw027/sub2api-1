import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import FieldLabel from '../FieldLabel.vue'
import { readUi } from './source'

describe('FieldLabel', () => {
  it('renders label, hint, and required marker', () => {
    const wrapper = mount(FieldLabel, {
      props: { htmlFor: 'name', hint: 'Shown in emails', required: true },
      slots: { default: 'Site name' }
    })
    expect(wrapper.attributes('for')).toBe('name')
    expect(wrapper.text()).toContain('Site name')
    expect(wrapper.text()).toContain('Shown in emails')
    expect(wrapper.find('.ui-field-label-required').exists()).toBe(true)
  })

  it('uses 13px label type', () => {
    expect(readUi('FieldLabel.vue')).toContain('font-size: 13px')
  })
})
