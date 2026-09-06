import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import AccountStatsModal from '../AccountStatsModal.vue'
import type { Account } from '@/types'

const { getStats } = vi.hoisted(() => ({ getStats: vi.fn() }))
vi.mock('@/api/admin', () => ({ adminAPI: { accounts: { getStats } } }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

enableAutoUnmount(afterEach)
beforeEach(() => getStats.mockReset().mockResolvedValue(null))

describe('AccountStatsModal initialization', () => {
  it('loads stats when its async component first mounts already open', async () => {
    shallowMount(AccountStatsModal, {
      props: { show: true, account: { id: 23, name: 'Account' } as Account }
    })
    await flushPromises()
    expect(getStats).toHaveBeenCalledTimes(1)
    expect(getStats).toHaveBeenCalledWith(23, 30)
  })

  it('waits while closed and reloads after each opening', async () => {
    const wrapper = shallowMount(AccountStatsModal, {
      props: { show: false, account: { id: 23, name: 'Account' } as Account }
    })
    await flushPromises()
    expect(getStats).not.toHaveBeenCalled()

    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(getStats).toHaveBeenCalledTimes(1)
    expect(getStats).toHaveBeenCalledWith(23, 30)

    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true })
    await flushPromises()
    expect(getStats).toHaveBeenCalledTimes(2)
  })
})
