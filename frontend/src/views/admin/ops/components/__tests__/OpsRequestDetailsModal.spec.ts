import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import OpsRequestDetailsModal from '../OpsRequestDetailsModal.vue'

const { listRequestDetails } = vi.hoisted(() => ({
  listRequestDetails: vi.fn(),
}))

vi.mock('@/api/admin/ops', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/ops')>('@/api/admin/ops')
  return {
    ...actual,
    opsAPI: {
      ...actual.opsAPI,
      listRequestDetails,
    },
  }
})

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true),
  }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) => {
        if (params?.n) return `${key}:${params.n}`
        if (params?.range) return `${key}:${params.range}`
        return key
      },
    }),
  }
})

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('OpsRequestDetailsModal request races', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('keeps the latest request details response when filters change quickly', async () => {
    const first = deferred<any>()
    const second = deferred<any>()
    listRequestDetails.mockImplementationOnce(() => first.promise)
    listRequestDetails.mockImplementationOnce(() => second.promise)

    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: true,
        timeRange: '1h',
        preset: { title: 'Requests', kind: 'all', sort: 'created_at_desc' },
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /></div>' },
          Pagination: true,
        },
      },
    })

    await wrapper.setProps({ platform: 'openai' })
    expect(listRequestDetails).toHaveBeenCalledTimes(2)

    second.resolve({
      items: [{ kind: 'success', created_at: '2026-05-24T00:00:00Z', request_id: 'new-id', platform: 'openai' }],
      total: 1,
    })
    await flushPromises()
    expect(wrapper.text()).toContain('new-id')

    first.resolve({
      items: [{ kind: 'success', created_at: '2026-05-24T00:00:00Z', request_id: 'old-id', platform: 'claude' }],
      total: 1,
    })
    await flushPromises()

    expect(wrapper.text()).toContain('new-id')
    expect(wrapper.text()).not.toContain('old-id')
  })

  it('clears loaded request details when the modal closes', async () => {
    listRequestDetails.mockResolvedValueOnce({
      items: [{ kind: 'success', created_at: '2026-05-24T00:00:00Z', request_id: 'loaded-id', platform: 'openai' }],
      total: 1,
    })

    const wrapper = mount(OpsRequestDetailsModal, {
      props: {
        modelValue: true,
        timeRange: '1h',
        preset: { title: 'Requests', kind: 'all', sort: 'created_at_desc' },
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /></div>' },
          Pagination: true,
        },
      },
    })

    await flushPromises()
    expect(wrapper.text()).toContain('loaded-id')

    await wrapper.setProps({ modelValue: false })
    await flushPromises()

    expect((wrapper.vm as any).items).toEqual([])
    expect((wrapper.vm as any).total).toBe(0)
    expect((wrapper.vm as any).loading).toBe(false)
  })

  it('uses provided custom start and end times for custom ranges', async () => {
    listRequestDetails.mockResolvedValueOnce({
      items: [],
      total: 0,
    })

    const wrapper = mount(OpsRequestDetailsModal as any, {
      props: {
        modelValue: true,
        timeRange: 'custom',
        customStartTime: '2026-06-01T00:00:00Z',
        customEndTime: '2026-06-01T01:30:00Z',
        preset: { title: 'Requests', kind: 'all', sort: 'created_at_desc' },
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show', 'title'], template: '<div v-if="show"><slot /></div>' },
          Pagination: true,
        },
      },
    })

    await flushPromises()

    expect(listRequestDetails).toHaveBeenCalledWith(
      expect.objectContaining({
        start_time: '2026-06-01T00:00:00Z',
        end_time: '2026-06-01T01:30:00Z',
      }),
      expect.any(Object)
    )
    expect(listRequestDetails.mock.calls[0][0]).not.toHaveProperty('time_range')
    expect(wrapper.text()).toContain('admin.ops.requestDetails.rangeLabel:admin.ops.timeRange.custom')
  })
})
