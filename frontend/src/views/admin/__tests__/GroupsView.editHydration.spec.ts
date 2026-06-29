import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import GroupsView from '../GroupsView.vue'

const {
  listGroups,
  getUsageSummary,
  getCapacitySummary,
  getModelsListCandidates,
  createGroup,
  updateGroup,
  getAccountById,
  showError
} = vi.hoisted(() => ({
  listGroups: vi.fn(),
  getUsageSummary: vi.fn(),
  getCapacitySummary: vi.fn(),
  getModelsListCandidates: vi.fn(),
  createGroup: vi.fn(),
  updateGroup: vi.fn(),
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
      create: createGroup,
      update: updateGroup,
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
        <slot name="cell-usage" :row="row" />
        <slot name="cell-actions" :row="row" />
      </div>
    </div>
  `
}

const BaseDialogStub = {
  props: ['show'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
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

async function settleAsyncState() {
  await flushPromises()
  await new Promise((resolve) => setTimeout(resolve, 0))
  await flushPromises()
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
    allow_video_generation: false,
    video_generation_route: 'native',
    video_price_480p_per_sec: null,
    video_price_720p_per_sec: null,
    video_price_1080p_per_sec: null,
    video_price_4k_per_sec: null,
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
          props: ['modelValue', 'options', 'placeholder', 'disabled'],
          emits: ['update:modelValue', 'change'],
          template: `
            <select
              data-test="select-stub"
              :value="modelValue"
              :disabled="disabled"
              @change="
                $emit('update:modelValue', $event.target.value);
                $emit('change', $event.target.value);
              "
            >
              <option value=""></option>
              <option
                v-for="option in options"
                :key="String(option.value)"
                :value="option.value ?? ''"
              >
                {{ option.label }}
              </option>
            </select>
          `
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
    createGroup.mockReset()
    updateGroup.mockReset()
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
    createGroup.mockResolvedValue({})
    updateGroup.mockResolvedValue({})
  })

  it('requests usage summary only for the current page groups', async () => {
    mountGroupsView()

    await flushPromises()

    expect(getUsageSummary).toHaveBeenCalledTimes(1)
    expect(getUsageSummary).toHaveBeenCalledWith(
      expect.any(String),
      [1, 2]
    )
  })

  it('ignores stale usage summary responses after the visible group page changes', async () => {
    const firstUsageSummary = deferred<Array<{ group_id: number; today_cost: number; total_cost: number }>>()
    const secondUsageSummary = deferred<Array<{ group_id: number; today_cost: number; total_cost: number }>>()
    getUsageSummary
      .mockImplementationOnce(() => firstUsageSummary.promise)
      .mockImplementationOnce(() => secondUsageSummary.promise)

    const wrapper = mountGroupsView()
    await flushPromises()

    expect(getUsageSummary).toHaveBeenCalledTimes(1)
    expect(getUsageSummary).toHaveBeenLastCalledWith(expect.any(String), [1, 2])

    listGroups.mockResolvedValueOnce({
      items: [
        buildGroup(3, 'Gamma', { 'gpt-4.1': [303] }),
        buildGroup(4, 'Delta', { 'gpt-4.1-mini': [404] }),
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1,
    })

    await wrapper.get('button[title="common.refresh"]').trigger('click')
    await flushPromises()

    expect(getUsageSummary).toHaveBeenCalledTimes(2)
    expect(getUsageSummary).toHaveBeenLastCalledWith(expect.any(String), [3, 4])

    secondUsageSummary.resolve([
      { group_id: 3, today_cost: 3.3, total_cost: 33.3 },
      { group_id: 4, today_cost: 4.4, total_cost: 44.4 },
    ])
    await settleAsyncState()
    expect(getUsageSummary).toHaveBeenCalledTimes(2)
    expect(wrapper.find('[data-test="group-row-3"]').text()).toContain('3.30')
    expect(wrapper.find('[data-test="group-row-4"]').text()).toContain('44.40')
    expect(wrapper.find('[data-test="group-row-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="group-row-2"]').exists()).toBe(false)

    firstUsageSummary.resolve([
      { group_id: 1, today_cost: 1.1, total_cost: 11.1 },
      { group_id: 2, today_cost: 2.2, total_cost: 22.2 },
    ])
    await settleAsyncState()

    expect(wrapper.find('[data-test="group-row-3"]').text()).toContain('3.30')
    expect(wrapper.find('[data-test="group-row-4"]').text()).toContain('44.40')
    expect(wrapper.find('[data-test="group-row-1"]').exists()).toBe(false)
    expect(wrapper.find('[data-test="group-row-2"]').exists()).toBe(false)
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
    getModelsListCandidates.mockClear()
    await wrapper.get('[data-test="group-row-1"] button').trigger('click')
    await wrapper.get('[data-test="group-row-2"] button').trigger('click')

    secondAccount.resolve({ id: 202, name: 'Account 202' })
    await flushPromises()

    const nameInput = wrapper.get('input[data-tour="edit-group-form-name"]')
    expect((nameInput.element as HTMLInputElement).value).toBe('Group Beta')
    expect(getModelsListCandidates).toHaveBeenCalledTimes(2)
    expect(getModelsListCandidates).toHaveBeenLastCalledWith(2, 'openai')

    firstAccount.resolve({ id: 101, name: 'Account 101' })
    await flushPromises()

    expect((wrapper.get('input[data-tour="edit-group-form-name"]').element as HTMLInputElement).value).toBe('Group Beta')
    expect(getModelsListCandidates).toHaveBeenCalledTimes(2)
    expect(getModelsListCandidates).toHaveBeenLastCalledWith(2, 'openai')
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

  it('submits the default refund multiplier when creating a group', async () => {
    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')
    await wrapper.get('input[data-tour="group-form-name"]').setValue('Group Delta')
    await wrapper.get('#create-group-form').trigger('submit')
    await flushPromises()

    expect(createGroup).toHaveBeenCalledTimes(1)
    expect(createGroup.mock.calls[0][0]).toMatchObject({
      name: 'Group Delta',
      refund_rate_multiplier: 1
    })
  })

  it('includes Kiro in group platform selectors', async () => {
    const wrapper = mountGroupsView()

    await flushPromises()

    const filterOptions = wrapper
      .findAll('[data-test="select-stub"]')[0]
      .findAll('option')
      .map((option) => option.text())
    expect(filterOptions).toContain('Kiro')

    await wrapper.get('[data-tour="groups-create-btn"]').trigger('click')

    const createOptions = wrapper
      .get('select[data-tour="group-form-platform"]')
      .findAll('option')
      .map((option) => option.text())
    expect(createOptions).toContain('Kiro')
  })

  it('resets pagination to the first page when a filter dropdown changes', async () => {
    const wrapper = mountGroupsView()

    await flushPromises()
    ;(wrapper.vm as any).pagination.page = 2
    await wrapper.vm.$nextTick()

    listGroups.mockClear()
    const platformFilter = wrapper.findAll('[data-test="select-stub"]')[0]
    await platformFilter.setValue('openai')
    await flushPromises()

    expect((wrapper.vm as any).pagination.page).toBe(1)
    expect(listGroups).toHaveBeenCalledTimes(1)
    expect(listGroups).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        platform: 'openai'
      }),
      expect.any(Object)
    )
  })

  it('normalizes legacy OpenAI web2api image pricing fields when editing', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(4, 'Group Image', {}),
          allow_image_generation: true,
          image_generation_route: 'web2api',
          image_rate_independent: true,
          image_rate_multiplier: 1.75,
          image_price_1k: 0.25,
          image_price_2k: 0.35,
          image_price_4k: 0.45,
          allow_video_generation: true,
          video_generation_route: 'native',
          video_price_480p_per_sec: 0.01,
          video_price_720p_per_sec: 0.02,
          video_price_1080p_per_sec: 0.03,
          video_price_4k_per_sec: 0.04,
          refund_rate_multiplier: 2.5
        } as any
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-4"] button').trigger('click')
    await flushPromises()
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(updateGroup.mock.calls[0][1]).toMatchObject({
      refund_rate_multiplier: 2.5,
      image_generation_route: 'codex',
      image_rate_independent: false,
      image_rate_multiplier: 1.75,
      image_price_1k: 0.25,
      image_price_2k: 0.35,
      image_price_4k: 0.45,
      allow_video_generation: true,
      video_generation_route: 'native',
      video_price_480p_per_sec: 0.01,
      video_price_720p_per_sec: 0.02,
      video_price_1080p_per_sec: 0.03,
      video_price_4k_per_sec: 0.04
    })
    expect(updateGroup.mock.calls[0][1]).not.toHaveProperty('images2api_price_1k')
    expect(updateGroup.mock.calls[0][1]).not.toHaveProperty('images2api_price_2k')
    expect(updateGroup.mock.calls[0][1]).not.toHaveProperty('images2api_price_4k')
  })

  it('preserves Grok video pricing fields when editing', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(5, 'Group Grok Video', {}),
          platform: 'grok',
          allow_video_generation: true,
          video_generation_route: 'native',
          video_price_480p_per_sec: 0.01,
          video_price_720p_per_sec: 0.02,
          video_price_1080p_per_sec: 0.03,
          video_price_4k_per_sec: 0.04
        } as any
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-5"] button').trigger('click')
    await flushPromises()
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(updateGroup.mock.calls[0][1]).toMatchObject({
      platform: 'grok',
      allow_video_generation: true,
      video_generation_route: 'native',
      video_price_480p_per_sec: 0.01,
      video_price_720p_per_sec: 0.02,
      video_price_1080p_per_sec: 0.03,
      video_price_4k_per_sec: 0.04
    })
  })

  it('shows Grok image pricing controls without a route selector', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(6, 'Group Grok Generic Video', {}),
          platform: 'grok',
          allow_video_generation: true,
          video_generation_route: 'native'
        } as any
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-6"] button').trigger('click')
    await flushPromises()

    const editForm = wrapper.get('#edit-group-form')
    expect(editForm.text()).toContain('admin.groups.videoPricing.title')
    expect(editForm.text()).toContain('admin.groups.imagePricing.title')
    expect(editForm.text()).not.toContain('admin.groups.imagePricing.routeCodex')
  })

  it('submits explicit nulls when cleared Grok video prices are saved', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(7, 'Group Grok Clear Video', {}),
          platform: 'grok',
          allow_video_generation: true,
          video_generation_route: 'native',
          video_price_480p_per_sec: 0.01,
          video_price_720p_per_sec: 0.02,
          video_price_1080p_per_sec: 0.03,
          video_price_4k_per_sec: 0.04
        } as any
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-7"] button').trigger('click')
    await flushPromises()

    const videoPriceInputs = wrapper
      .get('#edit-group-form')
      .findAll('input[type="number"]')
      .filter((input) => (input.element as HTMLInputElement).placeholder === '0.01' ||
        (input.element as HTMLInputElement).placeholder === '0.02' ||
        (input.element as HTMLInputElement).placeholder === '0.03')
    const video4kInput = wrapper
      .get('#edit-group-form')
      .findAll('input[type="number"]')
      .find((input) => (input.element as HTMLInputElement).placeholder === '0.04')

    expect(videoPriceInputs).toHaveLength(3)
    expect(video4kInput).toBeTruthy()
    for (const input of [...videoPriceInputs, video4kInput!]) {
      await input.setValue('')
    }
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(updateGroup.mock.calls[0][1]).toMatchObject({
      platform: 'grok',
      allow_video_generation: true,
      video_generation_route: 'native',
      video_price_480p_per_sec: null,
      video_price_720p_per_sec: null,
      video_price_1080p_per_sec: null,
      video_price_4k_per_sec: null
    })
  })

  it('shows explicit search and audio pricing only for Grok groups', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        buildGroup(8, 'Group OpenAI Explicit Hidden', {}),
        {
          ...buildGroup(9, 'Group Grok Explicit Visible', {}),
          platform: 'grok',
          search_price_per_1k: 5,
          audio_realtime_price_per_min: 0.2,
          audio_tts_price_per_million_chars: 1.5,
          audio_stt_price_per_hour: 3
        } as any
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-8"] button').trigger('click')
    await flushPromises()
    expect(wrapper.get('#edit-group-form').text()).not.toContain('admin.groups.explicitPricing.title')

    await wrapper.get('[data-test="group-row-9"] button').trigger('click')
    await flushPromises()
    const editForm = wrapper.get('#edit-group-form')
    expect(editForm.text()).toContain('admin.groups.explicitPricing.title')
    const placeholders = editForm
      .findAll('input[type="number"]')
      .map((input) => (input.element as HTMLInputElement).placeholder)
    expect(placeholders).toContain('admin.groups.explicitPricing.pricePlaceholder')
    expect(placeholders).not.toContain('0')
  })


  it('hydrates and submits cleared Grok explicit search and audio prices', async () => {
    listGroups.mockResolvedValueOnce({
      items: [
        {
          ...buildGroup(10, 'Group Grok Explicit Clear', {}),
          platform: 'grok',
          search_price_per_1k: 5,
          audio_realtime_price_per_min: 0.25,
          audio_tts_price_per_million_chars: 1.5,
          audio_stt_price_per_hour: 2.75
        } as any
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountGroupsView()

    await flushPromises()
    await wrapper.get('[data-test="group-row-10"] button').trigger('click')
    await flushPromises()

    const explicitInputs = wrapper
      .get('#edit-group-form')
      .findAll('input[type="number"]')
      .filter((input) => (input.element as HTMLInputElement).placeholder === 'admin.groups.explicitPricing.pricePlaceholder')

    expect(explicitInputs).toHaveLength(4)
    expect((explicitInputs[0].element as HTMLInputElement).value).toBe('5')
    expect((explicitInputs[1].element as HTMLInputElement).value).toBe('0.25')
    expect((explicitInputs[2].element as HTMLInputElement).value).toBe('1.5')
    expect((explicitInputs[3].element as HTMLInputElement).value).toBe('2.75')

    for (const input of explicitInputs) {
      await input.setValue('')
    }
    await wrapper.get('#edit-group-form').trigger('submit')
    await flushPromises()

    expect(updateGroup).toHaveBeenCalledTimes(1)
    expect(updateGroup.mock.calls[0][1]).toMatchObject({
      platform: 'grok',
      search_price_per_1k: null,
      audio_realtime_price_per_min: null,
      audio_tts_price_per_million_chars: null,
      audio_stt_price_per_hour: null
    })
  })

})
