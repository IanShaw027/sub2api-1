import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import TextInput from '../TextInput.vue'
import { styleCss, tokensCss } from './source'

describe('TextInput', () => {
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

    const withSlot = mount(TextInput, {
      props: { modelValue: '' },
      slots: { error: 'Slot error' }
    })
    expect(withSlot.text()).toContain('Slot error')
  })
})
