import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import OpsDashboard from '../OpsDashboard.vue'

const {
  getAdvancedSettingsMock,
  getMetricThresholdsMock,
  getDashboardSnapshotV2Mock,
  getThroughputTrendMock,
  getLatencyHistogramMock,
  getErrorDistributionMock,
  getDashboardOverviewMock,
  getErrorTrendMock,
  fetchAdminSettingsMock,
  routerReplaceMock,
  showErrorMock,
} = vi.hoisted(() => ({
  getAdvancedSettingsMock: vi.fn(),
  getMetricThresholdsMock: vi.fn(),
  getDashboardSnapshotV2Mock: vi.fn(),
  getThroughputTrendMock: vi.fn(),
  getLatencyHistogramMock: vi.fn(),
  getErrorDistributionMock: vi.fn(),
  getDashboardOverviewMock: vi.fn(),
  getErrorTrendMock: vi.fn(),
  fetchAdminSettingsMock: vi.fn(),
  routerReplaceMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

const adminSettingsStore = {
  opsMonitoringEnabled: true,
  opsQueryModeDefault: 'auto',
  fetch: fetchAdminSettingsMock,
}

const routeState = {
  query: {} as Record<string, unknown>,
}

class TestIntersectionObserver {
  static instances: TestIntersectionObserver[] = []

  callback: IntersectionObserverCallback
  observe = vi.fn()
  disconnect = vi.fn()
  unobserve = vi.fn()

  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    TestIntersectionObserver.instances.push(this)
  }
}

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace: routerReplaceMock,
  }),
}))

vi.mock('@/stores', () => ({
  useAdminSettingsStore: () => adminSettingsStore,
  useAppStore: () => ({
    showError: showErrorMock,
  }),
}))

vi.mock('@/api/admin/ops', () => ({
  default: {
    getAdvancedSettings: (...args: any[]) => getAdvancedSettingsMock(...args),
    getMetricThresholds: (...args: any[]) => getMetricThresholdsMock(...args),
    getDashboardSnapshotV2: (...args: any[]) => getDashboardSnapshotV2Mock(...args),
    getThroughputTrend: (...args: any[]) => getThroughputTrendMock(...args),
    getLatencyHistogram: (...args: any[]) => getLatencyHistogramMock(...args),
    getErrorDistribution: (...args: any[]) => getErrorDistributionMock(...args),
    getDashboardOverview: (...args: any[]) => getDashboardOverviewMock(...args),
    getErrorTrend: (...args: any[]) => getErrorTrendMock(...args),
  },
  opsAPI: {
    getAdvancedSettings: (...args: any[]) => getAdvancedSettingsMock(...args),
    getMetricThresholds: (...args: any[]) => getMetricThresholdsMock(...args),
    getDashboardSnapshotV2: (...args: any[]) => getDashboardSnapshotV2Mock(...args),
    getThroughputTrend: (...args: any[]) => getThroughputTrendMock(...args),
    getLatencyHistogram: (...args: any[]) => getLatencyHistogramMock(...args),
    getErrorDistribution: (...args: any[]) => getErrorDistributionMock(...args),
    getDashboardOverview: (...args: any[]) => getDashboardOverviewMock(...args),
    getErrorTrend: (...args: any[]) => getErrorTrendMock(...args),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const HeaderStub = defineComponent({
  name: 'OpsDashboardHeaderStub',
  emits: [
    'open-settings',
    'open-request-details',
    'update:customTimeRange',
    'update:timeRange',
  ],
  template: `
    <div>
      <button data-test="open-settings" @click="$emit('open-settings')">open settings</button>
      <button data-test="open-request-details" @click="$emit('open-request-details')">open request details</button>
      <button
        data-test="set-custom-range"
        @click="$emit('update:customTimeRange', '2026-06-02T00:00:00Z', '2026-06-02T02:00:00Z'); $emit('update:timeRange', 'custom')"
      >
        set custom range
      </button>
    </div>
  `,
})

const RequestDetailsModalStub = defineComponent({
  name: 'AsyncOpsRequestDetailsModal',
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
    timeRange: {
      type: String,
      default: '',
    },
    customStartTime: {
      type: String,
      default: null,
    },
    customEndTime: {
      type: String,
      default: null,
    },
  },
  template: `
    <div
      v-if="modelValue"
      data-test="request-details-modal"
      :data-time-range="timeRange"
      :data-start-time="customStartTime || ''"
      :data-end-time="customEndTime || ''"
    />
  `,
})

const SettingsDialogStub = defineComponent({
  name: 'AsyncOpsSettingsDialog',
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['saved', 'close'],
  template: `
    <div v-if="show">
      <button data-test="save-settings" @click="$emit('saved'); $emit('close')">save</button>
    </div>
  `,
})

function mountView() {
  return mount(OpsDashboard, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        BaseDialog: { template: '<div><slot /></div>' },
        OpsDashboardHeader: HeaderStub,
        OpsDashboardSkeleton: { template: '<div data-test="ops-skeleton" />' },
        OpsConcurrencyCard: true,
        OpsErrorDistributionChart: true,
        OpsErrorTrendChart: true,
        OpsLatencyChart: true,
        OpsThroughputTrendChart: true,
        OpsSwitchRateTrendChart: true,
        AsyncOpsSettingsDialog: SettingsDialogStub,
        AsyncOpsAlertEventsCard: { template: '<div data-test="alert-events-card" />' },
        AsyncOpsOpenAITokenStatsCard: { template: '<div data-test="openai-token-stats-card" />' },
        AsyncOpsSystemLogTable: { template: '<div data-test="system-log-table" />' },
        AsyncOpsAlertRulesCard: true,
        AsyncOpsErrorDetailsModal: true,
        AsyncOpsErrorDetailModal: true,
        AsyncOpsRequestDetailsModal: RequestDetailsModalStub,
      },
    },
  })
}

describe('OpsDashboard request orchestration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    TestIntersectionObserver.instances = []
    globalThis.IntersectionObserver = TestIntersectionObserver as unknown as typeof IntersectionObserver

    routeState.query = {}
    adminSettingsStore.opsMonitoringEnabled = true
    adminSettingsStore.opsQueryModeDefault = 'auto'
    adminSettingsStore.fetch = fetchAdminSettingsMock
    fetchAdminSettingsMock.mockResolvedValue(undefined)
    routerReplaceMock.mockResolvedValue(undefined)

    getAdvancedSettingsMock.mockResolvedValue({
      display_alert_events: false,
      display_openai_token_stats: false,
      auto_refresh_enabled: false,
      auto_refresh_interval_seconds: 30,
    })
    getMetricThresholdsMock.mockResolvedValue(null)
    getDashboardSnapshotV2Mock.mockResolvedValue({
      overview: null,
      throughput_trend: { points: [], by_platform: [], top_groups: [] },
      error_trend: { points: [] },
    })
    getThroughputTrendMock.mockResolvedValue({ points: [], by_platform: [], top_groups: [] })
    getLatencyHistogramMock.mockResolvedValue(null)
    getErrorDistributionMock.mockResolvedValue(null)
    getDashboardOverviewMock.mockResolvedValue(null)
    getErrorTrendMock.mockResolvedValue({ points: [] })
  })

  it('reloads advanced settings only once after saving the settings dialog', async () => {
    const wrapper = mountView()

    await flushPromises()
    expect(getAdvancedSettingsMock).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="open-settings"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-test="save-settings"]').trigger('click')
    await flushPromises()

    expect(getAdvancedSettingsMock).toHaveBeenCalledTimes(2)
  })

  it('mounts non-core lower panels only after they enter the viewport', async () => {
    getAdvancedSettingsMock.mockResolvedValue({
      display_alert_events: true,
      display_openai_token_stats: true,
      auto_refresh_enabled: false,
      auto_refresh_interval_seconds: 30,
    })

    const wrapper = mountView()

    await flushPromises()

    expect(wrapper.find('[data-test="openai-token-stats-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="alert-events-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="system-log-table"]').exists()).toBe(false)

    for (const observer of TestIntersectionObserver.instances) {
      observer.callback([{ isIntersecting: true } as IntersectionObserverEntry], observer as unknown as IntersectionObserver)
    }
    await flushPromises()

    expect(wrapper.find('[data-test="openai-token-stats-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="alert-events-card"]').exists()).toBe(true)
    expect(wrapper.find('[data-test="system-log-table"]').exists()).toBe(true)
  })

  it('does not fetch deferred visual analysis data until the section enters the viewport', async () => {
    mountView()

    await flushPromises()

    expect(getLatencyHistogramMock).not.toHaveBeenCalled()
    expect(getErrorDistributionMock).not.toHaveBeenCalled()

    for (const observer of TestIntersectionObserver.instances) {
      observer.callback([{ isIntersecting: true } as IntersectionObserverEntry], observer as unknown as IntersectionObserver)
    }
    await flushPromises()

    expect(getLatencyHistogramMock).toHaveBeenCalledTimes(1)
    expect(getErrorDistributionMock).toHaveBeenCalledTimes(1)
  })

  it('passes custom time range values to the request details modal', async () => {
    routeState.query = {
      tr: 'custom',
      start_time: '2026-06-01T00:00:00Z',
      end_time: '2026-06-01T01:30:00Z',
    }

    const wrapper = mountView()

    await flushPromises()
    await wrapper.get('[data-test="open-request-details"]').trigger('click')
    await flushPromises()

    const modal = wrapper.get('[data-test="request-details-modal"]')
    expect(modal.attributes('data-time-range')).toBe('custom')
    expect(modal.attributes('data-start-time')).toBe('2026-06-01T00:00:00Z')
    expect(modal.attributes('data-end-time')).toBe('2026-06-01T01:30:00Z')
  })

  it('refreshes dashboard data and syncs the route when custom range changes while already custom', async () => {
    routeState.query = {
      tr: 'custom',
      start_time: '2026-06-01T00:00:00Z',
      end_time: '2026-06-01T01:30:00Z',
    }

    const wrapper = mountView()

    await flushPromises()
    getDashboardSnapshotV2Mock.mockClear()
    routerReplaceMock.mockClear()

    await wrapper.get('[data-test="set-custom-range"]').trigger('click')
    await flushPromises()

    expect(getDashboardSnapshotV2Mock).toHaveBeenCalledWith(
      expect.objectContaining({
        start_time: '2026-06-02T00:00:00Z',
        end_time: '2026-06-02T02:00:00Z',
      }),
      expect.any(Object),
    )
    await new Promise((resolve) => setTimeout(resolve, 300))
    await flushPromises()

    expect(routerReplaceMock).toHaveBeenCalledWith({
      query: expect.objectContaining({
        tr: 'custom',
        start_time: '2026-06-02T00:00:00Z',
        end_time: '2026-06-02T02:00:00Z',
      }),
    })
  })
})
