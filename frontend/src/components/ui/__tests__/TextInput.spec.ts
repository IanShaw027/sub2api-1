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
    expect(wrapper.get('input').attributes('aria-invalid')).toBe('true')
  })

  it('preserves native attributes and hint associations while errors change', async () => {
    const wrapper = mount(TextInput, {
      props: { id: 'password', error: 'Required' },
      attrs: {
        name: 'password',
        autofocus: true,
        class: 'input-lg',
        'aria-describedby': 'password-hint'
      },
      slots: { label: 'Password <span>(optional)</span>' }
    })
    const input = wrapper.get('input')
    expect(input.attributes('name')).toBe('password')
    expect(input.element.autofocus).toBe(true)
    expect(input.classes()).toContain('input-lg')
    expect(input.attributes('aria-describedby')).toBe('password-hint password-error')
    expect(wrapper.get('label').attributes('for')).toBe('password')
    expect(wrapper.get('label').text()).toContain('(optional)')
    expect(wrapper.get('#password-error').attributes('role')).toBe('alert')

    await wrapper.setProps({ error: '' })
    expect(input.attributes('aria-describedby')).toBe('password-hint')
    expect(input.attributes('aria-invalid')).toBeUndefined()
    await wrapper.setProps({ 'aria-describedby': 'updated-hint' })
    expect(input.attributes('aria-describedby')).toBe('updated-hint')
  })

  it('updates the model before forwarding the native input event', async () => {
    let value: string | number = ''
    let valueDuringInput: string | number = ''
    let receivedEvent: Event | undefined
    const wrapper = mount(TextInput, {
      props: {
        'onUpdate:modelValue': (next) => { value = next },
        onInput: (event) => {
          valueDuringInput = value
          receivedEvent = event
        }
      }
    })
    await wrapper.get('input').setValue('INVITE-NEW')
    expect(valueDuringInput).toBe('INVITE-NEW')
    expect(receivedEvent?.target).toBe(wrapper.get('input').element)
    expect(wrapper.emitted('input')).toHaveLength(1)
  })

  it('renders a suffix without changing the input id or native attributes', () => {
    const wrapper = mount(TextInput, {
      props: { modelValue: 'secret', id: 'password', type: 'password', autocomplete: 'current-password' },
      slots: { suffix: '<button type="button" aria-label="Show password">eye</button>' }
    })
    const input = wrapper.get('input')
    expect(input.attributes('id')).toBe('password')
    expect(input.attributes('autocomplete')).toBe('current-password')
    expect(wrapper.get('[aria-label="Show password"]').exists()).toBe(true)
    expect(wrapper.get('.ui-text-input-control.has-suffix').exists()).toBe(true)
  })
})
