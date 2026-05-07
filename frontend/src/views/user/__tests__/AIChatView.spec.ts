import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AIChatView from '@/views/user/AIChatView.vue'

const routeState = reactive({
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) }
})

const aiStore = vi.hoisted(() => ({
  availableLines: [
    {
      group_id: 7,
      label: 'Primary Line',
      key_count: 1,
      platform: 'openai',
      keys: [{ id: 11, name: 'Primary Key' }],
      key_ids: [11],
    },
  ],
  runtimeInfo: {
    source_domain: 'runtime.example.com',
    chat: {
      supported_entries: ['responses', 'chat_completions'],
    },
  },
  chatSessions: {
    items: [
      {
        id: 5,
        title: 'Existing Session',
        line_id: 7,
        updated_at: '2026-05-07T00:00:00Z',
        last_message_at: '2026-05-07T00:00:00Z',
      },
    ],
  },
  loadingChatSessions: false,
  selectedLineId: null as number | null,
  selectedKeyId: null as number | null,
  activeSessionId: null as number | null,
  activeSession: null as null | { id: number; title: string; line_id: number },
  sessionMessages: [] as Array<{ id: number; role: string; content: string; created_at: string }>,
  loadRuntimeLines: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadChatSessions: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadChatSession: vi.fn<(sessionId: number) => Promise<void>>().mockImplementation(async (sessionId: number) => {
    aiStore.activeSessionId = sessionId
    aiStore.activeSession = {
      id: sessionId,
      title: 'Existing Session',
      line_id: 7,
    }
    aiStore.sessionMessages = [
      {
        id: 1,
        role: 'assistant',
        content: 'restored',
        created_at: '2026-05-07T00:00:00Z',
      },
    ]
  }),
  setSelectedLine: vi.fn((lineId: number | null) => {
    aiStore.selectedLineId = lineId
  }),
  setSelectedKey: vi.fn((keyId: number | null) => {
    aiStore.selectedKeyId = keyId
  }),
  ensureSelection: vi.fn(() => {
    aiStore.selectedLineId = aiStore.availableLines[0]?.group_id ?? null
    aiStore.selectedKeyId = aiStore.availableLines[0]?.keys[0]?.id ?? null
  }),
  syncSelectedKey: vi.fn(() => {
    const line = aiStore.availableLines.find((item) => item.group_id === aiStore.selectedLineId) ?? null
    aiStore.selectedKeyId = line?.keys[0]?.id ?? null
  }),
  setActiveSession: vi.fn((sessionId: number | null) => {
    aiStore.activeSessionId = sessionId
    aiStore.activeSession = sessionId == null
      ? null
      : {
          id: sessionId,
          title: 'Existing Session',
          line_id: 7,
        }
  }),
  createChatSession: vi.fn(),
  selectedLine: null as null | {
    group_id: number
    label: string
    key_count: number
    platform: string
    keys: Array<{ id: number; name: string }>
    key_ids: number[]
  },
}))

Object.defineProperty(aiStore, 'selectedLine', {
  get() {
    return aiStore.availableLines.find((line) => line.group_id === aiStore.selectedLineId) ?? null
  },
})

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
}))

const { listAIPrompts } = vi.hoisted(() => ({
  listAIPrompts: vi.fn(),
}))

const apiClientPost = vi.hoisted(() => vi.fn())

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback ?? key,
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
  useAiStudioStore: () => aiStore,
}))

vi.mock('@/api', () => ({
  listAIPrompts,
}))

vi.mock('@/api/client', () => ({
  apiClient: {
    post: apiClientPost,
  },
}))

vi.mock('@/utils/apiError', () => ({
  extractApiErrorMessage: (error: unknown, fallback: string) => error instanceof Error ? error.message : fallback,
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayoutStub',
    template: '<div><slot /></div>',
  },
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: {
    name: 'EmptyStateStub',
    template: '<div class="empty-state-stub" />',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'IconStub',
    template: '<span class="icon-stub" />',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'SelectStub',
    props: ['modelValue', 'options'],
    emits: ['update:modelValue'],
    template: '<div class="select-stub" :data-model-value="String(modelValue)" />',
  },
}))

vi.mock('@/components/common/TextArea.vue', () => ({
  default: {
    name: 'TextAreaStub',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<textarea :value="modelValue ?? ``" @input="$emit(`update:modelValue`, $event.target.value)" />',
  },
}))

function resetState(): void {
  routeState.query = {
    session: '5',
  }
  routerReplace.mockReset()
  listAIPrompts.mockReset()
  listAIPrompts.mockResolvedValue({ items: [] })
  apiClientPost.mockReset()
  appStore.showError.mockReset()
  aiStore.selectedLineId = null
  aiStore.selectedKeyId = null
  aiStore.activeSessionId = null
  aiStore.activeSession = null
  aiStore.sessionMessages = []
  aiStore.loadRuntimeLines.mockReset()
  aiStore.loadRuntimeLines.mockResolvedValue(undefined)
  aiStore.loadChatSessions.mockReset()
  aiStore.loadChatSessions.mockResolvedValue(undefined)
  aiStore.loadChatSession.mockClear()
  aiStore.setSelectedLine.mockClear()
  aiStore.setSelectedKey.mockClear()
  aiStore.ensureSelection.mockClear()
  aiStore.syncSelectedKey.mockClear()
  aiStore.setActiveSession.mockClear()
}

describe('AIChatView', () => {
  beforeEach(() => {
    resetState()
    Element.prototype.scrollIntoView = vi.fn()
  })

  it('continues runtime and session initialization when quick prompts loading fails', async () => {
    listAIPrompts.mockRejectedValue(new Error('quick prompts failed'))

    mount(AIChatView)
    await flushPromises()
    await flushPromises()

    expect(aiStore.loadRuntimeLines).toHaveBeenCalledWith(true)
    expect(aiStore.loadChatSessions).toHaveBeenCalledTimes(1)
    expect(aiStore.loadChatSession).toHaveBeenCalledWith(5)
    expect(aiStore.selectedLineId).toBe(7)
    expect(aiStore.selectedKeyId).toBe(11)
    expect(aiStore.activeSessionId).toBe(5)
    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        session: '5',
        line: '7',
        key: '11',
      },
    })
    expect(appStore.showError).toHaveBeenCalledWith('quick prompts failed')
  })
})
