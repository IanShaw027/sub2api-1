import { afterEach, beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) =>
        key.replace(/\{(\w+)\}/g, (_match, token) => String(params?.[token] ?? `{${token}}`)),
    }),
  }
})

import MessageStream from '../components/MessageStream.vue'
import TokenStats from '../components/TokenStats.vue'
import { useCreationStore } from '../stores/creation'

const stubs = {
  Icon: true,
}

describe('MessageStream token stats propagation', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('passes input/output tokens from an assistant message down to TokenStats', () => {
    const store = useCreationStore()
    store.messages = [
      { id: 1, session_id: 1, role: 'user', content: 'hello', created_at: '2026-01-01T00:00:00Z' },
      {
        id: 2,
        session_id: 1,
        role: 'assistant',
        content: 'hi there',
        input_tokens: 123,
        output_tokens: 456,
        created_at: '2026-01-01T00:01:00Z',
      },
    ]

    const wrapper = mount(MessageStream, { global: { stubs } })

    const tokenStats = wrapper.findAllComponents(TokenStats)
    expect(tokenStats).toHaveLength(1)
    expect(tokenStats[0].props()).toMatchObject({ inputTokens: 123, outputTokens: 456 })
    expect(wrapper.find('.studio-token-stats').exists()).toBe(true)
  })

  it('does not render token stats for a user message', () => {
    const store = useCreationStore()
    store.messages = [
      { id: 1, session_id: 1, role: 'user', content: 'hello', created_at: '2026-01-01T00:00:00Z' },
    ]

    const wrapper = mount(MessageStream, { global: { stubs } })

    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)
  })

  it('does not render token stats for an assistant message with no token data', () => {
    const store = useCreationStore()
    store.messages = [
      { id: 1, session_id: 1, role: 'assistant', content: 'hi', created_at: '2026-01-01T00:00:00Z' },
    ]

    const wrapper = mount(MessageStream, { global: { stubs } })

    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)
  })

  it('does not render token stats on a message that is still streaming', () => {
    const store = useCreationStore()
    store.streaming = true
    store.streamingContent = 'partial reply'

    const wrapper = mount(MessageStream, { global: { stubs } })

    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)
  })

  it('reveals token stats once the streamed message is finalized with usage data', async () => {
    const store = useCreationStore()
    store.streaming = true
    store.streamingContent = 'partial reply'

    const wrapper = mount(MessageStream, { global: { stubs } })
    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)

    // Simulate the store finishing the stream and reloading the persisted
    // message (with real usage numbers) from the backend.
    store.streaming = false
    store.streamingContent = ''
    store.messages = [
      {
        id: 3,
        session_id: 1,
        role: 'assistant',
        content: 'partial reply',
        input_tokens: 10,
        output_tokens: 20,
        created_at: '2026-01-01T00:02:00Z',
      },
    ]
    await wrapper.vm.$nextTick()

    const tokenStats = wrapper.findComponent(TokenStats)
    expect(tokenStats.exists()).toBe(true)
    expect(tokenStats.props()).toMatchObject({ inputTokens: 10, outputTokens: 20 })
    expect(wrapper.find('.studio-token-stats').exists()).toBe(true)
  })
})
