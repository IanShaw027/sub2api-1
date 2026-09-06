import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { parse } from 'vue/compiler-sfc'
import { mount } from '@vue/test-utils'
import { ref } from 'vue'
import { describe, expect, it, vi } from 'vitest'

// Render the actual page action slots with handler spies. Keeping More closed
// verifies direct reachability without requiring unrelated API/form fixtures.
function slotTemplate(file: string, slot: string) {
  const { descriptor } = parse(readFileSync(resolve(__dirname, '..', file), 'utf8'))
  type Node = { type: number; children?: Node[]; props?: Array<{ type: number; name: string; arg?: { type: number; content: string } }>; loc: { source: string } }
  const ast = descriptor.template!.ast as Node
  function find(node: Node): Node | undefined {
    for (const child of node.children ?? []) {
      if (child.type !== 1) continue
      if (child.props?.some(prop => prop.type === 7 && prop.name === 'slot' && prop.arg?.type === 4 && prop.arg.content === slot)) return child
      const nested = find(child)
      if (nested) return nested
    }
  }
  return `<div>${find(ast)!.children!.map(child => child.loc.source).join('')}</div>`
}

function mountSlot(file: string, slot: string, state: Record<string, unknown>) {
  return mount({ template: slotTemplate(file, slot), setup: () => ({ t: (key: string) => key, ...state }) }, {
    global: { stubs: { Icon: true, Button: { template: '<button><slot /></button>' } } },
  })
}

describe('pre-Glass list direct actions', () => {
  it('keeps proxy batch tools available with More closed and preserves disabled states', async () => {
    const state = {
      loading: false, batchTesting: false, batchQualityChecking: false, selectedCount: 1,
      showMoreMenu: ref(false), showImportData: ref(false), showExportDataDialog: ref(false),
      showCreateModal: ref(false), loadProxies: vi.fn(), handleBatchTest: vi.fn(),
      handleBatchQualityCheck: vi.fn(), openBatchDelete: vi.fn(),
    }
    const wrapper = mountSlot('ProxiesView.vue', 'actions', state)
    for (const [label, action] of [
      ['admin.proxies.testConnection', state.handleBatchTest],
      ['admin.proxies.batchQualityCheck', state.handleBatchQualityCheck],
      ['admin.proxies.batchDeleteAction', state.openBatchDelete],
    ] as const) {
      await wrapper.get(`[aria-label="${label}"]`).trigger('click')
      expect(action).toHaveBeenCalledOnce()
    }
    await wrapper.get('[aria-label="admin.proxies.dataImport"]').trigger('click')
    await wrapper.get('[aria-label="admin.proxies.dataExportSelected"]').trigger('click')
    expect(state.showImportData.value).toBe(true)
    expect(state.showExportDataDialog.value).toBe(true)
    expect(state.showMoreMenu.value).toBe(false)
    wrapper.unmount()
    const busy = mountSlot('ProxiesView.vue', 'actions', { ...state, batchTesting: true, batchQualityChecking: true, selectedCount: 0 })
    for (const label of ['admin.proxies.testConnection', 'admin.proxies.batchQualityCheck', 'admin.proxies.batchDeleteAction']) {
      expect(busy.get(`[aria-label="${label}"]`).attributes('disabled')).toBeDefined()
    }
    busy.unmount()
  })

  it('retains proxy test, quality check, edit and delete on every row', async () => {
    const row = { id: 1 }
    const state = { row, testingProxyIds: new Set(), qualityCheckingProxyIds: new Set(), handleTestConnection: vi.fn(), handleQualityCheck: vi.fn(), handleEdit: vi.fn(), handleDelete: vi.fn(), toggleRowMenu: vi.fn() }
    const wrapper = mountSlot('ProxiesView.vue', 'cell-actions', state)
    for (const [label, action] of [
      ['admin.proxies.testConnection', state.handleTestConnection], ['admin.proxies.qualityCheck', state.handleQualityCheck],
      ['common.edit', state.handleEdit], ['common.delete', state.handleDelete],
    ] as const) {
      await wrapper.get(`[aria-label="${label}"]`).trigger('click')
      expect(action).toHaveBeenCalledWith(row)
    }
    expect(state.toggleRowMenu).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('retains direct subscription reset/revoke only for active rows and the reset busy guard', async () => {
    const row = { id: 1, status: 'active' }
    const state = { row, resettingQuota: false, resettingSubscription: null, handleResetQuota: vi.fn(), handleRevoke: vi.fn(), handleExtend: vi.fn(), handleRestore: vi.fn(), toggleRowMenu: vi.fn() }
    const wrapper = mountSlot('SubscriptionsView.vue', 'cell-actions', state)
    await wrapper.get('[aria-label="admin.subscriptions.resetQuota"]').trigger('click')
    await wrapper.get('[aria-label="admin.subscriptions.revoke"]').trigger('click')
    expect(state.handleResetQuota).toHaveBeenCalledWith(row)
    expect(state.handleRevoke).toHaveBeenCalledWith(row)
    expect(state.toggleRowMenu).not.toHaveBeenCalled()
    wrapper.unmount()
    const busy = mountSlot('SubscriptionsView.vue', 'cell-actions', { ...state, resettingQuota: true, resettingSubscription: row })
    expect(busy.get('[aria-label="admin.subscriptions.resetQuota"]').attributes('disabled')).toBeDefined()
    busy.unmount()
    const expired = mountSlot('SubscriptionsView.vue', 'cell-actions', { ...state, row: { ...row, status: 'expired' } })
    expect(expired.find('[aria-label="admin.subscriptions.resetQuota"]').exists()).toBe(false)
    expect(expired.find('[aria-label="admin.subscriptions.revoke"]').exists()).toBe(false)
    expired.unmount()
  })

  it('exposes user filter and attribute configuration without opening generic More', async () => {
    const state = { loading: false, selectedCount: 0, showMoreDropdown: ref(false), showAttributesModal: ref(false), loadUsers: vi.fn(), builtInFilters: [], filterableAttributes: [] }
    const wrapper = mountSlot('UsersView.vue', 'actions', state)
    await wrapper.get('[aria-label="admin.users.attributes.configButton"]').trigger('click')
    expect(state.showAttributesModal.value).toBe(true)
    expect(state.showMoreDropdown.value).toBe(false)
    await wrapper.get('[aria-label="admin.users.filterSettings"]').trigger('click')
    expect(state.showMoreDropdown.value).toBe(true)
    wrapper.unmount()
  })
})
