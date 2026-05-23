import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

import UsageView from '../UsageView.vue'

enableAutoUnmount(afterEach)

const { list, getStats, getSnapshotV2, getModelStats, getById } = vi.hoisted(() => {
  vi.stubGlobal('localStorage', {
    getItem: vi.fn(() => null),
    setItem: vi.fn(),
    removeItem: vi.fn(),
  })

  return {
    list: vi.fn(),
    getStats: vi.fn(),
    getSnapshotV2: vi.fn(),
    getModelStats: vi.fn(),
    getById: vi.fn(),
  }
})

const messages: Record<string, string> = {
  'admin.dashboard.timeRange': 'Time Range',
  'admin.dashboard.day': 'Day',
  'admin.dashboard.hour': 'Hour',
  'admin.usage.failedToLoadUser': 'Failed to load user',
}

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      list,
      getStats,
    },
    dashboard: {
      getSnapshotV2,
      getModelStats,
    },
    users: {
      getById,
    },
  },
}))

vi.mock('@/api/admin/usage', () => ({
  adminUsageAPI: {
    list: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showWarning: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn(),
  }),
}))

vi.mock('@/utils/format', () => ({
  formatReasoningEffort: (value: string | null | undefined) => value ?? '-',
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

vi.mock('vue-router', () => ({
  useRoute: () => ({
    query: {}
  })
}))

const AppLayoutStub = { template: '<div><slot /></div>' }
const UsageFiltersStub = {
  props: ['modelValue'],
  emits: ['update:modelValue', 'change', 'refresh', 'reset', 'cleanup', 'export'],
  template: `
    <div>
      <button
        data-test="exclude-admin-toggle"
        @click="$emit('update:modelValue', { ...modelValue, exclude_admin: !modelValue.exclude_admin }); $emit('change')"
      >
        toggle
      </button>
      <slot name="after-reset" />
    </div>
  `,
}
const ModelDistributionChartStub = {
  props: ['metric', 'modelStats'],
  emits: ['update:metric'],
  template: `
    <div data-test="model-chart">
      <span class="metric">{{ metric }}</span>
      <span class="count">{{ modelStats?.length ?? 0 }}</span>
      <button class="switch-metric" @click="$emit('update:metric', 'actual_cost')">switch</button>
    </div>
  `,
}
const GroupDistributionChartStub = {
  props: ['metric'],
  emits: ['update:metric'],
  template: `
    <div data-test="group-chart">
      <span class="metric">{{ metric }}</span>
      <button class="switch-metric" @click="$emit('update:metric', 'actual_cost')">switch</button>
    </div>
  `,
}
const UsageStatsCardsStub = {
  props: ['stats'],
  template: '<div data-test="stats-cards">{{ stats?.total_requests ?? 0 }}</div>',
}
const TokenUsageTrendStub = {
  props: ['trendData'],
  template: '<div data-test="trend-chart">{{ trendData?.length ?? 0 }}</div>',
}
const UsageTableStub = {
  props: ['data', 'loading', 'columns'],
  emits: ['userClick'],
  template: `
    <div>
      <div v-for="row in data" :key="row.id">
        <button
          data-test="user-row-button"
          @click="$emit('userClick', row.user.id)"
        >
          {{ row.user.email }}
        </button>
      </div>
    </div>
  `,
}
const UserBalanceHistoryModalStub = {
  props: ['show', 'user'],
  emits: ['close'],
  template: `
    <div v-if="show" data-test="history-modal">
      <span data-test="history-user">{{ user?.email }}</span>
      <button type="button" class="history-close" @click="$emit('close')">close</button>
    </div>
  `,
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

describe('admin UsageView distribution metric toggles', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    list.mockReset()
    getStats.mockReset()
    getSnapshotV2.mockReset()
    getModelStats.mockReset()
    getById.mockReset()

    list.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    getModelStats.mockResolvedValue({ models: [] })
    getSnapshotV2.mockResolvedValue({
      trend: [],
      models: [],
      groups: [],
    })
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('keeps model and group metric toggles independent without refetching chart data', async () => {
    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    const now = new Date()
    const yesterday = new Date(now.getTime() - 24 * 60 * 60 * 1000)
    expect(getSnapshotV2).toHaveBeenCalledWith(expect.objectContaining({
      start_date: formatLocalDate(yesterday),
      end_date: formatLocalDate(now),
      granularity: 'hour'
    }))

    const modelChart = wrapper.find('[data-test="model-chart"]')
    const groupChart = wrapper.find('[data-test="group-chart"]')

    expect(modelChart.find('.metric').text()).toBe('tokens')
    expect(groupChart.find('.metric').text()).toBe('tokens')

    await modelChart.find('.switch-metric').trigger('click')
    await flushPromises()

    expect(modelChart.find('.metric').text()).toBe('actual_cost')
    expect(groupChart.find('.metric').text()).toBe('tokens')
    expect(getSnapshotV2).toHaveBeenCalledTimes(1)

    await groupChart.find('.switch-metric').trigger('click')
    await flushPromises()

    expect(modelChart.find('.metric').text()).toBe('actual_cost')
    expect(groupChart.find('.metric').text()).toBe('actual_cost')
    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
  })

  it('keeps the latest user detail response when two rows are clicked back to back', async () => {
    list.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          user: { id: 1, email: 'first@example.com' },
          created_at: '2026-05-22T00:00:00Z',
        } as any,
        {
          id: 2,
          user: { id: 2, email: 'second@example.com' },
          created_at: '2026-05-22T00:00:00Z',
        } as any,
      ],
      total: 2,
      pages: 1,
    })

    const firstUser = createDeferred<{ id: number; email: string }>()
    const secondUser = createDeferred<{ id: number; email: string }>()
    getById.mockImplementationOnce(() => firstUser.promise)
    getById.mockImplementationOnce(() => secondUser.promise)

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: UsageTableStub,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: UserBalanceHistoryModalStub,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          EndpointDistributionChart: true,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    const userButtons = wrapper.findAll('[data-test="user-row-button"]')
    expect(userButtons).toHaveLength(2)

    await userButtons[0].trigger('click')
    await userButtons[1].trigger('click')

    expect(getById).toHaveBeenNthCalledWith(1, 1)
    expect(getById).toHaveBeenNthCalledWith(2, 2)

    secondUser.resolve({ id: 2, email: 'second@example.com' })
    await flushPromises()

    const modal = wrapper.get('[data-test="history-modal"]')
    expect(modal.text()).toContain('second@example.com')

    firstUser.resolve({ id: 1, email: 'first@example.com' })
    await flushPromises()

    expect(modal.text()).toContain('second@example.com')
  })

  it('does not reopen the balance history modal after it is closed while a lookup is still pending', async () => {
    list.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          user: { id: 1, email: 'first@example.com' },
          created_at: '2026-05-22T00:00:00Z',
        } as any,
        {
          id: 2,
          user: { id: 2, email: 'second@example.com' },
          created_at: '2026-05-22T00:00:00Z',
        } as any,
      ],
      total: 2,
      pages: 1,
    })

    const firstUser = createDeferred<{ id: number; email: string }>()
    const secondUser = createDeferred<{ id: number; email: string }>()
    getById.mockImplementationOnce(() => firstUser.promise)
    getById.mockImplementationOnce(() => secondUser.promise)

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: true,
          UsageFilters: UsageFiltersStub,
          UsageTable: UsageTableStub,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: UserBalanceHistoryModalStub,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: true,
          EndpointDistributionChart: true,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
        },
      },
    })

    vi.advanceTimersByTime(120)
    await flushPromises()

    const userButtons = wrapper.findAll('[data-test="user-row-button"]')
    expect(userButtons).toHaveLength(2)

    await userButtons[0].trigger('click')
    firstUser.resolve({ id: 1, email: 'first@example.com' })
    await flushPromises()

    expect(wrapper.get('[data-test="history-modal"] [data-test="history-user"]').text()).toBe('first@example.com')

    await userButtons[1].trigger('click')
    await wrapper.get('[data-test="history-modal"] .history-close').trigger('click')
    secondUser.resolve({ id: 2, email: 'second@example.com' })
    await flushPromises()

    expect(wrapper.find('[data-test="history-modal"]').exists()).toBe(false)
    expect(getById).toHaveBeenCalledTimes(2)
  })

  it('does not let old chart or model responses repopulate after exclude-admin is toggled', async () => {
    list.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })

    const initialSnapshot = createDeferred<{ trend: Array<{ value: number }>; groups: Array<{ name: string }> }>()
    const initialModelStats = createDeferred<{ models: Array<{ model: string }> }>()
    getSnapshotV2.mockImplementationOnce(() => initialSnapshot.promise)
    getModelStats.mockImplementationOnce(() => initialModelStats.promise)

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: UsageStatsCardsStub,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: TokenUsageTrendStub,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
          EndpointDistributionChart: true,
        },
      },
    })

    await flushPromises()
    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getModelStats).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="exclude-admin-toggle"]').trigger('click')
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(1)
    expect(getModelStats).toHaveBeenCalledTimes(1)

    initialSnapshot.resolve({
      trend: [{ value: 1 }],
      groups: [{ name: 'old-group' }],
    })
    initialModelStats.resolve({
      models: [{ model: 'old-model' }],
    })
    await flushPromises()

    expect((wrapper.vm as any).trendData).toHaveLength(0)
    expect((wrapper.vm as any).requestedModelStats).toHaveLength(0)
    expect((wrapper.vm as any).upstreamModelStats).toHaveLength(0)
    expect((wrapper.vm as any).mappingModelStats).toHaveLength(0)
  })

  it('clears the delayed chart load when the view unmounts', async () => {
    list.mockResolvedValue({
      items: [],
      total: 0,
      pages: 0,
    })
    getStats.mockResolvedValue({
      total_requests: 0,
      total_input_tokens: 0,
      total_output_tokens: 0,
      total_cache_tokens: 0,
      total_tokens: 0,
      total_cost: 0,
      total_actual_cost: 0,
      average_duration_ms: 0,
    })
    getModelStats.mockResolvedValue({ models: [] })

    const wrapper = mount(UsageView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          UsageStatsCards: UsageStatsCardsStub,
          UsageFilters: UsageFiltersStub,
          UsageTable: true,
          UsageExportProgress: true,
          UsageCleanupDialog: true,
          UserBalanceHistoryModal: true,
          Pagination: true,
          Select: true,
          DateRangePicker: true,
          Icon: true,
          TokenUsageTrend: TokenUsageTrendStub,
          ModelDistributionChart: ModelDistributionChartStub,
          GroupDistributionChart: GroupDistributionChartStub,
          EndpointDistributionChart: true,
        },
      },
    })

    await flushPromises()
    expect(getSnapshotV2).toHaveBeenCalledTimes(0)

    wrapper.unmount()
    vi.advanceTimersByTime(120)
    await flushPromises()

    expect(getSnapshotV2).toHaveBeenCalledTimes(0)
  })
})
