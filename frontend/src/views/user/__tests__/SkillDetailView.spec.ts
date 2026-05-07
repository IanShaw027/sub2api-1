import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillDetailView from '@/views/user/SkillDetailView.vue'

const routeState = reactive({
  params: {
    id: '42',
  },
  query: {} as Record<string, unknown>,
})

const routerReplace = vi.fn(async () => undefined)

const detail = {
  id: 42,
  slug: 'locked-skill',
  name: 'Locked skill',
  tagline: '',
  description: 'Desc',
  type: 'prompt_chat',
  visibility: 'public',
  status: 'published',
  category: null,
  tags: [],
  cover_image_url: null,
  pricing: { mode: 'free', amount: 0, currency: 'USD' },
  source_locked: false,
  can_view_source: true,
  installed: false,
  owned: false,
  editable: false,
  author: { id: 1, name: 'Author', avatar_url: null },
  stats: { installs: 0, runs: 0, revenue: 0, rating: null, versions: 1 },
  latest_version: { id: 101, skill_id: 42, version: 'v1.0.0', status: 'published', review_status: 'approved', changelog: '', source_locked: false, is_current: true, created_at: '2026-01-01T00:00:00Z', published_at: null, submitted_at: null, reviewed_at: null, review_note: null, can_submit_review: false, can_publish: false, can_test: false, can_use: false },
  current_version: { id: 101, skill_id: 42, version: 'v1.0.0', status: 'published', review_status: 'approved', changelog: '', source_locked: false, is_current: true, created_at: '2026-01-01T00:00:00Z', published_at: null, submitted_at: null, reviewed_at: null, review_note: null, can_submit_review: false, can_publish: false, can_test: false, can_use: false },
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
  variable_schema: [],
  content: null,
  metadata: {},
  examples: [],
  readme: null,
  install_note: null,
  can_install: false,
  can_run: true,
}

const skillsStore = vi.hoisted(() => ({
  detail: null as any,
  versionsPagination: {
    items: [
      {
        id: 101,
        skill_id: 42,
        version: 'v1.0.0',
        status: 'published',
        review_status: 'approved',
        changelog: '',
        source_locked: false,
        is_current: true,
        created_at: '2026-01-01T00:00:00Z',
        published_at: null,
        submitted_at: null,
        reviewed_at: null,
        review_note: null,
        can_submit_review: false,
        can_publish: false,
        can_test: false,
        can_use: false,
        variable_schema: [],
        content: null,
        metadata: {},
      },
    ],
    total: 1,
    page: 1,
    page_size: 12,
    pages: 1,
  },
  togglingInstall: false,
  submittingVersionId: null as number | null,
  publishingVersionId: null as number | null,
  runningMode: null as 'test' | 'use' | null,
  runningVersionId: null as number | null,
  loadSkillDetail: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadVersions: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  toggleInstall: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  testSkillVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  useSkillVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  submitVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  publishVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
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

vi.mock('@/stores/skillsCenter', () => ({
  useSkillsCenterStore: () => skillsStore,
}))

vi.mock('@/stores', () => ({
  useAppStore: () => appStore,
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
    template: '<nav />',
  },
}))

vi.mock('@/components/skills/SkillVariableForm.vue', () => ({
  default: {
    name: 'SkillVariableFormStub',
    template: '<section />',
  },
}))

vi.mock('@/components/skills/presentation', () => ({
  formatCurrency: () => 'USD 0.00',
  skillStatusBadgeClass: () => '',
  skillStatusLabel: (value: string) => value,
  skillTypeLabel: (value: string) => value,
  skillVersionBadgeClass: () => '',
  skillVersionReviewBadgeClass: () => '',
  skillVersionReviewStatusLabel: (value: string) => value,
  skillVersionStatusLabel: (value: string) => value,
}))

function resetStore(): void {
  routeState.params.id = '42'
  routeState.query = {}
  routerReplace.mockReset()
  appStore.showError.mockReset()
  appStore.showSuccess.mockReset()
  skillsStore.detail = { ...detail }
  skillsStore.loadSkillDetail.mockReset()
  skillsStore.loadSkillDetail.mockImplementation(async (_skillId: number) => {
    skillsStore.detail = { ...detail }
    return skillsStore.detail
  })
  skillsStore.loadVersions.mockReset()
  skillsStore.loadVersions.mockResolvedValue(undefined)
  skillsStore.toggleInstall.mockReset()
  skillsStore.toggleInstall.mockResolvedValue(undefined)
  skillsStore.testSkillVersion.mockReset()
  skillsStore.testSkillVersion.mockResolvedValue(undefined)
  skillsStore.useSkillVersion.mockReset()
  skillsStore.useSkillVersion.mockResolvedValue(undefined)
  skillsStore.submitVersion.mockReset()
  skillsStore.submitVersion.mockResolvedValue(undefined)
  skillsStore.publishVersion.mockReset()
  skillsStore.publishVersion.mockResolvedValue(undefined)
}

describe('SkillDetailView', () => {
  beforeEach(() => {
    resetStore()
  })

  it('hides install and runs entries for non-owners', async () => {
    const wrapper = mount(SkillDetailView)
    await flushPromises()

    const buttonLabels = wrapper.findAll('button').map((item) => item.text())
    const hrefs = wrapper.findAll('a').map((item) => item.attributes('href'))

    expect(hrefs).not.toContain('/skills/42/runs')
    expect(buttonLabels).not.toContain('安装')
  })
})
