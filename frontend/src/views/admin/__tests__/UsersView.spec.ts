import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import { formatDateTime } from '@/utils/format'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getById,
  getPlatformQuotas,
  getAllGroups,
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getById: vi.fn(),
  getPlatformQuotas: vi.fn(),
  getAllGroups: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      getById,
      getPlatformQuotas,
      toggleStatus: vi.fn(),
      delete: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    },
    userAttributes: {
      listEnabledDefinitions,
      getBatchUserAttributes
    }
  }
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
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

const createAdminUser = (overrides: Partial<AdminUser> = {}): AdminUser => ({
  id: 42,
  username: 'scoped-user',
  email: 'scoped@example.com',
  role: 'user',
  balance: 0,
  concurrency: 1,
  status: 'active',
  allowed_groups: [],
  balance_notify_enabled: false,
  balance_notify_threshold: null,
  balance_notify_extra_emails: [],
  created_at: '2026-04-17T00:00:00Z',
  updated_at: '2026-04-17T00:00:00Z',
  notes: '',
  last_login_at: '2026-04-15T02:00:00Z',
  last_active_at: '2026-04-16T02:00:00Z',
  last_used_at: '2026-04-17T02:00:00Z',
  today_actual_cost: 0,
  today_balance_actual_cost: 0,
  today_subscription_actual_cost: 0,
  total_actual_cost: 0,
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <template v-for="col in columns" :key="col.key">
        <slot :name="'header-' + col.key" :column="col" />
      </template>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-email" :value="row.email" :row="row" />
        <div data-test="usage-cell">
          <slot name="cell-usage" :row="row" />
        </div>
        <div data-test="concurrency-cell">
          <slot name="cell-concurrency" :row="row" />
        </div>
        <div data-test="platform-quota-cell">
          <slot name="cell-balance_platform_quota" :row="row" />
        </div>
        <div data-test="last-active-cell">
          <slot name="cell-last_active_at" :value="row.last_active_at" :row="row" />
        </div>
        <div data-test="last-used-cell">
          <slot name="cell-last_used_at" :value="row.last_used_at" :row="row" />
        </div>
      </div>
    </div>
  `
}

describe('admin UsersView', () => {
  const openSpy = vi.spyOn(window, 'open').mockImplementation(() => null)

  beforeEach(() => {
    vi.useRealTimers()
    localStorage.clear()
    openSpy.mockClear()

    listUsers.mockReset()
    getAllGroups.mockReset()
    getById.mockReset()
    getPlatformQuotas.mockReset()
    listEnabledDefinitions.mockReset()
    getBatchUserAttributes.mockReset()

    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    getAllGroups.mockResolvedValue([])
    getById.mockResolvedValue(createAdminUser())
    getPlatformQuotas.mockResolvedValue({ platform_quotas: [] })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  it('defaults to sorting by created time in descending order', async () => {
    mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        include_subscriptions: false,
        include_usage_stats: true,
        sort_by: 'created_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('renders trusted avatar URLs with privacy-preserving image attributes', async () => {
    listUsers.mockResolvedValueOnce({
      items: [createAdminUser({ avatar_url: 'https://cdn.example.com/avatar.png' } as Partial<AdminUser>)],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const avatar = wrapper.get('img[alt="scoped@example.com"]')
    expect(avatar.attributes('src')).toBe('https://cdn.example.com/avatar.png')
    expect(avatar.attributes('referrerpolicy')).toBe('no-referrer')
    expect(avatar.attributes('loading')).toBe('lazy')
  })

  it('does not render unsafe avatar URLs in the admin users table', async () => {
    listUsers.mockResolvedValueOnce({
      items: [createAdminUser({ avatar_url: 'javascript:alert(1)' } as Partial<AdminUser>)],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.find('img[alt="scoped@example.com"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('S')
  })

  it('loads full user detail on demand before opening the platform quota modal', async () => {
    const detailedUser = createAdminUser({
      subscriptions: [
        {
          id: 7,
          user_id: 42,
          plan_id: 3,
          plan_name: 'Pro',
          status: 'active',
          billing_cycle: 'monthly',
          start_date: '2026-04-01T00:00:00Z',
          end_date: '2026-05-01T00:00:00Z',
          created_at: '2026-04-01T00:00:00Z',
          updated_at: '2026-04-01T00:00:00Z'
        } as any
      ]
    })
    getById.mockResolvedValueOnce(detailedUser)

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserPlatformQuotaModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await (wrapper.vm as unknown as {
      handlePlatformQuota: (user: AdminUser) => Promise<void>
    }).handlePlatformQuota(createAdminUser())
    await flushPromises()

    expect(getById).toHaveBeenCalledWith(42)
    expect((wrapper.vm as unknown as { showPlatformQuotaModal: boolean }).showPlatformQuotaModal).toBe(true)
    expect((wrapper.vm as unknown as { platformQuotaUser: AdminUser | null }).platformQuotaUser?.subscriptions).toEqual(
      detailedUser.subscriptions
    )
  })

  it('restores persisted special sort without DataTable overriding it', async () => {
    localStorage.setItem('admin-users-table-sort', JSON.stringify({
      key: 'available_concurrency',
      order: 'asc'
    }))

    mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'available_concurrency',
        sort_order: 'asc'
      }),
      expect.any(Object)
    )
    expect(JSON.parse(localStorage.getItem('admin-users-table-sort') || '{}')).toEqual({
      key: 'available_concurrency',
      order: 'asc'
    })
  })

  it('restores persisted standard column sort on reload', async () => {
    localStorage.setItem('admin-users-table-sort', JSON.stringify({
      key: 'email',
      order: 'asc'
    }))

    mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(listUsers).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'email',
        sort_order: 'asc'
      }),
      expect.any(Object)
    )
  })

  it('shows split usage totals from the list response', async () => {
    listUsers.mockResolvedValue({
      items: [
        createAdminUser({
          today_actual_cost: 0.17,
          today_balance_actual_cost: 0.12,
          today_subscription_actual_cost: 0.05,
          total_actual_cost: 1.23
        })
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const visibleColumns = wrapper.get('[data-test="columns"]').text().split(',')
    expect(visibleColumns).toContain('usage')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$0.1200')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$0.0500')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$1.2300')
  })

  it('sorts the current page by usage without replacing the persisted server sort flow', async () => {
    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'low-usage@example.com', today_balance_actual_cost: 1, total_actual_cost: 5 }),
        createAdminUser({ id: 2, email: 'high-usage@example.com', today_balance_actual_cost: 9, total_actual_cost: 2 })
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('low-usage@example.com,high-usage@example.com')

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today_balance"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-usage@example.com,low-usage@example.com')
    expect(localStorage.getItem('admin-users-usage-sort')).toContain('"key":"usage"')
    expect(listUsers).toHaveBeenCalledTimes(1)

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('clears current-page usage sort before applying server-side concurrency sort', async () => {
    const rows = [
      createAdminUser({ id: 1, email: 'low-usage@example.com', current_concurrency: 9, today_balance_actual_cost: 1 }),
      createAdminUser({ id: 2, email: 'high-usage@example.com', current_concurrency: 1, today_balance_actual_cost: 9 })
    ]
    listUsers.mockImplementation(async (_page, _pageSize, params) => {
      const items = [...rows]
      if (params.sort_by === 'current_concurrency') {
        items.sort((a, b) => (b.current_concurrency ?? 0) - (a.current_concurrency ?? 0))
      }
      return {
        items,
        total: 2,
        page: 1,
        page_size: 20,
        pages: 1
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="usage-sort-usage-today_balance"]').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-usage@example.com,low-usage@example.com')

    await wrapper.get('[data-test="concurrency-sort-trigger"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="concurrency-sort-option-current_concurrency"]').trigger('click')
    await flushPromises()

    expect(localStorage.getItem('admin-users-usage-sort')).toBeNull()
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'current_concurrency',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('low-usage@example.com,high-usage@example.com')
  })

  it('closes the usage sort menu when clicking outside the trigger', async () => {
    const wrapper = mount(UsersView, {
      attachTo: document.body,
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    await wrapper.get('[data-test="usage-sort-trigger-usage"]').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-test="usage-sort-usage-today_balance"]').exists()).toBe(true)

    document.body.click()
    await flushPromises()

    expect(wrapper.find('[data-test="usage-sort-usage-today_balance"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('falls back missing usage totals to zero instead of a dash', async () => {
    listUsers.mockResolvedValue({
      items: [
        createAdminUser({
          today_actual_cost: undefined,
          today_balance_actual_cost: undefined,
          today_subscription_actual_cost: undefined,
          total_actual_cost: undefined
        })
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$0.0000')
    expect(wrapper.get('[data-test="usage-cell"]').text()).not.toContain('-')
  })

  it('sorts the current page by runtime current and available concurrency', async () => {
    const rows = [
      createAdminUser({ id: 1, email: 'low-current@example.com', concurrency: 5, current_concurrency: 1 }),
      createAdminUser({ id: 2, email: 'high-current@example.com', concurrency: 5, current_concurrency: 4 }),
      createAdminUser({ id: 3, email: 'high-available@example.com', concurrency: 10, current_concurrency: 2 })
    ]
    listUsers.mockImplementation(async (_page, _pageSize, params) => {
      const items = [...rows]
      if (params.sort_by === 'current_concurrency') {
        items.sort((a, b) => params.sort_order === 'asc'
          ? (a.current_concurrency ?? 0) - (b.current_concurrency ?? 0)
          : (b.current_concurrency ?? 0) - (a.current_concurrency ?? 0))
      }
      if (params.sort_by === 'available_concurrency') {
        const available = (row: AdminUser) => Math.max((row.concurrency ?? 0) - (row.current_concurrency ?? 0), 0)
        items.sort((a, b) => params.sort_order === 'asc'
          ? available(a) - available(b)
          : available(b) - available(a))
      }
      return {
        items,
        total: 3,
        page: 1,
        page_size: 20,
        pages: 1
      }
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await wrapper.get('[data-test="concurrency-sort-trigger"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="concurrency-sort-option-current_concurrency"]').trigger('click')
    await flushPromises()
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'current_concurrency',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-current@example.com,high-available@example.com,low-current@example.com')

    await wrapper.get('[data-test="concurrency-sort-trigger"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="concurrency-sort-option-current_concurrency"]').trigger('click')
    await flushPromises()
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'current_concurrency',
        sort_order: 'asc'
      }),
      expect.any(Object)
    )

    await wrapper.get('[data-test="concurrency-sort-trigger"]').trigger('click')
    await flushPromises()
    await wrapper.get('[data-test="concurrency-sort-option-available_concurrency"]').trigger('click')
    await flushPromises()
    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'available_concurrency',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-available@example.com,low-current@example.com,high-current@example.com')
  })

  it('exits platform quota loading when a single getPlatformQuotas request rejects', async () => {
    vi.useFakeTimers()
    localStorage.setItem('user-hidden-columns', JSON.stringify([]))
    getPlatformQuotas.mockRejectedValueOnce(new Error('quota fetch failed'))

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          UserPlatformQuotaCell: {
            props: ['quotas'],
            template: '<span data-test="quota-state">{{ quotas === undefined ? "loading" : "loaded" }}</span>'
          },
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()
    expect(wrapper.get('[data-test="quota-state"]').text()).toBe('loading')

    await vi.advanceTimersByTimeAsync(50)
    await flushPromises()

    expect(getPlatformQuotas).toHaveBeenCalledWith(42)
    expect(wrapper.get('[data-test="quota-state"]').text()).toBe('loaded')
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('shows active, used, and created activity columns in order and requests last_used_at sort', async () => {
    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const columns = wrapper.get('[data-test="columns"]').text()
    const visibleColumns = columns.split(',')
    expect(visibleColumns).not.toContain('last_login_at')
    expect(visibleColumns.slice(-4, -1)).toEqual(['last_active_at', 'last_used_at', 'created_at'])
    expect(wrapper.get('[data-test="last-active-cell"]').text()).toBe(formatDateTime(createAdminUser().last_active_at))
    expect(wrapper.get('[data-test="last-used-cell"]').text()).toBe(formatDateTime(createAdminUser().last_used_at))

    await wrapper.get('[data-test="sort-last-used"]').trigger('click')
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'last_used_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('ignores saved last_login_at column and sort preferences', async () => {
    localStorage.setItem('user-hidden-columns', JSON.stringify(['last_login_at', 'last_active_at']))
    localStorage.setItem('admin-users-table-sort', JSON.stringify({ key: 'last_login_at', order: 'asc' }))

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    const visibleColumns = wrapper.get('[data-test="columns"]').text().split(',')
    expect(visibleColumns).not.toContain('last_login_at')
    expect(visibleColumns).toContain('last_active_at')

    expect(listUsers).toHaveBeenCalledWith(
      1,
      20,
      expect.objectContaining({
        sort_by: 'created_at',
        sort_order: 'desc'
      }),
      expect.any(Object)
    )
  })

  it('jumps to admin usage for the clicked user with the default date range', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-24T12:00:00Z'))

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    await wrapper.get('button[type="button"].font-medium').trigger('click')

    expect(openSpy).toHaveBeenCalledWith(
      `${window.location.origin}/admin/usage?user_id=42&start_date=2026-04-23&end_date=2026-04-24`,
      '_self'
    )
  })

  it('resets pagination to first page before applying filters', async () => {
    listUsers.mockResolvedValue({
      items: [createAdminUser()],
      total: 60,
      page: 3,
      page_size: 20,
      pages: 3
    })

    const wrapper = mount(UsersView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          TablePageLayout: {
            template: '<div><slot name="filters" /><slot name="table" /><slot name="pagination" /></div>'
          },
          DataTable: DataTableStub,
          Pagination: true,
          ConfirmDialog: true,
          EmptyState: true,
          GroupBadge: true,
          Select: true,
          UserAttributesConfigModal: true,
          UserConcurrencyCell: true,
          UserCreateModal: true,
          UserEditModal: true,
          UserApiKeysModal: true,
          UserAllowedGroupsModal: true,
          UserBalanceModal: true,
          UserBalanceHistoryModal: true,
          GroupReplaceModal: true,
          Icon: true,
          Teleport: true
        }
      }
    })

    await flushPromises()

    ;(wrapper.vm as unknown as { handlePageChange: (page: number) => void }).handlePageChange(3)
    await flushPromises()

    ;(wrapper.vm as unknown as {
      filters: { role: string }
      applyFilter: () => void
    }).filters.role = 'admin'
    ;(wrapper.vm as unknown as {
      applyFilter: () => void
    }).applyFilter()
    await flushPromises()

    expect(listUsers).toHaveBeenLastCalledWith(
      1,
      20,
      expect.objectContaining({
        role: 'admin'
      }),
      expect.any(Object)
    )
  })
})
