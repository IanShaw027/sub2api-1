import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, beforeEach, vi } from 'vitest'

import TicketsView from '../TicketsView.vue'

const { listAdminTickets, showError } = vi.hoisted(() => ({
  listAdminTickets: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/adminTickets', () => ({
  default: {
    listAdminTickets,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({
    push: vi.fn(),
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

describe('admin TicketsView', () => {
  beforeEach(() => {
    listAdminTickets.mockReset()
    showError.mockReset()
    listAdminTickets.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
  })

  it('omits empty filters when requesting admin ticket list data', async () => {
    mount(TicketsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          Pagination: true,
          DataTable: true,
          SearchInput: true,
          Select: true,
          Icon: true,
        },
      },
    })

    await flushPromises()

    expect(listAdminTickets).toHaveBeenCalledTimes(1)
    const call = listAdminTickets.mock.calls[0]?.[0]
    expect(call).toMatchObject({
      page: 1,
      page_size: 20,
    })
    expect(call).not.toHaveProperty('search')
    expect(call).not.toHaveProperty('category')
    expect(call).not.toHaveProperty('status')
    expect(call).not.toHaveProperty('start_date')
    expect(call).not.toHaveProperty('end_date')
  })
})
