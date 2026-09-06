import { flushPromises, mount } from '@vue/test-utils'
import { createMemoryHistory, createRouter } from 'vue-router'
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

  it('applies 42px lg size and remaining variants', () => {
    expect(styleCss).toMatch(/\.btn-lg,[\s\S]*?height:\s*42px/)
    expect(mount(Button, { props: { size: 'lg' } }).classes()).toContain('ui-btn-lg')
    expect(mount(Button, { props: { variant: 'secondary' } }).classes()).toContain('btn-glass-secondary')
    expect(mount(Button, { props: { variant: 'ghost' } }).classes()).toContain('ui-btn-ghost')
    expect(mount(Button, { props: { variant: 'danger' } }).classes()).toContain('ui-btn-danger')
    expect(mount(Button, { props: { variant: 'success' } }).classes()).toContain('btn-success')
    expect(mount(Button, { props: { variant: 'warning' } }).classes()).toContain('btn-warning')
    expect(mount(Button, { props: { variant: 'icon' } }).classes()).toContain('ui-btn-icon')
    expect(styleCss).toMatch(/\.btn-icon\s*\{[\s\S]*?width:\s*34px/)
    expect(readUi('Button.vue')).not.toContain('<style')
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

  it.each([{ disabled: true }, { loading: true }, {}])('preserves router navigation and guards %j', async (state) => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }, { path: '/next', component: { template: '<div />' } }]
    })
    await router.push('/')
    await router.isReady()
    const wrapper = mount(Button, { props: { to: '/next', ...state }, global: { plugins: [router] } })
    expect(wrapper.element.tagName).toBe('A')
    expect(wrapper.attributes('href')).toBe('/next')
    await wrapper.trigger('click')
    await flushPromises()
    const blocked = 'disabled' in state || 'loading' in state
    expect(router.currentRoute.value.path).toBe(blocked ? '/' : '/next')
    expect(Boolean(wrapper.emitted('click'))).toBe(!blocked)
    wrapper.unmount()
  })

  it('preserves external links and cancels their default action when disabled', async () => {
    const wrapper = mount(Button, { props: { href: 'https://example.com', disabled: true } })
    expect(wrapper.element.tagName).toBe('A')
    expect(wrapper.attributes('href')).toBe('https://example.com')
    expect(wrapper.attributes('tabindex')).toBe('-1')
    const event = new MouseEvent('click', { bubbles: true, cancelable: true })
    wrapper.element.dispatchEvent(event)
    expect(event.defaultPrevented).toBe(true)
    expect(wrapper.emitted('click')).toBeUndefined()
    wrapper.unmount()
  })
})
