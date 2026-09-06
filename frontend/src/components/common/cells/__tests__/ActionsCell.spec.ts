import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
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
