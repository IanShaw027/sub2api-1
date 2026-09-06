import { describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { i18n } from '@/i18n'
import SessionList from '../components/SessionList.vue'
import { useCreationStore } from '../stores/creation'

// Local-first workspace coverage lives in ChatWorkspaceLocalUI.spec.ts.
// Keep compatibility coverage for legacy cloud components used during migration.
describe('legacy chat session list compatibility', () => {
  it('preserves the mode selector when no fixed mode is provided', () => {
    setActivePinia(createPinia())
    const store = useCreationStore()
    store.sessionModeFilter = 'chat'
    const wrapper = mount(SessionList, { global: { plugins: [i18n] } })
    expect(wrapper.findComponent({ name: 'SegmentedControl' }).exists()).toBe(true)
    wrapper.unmount()
    store.reset()
  })
})
