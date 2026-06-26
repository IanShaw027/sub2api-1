import { describe, expect, it, vi, beforeEach } from 'vitest'
import { defineComponent } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import ProxiesView from '../ProxiesView.vue'

const {
  listProxies,
  getAllWithCount,
  testProxyMock,
  checkProxyQualityMock,
  batchTestAllFilteredMock,
  batchQualityCheckAllFilteredMock,
  showInfoMock,
  showSuccessMock,
  showErrorMock
} = vi.hoisted(() => ({
  listProxies: vi.fn(),
  getAllWithCount: vi.fn(),
  testProxyMock: vi.fn(),
  checkProxyQualityMock: vi.fn(),
  batchTestAllFilteredMock: vi.fn(),
  batchQualityCheckAllFilteredMock: vi.fn(),
  showInfoMock: vi.fn(),
  showSuccessMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    proxies: {
      list: listProxies,
      testProxy: testProxyMock,
      checkProxyQuality: checkProxyQualityMock,
      batchTestAllFiltered: batchTestAllFilteredMock,
      batchQualityCheckAllFiltered: batchQualityCheckAllFilteredMock,
      batchCreate: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      batchDelete: vi.fn(),
      exportData: vi.fn(),
      getProxyAccounts: vi.fn(),
      getAllWithCount,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
    showSuccess: showSuccessMock,
    showInfo: showInfoMock,
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

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    data: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div>
      <slot name="header-select" />
      <div v-for="row in data" :key="row.id" :data-test="'row-' + row.id">
        <slot name="cell-select" :row="row" />
      </div>
      <div data-test="proxy-data">{{ data.map((row) => \`\${row.latency_ms ?? ""}|\${row.country ?? ""}|\${row.country_code ?? ""}|\${row.ip_address ?? ""}|\${row.region ?? ""}|\${row.city ?? ""}\`).join(",") }}</div>
    </div>
  `,
})

const ConfirmDialogStub = defineComponent({
  name: 'ConfirmDialogStub',
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
    message: {
      type: String,
      default: '',
    },
  },
  emits: ['confirm', 'cancel'],
  template: `
    <div v-if="show" data-test="confirm-dialog" :data-title="title" :data-message="message">
      <button data-test="confirm-dialog-confirm" @click="$emit('confirm')">confirm</button>
      <button data-test="confirm-dialog-cancel" @click="$emit('cancel')">cancel</button>
    </div>
  `,
})

function mountView() {
  return mount(ProxiesView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot /></div>',
        },
        DataTable: DataTableStub,
        Pagination: true,
        BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
        ConfirmDialog: ConfirmDialogStub,
        EmptyState: true,
        ImportDataModal: true,
        Select: true,
        ProxyAdBanner: true,
        Icon: true,
        PlatformTypeBadge: true,
      },
    },
  })
}

describe('ProxiesView proxy quality state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getAllWithCount.mockResolvedValue([])
    testProxyMock.mockResolvedValue({
      success: true,
      latency_ms: 12,
      message: 'ok',
    })
    batchTestAllFilteredMock.mockResolvedValue({
      total: 1,
      success: 1,
      failed: 0,
    })
    checkProxyQualityMock.mockResolvedValue({
      score: 100,
      grade: 'A',
      summary: 'ok',
      checked_at: 1710000000,
      failed_count: 0,
      warn_count: 0,
      challenge_count: 0,
    })
    batchQualityCheckAllFilteredMock.mockResolvedValue({
      total: 1,
      healthy: 1,
      warn: 0,
      challenge: 0,
      failed: 0,
    })
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

    const wrapper = mountView()

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

  it('does not load backup proxy options until create modal is opened', async () => {
    listProxies.mockResolvedValueOnce({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()

    await flushPromises()
    expect(getAllWithCount).not.toHaveBeenCalled()

    const createButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.createProxy'))
    expect(createButton).toBeDefined()
    await createButton?.trigger('click')
    await flushPromises()

    expect(getAllWithCount).toHaveBeenCalledTimes(1)
  })

  it('uses selected proxies for batch connection tests without scanning all filtered pages', async () => {
    listProxies.mockResolvedValue({
      items: [
        {
          id: 7,
          name: 'proxy-7',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          status: 'active',
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('[data-test="row-7"] input[type="checkbox"]').setValue(true)
    await flushPromises()

    const batchTestButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.testConnection'))
    expect(batchTestButton).toBeDefined()
    await batchTestButton?.trigger('click')
    await flushPromises()

    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(testProxyMock).toHaveBeenCalledWith(7)
  })

  it('asks for confirmation before scanning all filtered proxies for batch tests', async () => {
    listProxies.mockResolvedValue({
      items: [
        {
          id: 11,
          name: 'proxy-11',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          status: 'active',
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    const batchTestButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.testConnection'))
    expect(batchTestButton).toBeDefined()
    await batchTestButton?.trigger('click')
    await flushPromises()

    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(testProxyMock).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)

    await wrapper.get('[data-test="confirm-dialog-cancel"]').trigger('click')
    await flushPromises()

    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(testProxyMock).not.toHaveBeenCalled()
  })

  it('uses a single batch endpoint after confirming all-filtered batch tests', async () => {
    listProxies.mockResolvedValue({
      items: [
        {
          id: 21,
          name: 'proxy-21',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          status: 'active',
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    const batchTestButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.testConnection'))
    expect(batchTestButton).toBeDefined()
    await batchTestButton?.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="confirm-dialog-confirm"]').trigger('click')
    await flushPromises()

    expect(batchTestAllFilteredMock).toHaveBeenCalledTimes(1)
    expect(batchTestAllFilteredMock).toHaveBeenCalledWith({
      protocol: undefined,
      status: undefined,
      search: undefined,
      sort_by: 'id',
      sort_order: 'desc'
    })
    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(testProxyMock).not.toHaveBeenCalled()
  })

  it('asks for confirmation before scanning all filtered proxies for batch quality checks', async () => {
    listProxies.mockResolvedValue({
      items: [
        {
          id: 13,
          name: 'proxy-13',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          status: 'active',
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    const batchQualityButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.batchQualityCheck'))
    expect(batchQualityButton).toBeDefined()
    await batchQualityButton?.trigger('click')
    await flushPromises()

    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(checkProxyQualityMock).not.toHaveBeenCalled()
    expect(wrapper.find('[data-test="confirm-dialog"]').exists()).toBe(true)
  })

  it('uses a single batch endpoint after confirming all-filtered batch quality checks', async () => {
    listProxies.mockResolvedValue({
      items: [
        {
          id: 31,
          name: 'proxy-31',
          protocol: 'http',
          host: '127.0.0.1',
          port: 8080,
          status: 'active',
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    const wrapper = mountView()
    await flushPromises()

    const batchQualityButton = wrapper.findAll('button').find((node) => node.text().includes('admin.proxies.batchQualityCheck'))
    expect(batchQualityButton).toBeDefined()
    await batchQualityButton?.trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="confirm-dialog-confirm"]').trigger('click')
    await flushPromises()

    expect(batchQualityCheckAllFilteredMock).toHaveBeenCalledTimes(1)
    expect(batchQualityCheckAllFilteredMock).toHaveBeenCalledWith({
      protocol: undefined,
      status: undefined,
      search: undefined,
      sort_by: 'id',
      sort_order: 'desc'
    })
    expect(listProxies.mock.calls.some(([, pageSize]) => pageSize === 200)).toBe(false)
    expect(checkProxyQualityMock).not.toHaveBeenCalled()
  })
})
