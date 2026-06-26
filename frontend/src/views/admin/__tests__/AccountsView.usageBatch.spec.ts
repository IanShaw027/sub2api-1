import { beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h, ref, computed } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'

import AccountsView from '../AccountsView.vue'

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getBatchUsage,
  getUsage,
  getAllProxies,
  getAllGroups
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getBatchUsage: vi.fn(),
  getUsage: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn()
}))

const visibleRowCount = ref(Number.POSITIVE_INFINITY)

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      getBatchUsage,
      getUsage,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn()
    },
    proxies: {
      getAll: getAllProxies
    },
    groups: {
      getAll: getAllGroups
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
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

const DataTableStub = defineComponent({
  name: 'DataTableStub',
  props: {
    data: {
      type: Array,
      default: () => []
    }
  },
  setup(props, { slots, expose }) {
    const wrapperRef = ref<HTMLElement | null>(null)
    const sortedData = computed(() => props.data as any[])
    const virtualizer = ref({
      getVirtualItems: () => sortedData.value.map((_, index) => ({ index }))
    })

    expose({
      virtualizer,
      sortedData,
      tableWrapperEl: wrapperRef
    })

    return () => h('div', { ref: wrapperRef, 'data-test': 'data-table' }, sortedData.value.slice(0, visibleRowCount.value).map((row: any) =>
      h('div', { key: row.id, 'data-test': `row-${row.id}` }, slots['cell-usage']?.({ row, value: null }))
    ))
  }
})

const AccountTableActionsStub = defineComponent({
  name: 'AccountTableActionsStub',
  emits: ['refresh', 'create'],
  template: `
    <div>
      <button data-test="refresh-btn" @click="$emit('refresh')">refresh</button>
      <button data-test="create-btn" @click="$emit('create')">create</button>
      <slot name="after" />
    </div>
  `
})

function mountView() {
  return mount(AccountsView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        TablePageLayout: {
          template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
        },
        DataTable: DataTableStub,
        Pagination: true,
        ConfirmDialog: true,
        AccountTableActions: AccountTableActionsStub,
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: true,
        AccountActionMenu: true,
        ImportDataModal: true,
        ReAuthAccountModal: true,
        AccountTestModal: true,
        AccountStatsModal: true,
        ScheduledTestsPanel: true,
        SyncFromCrsModal: true,
        TempUnschedStatusModal: true,
        ErrorPassthroughRulesModal: true,
        TLSFingerprintProfilesModal: true,
        TLSFingerprintRoutersModal: true,
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: true,
        PlatformTypeBadge: true,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        HelpTooltip: true,
        UsageProgressBar: true,
        AccountQuotaInfo: true,
        CodexInviteResetModal: true,
        Icon: true
      }
    }
  })
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

async function waitForBatchQueue() {
  await new Promise((resolve) => setTimeout(resolve, 0))
  await flushPromises()
}

describe('admin AccountsView usage batch loading', () => {
  beforeEach(() => {
    localStorage.clear()
    visibleRowCount.value = Number.POSITIVE_INFINITY
    Object.defineProperty(window, 'matchMedia', {
      writable: true,
      value: vi.fn().mockImplementation(() => ({
        matches: true,
        media: '(min-width: 768px)',
        onchange: null,
        addListener: vi.fn(),
        removeListener: vi.fn(),
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
        dispatchEvent: vi.fn(),
      }))
    })

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getBatchUsage.mockReset()
    getUsage.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()

    listAccounts.mockResolvedValue({
      items: [
        {
          id: 101,
          name: 'openai-oauth',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          concurrency: 1,
          priority: 1,
          schedulable: true,
          last_used_at: null,
          expires_at: null,
          auto_pause_on_expired: true,
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
          error_message: null,
          proxy_id: null,
          rate_limited_at: null,
          rate_limit_reset_at: null,
          overload_until: null,
          temp_unschedulable_until: null,
          temp_unschedulable_reason: null,
          session_window_start: null,
          session_window_end: null,
          session_window_status: null,
          extra: {},
          groups: []
        },
        {
          id: 102,
          name: 'anthropic-service-account',
          platform: 'anthropic',
          type: 'service_account',
          status: 'active',
          concurrency: 1,
          priority: 1,
          schedulable: true,
          last_used_at: null,
          expires_at: null,
          auto_pause_on_expired: true,
          created_at: '2026-06-25T00:00:00Z',
          updated_at: '2026-06-25T00:00:00Z',
          error_message: null,
          proxy_id: null,
          rate_limited_at: null,
          rate_limit_reset_at: null,
          overload_until: null,
          temp_unschedulable_until: null,
          temp_unschedulable_reason: null,
          session_window_start: null,
          session_window_end: null,
          session_window_status: null,
          extra: {},
          groups: []
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getBatchUsage.mockResolvedValue({
      usage: {
        '101': {
          five_hour: {
            utilization: 18,
            resets_at: '2026-06-25T12:00:00Z',
            remaining_seconds: 3600,
            window_stats: {
              requests: 9,
              tokens: 900,
              cost: 0.09,
              standard_cost: 0.09,
              user_cost: 0.09
            }
          }
        },
        '102': {
          source: 'passive',
          five_hour: {
            utilization: 31,
            resets_at: '2026-06-25T12:00:00Z',
            remaining_seconds: 3600,
            window_stats: {
              requests: 5,
              tokens: 500,
              cost: 0.05,
              standard_cost: 0.05,
              user_cost: 0.05
            }
          }
        }
      },
      errors: {}
    })
    getUsage.mockResolvedValue({})
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
  })

  it('desktop mount batches rendered account usage requests instead of calling per-row getUsage', async () => {
    mountView()

    await flushPromises()
    await flushPromises()

    expect(getBatchUsage).toHaveBeenCalledTimes(1)
    expect(getBatchUsage).toHaveBeenCalledWith([101, 102], false)
    expect(getUsage).not.toHaveBeenCalled()
  })

  it('does not load proxy options until the create modal is opened', async () => {
    const wrapper = mountView()

    await flushPromises()
    await flushPromises()

    expect(getAllGroups).toHaveBeenCalledTimes(1)
    expect(getAllProxies).not.toHaveBeenCalled()

    await wrapper.get('[data-test="create-btn"]').trigger('click')
    await flushPromises()

    expect(getAllProxies).toHaveBeenCalledTimes(1)
  })

  it('manual refresh re-runs batch usage loading for rendered rows', async () => {
    const wrapper = mountView()

    await flushPromises()
    await flushPromises()

    await wrapper.get('[data-test="refresh-btn"]').trigger('click')
    await flushPromises()
    await flushPromises()

    expect(getBatchUsage).toHaveBeenCalledTimes(2)
    expect(getBatchUsage).toHaveBeenLastCalledWith([101, 102], true)
    expect(getUsage).not.toHaveBeenCalled()
  })

  it('keeps earlier account batch state when a later batch request targets different rows', async () => {
    visibleRowCount.value = 1

    const firstBatch = deferred<{
      usage: Record<string, any>
      errors: Record<string, string>
    }>()
    const secondBatch = deferred<{
      usage: Record<string, any>
      errors: Record<string, string>
    }>()
    getBatchUsage
      .mockImplementationOnce(() => firstBatch.promise)
      .mockImplementationOnce(() => secondBatch.promise)

    const wrapper = mountView()
    await waitForBatchQueue()

    expect(getBatchUsage).toHaveBeenCalledTimes(1)
    expect(getBatchUsage).toHaveBeenLastCalledWith([101], false)
    expect((wrapper.vm as any).usageBatchLoadingByAccountId['101']).toBe(true)

    visibleRowCount.value = 2
    await waitForBatchQueue()

    expect(wrapper.find('[data-test="row-102"]').exists()).toBe(true)
    expect(getBatchUsage).toHaveBeenCalledTimes(2)
    expect(getBatchUsage).toHaveBeenLastCalledWith([102], false)
    expect((wrapper.vm as any).usageBatchLoadingByAccountId['102']).toBe(true)

    firstBatch.resolve({
      usage: {
        '101': {
          five_hour: {
            utilization: 27,
            resets_at: '2026-06-25T12:00:00Z',
            remaining_seconds: 1800,
            window_stats: {
              requests: 7,
              tokens: 700,
              cost: 0.07,
              standard_cost: 0.07,
              user_cost: 0.07
            }
          }
        }
      },
      errors: {}
    })
    await flushPromises()

    expect((wrapper.vm as any).usageBatchLoadingByAccountId['101']).toBe(false)
    expect((wrapper.vm as any).usageBatchByAccountId['101']?.five_hour?.utilization).toBe(27)

    secondBatch.resolve({
      usage: {
        '102': {
          source: 'passive',
          five_hour: {
            utilization: 31,
            resets_at: '2026-06-25T12:00:00Z',
            remaining_seconds: 3600,
            window_stats: {
              requests: 5,
              tokens: 500,
              cost: 0.05,
              standard_cost: 0.05,
              user_cost: 0.05
            }
          }
        }
      },
      errors: {}
    })
    await flushPromises()
  })
})
