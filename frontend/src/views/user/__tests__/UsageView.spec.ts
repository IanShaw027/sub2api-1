import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, h, nextTick } from 'vue'

import UsageView from '../UsageView.vue'

const { query, getStatsByDateRange, list, showError, showWarning, showSuccess, showInfo } = vi.hoisted(() => ({
  query: vi.fn(),
  getStatsByDateRange: vi.fn(),
  list: vi.fn(),
  showError: vi.fn(),
  showWarning: vi.fn(),
  showSuccess: vi.fn(),
  showInfo: vi.fn(),
}))

const messages: Record<string, string> = {
  'usage.costDetails': 'Cost Breakdown',
  'admin.usage.inputCost': 'Input Cost',
  'admin.usage.outputCost': 'Output Cost',
  'admin.usage.cacheCreationCost': 'Cache Creation Cost',
  'admin.usage.cacheReadCost': 'Cache Read Cost',
  'usage.inputTokenPrice': 'Input price',
  'usage.outputTokenPrice': 'Output price',
  'usage.perMillionTokens': '/ 1M tokens',
  'usage.imageCount': 'Image count',
  'usage.imageUnit': ' images',
  'usage.imageUnitPrice': 'Per-image price',
  'usage.imageSubtotal': 'Image subtotal',
  'usage.tokenSubtotal': 'Token subtotal',
  'usage.rowTotal': 'Row total',
  'usage.serviceTier': 'Service tier',
  'usage.serviceTierPriority': 'Fast',
  'usage.serviceTierFlex': 'Flex',
  'usage.serviceTierStandard': 'Standard',
  'usage.rate': 'Rate',
  'usage.original': 'Original',
  'usage.billed': 'Billed',
  'usage.allApiKeys': 'All API Keys',
  'usage.apiKeyFilter': 'API Key',
  'usage.model': 'Model',
  'usage.reasoningEffort': 'Reasoning Effort',
  'usage.type': 'Type',
  'usage.tokens': 'Tokens',
  'usage.cost': 'Cost',
  'usage.firstToken': 'First Token',
  'usage.duration': 'Duration',
  'usage.time': 'Time',
  'usage.userAgent': 'User Agent',
  'usage.endpoint': 'Endpoint',
  'usage.video': 'Video',
}

vi.mock('@/api', () => ({
  usageAPI: {
    query,
    getStatsByDateRange,
  },
  keysAPI: {
    list,
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError, showWarning, showSuccess, showInfo }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const AppLayoutStub = { template: '<div><slot /></div>' }
const TablePageLayoutStub = {
  template: '<div><slot name="actions" /><slot name="filters" /><slot name="table" /><slot /></div>',
}
const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    columns: { type: Array, required: true },
    data: { type: Array, required: true },
  },
  setup(props, { slots }) {
    return () => h('div', [
      ...(props.data as Array<Record<string, unknown>>).flatMap((row, rowIndex) =>
        (props.columns as Array<{ key: string }>).map((col) => {
          const slotName = `cell-${col.key}`
          const slot = slots[slotName]
          const value = row[col.key]
          return h('div', { key: `${rowIndex}-${String(col.key)}` }, slot
            ? slot({ value, row })
            : String(value ?? ''),
          )
        }),
      ),
    ])
  },
})

describe('user UsageView tooltip', () => {
  beforeEach(() => {
    query.mockReset()
    getStatsByDateRange.mockReset()
    list.mockReset()
    showError.mockReset()
    showWarning.mockReset()
    showSuccess.mockReset()
    showInfo.mockReset()

    vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockReturnValue({
      x: 0,
      y: 0,
      top: 20,
      left: 20,
      right: 120,
      bottom: 40,
      width: 100,
      height: 20,
      toJSON: () => ({}),
    } as DOMRect)

    ;(globalThis as any).ResizeObserver = class {
      observe() {}
      disconnect() {}
    }
  })

  it('shows fast service tier and unit prices in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-1',
          actual_cost: 0.092883,
          total_cost: 0.092883,
          rate_multiplier: 1,
          service_tier: 'priority',
          input_cost: 0.020285,
          output_cost: 0.00303,
          cache_creation_cost: 0,
          cache_read_cost: 0.069568,
          input_tokens: 4057,
          output_tokens: 101,
          cache_creation_tokens: 0,
          cache_read_tokens: 278272,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        mocks: {
          $t: (key: string) => messages[key] ?? key,
        },
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-1',
      actual_cost: 0.092883,
      total_cost: 0.092883,
      rate_multiplier: 1,
      service_tier: 'priority',
      input_cost: 0.020285,
      output_cost: 0.00303,
      cache_creation_cost: 0,
      cache_read_cost: 0.069568,
      input_tokens: 4057,
      output_tokens: 101,
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Service tier')
    expect(text).toContain('Fast')
    expect(text).toContain('Rate')
    expect(text).toContain('1.00x')
    expect(text).toContain('Billed')
    expect(text).toContain('$0.092883')
    expect(text).toContain('$5.0000 / 1M tokens')
    expect(text).toContain('$30.0000 / 1M tokens')
  })

  it('labels video usage rows as video requests', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-video',
          model: 'grok-4.3',
          request_type: 'video',
          actual_cost: 0,
          total_cost: 0,
          rate_multiplier: 1,
          service_tier: null,
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 0,
      total_cost: 0,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    expect(wrapper.text()).toContain('Video')
  })

  it('exports csv with input and output unit price columns', async () => {
    const exportedLogs = [
      {
        request_id: 'req-user-export',
        actual_cost: 0.092883,
        total_cost: 0.092883,
        rate_multiplier: 1,
        service_tier: 'priority',
        input_cost: 0.020285,
        output_cost: 0.00303,
        cache_creation_cost: 0.000001,
        cache_read_cost: 0.069568,
        input_tokens: 4057,
        output_tokens: 101,
        cache_creation_tokens: 4,
        cache_read_tokens: 278272,
        cache_creation_5m_tokens: 0,
        cache_creation_1h_tokens: 0,
        image_count: 0,
        image_size: null,
        first_token_ms: 12,
        duration_ms: 345,
        created_at: '2026-03-08T00:00:00Z',
        model: 'gpt-5.4',
        reasoning_effort: null,
        api_key: { name: 'demo-key' },
      },
    ]

    query.mockResolvedValue({
      items: exportedLogs,
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    let exportedBlob: Blob | null = null
    const originalCreateObjectURL = window.URL.createObjectURL
    const originalRevokeObjectURL = window.URL.revokeObjectURL
    window.URL.createObjectURL = vi.fn((blob: Blob | MediaSource) => {
      exportedBlob = blob as Blob
      return 'blob:usage-export'
    }) as typeof window.URL.createObjectURL
    window.URL.revokeObjectURL = vi.fn(() => {}) as typeof window.URL.revokeObjectURL
    const clickSpy = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {})

    const wrapper = mount(UsageView, {
      global: {
        mocks: {
          $t: (key: string) => messages[key] ?? key,
        },
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()

    const setupState = (wrapper.vm as any).$?.setupState
    await setupState.exportToCSV()

    expect(exportedBlob).not.toBeNull()
    const hasSortedExportQuery = query.mock.calls.some((call) => {
      const params = call[0] as Record<string, unknown> | undefined
      const config = call[1]
      return (
        params?.page_size === 100 &&
        params?.sort_by === 'created_at' &&
        params?.sort_order === 'desc' &&
        config === undefined
      )
    })
    expect(hasSortedExportQuery).toBe(true)
    expect(clickSpy).toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()

    window.URL.createObjectURL = originalCreateObjectURL
    window.URL.revokeObjectURL = originalRevokeObjectURL
    clickSpy.mockRestore()
  })

  it('shows upstream model and highlights higher upstream billing in red', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-route-1',
          model: 'gpt-5.4-mini',
          upstream_model: 'gpt-5.4',
          model_mapping_chain: 'gpt-5.4-mini→gpt-5.4',
          actual_cost: 0.120001,
          total_cost: 0.120001,
          billed_by_higher_priced_upstream: true,
          rate_multiplier: 1,
          service_tier: 'standard',
          input_cost: 0.1,
          output_cost: 0.02,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 1000,
          output_tokens: 200,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 0,
          image_size: null,
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 1200,
      total_cost: 0.120001,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        mocks: {
          $t: (key: string) => messages[key] ?? key,
        },
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('gpt-5.4-mini')
    expect(text).toContain('gpt-5.4')
    expect(text).toContain('route')
    expect(wrapper.html()).toContain('text-red-600')
  })

  it('splits image and token subtotals in user tooltip for image billing rows', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-image-1',
          billing_mode: 'image',
          inbound_endpoint: '/v1/responses',
          upstream_endpoint: '/v1/responses',
          actual_cost: 0.147388,
          total_cost: 0.147388,
          rate_multiplier: 1,
          service_tier: 'standard',
          input_cost: 0.04,
          output_cost: 0.02,
          cache_creation_cost: 0,
          cache_read_cost: 0.007388,
          input_tokens: 4806,
          output_tokens: 1213,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 1,
          image_size: '2K',
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-image-1',
      billing_mode: 'image',
      inbound_endpoint: '/v1/responses',
      upstream_endpoint: '/v1/responses',
      actual_cost: 0.147388,
      total_cost: 0.147388,
      rate_multiplier: 1,
      service_tier: 'standard',
      input_cost: 0.04,
      output_cost: 0.02,
      cache_creation_cost: 0,
      cache_read_cost: 0.007388,
      input_tokens: 4806,
      output_tokens: 1213,
      image_count: 1,
      image_size: '2K',
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Image subtotal')
    expect(text).toContain('$0.080000')
    expect(text).toContain('Token subtotal')
    expect(text).toContain('$0.067388')
    expect(text).toContain('Row total')
    expect(text).toContain('$0.147388')
  })

  it('shows pure image billing rows without token subtotal cost in user tooltip', async () => {
    query.mockResolvedValue({
      items: [
        {
          request_id: 'req-user-image-only-1',
          billing_mode: 'image',
          inbound_endpoint: '/v1/images/generations',
          upstream_endpoint: '/v1/images/generations',
          actual_cost: 0.08,
          total_cost: 0.08,
          rate_multiplier: 1,
          service_tier: 'standard',
          input_cost: 0,
          output_cost: 0,
          cache_creation_cost: 0,
          cache_read_cost: 0,
          input_tokens: 0,
          output_tokens: 0,
          cache_creation_tokens: 0,
          cache_read_tokens: 0,
          cache_creation_5m_tokens: 0,
          cache_creation_1h_tokens: 0,
          image_count: 2,
          image_size: '1K',
          first_token_ms: null,
          duration_ms: 1,
          created_at: '2026-03-08T00:00:00Z',
        },
      ],
      total: 1,
      pages: 1,
    })
    getStatsByDateRange.mockResolvedValue({
      total_requests: 1,
      total_tokens: 100,
      total_cost: 0.1,
      avg_duration_ms: 1,
    })
    list.mockResolvedValue({ items: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          TablePageLayout: TablePageLayoutStub,
          Pagination: true,
          EmptyState: true,
          DataTable: DataTableStub,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          Teleport: true,
        },
      },
    })

    await flushPromises()
    await nextTick()

    const setupState = (wrapper.vm as any).$?.setupState
    setupState.tooltipData = {
      request_id: 'req-user-image-only-1',
      billing_mode: 'image',
      inbound_endpoint: '/v1/images/generations',
      upstream_endpoint: '/v1/images/generations',
      actual_cost: 0.08,
      total_cost: 0.08,
      rate_multiplier: 1,
      service_tier: 'standard',
      input_cost: 0,
      output_cost: 0,
      cache_creation_cost: 0,
      cache_read_cost: 0,
      input_tokens: 0,
      output_tokens: 0,
      image_count: 2,
      image_size: '1K',
    }
    setupState.tooltipVisible = true
    await nextTick()

    const text = wrapper.text()
    expect(text).toContain('Per-image price')
    expect(text).toContain('$0.040000')
    expect(text).toContain('Image subtotal')
    expect(text).toContain('Token subtotal')
    expect(text).toContain('Row total')
    expect(text).toContain('$0.080000')
  })
})
