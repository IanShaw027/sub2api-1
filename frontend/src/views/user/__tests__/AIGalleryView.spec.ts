import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import AIGalleryView from '@/views/user/AIGalleryView.vue'

const routeState = reactive({
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) }
})

const aiStore = vi.hoisted(() => ({
  availableLines: [
    { group_id: 88, label: 'Primary', key_count: 2 },
  ],
  runtimeInfo: { source_domain: 'ai.example.com' },
  loadRuntimeLines: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
}))

const { listAIArtworks } = vi.hoisted(() => ({
  listAIArtworks: vi.fn(),
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
  listAIArtworks,
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
  aiStore.availableLines = [
    { group_id: 88, label: 'Primary', key_count: 2 },
  ]
  aiStore.loadRuntimeLines.mockReset()
  aiStore.loadRuntimeLines.mockResolvedValue(undefined)
  appStore.showError.mockReset()
  listAIArtworks.mockReset()
  listAIArtworks.mockResolvedValue({
    items: [],
    total: 0,
    page: 1,
    page_size: 24,
    pages: 1,
  })
}

describe('AIGalleryView', () => {
  beforeEach(() => {
    resetState()
  })

  it('hydrates gallery filters from the route query and syncs search back to the URL', async () => {
    routeState.query = {
      search: 'glass',
      visibility: 'public',
      status: 'ready',
      line_id: '88',
      page: '3',
      page_size: '24',
    }

    const wrapper = mount(AIGalleryView)
    await flushPromises()

    expect(aiStore.loadRuntimeLines).toHaveBeenCalledTimes(1)
    expect(listAIArtworks).toHaveBeenCalledWith(3, 24, {
      search: 'glass',
      visibility: 'public',
      status: 'ready',
      line_id: 88,
    })

    await wrapper.find('input').setValue('sunset')
    await wrapper.findAll('button')[2]?.trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'sunset',
        visibility: 'public',
        status: 'ready',
        line_id: '88',
      },
    })
  })
})
