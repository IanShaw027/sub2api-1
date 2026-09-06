import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import TicketsView from '../TicketsView.vue'

const { listMock, closeMock, pushMock, replaceMock, routeState } = vi.hoisted(() => ({
  listMock: vi.fn(),
  closeMock: vi.fn(),
  pushMock: vi.fn(),
  replaceMock: vi.fn(),
  routeState: { query: {} as Record<string, string> }
}))
vi.mock('@/api/tickets', () => ({
  ticketsAPI: { list: listMock, close: closeMock }
}))
vi.mock('@/stores', () => ({
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() })
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRouter: () => ({ push: pushMock, replace: replaceMock }),
  useRoute: () => routeState
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

describe('TicketsView row actions', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    routeState.query = {}
  })
  it('keeps close directly visible for open tickets without opening More', async () => {
    listMock.mockResolvedValue({ data: { total: 3, items: [
      { id: 1, status: 'submitted' },
      { id: 2, status: 'closed' },
      { id: 3, status: 'withdrawn' }
    ] } })
    closeMock.mockResolvedValue({})
    const wrapper = mount(TicketsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /></div>' },
          PageHeader: { template: '<header><slot name="actions" /></header>' },
          DataTable: {
            props: ['data'],
            template: '<div><div v-for="row in data" :key="row.id" :data-row="row.id"><slot name="cell-actions" :row="row" /></div></div>'
          },
          SearchInput: true,
          Select: true,
          DateRangePicker: true,
          RouterLink: true,
          Icon: true
        }
      }
    })
    await flushPromises()
    expect(wrapper.findAll('[aria-label="tickets.actions.close"]')).toHaveLength(1)
    expect(wrapper.get('[data-row="1"] [aria-label="tickets.actions.close"]').isVisible()).toBe(true)
    expect(wrapper.find('[data-row="2"] [aria-label="tickets.actions.close"]').exists()).toBe(false)
    expect(wrapper.find('[data-row="3"] [aria-label="tickets.actions.close"]').exists()).toBe(false)
    await wrapper.get('[data-row="1"] [aria-label="tickets.actions.close"]').trigger('click')
    await flushPromises()
    expect(closeMock).toHaveBeenCalledWith(1)
    expect(listMock).toHaveBeenCalledTimes(2)
    wrapper.unmount()
  })

  it('restores list context and carries it to both browser Back and create cancellation', async () => {
    routeState.query = {
      page: '3', page_size: '50', keyword: 'quota',
      category: 'concurrency_apply', status: 'processing',
      start_date: '2026-08-01', end_date: '2026-08-31'
    }
    listMock.mockResolvedValue({ data: { total: 200, items: [] } })
    const wrapper = mount(TicketsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div />' },
          PageHeader: { template: '<header><slot name="actions" /></header>' },
          Icon: true
        }
      }
    })
    await flushPromises()
    expect(listMock).toHaveBeenCalledWith({
      ...routeState.query, page: 3, page_size: 50
    })
    await wrapper.get('.tickets-create-desktop').trigger('click')
    await flushPromises()
    expect(replaceMock).toHaveBeenCalledWith({ path: '/tickets', query: routeState.query })
    expect(pushMock).toHaveBeenCalledWith({ path: '/tickets/new', query: routeState.query })
    expect(replaceMock.mock.invocationCallOrder[0]).toBeLessThan(pushMock.mock.invocationCallOrder[0])
    wrapper.unmount()
  })
})
