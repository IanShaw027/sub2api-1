import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillRuntimeMonitorView from '../SkillRuntimeMonitorView.vue'

const { getRuntimeOverview } = vi.hoisted(() => ({
  getRuntimeOverview: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, maybeArg?: unknown) => (typeof maybeArg === 'string' ? maybeArg : key),
    }),
  }
})

vi.mock('@/api/admin/skills', () => ({
  __esModule: true,
  default: {
    getRuntimeOverview,
  },
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
  }),
}))

const AppLayoutStub = defineComponent({
  name: 'AppLayoutStub',
  template: '<div><slot /></div>',
})

const TablePageLayoutStub = defineComponent({
  name: 'TablePageLayoutStub',
  template: `
    <div>
      <div data-test="filters"><slot name="filters" /></div>
      <div data-test="actions"><slot name="actions" /></div>
      <div data-test="table"><slot name="table" /></div>
      <div data-test="pagination"><slot name="pagination" /></div>
    </div>
  `,
})

const InputStub = defineComponent({
  name: 'InputStub',
  props: {
    label: {
      type: String,
      default: '',
    },
    placeholder: {
      type: String,
      default: '',
    },
  },
  template: '<div class="input-stub">{{ label }}|{{ placeholder }}</div>',
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    options: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div class="select-stub">
      <span v-for="option in options" :key="String(option.value)" class="select-option">{{ option.label }}</span>
    </div>
  `,
})

const EmptyStateStub = defineComponent({
  name: 'EmptyStateStub',
  props: {
    title: {
      type: String,
      default: '',
    },
    description: {
      type: String,
      default: '',
    },
  },
  template: '<div class="empty-state-stub">{{ title }}|{{ description }}</div>',
})

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    columns: {
      type: Array,
      default: () => [],
    },
    data: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div class="data-table-stub">
      <span v-for="column in columns" :key="String(column.key)" class="column-label">{{ column.label }}</span>
      <template v-for="row in data" :key="String(row.id)">
        <slot name="cell-health_status" :value="row.health_status" />
        <slot name="cell-actions" :row="row" />
      </template>
      <slot v-if="data.length === 0" name="empty" />
    </div>
  `,
})

const SkillAdminMetricGridStub = defineComponent({
  name: 'SkillAdminMetricGridStub',
  props: {
    items: {
      type: Array,
      default: () => [],
    },
  },
  template: `
    <div class="metric-grid-stub">
      <div v-for="item in items" :key="item.key" class="metric-item">{{ item.label }}|{{ item.hint }}</div>
    </div>
  `,
})

const SkillAdminStatusBadgeStub = defineComponent({
  name: 'SkillAdminStatusBadgeStub',
  props: {
    label: {
      type: String,
      default: '',
    },
  },
  template: '<span class="status-badge-stub">{{ label }}</span>',
})

function mountView() {
  return mount(SkillRuntimeMonitorView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        TablePageLayout: TablePageLayoutStub,
        Input: InputStub,
        Select: SelectStub,
        DataTable: DataTableStub,
        EmptyState: EmptyStateStub,
        Icon: true,
        Pagination: true,
        SkillAdminMetricGrid: SkillAdminMetricGridStub,
        SkillAdminStatusBadge: SkillAdminStatusBadgeStub,
      },
    },
  })
}

describe('SkillRuntimeMonitorView i18n wiring', () => {
  beforeEach(() => {
    getRuntimeOverview.mockReset()
  })

  it('routes core filter, metric, column, and empty-state copy through runtime translation keys', async () => {
    getRuntimeOverview.mockResolvedValue({
      items: [],
      events: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 1,
      summary: {
        total_skills: 0,
        active_skills: 0,
        requests_24h: 0,
        success_rate: 0,
        p95_latency_ms: 0,
        warning_count: 0,
        critical_count: 0,
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('健康状态')
    expect(wrapper.text()).toContain('当前运行指标由后端按真实运行数据汇总；页面不再伪造前端统计窗口切换。')
    expect(wrapper.text()).toContain('活跃技能')
    expect(wrapper.text()).toContain('skills.admin.runtime.metrics.totalSkills')
    expect(wrapper.text()).toContain('技能 / 版本')
    expect(wrapper.text()).toContain('暂无运行监控数据')
    expect(wrapper.text()).toContain('技能中心运行指标、告警和排队情况会展示在这里。')
    expect(wrapper.get('[data-test="runtime-detail-empty"]').text()).toContain('选择一条技能运行记录后，这里会展示延迟、错误和队列指标。')
    expect(wrapper.get('[data-test="runtime-events-empty"]').text()).toContain('当前没有告警事件。')
    expect(wrapper.text()).toContain('最近告警 / 事件')
    expect(wrapper.text()).toContain('默认展示全局事件；选中技能后优先过滤关联事件。')
  })

  it('uses runtime translation keys for health options and event severity labels', async () => {
    getRuntimeOverview.mockResolvedValue({
      items: [
        {
          id: 1,
          skill_id: 10,
          skill_name: 'Latency Guard',
          skill_slug: 'latency-guard',
          current_version: 'v1.0.0',
          health_status: 'warning',
          requests_24h: 12,
          success_rate: 96.2,
          p95_latency_ms: 142,
          error_rate: 3.8,
          queue_depth: 4,
          last_run_at: '2026-05-07T10:00:00Z',
          last_alert_at: '2026-05-07T10:05:00Z',
          last_error: null,
        },
      ],
      events: [
        {
          id: 99,
          skill_id: 10,
          skill_name: '',
          level: 'warning',
          message: 'latency spike',
          metric_name: 'p95_latency_ms',
          metric_value: 142,
          created_at: '2026-05-07T10:05:00Z',
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1,
      summary: {
        total_skills: 1,
        active_skills: 1,
        requests_24h: 12,
        success_rate: 96.2,
        p95_latency_ms: 142,
        warning_count: 1,
        critical_count: 0,
      },
    })

    const wrapper = mountView()
    await flushPromises()

    expect(wrapper.text()).toContain('健康')
    expect(wrapper.text()).toContain('预警')
    expect(wrapper.text()).toContain('严重')
    expect(wrapper.text()).toContain('全局事件')
    expect(wrapper.text()).toContain('当前技能没有最近错误记录。')
    expect(wrapper.text()).toContain('查看')
  })
})
