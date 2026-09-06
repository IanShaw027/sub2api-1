import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import MonitorSettingsPanel from '../MonitorSettingsPanel.vue'

const api = vi.hoisted(() => ({ getConfig: vi.fn(), updateConfig: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: {} }))
vi.mock('@/api/channelMonitorV2', async importOriginal => ({
  ...await importOriginal<typeof import('@/api/channelMonitorV2')>(),
  ...api
}))
vi.mock('@/api/admin', () => ({ adminAPI: { groups: { getAllIncludingInactive: async () => [] } } }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }) }))
vi.mock('@/utils/featureFlags', () => ({ isChannelMonitorV2Mode: () => true, getChannelMonitorMode: () => 'v2' }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key, te: () => true }) }))

describe('MonitorSettingsPanel refresh frequency', () => {
  it('retains its accessible group name and saves the chosen numeric interval', async () => {
    api.getConfig.mockResolvedValue({ enabled: true, refresh_interval_seconds: 60, platforms: [], group_ids: [] })
    api.updateConfig.mockImplementation(async config => JSON.parse(JSON.stringify(config)))
    const wrapper = mount(MonitorSettingsPanel, { global: { stubs: { Icon: true, RouterLink: true } } })
    await flushPromises()
    const group = wrapper.get('[role="radiogroup"][aria-label="channelMonitorV2.settings.refreshAria"]')
    const options = group.findAll('[role="radio"]')
    expect(options).toHaveLength(2)
    expect(options[0].attributes('aria-checked')).toBe('true')
    await options[1].trigger('click')
    expect(options[1].attributes('aria-checked')).toBe('true')
    await wrapper.get('header button').trigger('click')
    await flushPromises()
    expect(api.updateConfig).toHaveBeenCalledWith(expect.objectContaining({ refresh_interval_seconds: 300 }))
    expect(wrapper.get('header button').attributes('disabled')).toBeDefined()
    wrapper.unmount()
  })
})
