import { beforeEach, afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick, ref } from 'vue'
import { useAutoRefresh } from '@/composables/useAutoRefresh'

vi.mock('vue', async () => {
  const actual = await vi.importActual<typeof import('vue')>('vue')
  return {
    ...actual,
    onBeforeUnmount: vi.fn(),
  }
})

describe('useAutoRefresh', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  afterEach(() => {
    vi.clearAllMocks()
  })

  it('resyncs the active interval when the default interval arrives late', async () => {
    const defaultInterval = ref(60)
    const state = useAutoRefresh({
      storageKey: 'channel-status-auto-refresh',
      intervals: [30, 60, 120] as const,
      defaultInterval,
      onRefresh: vi.fn(),
    })

    state.setEnabled(true)
    expect(state.intervalSeconds.value).toBe(60)
    expect(state.countdown.value).toBe(60)

    defaultInterval.value = 120
    await nextTick()

    expect(state.intervalSeconds.value).toBe(120)
    expect(state.countdown.value).toBe(120)
  })

  it('does not overwrite a persisted non-default interval when the default changes later', async () => {
    window.localStorage.setItem('channel-status-auto-refresh', JSON.stringify({
      enabled: true,
      interval_seconds: 30,
    }))

    const defaultInterval = ref(60)
    const state = useAutoRefresh({
      storageKey: 'channel-status-auto-refresh',
      intervals: [30, 60, 120] as const,
      defaultInterval,
      onRefresh: vi.fn(),
    })

    expect(state.intervalSeconds.value).toBe(30)

    defaultInterval.value = 120
    await nextTick()

    expect(state.intervalSeconds.value).toBe(30)
  })
})
