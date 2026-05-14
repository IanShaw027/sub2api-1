import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

import type { AdminUser } from '@/types'
import { formatDateTime } from '@/utils/format'
import UsersView from '../UsersView.vue'

const {
  listUsers,
  getById,
  getAllGroups,
  getBatchUsersUsage,
  listEnabledDefinitions,
  getBatchUserAttributes
} = vi.hoisted(() => ({
  listUsers: vi.fn(),
  getById: vi.fn(),
  getAllGroups: vi.fn(),
  getBatchUsersUsage: vi.fn(),
  listEnabledDefinitions: vi.fn(),
  getBatchUserAttributes: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    users: {
      list: listUsers,
      getById,
      toggleStatus: vi.fn(),
      delete: vi.fn()
    },
    groups: {
      getAll: getAllGroups
    },
    dashboard: {
      getBatchUsersUsage
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
  current_concurrency: 0,
  ...overrides
})

const DataTableStub = {
  props: ['columns', 'data'],
  emits: ['sort'],
  template: `
    <div>
      <div data-test="columns">{{ columns.map(col => col.key).join(',') }}</div>
      <button data-test="sort-last-used" @click="$emit('sort', 'last_used_at', 'desc')">sort</button>
      <button data-test="sort-today-balance" @click="$emit('sort', 'today_balance_usage', 'desc')">today balance</button>
      <button data-test="sort-today-subscription" @click="$emit('sort', 'today_subscription_usage', 'desc')">today subscription</button>
      <button data-test="sort-last-30d" @click="$emit('sort', 'last_30d_usage', 'desc')">last 30d</button>
      <div data-test="row-order">{{ data.map(row => row.email).join(',') }}</div>
      <div v-for="row in data" :key="row.id">
        <slot name="cell-email" :value="row.email" :row="row" />
        <div data-test="usage-cell">
          <slot name="cell-usage" :row="row" />
        </div>
        <div data-test="concurrency-cell">
          <slot name="cell-concurrency" :row="row" />
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
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-04-24T12:00:00Z'))
    localStorage.clear()
    openSpy.mockClear()

    listUsers.mockReset()
    getAllGroups.mockReset()
    getById.mockReset()
    getBatchUsersUsage.mockReset()
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
    getBatchUsersUsage.mockResolvedValue({ stats: {} })
    listEnabledDefinitions.mockResolvedValue([])
    getBatchUserAttributes.mockResolvedValue({ values: {} })
  })

  it('shows split usage totals and requests backend usage sorts', async () => {
    getBatchUsersUsage.mockResolvedValue({
      stats: {
        42: {
          user_id: 42,
          today_actual_cost: 0.17,
          today_balance_actual_cost: 0.12,
          today_subscription_actual_cost: 0.05,
          total_actual_cost: 1.23
        }
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
    vi.advanceTimersByTime(60)
    await flushPromises()

    const visibleColumns = wrapper.get('[data-test="columns"]').text().split(',')
    expect(visibleColumns).toContain('usage')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$0.1200')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$0.0500')
    expect(wrapper.get('[data-test="usage-cell"]').text()).toContain('$1.2300')

    for (const [button, sortBy] of [
      ['[data-test="sort-today-balance"]', 'today_balance_usage'],
      ['[data-test="sort-today-subscription"]', 'today_subscription_usage'],
      ['[data-test="sort-last-30d"]', 'last_30d_usage']
    ] as const) {
      await wrapper.get(button).trigger('click')
      await flushPromises()
      expect(listUsers).toHaveBeenLastCalledWith(
        1,
        20,
        expect.objectContaining({
          sort_by: sortBy,
          sort_order: 'desc'
        }),
        expect.any(Object)
      )
    }
  })

  it('sorts the current page by runtime current and available concurrency', async () => {
    listUsers.mockResolvedValue({
      items: [
        createAdminUser({ id: 1, email: 'low-current@example.com', concurrency: 5, current_concurrency: 1 }),
        createAdminUser({ id: 2, email: 'high-current@example.com', concurrency: 5, current_concurrency: 4 }),
        createAdminUser({ id: 3, email: 'high-available@example.com', concurrency: 10, current_concurrency: 2 })
      ],
      total: 3,
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

    await wrapper.get('[data-test="concurrency-sort"]').setValue('current_desc')
    await flushPromises()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-current@example.com,high-available@example.com,low-current@example.com')

    await wrapper.get('[data-test="concurrency-sort"]').setValue('available_desc')
    await flushPromises()
    expect(wrapper.get('[data-test="row-order"]').text()).toBe('high-available@example.com,low-current@example.com,high-current@example.com')

    expect(listUsers).toHaveBeenCalledTimes(1)
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
