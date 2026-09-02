import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Button from '../Button.vue'
import { readUi, styleCss, tokensCss } from './source'

describe('Button', () => {
  it('uses glass primary at 34px by default', () => {
    const wrapper = mount(Button, { slots: { default: 'Save' } })
    expect(wrapper.classes()).toContain('btn-glass-primary')
    expect(wrapper.text()).toBe('Save')
    expect(styleCss).toMatch(/\.btn-glass-primary[\s\S]*?height:\s*34px/)
    expect(styleCss).toMatch(/\.btn-glass-primary[\s\S]*?border-radius:\s*var\(--radius-btn\)/)
    expect(tokensCss).toContain('--radius-btn: 10px')
  })

  it('applies 42px md size and remaining variants', () => {
    expect(readUi('Button.vue')).toContain('height: 42px')
    expect(mount(Button, { props: { size: 'md' } }).classes()).toContain('ui-btn-md')
    expect(mount(Button, { props: { variant: 'secondary' } }).classes()).toContain('btn-glass-secondary')
    expect(mount(Button, { props: { variant: 'ghost' } }).classes()).toContain('ui-btn-ghost')
    expect(mount(Button, { props: { variant: 'danger' } }).classes()).toContain('ui-btn-danger')
    expect(mount(Button, { props: { variant: 'icon' } }).classes()).toContain('ui-btn-icon')
    expect(readUi('Button.vue')).toContain('width: 34px')
  })

  it('blocks clicks while loading or disabled', async () => {
    const loading = mount(Button, { props: { loading: true }, slots: { default: 'Go' } })
    expect(loading.attributes('aria-busy')).toBe('true')
    expect(loading.attributes('disabled')).toBeDefined()
    await loading.trigger('click')
    expect(loading.emitted('click')).toBeUndefined()

    const disabled = mount(Button, { props: { disabled: true } })
    await disabled.trigger('click')
    expect(disabled.emitted('click')).toBeUndefined()
  })
})
