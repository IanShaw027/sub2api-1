import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent, reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillMySkillsView from '@/views/user/SkillMySkillsView.vue'

const routeState = reactive({
  query: {
    status: 'published' as string | undefined,
  },
})

const routerReplace = vi.fn(async ({ query }: { query?: Record<string, unknown> }) => {
  routeState.query = { ...(query ?? {}) } as typeof routeState.query
})

const skillsStore = vi.hoisted(() => ({
  mySkillFilters: {
    search: '',
    type: 'all',
    visibility: 'all',
    status: 'all',
    sort: 'latest',
  },
  mySkillsPagination: {
    items: [],
    total: 0,
    page: 1,
    page_size: 18,
    pages: 1,
  },
  loadingMySkills: false,
  loadMySkills: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  resetMySkillFilters: vi.fn(),
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

const InputStub = defineComponent({
  name: 'InputStub',
  template: '<input />',
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
  template: '<div class="select-stub" :data-model-value="String(modelValue)" />',
})

const IconStub = defineComponent({
  name: 'IconStub',
  template: '<span />',
})

const EmptyStateStub = defineComponent({
  name: 'EmptyStateStub',
  template: '<div class="empty-state-stub" />',
})

const PaginationStub = defineComponent({
  name: 'PaginationStub',
  template: '<div class="pagination-stub" />',
})

const SkillCardStub = defineComponent({
  name: 'SkillCardStub',
  template: '<div class="skill-card-stub" />',
})

const SkillCenterNavStub = defineComponent({
  name: 'SkillCenterNavStub',
  template: '<div class="skill-nav-stub" />',
})

function resetSkillsStore(): void {
  skillsStore.mySkillFilters.search = ''
  skillsStore.mySkillFilters.type = 'all'
  skillsStore.mySkillFilters.visibility = 'all'
  skillsStore.mySkillFilters.status = 'all'
  skillsStore.mySkillFilters.sort = 'latest'
  skillsStore.mySkillsPagination.items = []
  skillsStore.mySkillsPagination.total = 0
  skillsStore.mySkillsPagination.page = 1
  skillsStore.mySkillsPagination.page_size = 18
  skillsStore.mySkillsPagination.pages = 1
  skillsStore.loadingMySkills = false
  skillsStore.loadMySkills.mockReset()
  skillsStore.loadMySkills.mockResolvedValue(undefined)
  skillsStore.resetMySkillFilters.mockReset()
  skillsStore.resetMySkillFilters.mockImplementation(() => {
    skillsStore.mySkillFilters.search = ''
    skillsStore.mySkillFilters.type = 'all'
    skillsStore.mySkillFilters.visibility = 'all'
    skillsStore.mySkillFilters.status = 'all'
    skillsStore.mySkillFilters.sort = 'latest'
  })
}

describe('SkillMySkillsView route-driven status filter', () => {
  beforeEach(() => {
    routeState.query = {
      type: 'script',
      status: 'published',
      visibility: 'private',
      search: 'poster',
      sort: 'revenue',
      page: '2',
      page_size: '24',
    }
    routerReplace.mockReset()
    resetSkillsStore()
    skillsStore.mySkillsPagination.total = 48
    skillsStore.mySkillsPagination.page_size = 24
    appStore.showError.mockReset()
  })

  it('hydrates from the route query and updates the route when quick status buttons are clicked', async () => {
    const wrapper = mount(SkillMySkillsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Input: InputStub,
          Select: SelectStub,
          Icon: IconStub,
          EmptyState: EmptyStateStub,
          Pagination: PaginationStub,
          SkillCard: SkillCardStub,
          SkillCenterNav: SkillCenterNavStub,
        },
      },
    })

    await flushPromises()

    expect(skillsStore.mySkillFilters.type).toBe('all')
    expect(skillsStore.mySkillFilters.status).toBe('published')
    expect(skillsStore.mySkillFilters.visibility).toBe('private')
    expect(skillsStore.mySkillFilters.search).toBe('poster')
    expect(skillsStore.mySkillFilters.sort).toBe('revenue')
    expect(skillsStore.loadMySkills).toHaveBeenCalledTimes(1)
    expect(skillsStore.loadMySkills).toHaveBeenCalledWith(2, 24)

    await wrapper.get('[data-status-filter="draft"]').trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({
      query: {
        status: 'draft',
        visibility: 'private',
        search: 'poster',
        sort: 'revenue',
        page_size: '24',
      },
    })
    expect(skillsStore.mySkillFilters.status).toBe('draft')
    expect(skillsStore.loadMySkills).toHaveBeenCalledTimes(2)
    expect(skillsStore.loadMySkills).toHaveBeenCalled()

    const selectStubs = wrapper.findAllComponents({ name: 'SelectStub' })
    expect(selectStubs[0]?.props('options')).toEqual([
      { value: 'all', label: '全部' },
      { value: 'prompt_chat', label: '对话提示词' },
      { value: 'prompt_image', label: '图像提示词' },
    ])
    expect(selectStubs[2]?.props('options')).toEqual([
      { value: 'all', label: '全部' },
      { value: 'public', label: '公开' },
      { value: 'private', label: '私有' },
    ])

    await selectStubs[0]?.vm.$emit('update:modelValue', 'prompt_image')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        type: 'prompt_image',
        status: 'draft',
        visibility: 'private',
        search: 'poster',
        sort: 'revenue',
        page_size: '24',
      },
    })

    await selectStubs[2]?.vm.$emit('update:modelValue', 'public')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        type: 'prompt_image',
        status: 'draft',
        visibility: 'public',
        search: 'poster',
        sort: 'revenue',
        page_size: '24',
      },
    })

    await selectStubs[3]?.vm.$emit('update:modelValue', 'runs')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        type: 'prompt_image',
        status: 'draft',
        visibility: 'public',
        search: 'poster',
        sort: 'runs',
        page_size: '24',
      },
    })

    skillsStore.mySkillFilters.search = 'banner'
    const buttons = wrapper.findAll('button')
    await buttons[buttons.length - 1]?.trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        type: 'prompt_image',
        status: 'draft',
        visibility: 'public',
        search: 'banner',
        sort: 'runs',
        page_size: '24',
      },
    })

    const pagination = wrapper.findComponent({ name: 'PaginationStub' })
    await pagination.vm.$emit('update:page', 3)
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        type: 'prompt_image',
        status: 'draft',
        visibility: 'public',
        search: 'banner',
        sort: 'runs',
        page: '3',
        page_size: '24',
      },
    })
    expect(skillsStore.loadMySkills).toHaveBeenCalledWith(3, 24)
  })
})
