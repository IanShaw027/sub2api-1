import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorActionsCell from '@/components/admin/monitor/MonitorActionsCell.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({ t: (key: string) => key }),
}))

function makeMonitor(overrides: Partial<ChannelMonitor> = {}): ChannelMonitor {
  return {
    id: 42,
    name: 'primary',
    provider: 'openai',
    api_mode: 'chat_completions',
    endpoint: 'https://api.example.com',
    api_key_masked: 'sk-t***',
    primary_model: 'gpt-4o-mini',
    extra_models: [],
    group_name: '',
    enabled: true,
    interval_seconds: 60,
    jitter_seconds: 0,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-07-16T00:00:00Z',
    updated_at: '2026-07-16T00:00:00Z',
    primary_status: '',
    primary_latency_ms: null,
    availability_7d: 0,
    extra_models_status: [],
    template_id: null,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    check_mode: 'probe',
    account_id: null,
    ...overrides,
  }
}

describe('MonitorActionsCell duplicate action', () => {
  it('emits the selected monitor when duplicate is clicked', async () => {
    const row = makeMonitor()
    const wrapper = mount(MonitorActionsCell, {
      props: { row, running: false, duplicating: false },
      attachTo: document.body,
    })

    await wrapper.get('[data-testid="monitor-duplicate"]').trigger('click')

    expect(wrapper.emitted('duplicate')).toEqual([[row]])

    wrapper.unmount()
  })

  it('disables the action while the same monitor is being duplicated', async () => {
    const wrapper = mount(MonitorActionsCell, {
      props: { row: makeMonitor(), running: false, duplicating: true },
      attachTo: document.body,
    })

    const button = wrapper.get<HTMLButtonElement>('[data-testid="monitor-duplicate"]')

    expect(button.element.disabled).toBe(true)
    expect(button.attributes('title')).toBe('admin.channelMonitor.duplicating')
    expect(button.attributes('aria-busy')).toBe('true')
    await button.trigger('click')
    expect(wrapper.emitted('duplicate')).toBeUndefined()

    wrapper.unmount()
  })

  it('disables the action when the stored API key cannot be decrypted', async () => {
    const wrapper = mount(MonitorActionsCell, {
      props: {
        row: makeMonitor({ api_key_decrypt_failed: true }),
        running: false,
        duplicating: false,
      },
      attachTo: document.body,
    })

    const button = wrapper.get<HTMLButtonElement>('[data-testid="monitor-duplicate"]')

    expect(button.element.disabled).toBe(true)
    expect(button.attributes('title')).toBe('admin.channelMonitor.duplicateKeyUnavailable')
    await button.trigger('click')
    expect(wrapper.emitted('duplicate')).toBeUndefined()

    wrapper.unmount()
  })

  it('keeps original actions directly available and retains the added detail action', async () => {
    const row = makeMonitor()
    const wrapper = mount(MonitorActionsCell, {
      props: { row, running: false, duplicating: false }, attachTo: document.body,
    })

    for (const [label, event] of [
      ['admin.channelMonitor.runNow', 'run'],
      ['common.edit', 'edit'],
      ['common.delete', 'delete'],
    ]) {
      await wrapper.get(`[aria-label="${label}"]`).trigger('click')
      expect(wrapper.emitted(event)).toEqual([[row]])
    }
    await wrapper.get('[aria-label="common.moreActions"]').trigger('click')
    const items = document.querySelectorAll<HTMLButtonElement>('.dropdown-item')
    expect(items).toHaveLength(1)
    expect(items[0].textContent).toContain('admin.channelMonitor.viewDetails')
    items[0].click()
    expect(wrapper.emitted('detail')).toEqual([[row]])
    wrapper.unmount()
  })

  it('preserves run loading feedback and prevents another run while busy', async () => {
    const wrapper = mount(MonitorActionsCell, {
      props: { row: makeMonitor(), running: true, duplicating: false },
    })
    const button = wrapper.get<HTMLButtonElement>('[aria-label="admin.channelMonitor.runNow"]')
    expect(button.element.disabled).toBe(true)
    expect(button.attributes('aria-busy')).toBe('true')
    expect(button.find('.animate-spin').exists()).toBe(true)
    await button.trigger('click')
    expect(wrapper.emitted('run')).toBeUndefined()
    wrapper.unmount()
  })
})
