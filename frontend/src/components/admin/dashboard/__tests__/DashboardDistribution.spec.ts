import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import DashboardDistribution from '../DashboardDistribution.vue'

const { getModelStats, getUserBreakdown } = vi.hoisted(() => ({ getModelStats: vi.fn(), getUserBreakdown: vi.fn() }))
vi.mock('@/api/admin/dashboard', () => ({ getModelStats, getUserBreakdown }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
const model = { key: 'test-model', model: 'test-model', name: 'test-model', requests: 9, tokens: 500, cost: '1.00', pct: 40 }
const user = { key: 'user-7', userId: 7, name: 'User Seven', requests: 9, tokens: 500, cost: '1.00', pct: 40 }
const render = (view: 'models' | 'users' = 'models') => mount(DashboardDistribution, {
  props: { view, rows: [view === 'models' ? model : user], startDate: '2026-08-01', endDate: '2026-08-07' },
  global: { stubs: { DataTable: { props: ['data'], template: '<div class="detail-data">{{ JSON.stringify(data) }}</div>' } } }
})
describe('DashboardDistribution', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    getUserBreakdown.mockResolvedValue({ users: [{ user_id: 7, email: 'seven@example.com', requests: 9, total_tokens: 500, actual_cost: 1, cost: 2, account_cost: 0.5 }] })
    getModelStats.mockResolvedValue({ models: [{ model: 'test-model', requests: 9, total_tokens: 500, actual_cost: 1, cost: 2, account_cost: 0.5, cache_creation_tokens: 0, cache_read_tokens: 0 }] })
  })
  it('loads model users with the selected range and caches a collapsed row', async () => {
    const wrapper = render()
    await wrapper.get('.distribution-toggle').trigger('click')
    await flushPromises()
    expect(getUserBreakdown).toHaveBeenCalledWith({ model: 'test-model', start_date: '2026-08-01', end_date: '2026-08-07', limit: 100 })
    expect(wrapper.get('.detail-data').text()).toContain('seven@example.com')
    expect(wrapper.get('.detail-data').text()).toContain('$0.500')
    await wrapper.get('.distribution-toggle').trigger('click')
    await wrapper.get('.distribution-toggle').trigger('click')
    expect(getUserBreakdown).toHaveBeenCalledTimes(1)
    wrapper.unmount()
  })
  it('loads user models separately from navigating to usage', async () => {
    const wrapper = render('users')
    await wrapper.get('.distribution-toggle').trigger('click')
    await flushPromises()
    expect(getModelStats).toHaveBeenCalledWith({ user_id: 7, start_date: '2026-08-01', end_date: '2026-08-07' })
    expect(wrapper.get('.detail-data').text()).toContain('test-model')
    expect(wrapper.emitted('userRowClick')).toBeUndefined()
    await wrapper.get('.btn-icon').trigger('click')
    expect(wrapper.emitted('userRowClick')).toEqual([[7]])
    wrapper.unmount()
  })
  it('discards late responses after a range change', async () => {
    let resolve!: (value: unknown) => void
    getUserBreakdown.mockImplementationOnce(() => new Promise(done => { resolve = done }))
    const wrapper = render()
    await wrapper.get('.distribution-toggle').trigger('click')
    await wrapper.setProps({ startDate: '2026-08-08', endDate: '2026-08-14' })
    resolve({ users: [{ user_id: 999, email: 'stale@example.com' }] })
    await flushPromises()
    expect(wrapper.find('.distribution-details').exists()).toBe(false)
    await wrapper.get('.distribution-toggle').trigger('click')
    await flushPromises()
    expect(wrapper.text()).not.toContain('stale@example.com')
    expect(getUserBreakdown).toHaveBeenLastCalledWith(expect.objectContaining({ start_date: '2026-08-08', end_date: '2026-08-14' }))
    wrapper.unmount()
  })
  it('offers retry after failure and renders an empty response', async () => {
    getUserBreakdown.mockRejectedValueOnce(new Error('network')).mockResolvedValueOnce({ users: [] })
    const wrapper = render()
    await wrapper.get('.distribution-toggle').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(true)
    await wrapper.get('.distribution-error button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('.detail-data').text()).toBe('[]')
    wrapper.unmount()
  })
})
