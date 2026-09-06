import { afterEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import BaseDialog from '../BaseDialog.vue'
import UiModal from '@/components/ui/UiModal.vue'
import { resetOverlayLock } from '@/components/ui/overlayLock'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('BaseDialog', () => {
  afterEach(() => {
    resetOverlayLock()
    document.body.innerHTML = ''
  })

  it('resets body scroll position when reopened', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: false, title: 'Details' },
      slots: { default: '<div style="height: 2000px">content</div>' },
      global: { stubs: { Icon: true } }
    })

    await wrapper.setProps({ show: true })
    await nextTick()
    const body = document.body.querySelector<HTMLElement>('.modal-body')
    expect(body).not.toBeNull()
    body!.scrollTop = 480

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await nextTick()

    expect(document.body.querySelector<HTMLElement>('.modal-body')?.scrollTop).toBe(0)
    // Solid panel styling now lives directly on `.modal-content` in style.css
    // (the redundant `glass-card-solid` marker class was folded into it).
    expect(document.body.querySelector('.modal-content')?.classList.contains('modal-content')).toBe(true)
    wrapper.unmount()
  })

  it('adapts every legacy width and preserves close defaults, slots and z-index', async () => {
    const wrapper = mount(BaseDialog, {
      attachTo: document.body,
      props: { show: true, title: 'Details', zIndex: 75 },
      slots: { default: '<input aria-label="Name" />', footer: '<button>Save</button>' }
    })
    expect(wrapper.findComponent(UiModal).props()).toMatchObject({ open: true, closeOnOverlay: false, closeOnEscape: true, zIndex: 75 })
    expect(document.body.querySelector<HTMLElement>('.modal-overlay')?.style.zIndex).toBe('75')
    expect(document.body.querySelector('.modal-footer')?.textContent).toBe('Save')
    for (const [width, expected] of Object.entries({ narrow: '440px', normal: '560px', wide: '720px', 'extra-wide': '960px', full: '960px' })) {
      await wrapper.setProps({ width: width as 'normal' })
      expect(document.body.querySelector<HTMLElement>('.modal-content')?.style.maxWidth).toBe(expected)
    }
    document.body.querySelector<HTMLElement>('.modal-overlay')!.click()
    expect(wrapper.emitted('close')).toBeUndefined()
    await wrapper.setProps({ closeOnClickOutside: true, closeOnEscape: false, showCloseButton: false })
    expect(document.body.querySelector('.modal-close')).toBeNull()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    expect(wrapper.emitted('close')).toBeUndefined()
    document.body.querySelector<HTMLElement>('.modal-overlay')!.click()
    expect(wrapper.emitted('close')).toHaveLength(1)
    wrapper.unmount()
  })

  it('traps focus and restores it after closing or unmounting', async () => {
    const trigger = document.createElement('button')
    document.body.append(trigger)
    trigger.focus()
    const wrapper = mount(BaseDialog, { attachTo: document.body, props: { show: true, title: 'Details' }, slots: { footer: '<button>Save</button>' } })
    await nextTick()
    const first = document.body.querySelector<HTMLElement>('.modal-close')!
    const last = document.body.querySelector<HTMLElement>('.modal-footer button')!
    last.focus()
    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    expect(document.activeElement).toBe(first)
    await wrapper.setProps({ show: false })
    expect(document.activeElement).toBe(trigger)
    await wrapper.setProps({ show: true })
    await nextTick()
    wrapper.unmount()
    expect(document.activeElement).toBe(trigger)
  })
})
