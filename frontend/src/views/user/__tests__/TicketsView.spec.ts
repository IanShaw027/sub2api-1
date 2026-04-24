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
          Pagination: true,
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue'],
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
})
