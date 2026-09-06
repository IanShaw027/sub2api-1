import { flushPromises, mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import TicketCreateView from '../TicketCreateView.vue'
import Button from '@/components/ui/Button.vue'

const { query } = vi.hoisted(() => ({
  query: { page: '3', page_size: '50', keyword: 'quota', category: 'rate_apply', status: 'processing' }
}))
vi.mock('vue-router', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-router')>(),
  useRouter: () => ({ replace: vi.fn() }),
  useRoute: () => ({ query })
}))
vi.mock('@/api/tickets', () => ({
  ticketsAPI: { rateGroups: () => Promise.resolve({ data: [] }) }
}))
vi.mock('@/stores', () => ({
  useAuthStore: () => ({ user: { concurrency: 2 } }),
  useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() })
}))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

describe('TicketCreateView return links', () => {
  it('keeps the original list query on both Cancel and Back', async () => {
    const wrapper = mount(TicketCreateView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          PageHeader: { template: '<header><slot name="actions" /></header>' },
          TicketCategoryForm: true,
          RouterLink: { props: ['to'], template: '<a><slot /></a>' },
          Icon: true
        }
      }
    })
    await flushPromises()
    const links = wrapper.findAllComponents(Button).filter((button) => button.props('to'))
    expect(links).toHaveLength(2)
    for (const link of links) expect(link.props('to')).toEqual({ path: '/tickets', query })
    wrapper.unmount()
  })
})
