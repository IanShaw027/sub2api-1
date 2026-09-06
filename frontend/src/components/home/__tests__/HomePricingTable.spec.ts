import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import HomePricingTable, { type PricingRow } from '../HomePricingTable.vue'

const PlatformIconStub = defineComponent({
  name: 'PlatformIcon',
  props: ['platform', 'size'],
  template: '<span class="platform-icon-stub" />'
})
const StatusBadgeStub = defineComponent({
  name: 'StatusBadge',
  props: ['tone', 'dot', 'label'],
  template: '<span class="status-badge-stub">{{ label }}</span>'
})

function mountTable(rows: PricingRow[]) {
  return mount(HomePricingTable, {
    props: { rows },
    global: {
      stubs: { PlatformIcon: PlatformIconStub, StatusBadge: StatusBadgeStub }
    }
  })
}

const sampleRow: PricingRow = {
  model: 'claude-sonnet-4-5',
  vendor: 'Anthropic',
  platform: 'anthropic',
  input: '$3',
  output: '$15',
  context: '200K'
}

describe('HomePricingTable', () => {
  it('renders one row per model when rows are provided', () => {
    const wrapper = mountTable([sampleRow, { ...sampleRow, model: 'gpt-5', platform: 'openai' }])
    expect(wrapper.findAll('.pricing-row')).toHaveLength(2)
    expect(wrapper.find('.pricing-empty').exists()).toBe(false)
  })

  it('shows the empty state when there are no rows', () => {
    const wrapper = mountTable([])
    expect(wrapper.find('.pricing-empty').exists()).toBe(true)
    expect(wrapper.findAll('.pricing-row')).toHaveLength(0)
  })

  it('marks a limited model with the warning status label', () => {
    const wrapper = mountTable([{ ...sampleRow, limited: true }])
    expect(wrapper.findComponent(StatusBadgeStub).props('tone')).toBe('warning')
  })

  it('marks a non-limited model with the success status label', () => {
    const wrapper = mountTable([sampleRow])
    expect(wrapper.findComponent(StatusBadgeStub).props('tone')).toBe('success')
  })
})
