import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AccountsHeaderMenus from '../accounts/AccountsHeaderMenus.vue'
import { useAccountToolbarMenus } from '../accounts/useAccountToolbarMenus'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

describe('AccountsHeaderMenus preserved capacity forecast entry', () => {
  it('opens capacity directly without expanding More and retains other menu tools', async () => {
    const toolbarMenus = useAccountToolbarMenus()
    const wrapper = mount(AccountsHeaderMenus, { props: { toolbarMenus, selIds: [] }, global: { stubs: { Icon: true, Teleport: true } } })
    expect(toolbarMenus.showAccountToolsDropdown.value).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.capacityForecast.action')!.trigger('click')
    expect(wrapper.emitted('open-capacity-forecast')).toHaveLength(1)
    expect(toolbarMenus.showAccountToolsDropdown.value).toBe(false)
    await wrapper.findAll('button').find(button => button.text() === 'common.more')!.trigger('click')
    expect(wrapper.text()).toContain('admin.errorPassthrough.title')
    expect(wrapper.text()).toContain('admin.tlsFingerprintProfiles.title')
    expect(wrapper.text()).toContain('admin.tlsFingerprintRouters.title')
    wrapper.unmount()
  })
})
