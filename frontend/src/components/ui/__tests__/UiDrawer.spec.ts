import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import UiDrawer from '../UiDrawer.vue'

afterEach(() => {
  document.body.innerHTML = ''
  document.body.style.overflow = ''
})

describe('UiDrawer', () => {
  it('teleports a glass panel and locks body scroll', async () => {
    const wrapper = mount(UiDrawer, {
      attachTo: document.body,
      props: { open: false, title: 'Filters' },
      slots: { default: '<button type="button">Option</button>' },
      global: {
        stubs: {
          Icon: true,
          Transition: { props: ['name'], template: '<slot />' }
        }
      }
    })

    document.body.style.overflow = 'scroll'
    await wrapper.setProps({ open: true })
    await nextTick()
    expect(document.body.querySelector('.ui-drawer-panel')).not.toBeNull()
    expect(document.body.querySelector('.ui-drawer-overlay')).not.toBeNull()
    expect(document.body.style.overflow).toBe('hidden')

    window.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(wrapper.emitted('close')).toBeTruthy()

    await wrapper.setProps({ open: false })
    await nextTick()
    expect(document.body.style.overflow).toBe('scroll')
    wrapper.unmount()
  })

  it('supports a left side variant', async () => {
    const wrapper = mount(UiDrawer, {
      attachTo: document.body,
      props: { open: true, title: 'Nav', side: 'left' },
      global: { stubs: { Icon: true, Transition: { props: ['name'], template: '<slot />' } } }
    })
    await nextTick()
    expect(document.body.querySelector('.ui-drawer-panel')?.classList.contains('is-left')).toBe(true)
    wrapper.unmount()
  })
})
