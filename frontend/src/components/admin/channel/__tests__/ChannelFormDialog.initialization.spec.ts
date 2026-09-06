import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount, type VueWrapper } from '@vue/test-utils'
import type { Channel } from '@/api/admin/channels'
import ChannelFormDialog from '../ChannelFormDialog.vue'

const api = vi.hoisted(() => ({
  getGroups: vi.fn(),
  listChannels: vi.fn(),
  updateChannel: vi.fn(),
  getAccount: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: { getAll: api.getGroups },
    channels: { list: api.listChannels, update: api.updateChannel },
    accounts: { getById: api.getAccount },
    settings: { getWebSearchEmulationConfig: vi.fn().mockResolvedValue({ enabled: false }) },
  },
}))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError: vi.fn(), showSuccess: vi.fn() }),
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no })
  return { promise, resolve, reject }
}

const groups = [{ id: 10, platform: 'openai', name: 'OpenAI' }]
const channel = (id: number) => ({
  id,
  name: `Channel ${id}`,
  status: 'active',
  group_ids: [10],
  model_mapping: { openai: { [`public-${id}`]: `model-${id}` } },
  model_pricing: [{ platform: 'openai', models: [`model-${id}`], billing_mode: 'token', input_price: 0.000002 }],
} as Channel)

let wrapper: VueWrapper
function mountDialog() {
  wrapper = shallowMount(ChannelFormDialog, {
    props: { show: true, editingChannel: channel(1) },
    global: { stubs: { BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' } } },
  })
  return wrapper
}

beforeEach(() => {
  vi.clearAllMocks()
  api.getGroups.mockReset().mockResolvedValue(groups)
  api.listChannels.mockReset().mockResolvedValue({ items: [] })
  api.updateChannel.mockReset().mockResolvedValue({})
})
afterEach(() => wrapper?.unmount())

describe('ChannelFormDialog initialization', () => {
  it('blocks both the save button and form submission until initialization completes', async () => {
    const request = deferred<typeof groups>()
    api.getGroups.mockReturnValueOnce(request.promise)
    mountDialog()
    expect(wrapper.get('button[form="channel-form"]').attributes('disabled')).toBeDefined()
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).not.toHaveBeenCalled()

    request.resolve(groups)
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).toHaveBeenCalledWith(1, expect.objectContaining({
      group_ids: [10],
      model_mapping: { openai: { 'public-1': 'model-1' } },
      model_pricing: [expect.objectContaining({ models: ['model-1'], input_price: 0.000002 })],
    }))
  })

  it.each(['groups', 'channels'])('blocks saving after %s loading fails and permits an explicit retry', async (source) => {
    const failedAPI = source === 'groups' ? api.getGroups : api.listChannels
    failedAPI.mockRejectedValueOnce(new Error('Initialization failed'))
    mountDialog()
    await flushPromises()
    expect(wrapper.get('[role="alert"]').text()).toContain('Initialization failed')
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).not.toHaveBeenCalled()

    await wrapper.get('[role="alert"] button').trigger('click')
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).toHaveBeenCalledWith(1, expect.objectContaining({ group_ids: [10] }))
  })

  it.each([false, true])('ignores an old initialization after switching channels (close first: %s)', async (closeFirst) => {
    const oldRequest = deferred<typeof groups>()
    api.getGroups.mockReturnValueOnce(oldRequest.promise)
    mountDialog()
    if (closeFirst) await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, editingChannel: channel(2) })
    await flushPromises()
    oldRequest.resolve([{ id: 999, platform: 'anthropic', name: 'Old groups' }])
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).toHaveBeenCalledWith(2, expect.objectContaining({
      name: 'Channel 2',
      group_ids: [10],
      model_mapping: { openai: { 'public-2': 'model-2' } },
    }))
  })

  it('does not let a stale failure disable a newly initialized dialog', async () => {
    const oldRequest = deferred<typeof groups>()
    api.getGroups.mockReturnValueOnce(oldRequest.promise)
    mountDialog()
    await wrapper.setProps({ editingChannel: channel(2) })
    await flushPromises()
    oldRequest.reject(new Error('Old request failed'))
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).toHaveBeenCalledWith(2, expect.any(Object))
  })

  it('does not close a newly opened dialog when an older save completes', async () => {
    const save = deferred<object>()
    api.updateChannel.mockReturnValueOnce(save.promise)
    mountDialog()
    await flushPromises()
    await wrapper.get('form').trigger('submit')
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, editingChannel: channel(2) })
    await flushPromises()
    save.resolve({})
    await flushPromises()
    expect(wrapper.emitted('close')).toBeUndefined()
    expect(wrapper.get('button[form="channel-form"]').attributes('disabled')).toBeUndefined()
    await wrapper.get('form').trigger('submit')
    expect(api.updateChannel).toHaveBeenLastCalledWith(2, expect.objectContaining({ name: 'Channel 2' }))
  })
})
