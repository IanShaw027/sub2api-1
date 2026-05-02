import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillRunsView from '@/views/user/SkillRunsView.vue'

const routeState = reactive({
  params: {
    id: '42',
  },
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) }
})

const skillsStore = vi.hoisted(() => ({
  detail: {
    id: 42,
    owned: true,
  },
  runFilters: {
    search: '',
    status: 'all',
    version_id: 'all' as number | 'all',
  },
  versionOptions: [
    { value: 'all', label: 'All Versions' },
    { value: 7, label: 'v7' },
  ],
  runsPagination: {
    items: [],
    total: 80,
    page: 1,
    page_size: 20,
    pages: 4,
  },
  loadingRuns: false,
  loadSkillDetail: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadVersions: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadRuns: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  resetRunFilters: vi.fn(),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
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

vi.mock('@/stores/skillsCenter', () => ({
  useSkillsCenterStore: () => skillsStore,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayoutStub',
    template: '<div><slot /></div>',
  },
}))

vi.mock('@/components/layout/TablePageLayout.vue', () => ({
  default: {
    name: 'TablePageLayoutStub',
    template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
  },
}))

vi.mock('@/components/common/DataTable.vue', () => ({
  default: {
    name: 'DataTableStub',
    props: ['data', 'loading'],
    template: '<div class="datatable-stub" />',
  },
}))

vi.mock('@/components/common/Input.vue', () => ({
  default: {
    name: 'InputStub',
    template: '<input />',
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

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'IconStub',
    template: '<span class="icon-stub" />',
  },
}))

vi.mock('@/components/skills/SkillCenterNav.vue', () => ({
  default: {
    name: 'SkillCenterNavStub',
    template: '<div class="skill-center-nav-stub" />',
  },
}))

function resetStore(): void {
  routeState.params.id = '42'
  routeState.query = {}
  routerReplace.mockReset()
  skillsStore.detail = {
    id: 42,
    owned: true,
  }
  skillsStore.runFilters.search = ''
  skillsStore.runFilters.status = 'all'
  skillsStore.runFilters.version_id = 'all'
  skillsStore.runsPagination.items = []
  skillsStore.runsPagination.total = 80
  skillsStore.runsPagination.page = 1
  skillsStore.runsPagination.page_size = 20
  skillsStore.runsPagination.pages = 4
  skillsStore.loadingRuns = false
  skillsStore.loadSkillDetail.mockReset()
  skillsStore.loadSkillDetail.mockResolvedValue(undefined)
  skillsStore.loadVersions.mockReset()
  skillsStore.loadVersions.mockResolvedValue(undefined)
  skillsStore.loadRuns.mockReset()
  skillsStore.loadRuns.mockImplementation(async (_skillId: number, page: number, pageSize: number) => {
    skillsStore.runsPagination.page = page
    skillsStore.runsPagination.page_size = pageSize
  })
  skillsStore.resetRunFilters.mockReset()
  skillsStore.resetRunFilters.mockImplementation(() => {
    skillsStore.runFilters.search = ''
    skillsStore.runFilters.status = 'all'
    skillsStore.runFilters.version_id = 'all'
  })
  appStore.showError.mockReset()
}

describe('SkillRunsView', () => {
  beforeEach(() => {
    resetStore()
  })

  it('hydrates run filters from route query and syncs pagination updates back to the URL', async () => {
    routeState.query = {
      search: 'latency',
      status: 'failed',
      version_id: '7',
      page: '2',
      page_size: '40',
    }

    const wrapper = mount(SkillRunsView)
    await flushPromises()

    expect(skillsStore.runFilters.search).toBe('latency')
    expect(skillsStore.runFilters.status).toBe('failed')
    expect(skillsStore.runFilters.version_id).toBe(7)
    expect(skillsStore.loadSkillDetail).toHaveBeenCalledWith(42, false)
    expect(skillsStore.loadVersions).toHaveBeenCalledWith(42, 1, 100)
    expect(skillsStore.loadRuns).toHaveBeenCalledWith(42, 2, 40)

    const pagination = wrapper.findComponent({ name: 'PaginationStub' })
    await pagination.vm.$emit('update:page', 3)
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'latency',
        status: 'failed',
        version_id: '7',
        page: '3',
        page_size: '40',
      },
    })
    expect(skillsStore.loadRuns).toHaveBeenLastCalledWith(42, 3, 40)
  })
})
