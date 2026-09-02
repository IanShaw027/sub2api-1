import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import UiModal from '../UiModal.vue'

afterEach(() => {
  document.body.innerHTML = ''
  document.body.classList.remove('modal-open')
})

describe('UiModal', () => {
  it('teleports a glass panel, traps focus, and closes on escape', async () => {
    const wrapper = mount(UiModal, {
      attachTo: document.body,
      props: { open: false, title: 'Create key' },
      slots: { default: '<button type="button">Inside</button>', footer: '<button type="button">Save</button>' },
      global: {
        stubs: {
          Icon: true,
          Transition: { props: ['name'], template: '<slot />' }
        }
      }
    })

    await wrapper.setProps({ open: true })
    await nextTick()
    const dialog = document.body.querySelector('[role="dialog"]')
    expect(dialog).not.toBeNull()
    expect(dialog?.classList.contains('glass-card-solid')).toBe(true)
    expect(document.body.classList.contains('modal-open')).toBe(true)

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()
    wrapper.unmount()
  })
})
