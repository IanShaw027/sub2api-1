import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

import AccountsView from '../AccountsView.vue'
import BulkEditAccountModal from '@/components/account/BulkEditAccountModal.vue'

vi.mock('@/components/layout/AppLayout.vue', () => ({
  default: {
    name: 'AppLayout',
    template: '<div><slot /></div>'
  }
}))

const {
  listAccounts,
  listWithEtag,
  getBatchTodayStats,
  getAllProxies,
  getAllGroups,
  setSchedulable,
  bulkUpdate,
  listTLSFingerprintProfiles,
  listTLSFingerprintRouters,
  showError
} = vi.hoisted(() => ({
  listAccounts: vi.fn(),
  listWithEtag: vi.fn(),
  getBatchTodayStats: vi.fn(),
  getAllProxies: vi.fn(),
  getAllGroups: vi.fn(),
  setSchedulable: vi.fn(),
  bulkUpdate: vi.fn(),
  listTLSFingerprintProfiles: vi.fn(),
  listTLSFingerprintRouters: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      list: listAccounts,
      listWithEtag,
      getBatchTodayStats,
      delete: vi.fn(),
      batchClearError: vi.fn(),
      batchRefresh: vi.fn(),
      toggleSchedulable: vi.fn(),
      setSchedulable,
      bulkUpdate
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
    showError,
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    token: 'test-token'
  })
}))

vi.mock('@/api/admin/tlsFingerprintProfile', () => ({
  list: listTLSFingerprintProfiles
}))

vi.mock('@/api/admin/tlsFingerprintRouter', () => ({
  list: listTLSFingerprintRouters
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

vi.mock('@/composables/useModelWhitelist', () => ({
  commonErrorCodes: [],
  getPresetMappingsByPlatform: vi.fn(() => []),
  buildModelMappingPayload: vi.fn((mode: string, allowed: string[], mappings: Array<{ from: string; to: string }>) => {
    if (mode === 'whitelist') {
      return Object.fromEntries((allowed || []).map((model) => [model, model]))
    }
    return Object.fromEntries(
      (mappings || [])
        .filter((mapping) => mapping.from && mapping.to)
        .map((mapping) => [mapping.from, mapping.to])
    )
  })
}))

const DataTableStub = {
  props: ['columns', 'data'],
  template: `
    <div data-test="data-table">
      <span v-for="column in columns" :key="column.key" data-test="column-key">{{ column.key }}</span>
      <div v-for="row in data" :key="row.id" :data-test="['row', row.id].join('-')">
        <div data-test="name-cell">
          <slot name="cell-name" :row="row" :value="row.name" />
        </div>
        <div data-test="platform-type-cell">
          <slot name="cell-platform_type" :row="row" :value="row.type" />
        </div>
        <div data-test="schedulable-cell">
          <slot name="cell-schedulable" :row="row" />
        </div>
        <slot name="cell-created_at" :value="row.created_at" :row="row" />
      </div>
    </div>
  `
}

const AccountBulkActionsBarStub = {
  props: ['selectedIds'],
  emits: ['edit-filtered'],
  template: '<button data-test="edit-filtered" @click="$emit(\'edit-filtered\')">edit filtered</button>'
}

const BulkEditAccountModalStub = {
  props: ['show', 'target'],
  template: '<div data-test="bulk-edit-modal" :data-show="String(show)" :data-target-mode="target?.mode ?? \'\'" :data-preview-count="String(target?.previewCount ?? 0)" :data-selected-platforms="(target?.selectedPlatforms ?? []).join(\',\')" :data-selected-types="(target?.selectedTypes ?? []).join(\',\')"></div>'
}

const PlatformTypeBadgeStub = defineComponent({
  name: 'PlatformTypeBadgeStub',
  props: {
    platform: { type: String, default: '' },
    type: { type: String, default: '' },
    planType: { type: String, default: '' },
    typeLabelOverride: { type: String, default: '' },
    organizationRole: { type: String, default: '' }
  },
  template: `
    <div
      data-test="platform-type-badge"
      :data-platform="platform"
      :data-type="type"
      :data-plan-type="planType"
      :data-type-label-override="typeLabelOverride"
      :data-organization-role="organizationRole"
    />
  `
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false
    }
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>'
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: ''
    },
    options: {
      type: Array,
      default: () => []
    }
  },
  emits: ['update:modelValue'],
  template: `
    <select
      v-bind="$attrs"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option v-for="option in options" :key="option.value" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `
})

function mountAccountsView() {
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
        AccountTableActions: { template: '<div><slot name="beforeCreate" /><slot name="after" /></div>' },
        AccountTableFilters: { template: '<div></div>' },
        AccountBulkActionsBar: AccountBulkActionsBarStub,
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
        CreateAccountModal: true,
        EditAccountModal: true,
        BulkEditAccountModal: BulkEditAccountModalStub,
        PlatformTypeBadge: PlatformTypeBadgeStub,
        AccountCapacityCell: true,
        AccountStatusIndicator: true,
        AccountTodayStatsCell: true,
        AccountGroupsCell: true,
        AccountUsageCell: true,
        Icon: true
      }
    }
  })
}

function mountBulkEditModal(props: Record<string, unknown> = {}) {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth'],
      proxies: [],
      groups: [],
      ...props
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: true,
        Select: SelectStub,
        ProxySelector: true,
        GroupSelector: true,
        ModelWhitelistSelector: true,
        Icon: true
      }
    }
  })
}

describe('admin AccountsView bulk edit scope', () => {
  beforeEach(() => {
    localStorage.clear()

    listAccounts.mockReset()
    listWithEtag.mockReset()
    getBatchTodayStats.mockReset()
    getAllProxies.mockReset()
    getAllGroups.mockReset()
    setSchedulable.mockReset()
    bulkUpdate.mockReset()
    listTLSFingerprintProfiles.mockReset()
    listTLSFingerprintRouters.mockReset()
    showError.mockReset()

    listAccounts.mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
      pages: 0
    })
    listWithEtag.mockResolvedValue({
      notModified: true,
      etag: null,
      data: null
    })
    getBatchTodayStats.mockResolvedValue({ stats: {} })
    getAllProxies.mockResolvedValue([])
    getAllGroups.mockResolvedValue([])
    listTLSFingerprintProfiles.mockResolvedValue([])
    listTLSFingerprintRouters.mockResolvedValue([])
  })

  it('opens bulk edit in filtered-results mode from the bulk actions dropdown', async () => {
    const wrapper = mountAccountsView()

    await flushPromises()
    expect(getAllGroups).toHaveBeenCalledTimes(1)
    expect(getAllProxies).not.toHaveBeenCalled()

    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    expect(getAllProxies).toHaveBeenCalledTimes(1)
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-show')).toBe('true')
    expect(wrapper.get('[data-test="bulk-edit-modal"]').attributes('data-target-mode')).toBe('filtered')
  })

  it('uses all filtered pages to derive bulk edit platform/type scope', async () => {
    listAccounts
      .mockResolvedValueOnce({
        items: [],
        total: 0,
        page: 1,
        page_size: 20,
        pages: 0
      })
      .mockResolvedValueOnce({
        items: Array.from({ length: 100 }, (_, index) => ({
          id: index + 1,
          name: `OpenAI ${index + 1}`,
          platform: 'openai',
          type: 'oauth'
        })),
        total: 105,
        page: 1,
        page_size: 100,
        pages: 2
      })
      .mockResolvedValueOnce({
        items: Array.from({ length: 5 }, (_, index) => ({
          id: 101 + index,
          name: `Anthropic ${101 + index}`,
          platform: 'anthropic',
          type: 'apikey'
        })),
        total: 105,
        page: 2,
        page_size: 100,
        pages: 2
      })

    const wrapper = mountAccountsView()

    await flushPromises()
    await wrapper.get('[data-test="edit-filtered"]').trigger('click')
    await flushPromises()

    const modal = wrapper.get('[data-test="bulk-edit-modal"]')
    expect(modal.attributes('data-preview-count')).toBe('105')
    expect(modal.attributes('data-selected-platforms')).toBe('openai,anthropic')
    expect(modal.attributes('data-selected-types')).toBe('oauth,apikey')
    expect(listAccounts).toHaveBeenCalledTimes(3)
  })

  it('renders the created_at column by default', async () => {
    listAccounts.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'test-account',
          platform: 'anthropic',
          type: 'oauth',
          status: 'active',
          schedulable: true,
          created_at: '2026-03-07T10:00:00Z',
          updated_at: '2026-03-07T10:00:00Z'
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const columnKeys = wrapper.findAll('[data-test="column-key"]').map(node => node.text())
    expect(columnKeys).toContain('created_at')
    const columns = wrapper.getComponent(DataTableStub).props('columns') as Array<{ key: string; label: string; sortable: boolean }>
    expect(columns.find(column => column.key === 'created_at')).toMatchObject({
      label: 'admin.accounts.columns.createdAt',
      sortable: true
    })
  })

  it('shows openai oauth rows as email plus workspace or personal fallback', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'Fallback Name',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            email: 'owner@example.com',
            workspace_name: 'Team Alpha',
            plan_type: 'team'
          },
          extra: {
            email_address: 'owner@example.com'
          }
        },
        {
          id: 2,
          name: 'Second Fallback',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            email: 'solo@example.com',
            plan_type: 'team'
          },
          extra: {
            email_address: 'solo@example.com',
            team_name: 'Crew Beta'
          }
        },
        {
          id: 3,
          name: 'Third Fallback',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            email: 'personal@example.com',
            plan_type: 'free'
          },
          extra: {
            email_address: 'personal@example.com'
          }
        }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const nameCells = wrapper.findAll('[data-test="name-cell"]')
    expect(nameCells).toHaveLength(3)
    expect(nameCells[0].text()).toContain('owner@example.com (Team Alpha)')
    expect(nameCells[1].text()).toContain('solo@example.com (Crew Beta)')
    expect(nameCells[2].text()).toContain('personal@example.com')
    expect(nameCells[2].text()).not.toContain('(personal)')
  })

  it('limits account name cell width and exposes full text via title', async () => {
    const longName = 'Gemini account name '.repeat(30).trim()
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: longName,
          platform: 'gemini',
          type: 'oauth',
          credentials: {},
          extra: {
            email_address: 'very.long.email@example.com'
          }
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })
    const wrapper = mountAccountsView()
    await flushPromises()

    const nameText = wrapper.get('[data-test="name-cell"] .font-medium')
    expect(nameText.attributes('title')).toBe(longName)
    expect(nameText.classes()).toContain('truncate')
    expect(nameText.classes()).toContain('max-w-[400px]')
  })

  it('passes the openai organization role through to the platform badge', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'Owner Row',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            email: 'owner@example.com',
            organization_role: 'owner',
            plan_type: 'plus'
          },
          extra: {
            email_address: 'owner@example.com'
          }
        },
        {
          id: 2,
          name: 'Fallback Owner Row',
          platform: 'openai',
          type: 'oauth',
          credentials: {
            email: 'fallback@example.com',
            plan_type: 'team'
          },
          extra: {
            organization_role: 'owner',
            email_address: 'fallback@example.com'
          }
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const badges = wrapper.findAll('[data-test="platform-type-badge"]')
    expect(badges).toHaveLength(2)
    expect(badges[0].attributes('data-organization-role')).toBe('owner')
    expect(badges[1].attributes('data-organization-role')).toBe('owner')
    expect(badges[0].attributes('data-plan-type')).toBe('plus')
    expect(badges[1].attributes('data-plan-type')).toBe('team')
  })

  it('maps canonical gemini tier ids without relying on subscription_type fallbacks', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'Gemini Pro',
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            tier_id: 'google_ai_pro'
          },
          extra: {
            subscription_type: 'Gemini Code Assist in Google One AI Pro'
          }
        },
        {
          id: 2,
          name: 'Gemini Unknown',
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            tier_id: 'gcp_enterprise'
          },
          extra: {}
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const badges = wrapper.findAll('[data-test="platform-type-badge"]')
    expect(badges).toHaveLength(2)
    expect(badges[0].attributes('data-plan-type')).toBe('google_ai_pro')
    expect(badges[1].attributes('data-plan-type')).toBe('gcp_enterprise')
  })

  it('prefers gemini paid tier metadata over free google one tier when both exist', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'Gemini Paid Override',
          platform: 'gemini',
          type: 'oauth',
          credentials: {
            oauth_type: 'google_one',
            tier_id: 'google_one_free',
            plan_name: 'Gemini Code Assist in Google One Free'
          },
          extra: {
            gemini_current_tier_id: 'standard-tier',
            gemini_paid_tier_id: 'g1-pro-tier',
            gemini_paid_tier_name: 'Gemini Code Assist in Google One AI Pro'
          }
        }
      ],
      total: 1,
      page: 1,
      page_size: 20,
      pages: 1
    })

    const wrapper = mountAccountsView()
    await flushPromises()

    const badges = wrapper.findAll('[data-test="platform-type-badge"]')
    expect(badges).toHaveLength(1)
    expect(badges[0].attributes('data-plan-type')).toBe('google_ai_pro')
  })

  it('shows an error and aborts when filtered preview pagination fails', async () => {
    listAccounts
      .mockResolvedValueOnce({
        items: [],
        total: 105,
        page: 1,
        page_size: 100,
        pages: 2
      })
      .mockRejectedValueOnce(new Error('boom'))

    const wrapper = mountAccountsView()
    const consoleErrorSpy = vi.spyOn(console, 'error').mockImplementation(() => {})
    try {
      await flushPromises()
      await wrapper.get('[data-test="edit-filtered"]').trigger('click')
      await flushPromises()

      expect(showError).toHaveBeenCalledWith('admin.accounts.bulkEdit.failedToLoadPreview')
      expect(wrapper.find('[data-test="bulk-edit-modal"]').exists()).toBe(false)
    } finally {
      consoleErrorSpy.mockRestore()
    }
  })

  it('removes a toggled row from the active filter and marks the list pending sync', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 1,
          name: 'Active Schedulable',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: true
        }
      ],
      total: 3,
      page: 1,
      page_size: 20,
      pages: 1
    })
    setSchedulable.mockResolvedValueOnce({
      id: 1,
      name: 'Active Schedulable',
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      schedulable: false
    })

    const wrapper = mountAccountsView()

    await flushPromises()
    ;(wrapper.vm as any).params.status = 'active'
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-test="row-1"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="row-1"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.listPendingSyncHint')
  })

  it('removes a toggled row from the unschedulable filter and marks the list pending sync', async () => {
    listAccounts.mockResolvedValueOnce({
      items: [
        {
          id: 2,
          name: 'Active Unschedulable',
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          schedulable: false
        }
      ],
      total: 2,
      page: 1,
      page_size: 20,
      pages: 1
    })
    setSchedulable.mockResolvedValueOnce({
      id: 2,
      name: 'Active Unschedulable',
      platform: 'openai',
      type: 'oauth',
      status: 'active',
      schedulable: true
    })

    const wrapper = mountAccountsView()

    await flushPromises()
    ;(wrapper.vm as any).params.status = 'unschedulable'
    await wrapper.vm.$nextTick()
    await wrapper.get('[data-test="row-2"] button').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-test="row-2"]').exists()).toBe(false)
    expect(wrapper.text()).toContain('admin.accounts.listPendingSyncHint')
  })
})

describe('BulkEditAccountModal OpenAI image generation', () => {
  it('writes only the dedicated OpenAI image generation extra override when enabled', async () => {
    bulkUpdate.mockResolvedValue({ success: 2, failed: 0 })

    const wrapper = mountBulkEditModal()

    await wrapper.get('#bulk-edit-openai-image-generation-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-image-generation-toggle').trigger('click')
    await wrapper.get('form#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_image_generation_enabled: false
      }
    })
  })
})

describe('admin account modal layering', () => {
  it('keeps table and account action overlays below BaseDialog modals', () => {
    const dataTableSource = readFileSync(
      resolve(process.cwd(), 'src/components/common/DataTable.vue'),
      'utf8'
    )
    const actionMenuSource = readFileSync(
      resolve(process.cwd(), 'src/components/admin/account/AccountActionMenu.vue'),
      'utf8'
    )
    const dataTableZIndexes = [...dataTableSource.matchAll(/z-index:\s*(\d+)/g)].map(match => Number(match[1]))

    expect(Math.max(...dataTableZIndexes)).toBeLessThan(50)
    expect(actionMenuSource).not.toContain('z-[9998]')
    expect(actionMenuSource).not.toContain('z-[9999]')
  })
})
