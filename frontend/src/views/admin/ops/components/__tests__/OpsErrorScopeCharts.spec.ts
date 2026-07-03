import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent } from 'vue'
import OpsErrorDistributionChart from '../OpsErrorDistributionChart.vue'
import OpsErrorTrendChart from '../OpsErrorTrendChart.vue'

vi.mock('chart.js', () => ({
  Chart: { register: vi.fn() },
  ArcElement: {},
  CategoryScale: {},
  Filler: {},
  Legend: {},
  LineElement: {},
  LinearScale: {},
  PointElement: {},
  Title: {},
  Tooltip: {},
}))

vi.mock('vue-chartjs', async () => {
  const { defineComponent } = await import('vue')

  return {
    Doughnut: defineComponent({
      name: 'Doughnut',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, default: () => ({}) },
      },
      template: '<div class="doughnut-stub" />',
    }),
    Line: defineComponent({
      name: 'LineChartStub',
      props: {
        data: { type: Object, required: true },
        options: { type: Object, default: () => ({}) },
      },
      template: '<div class="line-stub" />',
    }),
  }
})

vi.mock('../../utils/opsFormatters', () => ({
  formatHistoryLabel: (date: string | undefined) => date ?? '',
  sumNumbers: (values: Array<number | null | undefined>) =>
    values.reduce<number>((total, value) => total + (typeof value === 'number' && Number.isFinite(value) ? value : 0), 0),
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

const HelpTooltipStub = defineComponent({
  name: 'HelpTooltip',
  props: {
    content: { type: String, default: '' },
  },
  template: '<span class="help-tooltip-stub" />',
})

const EmptyStateStub = defineComponent({
  name: 'EmptyState',
  props: {
    title: { type: String, default: '' },
    description: { type: String, default: '' },
  },
  template: '<div class="empty-state-stub" />',
})

const globalStubs = {
  stubs: {
    HelpTooltip: HelpTooltipStub,
    EmptyState: EmptyStateStub,
  },
}

describe('Ops SLA-scoped error charts', () => {
  it('错误分布图按 SLA 错误数统计，不把业务限制错误算进请求错误分布', () => {
    const wrapper = mount(OpsErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 10,
          recovered_telemetry_total: 0,
          items: [
            { status_code: 400, total: 7, sla: 2, business_limited: 5 },
            { status_code: 503, total: 3, sla: 0, business_limited: 3 },
          ],
          owners: [],
        },
      },
      global: globalStubs,
    })

    const doughnut = wrapper.findComponent({ name: 'Doughnut' })
    expect(doughnut.exists()).toBe(true)
    expect(doughnut.props('data')).toMatchObject({
      labels: ['admin.ops.client'],
      datasets: [{ data: [2] }],
    })
  })

  it('错误分布图在只有业务限制错误时显示为空态', () => {
    const wrapper = mount(OpsErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 4,
          recovered_telemetry_total: 0,
          items: [{ status_code: 500, total: 4, sla: 0, business_limited: 4 }],
          owners: [],
        },
      },
      global: globalStubs,
    })

    expect(wrapper.findComponent({ name: 'Doughnut' }).exists()).toBe(false)
    expect(wrapper.find('.empty-state-stub').exists()).toBe(true)
  })

  it('错误趋势图的请求错误详情按钮只按 SLA 错误启用', () => {
    const wrapper = mount(OpsErrorTrendChart, {
      props: {
        loading: false,
        timeRange: '1h',
        points: [
          {
            bucket_start: '2026-05-18T00:00:00Z',
            error_count_total: 5,
            business_limited_count: 5,
            error_count_sla: 0,
            upstream_error_count_excl_429_529: 0,
            upstream_429_count: 0,
            upstream_529_count: 0,
            recovered_telemetry_count: 0,
          },
        ],
      },
      global: globalStubs,
    })

    const requestErrorsButton = wrapper.findAll('button')[0]
    expect(requestErrorsButton.attributes('disabled')).toBeDefined()
  })


  it('错误趋势图单独显示 recovered telemetry，不计入错误详情入口', async () => {
    const wrapper = mount(OpsErrorTrendChart, {
      props: {
        loading: false,
        timeRange: '1h',
        points: [
          {
            bucket_start: '2026-05-18T00:00:00Z',
            error_count_total: 0,
            business_limited_count: 0,
            error_count_sla: 0,
            upstream_error_count_excl_429_529: 0,
            upstream_429_count: 0,
            upstream_529_count: 0,
            recovered_telemetry_count: 7,
          },
        ],
      },
      global: globalStubs,
    })

    const line = wrapper.findComponent({ name: 'LineChartStub' })
    expect(line.exists()).toBe(true)
    expect(line.props('data')).toMatchObject({
      datasets: expect.arrayContaining([
        expect.objectContaining({
          label: 'admin.ops.recoveredTelemetry',
          data: [7],
        }),
      ]),
    })
    const buttons = wrapper.findAll('button')
    expect(buttons[0].attributes('disabled')).toBeDefined()
    expect(buttons[1].attributes('disabled')).toBeDefined()
    await buttons[1].trigger('click')
    expect(wrapper.emitted('openUpstreamErrors')).toBeUndefined()
  })

  it('错误趋势图只有 upstream 429/529 时不启用默认 upstream errors 入口', async () => {
    const wrapper = mount(OpsErrorTrendChart, {
      props: {
        loading: false,
        timeRange: '1h',
        points: [
          {
            bucket_start: '2026-05-18T00:00:00Z',
            error_count_total: 2,
            business_limited_count: 2,
            error_count_sla: 0,
            upstream_error_count_excl_429_529: 0,
            upstream_429_count: 1,
            upstream_529_count: 1,
            recovered_telemetry_count: 0,
          },
        ],
      },
      global: globalStubs,
    })

    const buttons = wrapper.findAll('button')
    expect(buttons[1].attributes('disabled')).toBeDefined()
    await buttons[1].trigger('click')
    expect(wrapper.emitted('openUpstreamErrors')).toBeUndefined()
  })

  it('错误分布图显示 recovered telemetry 总数但不并入 SLA 甜甜圈', () => {
    const wrapper = mount(OpsErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 4,
          recovered_telemetry_total: 9,
          items: [{ status_code: 503, total: 4, sla: 4, business_limited: 0 }],
          owners: [{ owner: 'provider', total: 4, sla: 4, business_limited: 0 }],
        },
      },
      global: globalStubs,
    })

    const doughnut = wrapper.findComponent({ name: 'Doughnut' })
    expect(doughnut.exists()).toBe(true)
    expect(doughnut.props('data')).toMatchObject({ datasets: [{ data: [4] }] })
    expect(wrapper.text()).toContain('admin.ops.recoveredTelemetry: 9')
  })

  it('错误分布图在只有 recovered telemetry 时显示独立标记但不渲染 SLA 甜甜圈', () => {
    const wrapper = mount(OpsErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 0,
          recovered_telemetry_total: 9,
          items: [],
          owners: [],
        },
      },
      global: globalStubs,
    })

    expect(wrapper.findComponent({ name: 'Doughnut' }).exists()).toBe(false)
    expect(wrapper.find('.empty-state-stub').exists()).toBe(true)
    expect(wrapper.text()).toContain('admin.ops.recoveredTelemetry: 9')
  })

  it('owner 甜甜圈按 SLA scope 统计 owner 分布，不使用 raw total', () => {
    const wrapper = mount(OpsErrorDistributionChart, {
      props: {
        loading: false,
        data: {
          total: 14,
          recovered_telemetry_total: 0,
          items: [
            { status_code: 400, total: 10, sla: 3, business_limited: 7 },
            { status_code: 500, total: 4, sla: 1, business_limited: 3 },
          ],
          owners: [
            { owner: 'provider', total: 10, sla: 3, business_limited: 7 },
            { owner: 'platform', total: 4, sla: 1, business_limited: 3 },
          ],
        },
      },
      global: globalStubs,
    })

    const doughnut = wrapper.findComponent({ name: 'Doughnut' })
    expect(doughnut.exists()).toBe(true)
    expect(doughnut.props('data')).toMatchObject({
      labels: [
        'admin.ops.errorDetails.owner.provider',
        'admin.ops.errorDetails.owner.platform',
      ],
      datasets: [{ data: [3, 1] }],
    })
  })
})
