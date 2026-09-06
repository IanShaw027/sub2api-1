import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import LocaleSwitcher from '../LocaleSwitcher.vue'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ locale: { value: 'en' } }) }))
vi.mock('@/i18n', () => ({
  availableLocales: [{ code: 'en', name: 'English', flag: 'EN' }, { code: 'zh', name: 'Chinese', flag: 'ZH' }],
  setLocale: vi.fn().mockResolvedValue(undefined),
}))

afterEach(() => { document.body.innerHTML = '' })

describe('LocaleSwitcher keyboard interaction', () => {
  it('labels compact actions, focuses options and restores only its own trigger on Escape', async () => {
    const wrapper = mount(LocaleSwitcher, { attachTo: document.body, props: { menuMode: true } })
    const trigger = wrapper.get('button')
    expect(trigger.text()).toContain('English')
    expect(trigger.attributes('aria-expanded')).toBe('false')
    await trigger.trigger('click')
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('true')
    expect(document.activeElement).toBe(wrapper.get('.dropdown-item').element)
    const parentEscape = vi.fn()
    document.addEventListener('keydown', parentEscape)
    document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await flushPromises()
    expect(trigger.attributes('aria-expanded')).toBe('false')
    expect(document.activeElement).toBe(trigger.element)
    expect(parentEscape).not.toHaveBeenCalled()
    document.activeElement?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(parentEscape).toHaveBeenCalledOnce()
    document.removeEventListener('keydown', parentEscape)
    wrapper.unmount()
  })
})
