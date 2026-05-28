import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillCenterNav from '@/components/skills/SkillCenterNav.vue'
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
  availableCategories: [] as string[],
  loadingMarket: false,
  togglingInstall: false,
  loadMarket: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  toggleInstall: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  resetMarketFilters: vi.fn(),
}))

const appStore = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
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
  RouterLink: {
    name: 'RouterLinkStub',
    props: ['to'],
    template: '<a :href="typeof to === `string` ? to : (to?.path ?? ``)"><slot /></a>',
  },
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
    template: '<input />',
  },
}))

vi.mock('@/components/common/Pagination.vue', () => ({
  default: {
    name: 'PaginationStub',
    template: '<div class="pagination-stub" />',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'SelectStub',
    props: ['modelValue', 'disabled'],
    template: '<div class="select-stub" :data-model-value="String(modelValue)" :data-disabled="String(disabled)" />',
  },
}))

vi.mock('@/components/skills/SkillCard.vue', () => ({
  default: {
    name: 'SkillCardStub',
    props: ['skill'],
    emits: ['toggle-install'],
    template: '<button class="skill-card-stub" type="button" @click="$emit(`toggle-install`, skill)">{{ skill?.name }}</button>',
  },
}))

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
  skillsStore.availableCategories = []
  skillsStore.loadingMarket = false
  skillsStore.togglingInstall = false
  skillsStore.loadMarket.mockClear()
  skillsStore.loadMarket.mockResolvedValue(undefined)
  skillsStore.toggleInstall.mockClear()
  skillsStore.toggleInstall.mockResolvedValue(undefined)
  skillsStore.resetMarketFilters.mockReset()
  skillsStore.resetMarketFilters.mockImplementation(() => {
    skillsStore.marketFilters.search = ''
    skillsStore.marketFilters.type = 'all'
    skillsStore.marketFilters.price_mode = 'all'
    skillsStore.marketFilters.installed = 'all'
    skillsStore.marketFilters.category = 'all'
    skillsStore.marketFilters.sort = 'latest'
  })
}

describe('skill installed entry', () => {
  beforeEach(() => {
    routeState.query = {}
    routerReplace.mockReset()
    resetSkillsStore()
    appStore.showError.mockReset()
    appStore.showSuccess.mockReset()
  })

  it('renders an installed entry in SkillCenterNav', () => {
    const wrapper = mount(SkillCenterNav, {
      props: {
        active: 'market',
      },
    })

    const links = wrapper.findAll('a').map((item) => item.attributes('href'))

    expect(links).toContain('/skills/installed')
    expect(wrapper.text()).toContain('已安装技能')
  })

  it('hides protected skill links unless the caller explicitly allows them', () => {
    const lockedWrapper = mount(SkillCenterNav, {
      props: {
        active: 'detail',
        skillId: 42,
        canEditSkill: false,
        canViewRuns: false,
        canViewRevenue: false,
      },
    })

    const lockedLinks = lockedWrapper.findAll('a').map((item) => item.attributes('href'))
    expect(lockedLinks).toContain('/skills/42')
    expect(lockedLinks).not.toContain('/skills/42/edit')
    expect(lockedLinks).not.toContain('/skills/42/versions')
    expect(lockedLinks).not.toContain('/skills/42/runs')
    expect(lockedLinks).not.toContain('/skills/42/revenue')

    const unlockedWrapper = mount(SkillCenterNav, {
      props: {
        active: 'detail',
        skillId: 42,
        canEditSkill: true,
        canViewRuns: true,
        canViewRevenue: true,
      },
    })

    const unlockedLinks = unlockedWrapper.findAll('a').map((item) => item.attributes('href'))
    expect(unlockedLinks).toContain('/skills/42/edit')
    expect(unlockedLinks).toContain('/skills/42/versions')
    expect(unlockedLinks).toContain('/skills/42/runs')
    expect(unlockedLinks).toContain('/skills/42/revenue')
  })

  it('forces the installed filter and refreshes market data on the installed route view', async () => {
    const wrapper = mount(SkillMarketView, {
      props: {
        installedOnly: true,
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('已安装技能')
    expect(skillsStore.marketFilters.installed).toBe('installed')
    expect(skillsStore.loadMarket).toHaveBeenCalledTimes(1)
    expect(skillsStore.loadMarket).toHaveBeenCalledWith(1, 18)
    expect(wrapper.findAll('.select-stub').filter((item) => item.attributes('data-disabled') === 'true')).toHaveLength(1)

    await wrapper.setProps({ installedOnly: false })
    await flushPromises()

    expect(skillsStore.marketFilters.installed).toBe('all')
    expect(skillsStore.loadMarket).toHaveBeenCalledTimes(2)
    expect(skillsStore.loadMarket).toHaveBeenLastCalledWith(1, 18)
    expect(wrapper.text()).toContain('技能市场')
  })

  it('hydrates market filters from the route query and syncs query updates back on filter changes', async () => {
    routeState.query = {
      search: 'retro',
      type: 'script',
      price_mode: 'paid',
      category: 'automation',
      installed: 'not_installed',
      sort: 'price_high',
      page: '3',
      page_size: '24',
    }
    skillsStore.marketPagination.total = 60
    skillsStore.marketPagination.page_size = 24

    const wrapper = mount(SkillMarketView)

    await flushPromises()

    expect(skillsStore.marketFilters.type).toBe('script')
    expect(skillsStore.marketFilters.search).toBe('retro')
    expect(skillsStore.marketFilters.price_mode).toBe('paid')
    expect(skillsStore.marketFilters.category).toBe('automation')
    expect(skillsStore.marketFilters.installed).toBe('not_installed')
    expect(skillsStore.marketFilters.sort).toBe('price_high')
    expect(skillsStore.loadMarket).toHaveBeenCalled()
    expect(skillsStore.loadMarket).toHaveBeenCalledWith(3, 24)

    const selectStubs = wrapper.findAllComponents({ name: 'SelectStub' })
    await selectStubs[0]?.vm.$emit('update:modelValue', 'prompt_image')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'paid',
        category: 'automation',
        installed: 'not_installed',
        sort: 'price_high',
        page_size: '24',
      },
    })
    expect(skillsStore.marketFilters.type).toBe('prompt_image')

    await selectStubs[1]?.vm.$emit('update:modelValue', 'free')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'automation',
        installed: 'not_installed',
        sort: 'price_high',
        page_size: '24',
      },
    })

    await selectStubs[2]?.vm.$emit('update:modelValue', 'installed')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'automation',
        installed: 'installed',
        sort: 'price_high',
        page_size: '24',
      },
    })

    await selectStubs[3]?.vm.$emit('update:modelValue', 'design')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'design',
        installed: 'installed',
        sort: 'price_high',
        page_size: '24',
      },
    })
    expect(skillsStore.marketFilters.category).toBe('design')

    await selectStubs[4]?.vm.$emit('update:modelValue', 'runs')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'design',
        installed: 'installed',
        sort: 'runs',
        page_size: '24',
      },
    })

    const pagination = wrapper.findComponent({ name: 'PaginationStub' })
    await pagination.vm.$emit('update:page', 2)
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'retro',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'design',
        installed: 'installed',
        sort: 'runs',
        page: '2',
        page_size: '24',
      },
    })

    skillsStore.marketFilters.search = 'cyberpunk'
    const buttons = wrapper.findAll('button')
    await buttons[1]?.trigger('click')
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        search: 'cyberpunk',
        type: 'prompt_image',
        price_mode: 'free',
        category: 'design',
        installed: 'installed',
        sort: 'runs',
        page_size: '24',
      },
    })
  })

  it('refreshes the installed-only list after uninstall so the card is removed', async () => {
    skillsStore.marketPagination.items = [
      {
        id: 7,
        slug: 'installed-skill',
        name: 'Installed skill',
        tagline: '',
        description: '',
        type: 'prompt_chat',
        visibility: 'public',
        status: 'published',
        category: null,
        tags: [],
        cover_image_url: null,
        pricing: { mode: 'free', amount: 0, currency: 'USD' },
        source_locked: false,
        can_view_source: true,
        installed: true,
        owned: false,
        editable: false,
        author: { id: 1, name: 'Author', avatar_url: null },
        stats: { installs: 3, runs: 0, revenue: 0, rating: null, versions: 1 },
        latest_version: null,
        current_version: null,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      } as never,
    ]
    skillsStore.marketPagination.total = 1
    skillsStore.loadMarket.mockImplementationOnce(async () => undefined)
    skillsStore.loadMarket.mockImplementationOnce(async () => {
      skillsStore.marketPagination.items = []
      skillsStore.marketPagination.total = 0
    })

    const wrapper = mount(SkillMarketView, {
      props: {
        installedOnly: true,
      },
    })

    await flushPromises()
    expect(wrapper.findAll('.skill-card-stub')).toHaveLength(1)

    await wrapper.get('.skill-card-stub').trigger('click')
    await flushPromises()

    expect(skillsStore.toggleInstall).toHaveBeenCalledWith(7, true)
    expect(skillsStore.loadMarket.mock.calls[skillsStore.loadMarket.mock.calls.length - 1]).toEqual([1, 18])
    expect(skillsStore.loadMarket.mock.calls.length).toBeGreaterThanOrEqual(2)
    expect(skillsStore.marketPagination.items).toHaveLength(0)
  })
})
