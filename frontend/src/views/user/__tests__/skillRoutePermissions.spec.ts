import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillEditorView from '@/views/user/SkillEditorView.vue'
import SkillVersionsView from '@/views/user/SkillVersionsView.vue'
import SkillRunsView from '@/views/user/SkillRunsView.vue'
import SkillRevenueView from '@/views/user/SkillRevenueView.vue'

const routeState = reactive({
  params: {
    id: '42',
  },
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async () => undefined)

const lockedDetail = {
  id: 42,
  name: 'Locked skill',
  slug: 'locked-skill',
  type: 'prompt_chat',
  editable: false,
  owned: false,
  latest_version: { version: 'v1.0.0' },
  pricing: { mode: 'free', amount: 0, currency: 'USD' },
  stats: { installs: 0, runs: 0, revenue: 0, rating: null, versions: 1 },
  author: { id: 1, name: 'Author', avatar_url: null },
  source_locked: false,
  installed: false,
  can_view_source: true,
  visibility: 'private',
  status: 'draft',
  category: null,
  tags: [],
  cover_image_url: null,
  tagline: '',
  description: '',
  current_version: null,
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  variable_schema: [],
  content: null,
  metadata: {},
  examples: [],
  readme: null,
  install_note: null,
  can_install: true,
  can_run: true,
}

const skillsStore = vi.hoisted(() => ({
  detail: null as any,
  editorDraft: {
    id: undefined,
    slug: '',
    name: '',
    tagline: '',
    description: '',
    type: 'prompt_chat',
    visibility: 'private',
    status: 'draft',
    category: '',
    cover_image_url: '',
    tags: [],
    pricing: { mode: 'free', amount: 0, currency: 'USD' },
    source_locked: false,
    variable_schema: [],
    content: {
      type: 'prompt_chat',
      system_prompt: '',
      user_prompt_template: '',
      assistant_prefill: '',
      model: '',
      temperature: 0.7,
      max_tokens: 2048,
    },
    readme: '',
    install_note: '',
  },
  versionsPagination: {
    items: [{ id: 101, version: 'v1.0.0', review_status: 'draft', status: 'draft', is_current: false, source_locked: false, changelog: '', created_at: '2026-01-01T00:00:00Z', published_at: null, submitted_at: null, reviewed_at: null, review_note: null, can_submit_review: true, can_publish: false, can_test: false, can_use: false, skill_id: 42, variable_schema: [], content: null, metadata: {} }],
    total: 1,
    page: 1,
    page_size: 12,
    pages: 1,
  },
  runsPagination: {
    items: [],
    total: 0,
    page: 1,
    page_size: 20,
    pages: 1,
  },
  versionOptions: [
    { value: 'all', label: 'All Versions' },
    { value: 101, label: 'v1.0.0' },
  ],
  revenue: {
    summary: {
      total_revenue: 0,
      total_sales: 0,
      total_runs: 0,
      pending_amount: 0,
      settled_amount: 0,
      refunded_amount: 0,
      currency: 'USD',
    },
    trend: [],
    orders: [],
  },
  loadingEditor: false,
  loadingVersions: false,
  loadingRuns: false,
  loadingRevenue: false,
  runFilters: {
    search: '',
    status: 'all',
    version_id: 'all',
  },
  savingEditor: false,
  creatingVersion: false,
  updatingVersion: false,
  submittingVersionId: null as number | null,
  publishingVersionId: null as number | null,
  loadEditor: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadSkillDetail: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadVersions: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadRuns: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadRevenue: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
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

vi.mock('@/components/layout/TablePageLayout.vue', () => ({
  default: {
    name: 'TablePageLayoutStub',
    template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>',
  },
}))

vi.mock('@/components/common/Input.vue', () => ({
  default: {
    name: 'InputStub',
    template: '<input />',
  },
}))

vi.mock('@/components/common/Select.vue', () => ({
  default: {
    name: 'SelectStub',
    template: '<div />',
  },
}))

vi.mock('@/components/common/TextArea.vue', () => ({
  default: {
    name: 'TextAreaStub',
    template: '<textarea />',
  },
}))

vi.mock('@/components/common/DataTable.vue', () => ({
  default: {
    name: 'DataTableStub',
    template: '<div />',
  },
}))

vi.mock('@/components/common/Pagination.vue', () => ({
  default: {
    name: 'PaginationStub',
    template: '<div />',
  },
}))

vi.mock('@/components/common/EmptyState.vue', () => ({
  default: {
    name: 'EmptyStateStub',
    template: '<div />',
  },
}))

vi.mock('@/components/icons/Icon.vue', () => ({
  default: {
    name: 'IconStub',
    template: '<span />',
  },
}))

vi.mock('@/components/skills/SkillCenterNav.vue', () => ({
  default: {
    name: 'SkillCenterNavStub',
    template: '<nav class="skill-center-nav-stub" />',
  },
}))

vi.mock('@/components/skills/SkillTypeEditor.vue', () => ({
  default: {
    name: 'SkillTypeEditorStub',
    template: '<section />',
  },
}))

vi.mock('@/components/skills/SkillVariableSchemaEditor.vue', () => ({
  default: {
    name: 'SkillVariableSchemaEditorStub',
    template: '<section />',
  },
}))

vi.mock('@/components/skills/presentation', () => ({
  skillVersionBadgeClass: () => '',
  skillVersionReviewBadgeClass: () => '',
  skillVersionReviewStatusLabel: (value: string) => value,
  skillVersionStatusLabel: (value: string) => value,
  skillRunBadgeClass: () => '',
  skillRunStatusLabel: (value: string) => value,
  skillRunTriggerLabel: (value: string | null) => value ?? '-',
  formatCurrency: () => 'USD 0.00',
}))

function resetStore(): void {
  routeState.params.id = '42'
  routeState.query = {}
  routerReplace.mockReset()
  appStore.showError.mockReset()
  appStore.showSuccess.mockReset()
  skillsStore.detail = null
  skillsStore.loadEditor.mockReset()
  skillsStore.loadEditor.mockImplementation(async (_skillId: number) => {
    skillsStore.detail = { ...lockedDetail }
  })
  skillsStore.loadSkillDetail.mockReset()
  skillsStore.loadSkillDetail.mockImplementation(async (_skillId: number) => {
    skillsStore.detail = { ...lockedDetail }
    return skillsStore.detail
  })
  skillsStore.loadVersions.mockReset()
  skillsStore.loadVersions.mockResolvedValue(undefined)
  skillsStore.loadRuns.mockReset()
  skillsStore.loadRuns.mockResolvedValue(undefined)
  skillsStore.loadRevenue.mockReset()
  skillsStore.loadRevenue.mockResolvedValue(undefined)
}

describe('skill route permissions', () => {
  beforeEach(() => {
    resetStore()
  })

  it('redirects the edit page to detail when the skill is not editable', async () => {
    const wrapper = mount(SkillEditorView)
    await flushPromises()

    expect(skillsStore.loadEditor).toHaveBeenCalledWith(42, false)
    expect(routerReplace).toHaveBeenCalledWith('/skills/42')
    expect(wrapper.text()).not.toContain('编辑技能')
  })

  it('redirects the versions page to detail when the skill is not editable', async () => {
    mount(SkillVersionsView)
    await flushPromises()

    expect(skillsStore.loadSkillDetail).toHaveBeenCalledWith(42, false)
    expect(skillsStore.loadVersions).not.toHaveBeenCalled()
    expect(routerReplace).toHaveBeenCalledWith('/skills/42')
  })

  it('redirects the runs page to detail when the skill is not owned', async () => {
    mount(SkillRunsView)
    await flushPromises()

    expect(skillsStore.loadSkillDetail).toHaveBeenCalledWith(42, false)
    expect(skillsStore.loadRuns).not.toHaveBeenCalled()
    expect(routerReplace).toHaveBeenCalledWith('/skills/42')
  })

  it('redirects the revenue page to detail when the skill is not owned', async () => {
    mount(SkillRevenueView)
    await flushPromises()

    expect(skillsStore.loadSkillDetail).toHaveBeenCalledWith(42, false)
    expect(skillsStore.loadRevenue).not.toHaveBeenCalled()
    expect(routerReplace).toHaveBeenCalledWith('/skills/42')
  })
})
