import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import { createI18n } from 'vue-i18n'
import type { ApiKey } from '@/types'
import { formatDate } from '@/utils/format'
import KeysMobileList from '../KeysMobileList.vue'
import KeyMobileCard from '../KeyMobileCard.vue'
import KeysDataTable from '../KeysDataTable.vue'

const row = {
  id: 1, name: 'production-long-key-name', key: 'sk-test-production-long-secret', status: 'active',
  group: { name: 'long-production-group-name' }, quota: 500, quota_used: 2,
  current_concurrency: 3, last_used_ip: '2001:0db8:85a3:0000:0000:8a2e:0370:7334',
  created_at: '2026-08-01T00:00:00Z', last_used_at: '2026-08-02T00:00:00Z', expires_at: null,
  rate_limit_5h: 10, usage_5h: 1, reset_5h_at: '2026-09-07T12:00:00Z',
  rate_limit_1d: 20, usage_1d: 5, reset_1d_at: '2026-09-08T00:00:00Z',
  rate_limit_7d: 100, usage_7d: 20, reset_7d_at: '2026-09-14T00:00:00Z',
} as ApiKey

describe('mobile key information preservation', () => {
  it('keeps cumulative billed usage separate from resettable quota and exposes all windows', () => {
    const wrapper = mount(KeysMobileList, {
      props: {
        loading: false, apiKeys: [row], copiedKeyId: null, isKeyRevealed: () => false,
        usageStats: { 1: { api_key_id: 1, today_actual_cost: 1, total_actual_cost: 321.2345 } },
        now: new Date('2026-09-07T00:00:00Z'),
      },
      global: { plugins: [createI18n({ legacy: false, missingWarn: false, fallbackWarn: false, locale: 'en', messages: { en: { dashboard: { platformQuota: { resetsAt: ({ named }) => `Resets at ${named('time')}` } } } } })] },
    })
    const card = wrapper.getComponent(KeyMobileCard)
    expect(card.props('totalCost')).toBe(321.2345)
    expect(card.text()).toContain('$321.2345')
    expect(card.text()).toContain('$2.00 / $500.00')
    expect(card.text()).toContain(row.last_used_ip)
    expect(card.text()).toContain(formatDate(row.created_at))
    const windows = card.findAll('.keys-card-limit')
    expect(windows).toHaveLength(3)
    expect(windows.map(item => item.get('b').text())).toEqual([
      '5h: $1.0000 / $10.0000', '1d: $5.0000 / $20.0000', '7d: $20.0000 / $100.0000',
    ])
    for (const [index, resetAt] of [row.reset_5h_at, row.reset_1d_at, row.reset_7d_at].entries()) {
      expect(windows[index].text()).toContain(`Resets at ${formatDate(resetAt!)}`)
    }
    wrapper.unmount()
  })

  it('preserves the reveal, copy and row action events', async () => {
    const wrapper = mount(KeyMobileCard, {
      props: { row, revealed: false, copied: false, todayCost: 1, totalCost: 321, now: new Date() },
      global: { plugins: [createI18n({ legacy: false, missingWarn: false, fallbackWarn: false, locale: 'en', messages: { en: {} } })] },
    })
    expect(wrapper.text()).not.toContain(row.key)
    await wrapper.get('.keys-card-more').trigger('click')
    const buttons = wrapper.findAll('.keys-card-key-btn')
    await buttons[0].trigger('click')
    await buttons[1].trigger('click')
    expect(wrapper.emitted('more')).toHaveLength(1)
    expect(wrapper.emitted('toggle-reveal')).toHaveLength(1)
    expect(wrapper.emitted('copy')).toHaveLength(1)
    await wrapper.setProps({ revealed: true })
    expect(wrapper.text()).toContain(row.key)
    wrapper.unmount()
  })

  it('forwards every pre-Glass direct row action without opening More', async () => {
    const wrapper = mount(KeysMobileList, {
      props: {
        loading: false, apiKeys: [row], copiedKeyId: null, isKeyRevealed: () => false,
        usageStats: {}, now: new Date(),
      },
      global: { plugins: [createI18n({ legacy: false, missingWarn: false, fallbackWarn: false, locale: 'en', messages: { en: {} } })] },
    })
    for (const [label, event] of [
      ['keys.useKey', 'use'], ['keys.importToCcSwitch', 'import-ccs'],
      ['keys.disable', 'toggle-status'], ['common.edit', 'edit'], ['common.delete', 'delete'],
      ['keys.resetRateLimitUsage', 'reset-rate-limit'], ['keys.group', 'open-group-selector'],
    ]) {
      await wrapper.get(`button[aria-label="${label}"]`).trigger('click')
      expect(wrapper.emitted(event)?.[0]).toEqual([row])
    }
    expect(wrapper.emitted('more')).toBeUndefined()
    await wrapper.setProps({ hideCcsImport: true })
    expect(wrapper.find('button[aria-label="keys.importToCcSwitch"]').exists()).toBe(false)
    wrapper.unmount()
  })

  it('keeps the same direct actions in desktop table cells and preserves row identity', async () => {
    const wrapper = mount(KeysDataTable, {
      props: {
        columns: [], apiKeys: [row], loading: false, usageStats: {}, now: new Date(), copiedKeyId: null,
        isKeyRevealed: () => false, setGroupButtonRef: () => {}, groupCellSuffix: () => '',
        groupCellTooltip: () => '', rateLimitDetail: () => '',
      },
      global: {
        plugins: [createI18n({ legacy: false, missingWarn: false, fallbackWarn: false, locale: 'en', messages: { en: {} } })],
        stubs: { DataTable: { props: ['data'], template: '<div><slot name="cell-actions" :row="data[0]" /><slot name="cell-rate_limit" :row="data[0]" /></div>' } },
      },
    })
    for (const [label, event] of [
      ['keys.useKey', 'use'], ['keys.importToCcSwitch', 'import-ccs'], ['keys.disable', 'toggle-status'],
      ['common.edit', 'edit'], ['common.delete', 'delete'], ['keys.resetRateLimitUsage', 'reset-rate-limit'],
    ]) {
      await wrapper.get(`button[aria-label="${label}"]`).trigger('click')
      expect(wrapper.emitted(event)?.[0]).toEqual([row])
    }
    expect(wrapper.emitted('more')).toBeUndefined()
    wrapper.unmount()
  })
})
