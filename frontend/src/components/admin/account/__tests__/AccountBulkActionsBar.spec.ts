import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountBulkActionsBar from '../AccountBulkActionsBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

describe('AccountBulkActionsBar', () => {
  // Glass redesign (Issue 1): the bar used to render unconditionally whenever
  // `totalResults > 0` — i.e. almost always — which is exactly why it consumed a
  // permanent 58px slot above the filter row. It now overlays the filter row and only
  // mounts once there is an active selection, so the pre-selection "select all results"
  // shortcut this test used to cover no longer exists; the escalation button is instead
  // reachable once the user has selected at least one row (e.g. via "select current page").
  it('does not render before any row is selected, even with results available', () => {
    const wrapper = mount(AccountBulkActionsBar, {
      props: {
        selectedIds: [],
        totalResults: 45,
        selectingAll: false,
        allResultsSelected: false
      }
    })

    expect(wrapper.find('.acct-bulk-overlay').exists()).toBe(false)
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
