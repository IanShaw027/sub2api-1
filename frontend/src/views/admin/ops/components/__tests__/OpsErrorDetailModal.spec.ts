import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import OpsErrorDetailModal from '../OpsErrorDetailModal.vue'

const { getRequestErrorDetail, listRequestErrorUpstreamErrors } = vi.hoisted(() => ({
  getRequestErrorDetail: vi.fn(),
  listRequestErrorUpstreamErrors: vi.fn(),
}))

vi.mock('@/api/admin/ops', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/ops')>('@/api/admin/ops')
  return {
    ...actual,
    opsAPI: {
      ...actual.opsAPI,
      getRequestErrorDetail,
      getUpstreamErrorDetail: vi.fn(),
      listRequestErrorUpstreamErrors,
    },
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('OpsErrorDetailModal request races', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('keeps the latest detail response when the selected error changes', async () => {
    const first = deferred<any>()
    const second = deferred<any>()
    getRequestErrorDetail.mockImplementationOnce(() => first.promise)
    getRequestErrorDetail.mockImplementationOnce(() => second.promise)
    listRequestErrorUpstreamErrors.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 100,
      pages: 0,
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 1,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })

    await wrapper.setProps({ errorId: 2 })
    expect(getRequestErrorDetail).toHaveBeenCalledTimes(2)
    second.resolve({
      id: 2,
      created_at: '2026-05-24T00:00:00Z',
      request_id: 'new-request',
      status_code: 500,
      error_body: '{}',
      request_type: 1,
      phase: 'request',
      type: 'gateway',
      severity: 'error',
      error_owner: 'client',
      error_source: 'gateway',
      platform: 'openai',
      model: 'gpt-4',
      resolved: false,
      client_request_id: 'new-request',
      message: 'new message',
      user_email: 'a@example.com',
      account_name: 'acct',
      group_name: 'grp',
    })
    await flushPromises()
    expect(wrapper.text()).toContain('new-request')

    first.resolve({
      id: 1,
      created_at: '2026-05-24T00:00:00Z',
      request_id: 'old-request',
      status_code: 500,
      error_body: '{}',
      request_type: 1,
      phase: 'request',
      type: 'gateway',
      severity: 'error',
      error_owner: 'client',
      error_source: 'gateway',
      platform: 'openai',
      model: 'gpt-4',
      resolved: false,
      client_request_id: 'old-request',
      message: 'old message',
      user_email: 'a@example.com',
      account_name: 'acct',
      group_name: 'grp',
    })
    await flushPromises()

    expect(wrapper.text()).toContain('new-request')
    expect(wrapper.text()).not.toContain('old-request')
  })

  it('clears loaded detail when the selected error is reset while open', async () => {
    getRequestErrorDetail.mockResolvedValueOnce({
      id: 1,
      created_at: '2026-05-24T00:00:00Z',
      request_id: 'loaded-request',
      status_code: 500,
      error_body: '{}',
      request_type: 1,
      phase: 'request',
      type: 'gateway',
      severity: 'error',
      error_owner: 'client',
      error_source: 'gateway',
      platform: 'openai',
      model: 'gpt-4',
      resolved: false,
      client_request_id: 'loaded-request',
      message: 'loaded message',
    })
    listRequestErrorUpstreamErrors.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 100,
      pages: 0,
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 1,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('loaded-request')

    await wrapper.setProps({ errorId: null })
    await flushPromises()

    expect((wrapper.vm as any).detail).toBeNull()
    expect(wrapper.text()).not.toContain('loaded-request')
  })

  it('labels canonical cyber request_type rows', async () => {
    getRequestErrorDetail.mockResolvedValueOnce({
      id: 6,
      created_at: '2026-05-24T00:00:00Z',
      request_id: 'cyber-request',
      status_code: 403,
      error_body: '{}',
      request_type: 4,
      phase: 'request',
      type: 'cyber_policy_session_blocked',
      severity: 'error',
      error_owner: 'platform',
      error_source: 'gateway_local',
      platform: 'openai',
      model: 'gpt-5',
      resolved: false,
      client_request_id: 'cyber-request',
      message: 'blocked',
    })
    listRequestErrorUpstreamErrors.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 100,
      pages: 0,
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 6,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('admin.ops.errorDetail.requestTypeCyber')
  })

  it('labels image request_type rows after cyber value restoration', async () => {
    getRequestErrorDetail.mockResolvedValueOnce({
      id: 8,
      created_at: '2026-05-24T00:00:00Z',
      request_id: 'image-request',
      status_code: 500,
      error_body: '{}',
      request_type: 8,
      phase: 'request',
      type: 'upstream_error',
      severity: 'error',
      error_owner: 'upstream',
      error_source: 'upstream',
      platform: 'openai',
      model: 'gpt-image-1',
      resolved: false,
      client_request_id: 'image-request',
      message: 'failed',
    })
    listRequestErrorUpstreamErrors.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })

    const wrapper = mount(OpsErrorDetailModal, {
      props: {
        show: true,
        errorId: 8,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Icon: { template: '<span />' },
        },
      },
    })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.ops.errorDetail.requestTypeImage')
  })
})
