import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import GroupsView from '../GroupsView.vue'

const {
  listGroups,
  getUsageSummary,
  getCapacitySummary,
  getModelsListCandidates,
  getAccountById,
  showError
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getModelsListCandidates: vi.fn(),
  getAccountById: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    groups: {
      list: listGroups,
      getUsageSummary,
      getCapacitySummary,
      getModelsListCandidates,
      create: vi.fn(),
      update: vi.fn(),
      deleteGroup: vi.fn(),
      getAll: vi.fn(),
      updateSortOrder: vi.fn()
    },
    accounts: {
      getById: getAccountById
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/onboarding', () => ({
  useOnboardingStore: () => ({
    isCurrentStep: vi.fn(() => false),
    nextStep: vi.fn()
  })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

const DataTableStub = {
  props: ['data'],
  template: `
    <div data-test="groups-table">
      <div v-for="row in data" :key="row.id" :data-test="['group-row', row.id].join('-')">
        <slot name="cell-account_count" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /></div>'
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function buildGroup(id: number, name: string, modelRouting: Record<string, number[]>) {
  return {
    id,
    name,
    description: `${name} description`,
    platform: 'openai',
    rate_multiplier: 1,
    is_exclusive: false,
    status: 'active',
    subscription_type: 'standard',
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    allow_image_generation: false,
    image_generation_route: 'codex',
    image_rate_independent: false,
    image_rate_multiplier: 1,
    image_price_1k: null,
    image_price_2k: null,
    image_price_4k: null,
    images2api_price_1k: null,
    images2api_price_2k: null,
    images2api_price_4k: null,
    claude_code_only: false,
    fallback_group_id: null,
    fallback_group_id_on_invalid_request: null,
    allow_messages_dispatch: false,
    messages_dispatch_model_config: null,
    require_oauth_only: false,
    require_privacy_set: false,
    model_routing_enabled: true,
    model_routing: modelRouting,
    supported_model_scopes: ['claude'],
    mcp_xml_inject: true,
    rpm_limit: 0,
    account_count: 0,
    active_account_count: 0,
    rate_limited_account_count: 0
  } as any
}

function mountGroupsView() {
  return mount(GroupsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        BaseDialog: BaseDialogStub,
        Pagination: true,
        ConfirmDialog: true,
        Select: {
          props: ['modelValue', 'options', 'placeholder'],
          template: '<div data-test="select-stub"></div>'
        },
        Icon: true,
        PlatformIcon: true,
        EmptyState: true,
        GroupCapacityBadge: true,
        GroupRateMultipliersModal: true,
        GroupRPMOverridesModal: true
      }
    }
  })
}

describe('admin GroupsView edit hydration', () => {
  beforeEach(() => {
    listGroups.mockReset()
    getUsageSummary.mockReset()
    getCapacitySummary.mockReset()
    getModelsListCandidates.mockReset()
    getAccountById.mockReset()
    showError.mockReset()

    listGroups.mockResolvedValue({
      items: [
        buildGroup(1, 'Group Alpha', { 'gpt-4o': [101] }),
        buildGroup(2, 'Group Beta', { 'gpt-4.1': [202] })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getUsageSummary.mockResolvedValue([])
    getCapacitySummary.mockResolvedValue([])
    getModelsListCandidates.mockResolvedValue([])
  })

  it('ignores stale async hydration when edit is clicked again before the first request resolves', async () => {
    const firstAccount = deferred<{ id: number; name: string }>()
    const secondAccount = deferred<{ id: number; name: string }>()

    getAccountById.mockImplementation((id: number) => {
      if (id === 101) return firstAccount.promise
      if (id === 202) return secondAccount.promise
      throw new Error(`unexpected account id ${id}`)
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-1"] button').trigger('click')
    await wrapper.get('[data-test="group-row-2"] button').trigger('click')

    secondAccount.resolve({ id: 202, name: 'Account 202' })
    await flushPromises()

    const nameInput = wrapper.get('input[data-tour="edit-group-form-name"]')
    expect((nameInput.element as HTMLInputElement).value).toBe('Group Beta')

    firstAccount.resolve({ id: 101, name: 'Account 101' })
    await flushPromises()

    expect((wrapper.get('input[data-tour="edit-group-form-name"]').element as HTMLInputElement).value).toBe('Group Beta')
  })

  it('renders available accounts directly from active_account_count without subtracting rate-limited twice', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(3, 'Group Gamma', {}),
          account_count: 5,
          active_account_count: 2,
          rate_limited_account_count: 3
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()
    await flushPromises()

    const rowText = wrapper.get('[data-test="group-row-3"]').text()
    expect(rowText).toContain('admin.groups.accountsAvailable')
    expect(rowText).toContain('2')
    expect(rowText).toContain('admin.groups.accountsRateLimited')
    expect(rowText).toContain('3')
    expect(rowText).not.toContain('-1')
  })
})
