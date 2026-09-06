import { mount, type VueWrapper } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import ActionsCell from '../ActionsCell.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key })
}))

describe('ActionsCell', () => {
  it('emits edit when the pencil button is clicked', async () => {
    const wrapper = mount(ActionsCell, { attachTo: document.body })
    await wrapper.find('.icon-btn').trigger('click')
    expect(wrapper.emitted('edit')).toHaveLength(1)
    wrapper.unmount()
  })

  it('opens the dropdown menu and runs an item action', async () => {
    const onClick = vi.fn()
    const wrapper = mount(ActionsCell, {
      props: { items: [{ label: 'Delete', danger: true, onClick }] },
      attachTo: document.body
    })
    const buttons = wrapper.findAll('.icon-btn')
    // second icon-btn is the "more" trigger (first is edit)
    await buttons[1].trigger('click')
    const item = document.querySelector('.dropdown-item') as HTMLElement
    expect(item).toBeTruthy()
    item.click()
    expect(onClick).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })

  it('hides the edit button when showEdit is false', () => {
    const wrapper = mount(ActionsCell, { props: { showEdit: false, items: [{ label: 'X' }] } })
    expect(wrapper.findAll('.icon-btn')).toHaveLength(1)
  })
})

describe('ActionsCell viewport positioning', () => {
  let wrapper: VueWrapper | undefined

  afterEach(() => {
    wrapper?.unmount()
    wrapper = undefined
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
  })

  async function openMenu(viewportHeight: number, triggerBottom: number, menuHeight = 264) {
    vi.stubGlobal('innerHeight', viewportHeight)
    vi.stubGlobal('innerWidth', 1024)
    vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockReturnValue(menuHeight)
    vi.spyOn(HTMLElement.prototype, 'scrollHeight', 'get').mockReturnValue(menuHeight)
    vi.spyOn(HTMLElement.prototype, 'offsetWidth', 'get').mockReturnValue(180)
    wrapper = mount(ActionsCell, {
      props: { items: Array.from({ length: 7 }, (_, index) => ({ label: `Action ${index}` })) },
      attachTo: document.body
    })
    const trigger = wrapper.findAll('.icon-btn')[1]
    vi.spyOn(trigger.element, 'getBoundingClientRect').mockReturnValue({
      x: 940, y: triggerBottom - 28, left: 940, right: 968,
      top: triggerBottom - 28, bottom: triggerBottom,
      width: 28, height: 28, toJSON: () => ({})
    })
    await trigger.trigger('click')
    await nextTick()
    return { trigger, menu: document.querySelector<HTMLElement>('.cell-actions-menu')! }
  }

  it('opens above a bottom row and keeps all actions inside the viewport', async () => {
    const { menu } = await openMenu(600, 580)
    expect(menu.style.top).toBe('')
    const bottom = Number.parseFloat(menu.style.bottom)
    const visibleHeight = Math.min(menu.offsetHeight, Number.parseFloat(menu.style.maxHeight))
    expect(bottom).toBeGreaterThanOrEqual(8)
    expect(window.innerHeight - bottom - visibleHeight).toBeGreaterThanOrEqual(8)
  })

  it('limits a tall menu to available space without closing on its own scroll', async () => {
    const { menu } = await openMenu(240, 180, 500)
    expect(Number.parseFloat(menu.style.maxHeight)).toBeLessThan(500)
    expect(Number.parseFloat(menu.style.maxHeight)).toBeGreaterThan(0)

    menu.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.querySelector('.cell-actions-menu')).toBe(menu)

    window.dispatchEvent(new Event('scroll'))
    await nextTick()
    expect(document.querySelector('.cell-actions-menu')).toBeNull()
  })

  it('opens below a top row and restores trigger focus on Escape', async () => {
    const { trigger, menu } = await openMenu(600, 40)
    expect(menu.style.top).toBe('44px')
    expect(menu.style.bottom).toBe('')
    menu.querySelector<HTMLButtonElement>('button')!.focus()
    menu.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true }))
    await nextTick()
    expect(document.querySelector('.cell-actions-menu')).toBeNull()
    expect(document.activeElement).toBe(trigger.element)
  })
})
