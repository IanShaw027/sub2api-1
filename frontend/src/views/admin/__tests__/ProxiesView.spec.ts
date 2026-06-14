import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import ProxiesView from '../ProxiesView.vue'

const { listProxies } = vi.hoisted(() => ({
  listProxies: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      list: listProxies,
      testProxy: vi.fn(),
      checkProxyQuality: vi.fn(),
      batchCreate: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      exportData: vi.fn(),
      getProxyAccounts: vi.fn(),
      getAllWithCount: vi.fn().mockResolvedValue([]),
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true),
  }),
}))

vi.mock('@/composables/useSwipeSelect', () => ({
  useSwipeSelect: vi.fn(),
}))

vi.mock('@/composables/usePersistedPageSize', () => ({
  getPersistedPageSize: () => 20,
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

describe('ProxiesView proxy quality state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('clears stale latency and geolocation fields from failed proxy checks returned by the list API', async () => {
    listProxies.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'proxy-1',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          username: null,
          password: null,
          status: 'active',
          latency_status: 'failed',
          latency_ms: 123,
          latency_message: 'timeout',
          ip_address: '203.0.113.10',
          country: 'United States',
          country_code: 'US',
          region: 'California',
          city: 'Los Angeles',
          quality_status: 'failed',
          quality_score: 0,
          quality_grade: 'F',
          quality_summary: 'failed',
          quality_checked: 1710000000,
          created_at: '2026-05-24T00:00:00Z',
          updated_at: '2026-05-24T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mount(ProxiesView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot /></div>',
          },
          DataTable: {
            props: ['data'],
            template: '<div data-test="proxy-data">{{ data.map((row) => `${row.latency_ms ?? ""}|${row.country ?? ""}|${row.country_code ?? ""}|${row.ip_address ?? ""}|${row.region ?? ""}|${row.city ?? ""}`).join(",") }}</div>',
          },
          Pagination: true,
          BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
          ConfirmDialog: true,
          EmptyState: true,
          ImportDataModal: true,
          Select: true,
          ProxyAdBanner: true,
          Icon: true,
          PlatformTypeBadge: true,
        },
      },
    })

    await flushPromises()

    expect((wrapper.vm as any).proxies[0]).toMatchObject({
      latency_status: 'failed',
      latency_ms: undefined,
      ip_address: undefined,
      country: undefined,
      country_code: undefined,
      region: undefined,
      city: undefined,
    })
    expect(wrapper.get('[data-test="proxy-data"]').text()).not.toContain('United States')
    expect(wrapper.get('[data-test="proxy-data"]').text()).not.toContain('203.0.113.10')
  })
})
