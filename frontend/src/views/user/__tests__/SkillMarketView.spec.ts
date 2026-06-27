import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillMarketView from '@/views/user/SkillMarketView.vue'

const routeState = reactive({
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) }
})

const skillsStore = vi.hoisted(() => ({
  marketFilters: {
    search: '',
    type: 'all',
    price_mode: 'all',
    installed: 'all',
    category: 'all',
    sort: 'latest',
  },
  marketPagination: {
    items: [],
    total: 0,
    page: 1,
    page_size: 18,
    pages: 1,
  },
  loadingMarket: false,
  togglingInstall: false,
  availableCategories: [],
  loadMarket: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  resetMarketFilters: vi.fn(),
  toggleInstall: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (_key: string, fallback?: string) => fallback ?? _key,
    }),
  }
})

vi.mock('vue-router', () => ({
  useRoute: () => routeState,
  useRouter: () => ({
    replace: routerReplace,
  }),
}))

vi.mock('@/stores/skillsCenter', () => ({
  useSkillsCenterStore: () => skillsStore,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
}))

const AppLayoutStub = defineComponent({
  name: 'AppLayoutStub',
  template: '<div><slot /></div>',
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean],
      default: null,
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  template: '<div class="select-stub" />',
})

function resetSkillsStore(): void {
  skillsStore.marketFilters.search = ''
  skillsStore.marketFilters.type = 'all'
  skillsStore.marketFilters.price_mode = 'all'
  skillsStore.marketFilters.installed = 'all'
  skillsStore.marketFilters.category = 'all'
  skillsStore.marketFilters.sort = 'latest'
  skillsStore.marketPagination.items = []
  skillsStore.marketPagination.total = 0
  skillsStore.marketPagination.page = 1
  skillsStore.marketPagination.page_size = 18
  skillsStore.marketPagination.pages = 1
  skillsStore.loadingMarket = false
  skillsStore.loadMarket.mockReset()
  skillsStore.loadMarket.mockResolvedValue(undefined)
  skillsStore.resetMarketFilters.mockReset()
  skillsStore.toggleInstall.mockReset()
  skillsStore.toggleInstall.mockResolvedValue(undefined)
}

describe('SkillMarketView filters', () => {
  beforeEach(() => {
    routeState.query = { type: 'script' }
    routerReplace.mockReset()
    resetSkillsStore()
    appStore.showError.mockReset()
    appStore.showSuccess.mockReset()
  })

  it('exposes script as a selectable market skill type', async () => {
    const wrapper = mount(SkillMarketView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          EmptyState: true,
          Icon: true,
          Input: true,
          Pagination: true,
          Select: SelectStub,
          SkillCard: true,
          SkillCenterNav: true,
        },
      },
    })

    await flushPromises()

    const typeSelect = wrapper.findAllComponents({ name: 'SelectStub' })[0]
    expect(typeSelect.props('options')).toEqual([
      { value: 'all', label: '全部' },
      { value: 'prompt_chat', label: 'prompt_chat' },
      { value: 'prompt_image', label: 'prompt_image' },
      { value: 'script', label: 'script' },
    ])
    expect(skillsStore.marketFilters.type).toBe('script')
  })
})
