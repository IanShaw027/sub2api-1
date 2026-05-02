import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AIPromptLibraryView from '@/views/user/AIPromptLibraryView.vue'

const routeState = reactive({
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) }
})

const aiStore = vi.hoisted(() => ({
  availableLines: [
    { group_id: 12, label: 'Creative', key_count: 1 },
  ],
  loadRuntimeLines: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

const { listAIPrompts, createAIPrompt, updateAIPrompt, deleteAIPrompt, cloneAIPrompt } = vi.hoisted(() => ({
  listAIPrompts: vi.fn(),
  createAIPrompt: vi.fn(),
  updateAIPrompt: vi.fn(),
  deleteAIPrompt: vi.fn(),
  cloneAIPrompt: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    createI18n: actual.createI18n,
    useI18n: () => ({
      t: (key: string, fallback?: string) => fallback ?? key,
    }),
  }
})

vi.mock('@/stores', () => ({
  useAiStudioStore: () => aiStore,
  useAppStore: () => appStore,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

vi.mock('@/api', () => ({
  listAIPrompts,
  createAIPrompt,
  updateAIPrompt,
  deleteAIPrompt,
  cloneAIPrompt,
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayoutStub',
    template: '<div><slot /></div>',
  },
}))

vi.mock('@/components/ai/AiPromptEditorDialog.vue', () => ({
  default: {
    name: 'AiPromptEditorDialogStub',
    template: '<div class="editor-dialog-stub" />',
  },
}))

vi.mock('@/components/common/ConfirmDialog.vue', () => ({
  default: {
    name: 'ConfirmDialogStub',
    template: '<div class="confirm-dialog-stub" />',
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

vi.mock('@/components/common/Input.vue', () => ({
  default: {
    name: 'InputStub',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<input :value="modelValue ?? ``" @input="$emit(`update:modelValue`, $event.target.value)" />',
  },
}))

vi.mock('@/components/common/Pagination.vue', () => ({
  default: {
    name: 'PaginationStub',
    props: ['page', 'total', 'pageSize'],
    emits: ['update:page', 'update:pageSize'],
    template: '<div class="pagination-stub" />',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'SelectStub',
    props: ['modelValue'],
    emits: ['update:modelValue'],
    template: '<div class="select-stub" :data-model-value="String(modelValue)" />',
  },
}))

function resetState(): void {
  routeState.query = {}
  routerReplace.mockReset()
  aiStore.loadRuntimeLines.mockReset()
  aiStore.loadRuntimeLines.mockResolvedValue(undefined)
  appStore.showError.mockReset()
  appStore.showSuccess.mockReset()
  listAIPrompts.mockReset()
  listAIPrompts.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 18,
    pages: 1,
  })
  createAIPrompt.mockReset()
  updateAIPrompt.mockReset()
  deleteAIPrompt.mockReset()
  cloneAIPrompt.mockReset()
}

describe('AIPromptLibraryView', () => {
  beforeEach(() => {
    resetState()
  })

  it('hydrates prompt scope and filters from the route query and syncs search back to the URL', async () => {
    routeState.query = {
      scope: 'private',
      search: 'robot',
      status: 'published',
      line_id: '12',
      page: '2',
      page_size: '18',
    }

    const wrapper = mount(AIPromptLibraryView)
    await flushPromises()

    expect(aiStore.loadRuntimeLines).toHaveBeenCalledTimes(1)
    expect(listAIPrompts).toHaveBeenCalledWith(2, 18, {
      search: 'robot',
      visibility: 'private',
      status: 'published',
      line_id: 12,
      scope: 'mine',
    })

    await wrapper.find('input').setValue('workflow')
    await wrapper.findAll('button')[3]?.trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        scope: 'private',
        search: 'workflow',
        status: 'published',
        line_id: '12',
      },
    })
  })
})
