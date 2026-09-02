import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import ToggleSwitch from '../ToggleSwitch.vue'
import { readUi } from './source'

describe('ToggleSwitch', () => {
  it('renders compact 32x18 and form 36x20 tracks', () => {
    const compact = mount(ToggleSwitch, { props: { modelValue: true, size: 'compact' } })
    expect(compact.find('.ui-toggle-compact').exists()).toBe(true)
    const form = mount(ToggleSwitch, { props: { modelValue: false, size: 'form' } })
    expect(form.find('.ui-toggle-form').exists()).toBe(true)
    const src = readUi('ToggleSwitch.vue')
    expect(src).toMatch(/width:\s*32px/)
    expect(src).toMatch(/height:\s*18px/)
    expect(src).toMatch(/width:\s*36px/)
    expect(src).toMatch(/height:\s*20px/)
  })

  it('toggles and respects disabled', async () => {
    const wrapper = mount(ToggleSwitch, { props: { modelValue: false } })
    expect(wrapper.attributes('aria-checked')).toBe('false')
    await wrapper.trigger('click')
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual([true])

    const disabled = mount(ToggleSwitch, { props: { modelValue: false, disabled: true } })
    await disabled.trigger('click')
    expect(disabled.emitted('update:modelValue')).toBeUndefined()
  })
})
