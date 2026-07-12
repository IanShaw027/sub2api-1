import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UsageProgressBar from '../UsageProgressBar.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('UsageProgressBar', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-03-17T00:00:00Z'))
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('showNowWhenIdle=true 且利用率为 0 时显示“现在”', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 0,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: true,
        color: 'indigo'
      }
    })

    expect(wrapper.text()).toContain('usage.resetNow')
    expect(wrapper.text()).not.toContain('2h 30m')
  })

  it('showNowWhenIdle=true 但利用率大于 0 时显示倒计时', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '7d',
        utilization: 12,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: true,
        color: 'emerald'
      }
    })

    expect(wrapper.text()).toContain('2h 30m')
    expect(wrapper.text()).not.toContain('usage.resetNow')
    expect(wrapper.text()).not.toContain('usage.resetPending')
  })

  it('showNowWhenIdle=false 时保持原有倒计时行为', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '1d',
        utilization: 0,
        resetsAt: '2026-03-17T02:30:00Z',
        showNowWhenIdle: false,
        color: 'indigo'
      }
    })

    expect(wrapper.text()).toContain('2h 30m')
    expect(wrapper.text()).not.toContain('usage.resetNow')
  })

  it('resetsAt 已过期且利用率大于 0 时显示「待刷新」', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 53,
        // 早于 fake system time 2026-03-17T00:00:00Z
        resetsAt: '2026-03-16T22:00:00Z',
        color: 'indigo'
      }
    })

    expect(wrapper.text()).toContain('usage.resetPending')
    expect(wrapper.text()).not.toContain('usage.resetNow')
  })

  it('resetsAt 已过期且利用率为 0 时仍显示「现在」', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 0,
        resetsAt: '2026-03-16T22:00:00Z',
        color: 'indigo'
      }
    })

    expect(wrapper.text()).toContain('usage.resetNow')
    expect(wrapper.text()).not.toContain('usage.resetPending')
  })

  it('utilization=100 时显示 100%，表示限额已打满', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '5h',
        utilization: 100,
        resetsAt: null,
        color: 'indigo'
      }
    })

    expect(wrapper.text()).toContain('100%')
    expect(wrapper.text()).not.toContain('remaining')
  })

  it('showEmptyWindowStats=true 时固定显示零请求统计', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: '30d',
        utilization: 0,
        resetsAt: null,
        showEmptyWindowStats: true,
        windowStats: {
          requests: 0,
          tokens: 0,
          cost: 0,
          standard_cost: 0,
          user_cost: 0
        },
        color: 'cyan'
      }
    })

    expect(wrapper.text()).toContain('0 req')
    expect(wrapper.text()).toContain('A $0.00')
    expect(wrapper.text()).toContain('U $0.00')
  })

  it('长标签使用更宽的最小宽度且禁止换行', () => {
    const wrapper = mount(UsageProgressBar, {
      props: {
        label: 'img 5h',
        utilization: 1,
        resetsAt: null,
        color: 'amber'
      }
    })

    const label = wrapper.find('span')
    expect(label.classes()).toContain('min-w-[48px]')
    expect(label.classes()).toContain('whitespace-nowrap')
  })
})
