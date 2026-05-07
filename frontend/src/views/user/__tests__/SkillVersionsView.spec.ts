import { flushPromises, mount } from '@vue/test-utils'
import { reactive } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import SkillVersionsView from '@/views/user/SkillVersionsView.vue'

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
    editable: true,
    owned: true,
    latest_version: { version: 'v1.0.0' },
    type: 'prompt_chat',
    name: 'Skill',
  },
  versionsPagination: {
    items: [
      {
        id: 101,
        skill_id: 42,
        version: 'v1.0.0',
        status: 'draft',
        review_status: 'draft',
        changelog: '',
        source_locked: false,
        is_current: false,
        created_at: '2026-01-01T00:00:00Z',
        published_at: null,
        submitted_at: null,
        reviewed_at: null,
        review_note: null,
        can_submit_review: true,
        can_publish: false,
        can_test: false,
        can_use: false,
        variable_schema: [],
        content: null,
        metadata: {},
      },
    ],
    total: 60,
    page: 1,
    page_size: 12,
    pages: 5,
  },
  loadingVersions: false,
  loadingDetail: false,
  creatingVersion: false,
  updatingVersion: false,
  submittingVersionId: null as number | null,
  publishingVersionId: null as number | null,
  runningMode: null as 'test' | 'use' | null,
  runningVersionId: null as number | null,
  loadSkillDetail: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  loadVersions: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  addVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  saveVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  submitVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  publishVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  testSkillVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
  useSkillVersion: vi.fn<(...args: unknown[]) => Promise<void>>().mockResolvedValue(undefined),
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

vi.mock('@/components/common/Input.vue', () => ({
  default: {
    name: 'InputStub',
    template: '<input />',
  },
}))

vi.mock('@/components/common/Pagination.vue', () => ({
  default: {
    name: 'PaginationStub',
    emits: ['update:page', 'update:pageSize'],
    template: '<div class="pagination-stub" />',
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

vi.mock('@/components/skills/presentation', () => ({
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
  skillsStore.versionsPagination.page = 1
  skillsStore.versionsPagination.page_size = 12
  skillsStore.loadSkillDetail.mockReset()
  skillsStore.loadSkillDetail.mockResolvedValue(undefined)
  skillsStore.loadVersions.mockReset()
  skillsStore.loadVersions.mockImplementation(async (_skillId: number, page: number, pageSize: number) => {
    skillsStore.versionsPagination.page = page
    skillsStore.versionsPagination.page_size = pageSize
  })
  skillsStore.addVersion.mockReset()
  skillsStore.saveVersion.mockReset()
  skillsStore.submitVersion.mockReset()
  skillsStore.publishVersion.mockReset()
  skillsStore.testSkillVersion.mockReset()
  skillsStore.useSkillVersion.mockReset()
}

describe('SkillVersionsView', () => {
  beforeEach(() => {
    resetStore()
  })

  it('restores page and page_size from the URL and writes pagination changes back', async () => {
    routeState.query = {
      page: '3',
      page_size: '24',
    }

    const wrapper = mount(SkillVersionsView)
    await flushPromises()

    expect(skillsStore.loadSkillDetail).toHaveBeenCalledWith(42, false)
    expect(skillsStore.loadVersions).toHaveBeenCalledWith(42, 3, 24)

    const pagination = wrapper.findComponent({ name: 'PaginationStub' })
    await pagination.vm.$emit('update:page', 4)
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        page: '4',
        page_size: '24',
      },
    })
    expect(skillsStore.loadVersions).toHaveBeenLastCalledWith(42, 4, 24)

    await pagination.vm.$emit('update:pageSize', 50)
    await flushPromises()

    expect(routerReplace).toHaveBeenLastCalledWith({
      query: {
        page_size: '50',
      },
    })
    expect(skillsStore.loadVersions).toHaveBeenLastCalledWith(42, 1, 50)
  })
})
