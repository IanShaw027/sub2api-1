import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('AccountBulkActionsBar', () => {
  it('preserves filtered editing and select-all before selection without exposing selected-only operations', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [],
        totalResults: 45,
        selectingAll: false,
        allResultsSelected: false
      }
    })

    expect(wrapper.find('.acct-bulk-overlay').exists()).toBe(true)
    const buttons = wrapper.findAll('button')
    await buttons.find(button => button.text() === 'admin.accounts.bulkEdit.submit')!.trigger('click')
    expect(wrapper.emitted('edit-filtered')).toHaveLength(1)
    await buttons.find(button => button.text() === 'admin.accounts.bulkActions.selectAllResults')!.trigger('click')
    expect(wrapper.emitted('select-all-results')).toHaveLength(1)
    expect(buttons.some(button => button.text() === 'admin.accounts.bulkActions.delete')).toBe(false)
    expect(buttons.some(button => button.text() === 'admin.accounts.bulkActions.edit')).toBe(false)
  })

  it('keeps selected editing and filtered editing as independent visible actions', async () => {
    const wrapper = mount(AccountBulkActionsBar, { props: { selectedIds: [1], totalResults: 45, selectingAll: false, allResultsSelected: false } })
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.bulkEdit.submit')!.trigger('click')
    expect(wrapper.emitted('edit-filtered')).toHaveLength(1)
    expect(wrapper.emitted('edit-selected')).toBeUndefined()
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.bulkActions.edit')!.trigger('click')
    expect(wrapper.emitted('edit-selected')).toHaveLength(1)
  })

  it('allows escalating to select all results once the current page is selected', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1, 2, 3],
        totalResults: 45,
        selectingAll: false,
        allResultsSelected: false
      }
    })

    const button = wrapper.findAll('button').find(item =>
      item.text().includes('admin.accounts.bulkActions.selectAllResults')
    )

    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('select-all-results')).toHaveLength(1)
  })

  it('preserves the upstream billing probe action from v0.1.166', async () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [1],
        totalResults: 45,
        selectingAll: false,
        allResultsSelected: false
      }
    })

    const button = wrapper.findAll('button').find(item =>
      item.text().includes('admin.accounts.bulkActions.probeUpstreamBilling')
    )

    expect(button).toBeDefined()
    await button!.trigger('click')
    expect(wrapper.emitted('probe-upstream-billing')).toHaveLength(1)
  })
})
