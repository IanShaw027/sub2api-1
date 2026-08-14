import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, unknown>) =>
        params ? `${key}:${JSON.stringify(params)}` : key
    })
  }
})

import AccountCapacityCell from '../AccountCapacityCell.vue'

const QuotaBadgeStub = defineComponent({
  name: 'QuotaBadge',
  template: '<span />'
})

function mountCell(account: Record<string, unknown>) {
  return mount(AccountCapacityCell, {
    props: { account: account as any },
    global: {
      stubs: { QuotaBadge: QuotaBadgeStub }
    }
  })
}

describe('AccountCapacityCell RPM buffer', () => {
  it('uses max(effectiveConcurrency, max(baseRPM/5, 1)) when no manual key', () => {
    const wrapper = mountCell({
      platform: 'anthropic',
      type: 'oauth',
      concurrency: 3,
      base_rpm: 15,
      current_rpm: 15,
      rpm_strategy: 'tiered'
    })

    const rpmBadge = wrapper.findAll('span[title]').find((node) =>
      (node.attributes('title') || '').includes('capacity.rpm')
    )
    expect(rpmBadge?.attributes('title')).toContain('"buffer":3')
  })

  it('uses EffectiveConcurrency fallback when stored concurrency is out of range', () => {
    const wrapper = mountCell({
      platform: 'anthropic',
      type: 'oauth',
      concurrency: 0,
      base_rpm: 15,
      current_rpm: 15,
      rpm_strategy: 'tiered'
    })

    const rpmBadge = wrapper.findAll('span[title]').find((node) =>
      (node.attributes('title') || '').includes('capacity.rpm')
    )
    expect(rpmBadge?.attributes('title')).toContain('"buffer":12')
  })

  it('uses a manual rpm_sticky_buffer when present', () => {
    const wrapper = mountCell({
      platform: 'anthropic',
      type: 'oauth',
      concurrency: 3,
      base_rpm: 15,
      current_rpm: 15,
      rpm_strategy: 'tiered',
      rpm_sticky_buffer: 5
    })

    const rpmBadge = wrapper.findAll('span[title]').find((node) =>
      (node.attributes('title') || '').includes('capacity.rpm')
    )
    expect(rpmBadge?.attributes('title')).toContain('"buffer":5')
  })
})
