import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'

const listChannelMonitorViews = vi.hoisted(() => vi.fn())
const fetchChannelMonitorDetail = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/api/channelMonitor', () => ({
  list: listChannelMonitorViews,
  status: fetchChannelMonitorDetail,
}))

import ChannelStatusView from '../ChannelStatusView.vue'

describe('ChannelStatusView', () => {
  beforeEach(() => {
    const pinia = createPinia()
    setActivePinia(pinia)
    listChannelMonitorViews.mockReset()
    fetchChannelMonitorDetail.mockReset()
    window.localStorage.clear()
  })

  it('keeps the overall status operational when a fresh monitor has no history yet', async () => {
    listChannelMonitorViews.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'fresh',
          provider: 'openai',
          group_name: 'g1',
          primary_model: 'gpt-4.1',
          primary_status: '',
          primary_latency_ms: null,
          primary_ping_latency_ms: null,
          availability_7d: 0,
          extra_models: [],
          timeline: [],
        },
        {
          id: 2,
          name: 'healthy',
          provider: 'openai',
          group_name: 'g1',
          primary_model: 'gpt-4.1',
          primary_status: 'operational',
          primary_latency_ms: 120,
          primary_ping_latency_ms: 40,
          availability_7d: 100,
          extra_models: [],
          timeline: [],
        },
      ],
    })

    const wrapper = mount(ChannelStatusView, {
      global: {
        plugins: [createPinia()],
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          MonitorHero: {
            props: ['overallStatus'],
            template: '<div data-testid="overall-status">{{ overallStatus }}</div>',
          },
          MonitorCardGrid: true,
          MonitorDetailDialog: true,
        },
      },
    })

    await flushPromises()

    expect(listChannelMonitorViews).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-testid="overall-status"]').text()).toBe('operational')
  })
})
