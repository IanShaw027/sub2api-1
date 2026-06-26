import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, reactive } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import OpsDashboardHeader from '../OpsDashboardHeader.vue'

const { getAllGroups, getRealtimeTrafficSummary } = vi.hoisted(() => ({
  getAllGroups: vi.fn(),
  getRealtimeTrafficSummary: vi.fn()
}))

const adminSettingsStore = reactive({
  opsRealtimeMonitoringEnabled: true,
  setOpsRealtimeMonitoringEnabledLocal(enabled: boolean) {
    this.opsRealtimeMonitoringEnabled = enabled
  }
})

vi.mock('@/api', () => ({
  adminAPI: {
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/api/admin/ops', () => ({
  opsAPI: {
    getRealtimeTrafficSummary: (...args: any[]) => getRealtimeTrafficSummary(...args)
  }
}))

vi.mock('@/stores', () => ({
  useAdminSettingsStore: () => adminSettingsStore
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: null,
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: '<button data-test="select-stub" @click="$emit(\'update:modelValue\', modelValue)">{{ options.length }}</button>',
})

function mountView(props: Record<string, unknown> = {}) {
  return mount(OpsDashboardHeader, {
    props: {
      overview: null,
      platform: 'openai',
      groupId: null,
      timeRange: '1h',
      queryMode: 'auto',
      loading: false,
      lastUpdated: new Date('2026-06-25T00:00:00Z'),
      thresholds: null,
      autoRefreshEnabled: false,
      autoRefreshCountdown: 0,
      fullscreen: false,
      customStartTime: null,
      customEndTime: null,
      ...props
    },
    global: {
      stubs: {
        Select: SelectStub,
        HelpTooltip: true,
        BaseDialog: true,
        Icon: true
      }
    }
  })
}

describe('OpsDashboardHeader realtime summary orchestration', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    adminSettingsStore.opsRealtimeMonitoringEnabled = true
    getAllGroups.mockResolvedValue([])
    getRealtimeTrafficSummary.mockResolvedValue({
      enabled: true,
      summary: {
        window: '1min',
        start_time: '2026-06-25T00:00:00Z',
        end_time: '2026-06-25T00:01:00Z',
        platform: 'openai',
        group_id: 7,
        qps: { current: 1, peak: 2, avg: 1.5 },
        tps: { current: 3, peak: 4, avg: 3.5 }
      }
    })
  })

  it('mount 时只请求一次 realtime summary', async () => {
    mountView({ groupId: 7 })
    await flushPromises()

    expect(getRealtimeTrafficSummary).toHaveBeenCalledTimes(1)
    expect(getRealtimeTrafficSummary).toHaveBeenCalledWith('1min', 'openai', 7)
  })

  it('timeRange 切换时只追加一次 realtime summary 请求', async () => {
    const wrapper = mountView({ timeRange: '1h', groupId: 7 })
    await flushPromises()
    expect(getRealtimeTrafficSummary).toHaveBeenCalledTimes(1)

    await wrapper.setProps({ timeRange: '30m' })
    await flushPromises()

    expect(getRealtimeTrafficSummary).toHaveBeenCalledTimes(2)
    expect(getRealtimeTrafficSummary).toHaveBeenLastCalledWith('1min', 'openai', 7)
  })

  it('does not load groups on mount without an active group filter', async () => {
    const wrapper = mountView({ groupId: null })
    await flushPromises()

    expect(getAllGroups).not.toHaveBeenCalled()

    await wrapper.findAll('[data-test="select-stub"]')[1]?.trigger('click')
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalledTimes(1)
  })

  it('still loads groups on mount when a group filter is already selected', async () => {
    mountView({ groupId: 7 })
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalledTimes(1)
  })
})
