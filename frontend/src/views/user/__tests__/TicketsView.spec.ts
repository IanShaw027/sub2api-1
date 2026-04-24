import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { describe, expect, it, vi, beforeEach } from 'vitest'

import TicketsView from '../TicketsView.vue'

const routeState = reactive<{ name: string }>({ name: 'Tickets' })

const { listTickets, getAvailable, getUserGroupRates, showError, showSuccess, routerPush, routerReplace } = vi.hoisted(() => ({
  listTickets: vi.fn(),
  getAvailable: vi.fn(),
  getUserGroupRates: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
  routerPush: vi.fn(),
  routerReplace: vi.fn(),
}))

vi.mock('@/api/tickets', () => ({
  default: {
    listTickets,
    closeTicket: vi.fn(),
    createTicket: vi.fn(),
  },
}))

vi.mock('@/api/groups', () => ({
  default: {
    getAvailable,
    getUserGroupRates,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
  useAuthStore: () => ({
    user: {
      concurrency: 4,
    },
  }),
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    push: routerPush,
    replace: routerReplace,
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

describe('TicketsView', () => {
  beforeEach(() => {
    routeState.name = 'TicketCreate'
    listTickets.mockReset()
    getAvailable.mockReset()
    getUserGroupRates.mockReset()
    showError.mockReset()
    showSuccess.mockReset()
    routerPush.mockReset()
    routerReplace.mockReset()

    listTickets.mockResolvedValue({ items: [], total: 0, page: 1, page_size: 20 })
    getAvailable.mockResolvedValue([])
    getUserGroupRates.mockResolvedValue({})
  })

  it('opens the create dialog for the direct create route and returns to list on close', async () => {
    const wrapper = mount(TicketsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          Pagination: true,
          DataTable: {
            props: ['data', 'loading'],
            template: `
              <div>
                <div v-if="loading">common.loading</div>
                <div v-else-if="!data?.length"><slot name="empty" /></div>
                <div v-else>
                  <div v-for="row in data" :key="row.id">
                    <slot name="cell-title" :row="row" :value="row.title">{{ row.title }}</slot>
                  </div>
                </div>
              </div>
            `,
          },
          SearchInput: {
            props: ['modelValue'],
            emits: ['update:modelValue', 'search'],
            template: `
              <input
                :value="modelValue"
                @input="$emit('update:modelValue', $event.target.value); $emit('search', $event.target.value)"
              />
            `,
          },
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue', 'change'],
            template: '<div class="select-stub" />',
          },
          TicketCreateDialog: {
            props: ['show'],
            emits: ['close', 'submit'],
            template: `
              <div data-test="create-dialog" :data-open="String(show)">
                <button type="button" class="close-dialog" @click="$emit('close')">close</button>
              </div>
            `,
          },
        },
      },
    })

    await flushPromises()

    expect(wrapper.get('[data-test="create-dialog"]').attributes('data-open')).toBe('true')

    await wrapper.get('.close-dialog').trigger('click')

    expect(routerReplace).toHaveBeenCalledWith({ name: 'Tickets' })
  })

  it('keeps the existing rows visible while a follow-up search is loading', async () => {
    routeState.name = 'Tickets'

    let resolveSearch: ((value: { items: Array<{ id: number; category: string; title: string; status: string; created_at: string; updated_at: string }>; total: number; page: number; page_size: number }) => void) | null = null

    listTickets
      .mockResolvedValueOnce({
        items: [
          {
            id: 1,
            category: 'consult',
            title: 'Initial ticket',
            status: 'submitted',
            created_at: '2026-04-24T00:00:00Z',
            updated_at: '2026-04-24T00:00:00Z',
          },
        ],
        total: 1,
        page: 1,
        page_size: 20,
      })
      .mockImplementationOnce(() => new Promise((resolve) => {
        resolveSearch = resolve
      }))

    const wrapper = mount(TicketsView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: { template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>' },
          Pagination: true,
          DataTable: {
            props: ['data', 'loading'],
            template: `
              <div>
                <div v-if="loading">common.loading</div>
                <div v-else-if="!data?.length"><slot name="empty" /></div>
                <div v-else>
                  <div v-for="row in data" :key="row.id">
                    <slot name="cell-title" :row="row" :value="row.title">{{ row.title }}</slot>
                  </div>
                </div>
              </div>
            `,
          },
          SearchInput: {
            props: ['modelValue'],
            emits: ['update:modelValue', 'search'],
            template: `
              <input
                :value="modelValue"
                @input="$emit('update:modelValue', $event.target.value); $emit('search', $event.target.value)"
              />
            `,
          },
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue', 'change'],
            template: '<div class="select-stub" />',
          },
          TicketCreateDialog: true,
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('Initial ticket')

    await wrapper.get('input').setValue('abc')
    await flushPromises()

    expect(wrapper.text()).toContain('Initial ticket')
    expect(wrapper.text()).not.toContain('common.loading')

    resolveSearch?.({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    })

    await flushPromises()
  })
})
