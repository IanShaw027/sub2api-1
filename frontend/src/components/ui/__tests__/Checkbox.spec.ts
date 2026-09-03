import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Checkbox from '../Checkbox.vue'
import { readUi } from './source'

describe('Checkbox', () => {
  it('emits updates and renders a label slot', async () => {
    const wrapper = mount(Checkbox, {
      props: { modelValue: false },
      slots: { default: 'Accept' }
    })
    expect(wrapper.text()).toContain('Accept')
    await wrapper.find('input').setValue(true)
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([true])
  })

  it('uses a 16px token-colored box', () => {
    const src = readUi('Checkbox.vue')
    expect(src).toContain('width: 16px')
    expect(src).toContain('height: 16px')
    expect(src).toContain('var(--accent)')
    expect(src).toContain('var(--field-shadow)')
  })
})
