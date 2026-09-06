import { createPinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import AvailableChannelsTable from '../AvailableChannelsTable.vue'
import type { UserAvailableChannel } from '@/api/channels'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key }),
  }
})

const rows: UserAvailableChannel[] = [
  {
    name: 'Primary channel',
    description: 'Fast and reliable access',
    platforms: [
      {
        platform: 'anthropic',
        groups: [
          {
            id: 1,
            name: 'Exclusive Pro',
            platform: 'anthropic',
            subscription_type: 'standard',
            rate_multiplier: 1.2,
            peak_rate_enabled: true,
            peak_start: '08:00',
            peak_end: '10:00',
            peak_rate_multiplier: 1.5,
            is_exclusive: true,
          },
          {
            id: 2,
            name: 'Public',
            platform: 'anthropic',
            subscription_type: 'standard',
            rate_multiplier: 1,
            peak_rate_enabled: false,
            peak_start: '',
            peak_end: '',
            peak_rate_multiplier: 1,
            is_exclusive: false,
          },
        ],
        supported_models: [
          { name: 'claude-test', platform: 'anthropic', pricing: null },
          {
            name: 'gemini-2.5-pro',
            platform: 'anthropic',
            pricing: {
              billing_mode: 'token',
              input_price: 1,
              output_price: 2,
              cache_write_price: null,
              cache_read_price: null,
              image_input_price: null,
              image_output_price: null,
              per_request_price: null,
              intervals: [
                {
                  min_tokens: 0,
                  max_tokens: 200000,
                  tier_label: '<=200K',
                  input_price: 1,
                  output_price: 2,
                  cache_write_price: null,
                  cache_read_price: null,
                  per_request_price: null,
                },
                {
                  min_tokens: 200000,
                  max_tokens: null,
                  tier_label: '>200K',
                  input_price: 1.5,
                  output_price: 3,
                  cache_write_price: null,
                  cache_read_price: null,
                  per_request_price: null,
                },
              ],
            },
          },
        ],
      },
    ],
  },
]

const baseProps = {
  columns: {
    name: 'Channel',
    description: 'Description',
    platform: 'Platform',
    groups: 'Groups and rates',
    supportedModels: 'Models and pricing',
  },
  rows,
  loading: false,
  pricingKeyPrefix: 'availableChannels.pricing',
  noPricingLabel: 'No pricing',
  noModelsLabel: 'No models',
  emptyLabel: 'No channels',
  userGroupRates: { 1: 0.8 },
}

function mountTable(props = {}) {
  return mount(AvailableChannelsTable, {
    props: { ...baseProps, ...props },
    global: {
      plugins: [createPinia()],
      stubs: {
        Icon: { props: ['name'], template: '<i :data-icon="name" />' },
        PlatformIcon: { template: '<i data-platform-icon />' },
        GroupBadge: {
          props: ['name', 'rateMultiplier', 'userRateMultiplier'],
          template:
            '<span data-group-badge>{{ name }}:{{ rateMultiplier }}:{{ userRateMultiplier }}</span>',
        },
        SupportedModelChip: {
          props: ['model', 'noPricingLabel'],
          template: '<span data-model-chip>{{ model.name }}:{{ noPricingLabel }}</span>',
        },
      },
    },
  })
}

describe('AvailableChannelsTable glass-card grid', () => {
  it('renders one glass card per channel with name, description, groups and model chips', () => {
    const wrapper = mountTable()
    const cards = wrapper.findAll('[data-testid="channel-card"]')

    expect(cards).toHaveLength(1)
    expect(cards[0].text()).toContain('Primary channel')
    expect(cards[0].text()).toContain('Fast and reliable access')
    expect(cards[0].text()).toContain('Groups and rates')
    expect(cards[0].text()).toContain('Models and pricing')
    expect(cards[0].text()).toContain('availableChannels.exclusive')
    expect(cards[0].text()).toContain('availableChannels.public')
    expect(cards[0].get('[data-group-badge]').text()).toBe('Exclusive Pro:1.2:0.8')
    expect(cards[0].findAll('[data-group-badge]')).toHaveLength(2)
    expect(cards[0].get('[data-icon="clock"]')).toBeTruthy()
    expect(cards[0].text()).toContain('08:00')
    expect(cards[0].text()).toContain('10:00')
    expect(cards[0].text()).toContain('×1.5')
    expect(cards[0].findAll('[data-model-chip]')).toHaveLength(2)
  })

  it('renders a collapsible tiered-pricing details block only for models with intervals', () => {
    const wrapper = mountTable()
    const details = wrapper.findAll('.pricing-details')

    expect(details).toHaveLength(1)
    expect(details[0].text()).toContain('gemini-2.5-pro')
    expect(details[0].text()).toContain('<=200K')
    expect(details[0].text()).toContain('>200K')
  })

  it('keeps the placeholder text when a platform has no groups or models', () => {
    const wrapper = mountTable({
      rows: [
        {
          name: 'Fallback channel',
          description: '',
          platforms: [{ platform: 'openai', groups: [], supported_models: [] }],
        },
      ],
    })
    const card = wrapper.get('[data-testid="channel-card"]')

    expect(card.text()).toContain('Fallback channel')
    expect(card.text()).toContain('openai')
    expect(card.text()).toContain('No models')
  })

  it('shows loading and empty states', async () => {
    const wrapper = mountTable({ loading: true, rows: [] })

    expect(wrapper.get('[data-testid="channels-loading"] [data-icon="refresh"]')).toBeTruthy()

    await wrapper.setProps({ loading: false })

    expect(wrapper.get('[data-testid="channels-empty"]').text()).toContain('No channels')
  })
})
