import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TextInput from '../TextInput.vue'
import { styleCss, tokensCss } from './source'

describe('TextInput', () => {
  it('forwards required to native constraint validation and updates it reactively', async () => {
    const wrapper = mount(TextInput, { props: { modelValue: '', required: true, label: 'Name' } })
    const input = wrapper.get('input').element
    expect(input.required).toBe(true)
    expect(input.validity.valueMissing).toBe(true)
    expect(input.checkValidity()).toBe(false)

    await wrapper.setProps({ modelValue: 'Name' })
    expect(input.checkValidity()).toBe(true)
    await wrapper.setProps({ modelValue: '', required: false })
    expect(input.required).toBe(false)
    expect(input.checkValidity()).toBe(true)
    wrapper.unmount()
  })

  it('wraps the field class at 36px', async () => {
    const wrapper = mount(TextInput, {
      props: { modelValue: 'alpha', label: 'Name', placeholder: 'Type' }
    })
    const input = wrapper.get('input.field')
    expect(input.element.value).toBe('alpha')
    expect(styleCss).toMatch(/\.field[\s\S]*?height:\s*36px/)
    expect(styleCss).toMatch(/\.field[\s\S]*?border-radius:\s*var\(--radius-field\)/)
    expect(tokensCss).toContain('--radius-field: 12px')

    await input.setValue('beta')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['beta'])
  })

  it('renders error prop and error slot', () => {
    const withProp = mount(TextInput, { props: { modelValue: '', error: 'Required' } })
    expect(withProp.text()).toContain('Required')
    expect(withProp.get('input').attributes('aria-invalid')).toBe('true')
    expect(withProp.get('input').attributes('aria-describedby')).toBe(
      `${withProp.get('input').attributes('id')}-error`
    )

    const withSlot = mount(TextInput, {
      props: { modelValue: '' },
      slots: { error: 'Slot error' }
    })
    expect(withSlot.text()).toContain('Slot error')
  })

  it('emits an empty string when a number input is cleared', async () => {
    const wrapper = mount(TextInput, { props: { modelValue: 5, type: 'number' } })
    await wrapper.get('input').setValue('')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([''])
  })

  it('sets aria-describedby when an error slot is provided', () => {
    const wrapper = mount(TextInput, {
      props: { modelValue: '', id: 'qty' },
      slots: { error: 'Slot error' }
    })
    expect(wrapper.get('input').attributes('aria-describedby')).toBe('qty-error')
    expect(wrapper.get('#qty-error').text()).toBe('Slot error')
  })
})
