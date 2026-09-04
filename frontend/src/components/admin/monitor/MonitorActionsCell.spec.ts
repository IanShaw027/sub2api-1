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

// MonitorActionsCell delegates to the shared ActionsCell primitive: an
// edit icon-btn plus a "more" icon-btn that opens a teleported dropdown.
// The duplicate action lives in that dropdown as a `dropdown-item` entry
// rather than its own labeled button (see components/common/cells/ActionsCell.vue).
async function openMenu(wrapper: ReturnType<typeof mount>) {
  await wrapper.get('[aria-label="common.moreActions"]').trigger('click')
}

function duplicateItem() {
  // Order matches the `items` computed in MonitorActionsCell.vue:
  // [viewDetails, runNow, duplicate, delete].
  const items = document.querySelectorAll('.dropdown-item')
  const match = items[2]
  if (!match) throw new Error('duplicate dropdown item not found')
  return match as HTMLButtonElement
}

describe('MonitorActionsCell duplicate action', () => {
  it('emits the selected monitor when duplicate is clicked', async () => {
    const row = makeMonitor()
    const wrapper = mount(MonitorActionsCell, {
      props: { row, running: false, duplicating: false },
      attachTo: document.body,
    })

    await openMenu(wrapper)
    duplicateItem().click()
    await wrapper.vm.$nextTick()

    expect(wrapper.emitted('duplicate')).toEqual([[row]])

    wrapper.unmount()
  })

  it('disables the action while the same monitor is being duplicated', async () => {
    const wrapper = mount(MonitorActionsCell, {
      props: { row: makeMonitor(), running: false, duplicating: true },
      attachTo: document.body,
    })

    await openMenu(wrapper)
    const button = duplicateItem()

    expect(button.disabled).toBe(true)
    expect(button.textContent).toContain('admin.channelMonitor.duplicating')

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

    await openMenu(wrapper)
    const button = duplicateItem()

    expect(button.disabled).toBe(true)
    expect(button.textContent).toContain('admin.channelMonitor.duplicateKeyUnavailable')

    wrapper.unmount()
  })
})
