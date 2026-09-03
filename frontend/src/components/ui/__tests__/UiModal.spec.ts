import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import UiModal from '../UiModal.vue'
import { resetOverlayLock } from '../overlayLock'

const stubs = {
  Icon: true,
  Transition: { props: ['name'], template: '<slot />' }
}

function mountModal(props: Record<string, unknown> = {}, slots: Record<string, string> = {}) {
  return mount(UiModal, {
    attachTo: document.body,
    props: { open: false, title: 'Create key', ...props },
    slots: {
      default: '<button type="button">Inside</button>',
      footer: '<button type="button">Save</button>',
      ...slots
    },
    global: { stubs }
  })
}

afterEach(() => {
  resetOverlayLock()
  document.body.innerHTML = ''
})

describe('UiModal', () => {
  it('teleports a glass panel, traps focus, and closes on escape', async () => {
    const wrapper = mountModal()

    await wrapper.setProps({ open: true })
    await nextTick()
    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog?.classList.contains('glass-card-solid')).toBe(true)
    expect(dialog?.getAttribute('tabindex')).toBe('-1')
    expect(document.body.classList.contains('modal-open')).toBe(true)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
    wrapper.unmount()
  })

  it('keeps body locked when one of two open modals closes', async () => {
    const outer = mountModal({ open: true, title: 'Outer' })
    const inner = mountModal({ open: true, title: 'Inner' })
    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(true)

    await inner.setProps({ open: false })
    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(true)

    inner.unmount()
    expect(document.body.classList.contains('modal-open')).toBe(true)

    await outer.setProps({ open: false })
    await nextTick()
    expect(document.body.classList.contains('modal-open')).toBe(false)
    outer.unmount()
  })

  it('closes only the topmost overlay on Escape', async () => {
    const outer = mountModal({ open: true, title: 'Outer' })
    const inner = mountModal({ open: true, title: 'Inner' })
    await nextTick()

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()

    expect(inner.emitted('close')).toBeTruthy()
    expect(outer.emitted('close')).toBeFalsy()

    inner.unmount()
    outer.unmount()
  })

  it('wraps Tab from the last control back to the first', async () => {
    const wrapper = mountModal({ open: true })
    await nextTick()

    const dialog = document.body.querySelector('[role="dialog"]') as HTMLElement
    const tabbables = Array.from(dialog.querySelectorAll<HTMLElement>('button'))
    expect(tabbables.length).toBeGreaterThanOrEqual(2)
    const first = tabbables[0]
    const last = tabbables[tabbables.length - 1]
    last.focus()
    expect(document.activeElement).toBe(last)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true }))
    await nextTick()
    expect(document.activeElement).toBe(first)

    first.focus()
    window.dispatchEvent(
      new KeyboardEvent('keydown', { key: 'Tab', bubbles: true, cancelable: true, shiftKey: true })
    )
    await nextTick()
    expect(document.activeElement).toBe(last)

    wrapper.unmount()
  })
})
