import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import OpsErrorDetailsModal from '../OpsErrorDetailsModal.vue'

const { listRequestErrors } = vi.hoisted(() => ({
  listRequestErrors: vi.fn(),
}))

vi.mock('@/api/admin/ops', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/ops')>('@/api/admin/ops')
  return {
    ...actual,
    opsAPI: {
      ...actual.opsAPI,
      listRequestErrors,
      listUpstreamErrors: vi.fn(),
    },
  }
})

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

describe('OpsErrorDetailsModal request races', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('ignores stale error list responses after scope changes', async () => {
    const first = deferred<any>()
    const second = deferred<any>()
    listRequestErrors.mockImplementationOnce(() => first.promise)
    listRequestErrors.mockImplementationOnce(() => second.promise)

    const wrapper = mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Select: true,
          OpsErrorLogTable: {
            props: ['rows'],
            template: '<div data-test="rows">{{ rows.map((row) => row.request_id).join(",") }}</div>',
          },
        },
      },
    })

    await wrapper.setProps({ platform: 'openai' })
    expect(listRequestErrors).toHaveBeenCalledTimes(2)

    second.resolve({ items: [{ id: 2, request_id: 'new-error' }], total: 1 })
    await flushPromises()
    expect(wrapper.get('[data-test="rows"]').text()).toBe('new-error')

    first.resolve({ items: [{ id: 1, request_id: 'old-error' }], total: 1 })
    await flushPromises()

    expect(wrapper.get('[data-test="rows"]').text()).toBe('new-error')
  })

  it('clears loaded error rows when the modal closes', async () => {
    listRequestErrors.mockResolvedValueOnce({
      items: [{ id: 1, request_id: 'loaded-error' }],
      total: 1,
    })

    const wrapper = mount(OpsErrorDetailsModal, {
      props: {
        show: true,
        timeRange: '1h',
        platform: '',
        groupId: null,
        errorType: 'request',
      },
      global: {
        stubs: {
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          Select: true,
          OpsErrorLogTable: {
            props: ['rows'],
            template: '<div data-test="rows">{{ rows.map((row) => row.request_id).join(",") }}</div>',
          },
        },
      },
    })

    await flushPromises()
    expect(wrapper.get('[data-test="rows"]').text()).toBe('loaded-error')

    await wrapper.setProps({ show: false })
    await flushPromises()

    expect((wrapper.vm as any).rows).toEqual([])
    expect((wrapper.vm as any).total).toBe(0)
    expect((wrapper.vm as any).loading).toBe(false)
  })
})
