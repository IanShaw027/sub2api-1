import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import ProviderCard from '@/components/payment/ProviderCard.vue'
import type { ProviderInstance } from '@/types/payment'

const messages: Record<string, string> = {
  'common.enabled': 'Enabled',
  'common.edit': 'Edit',
  'common.delete': 'Delete',
  'admin.settings.payment.providerAlipay': 'Alipay',
  'admin.settings.payment.modeQRCode': 'QR Code',
  'admin.settings.payment.modePopup': 'Popup',
  'admin.settings.payment.modeRedirect': 'Redirect',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
  }),
}))

const toggleSwitchStub = {
  template: '<div />',
  props: ['label', 'checked'],
}

const iconStub = {
  template: '<div />',
  props: ['name', 'size'],
}

function providerFactory(overrides: Partial<ProviderInstance> = {}): ProviderInstance {
  return {
    id: 1,
    provider_key: 'alipay',
    name: 'Alipay Direct',
    config: {},
    supported_types: ['alipay'],
    enabled: true,
    payment_mode: '',
    refund_enabled: false,
    allow_user_refund: false,
    invoice_enabled: false,
    ...overrides,
  }
}

describe('ProviderCard', () => {
  it('shows redirect label for redirect payment mode', () => {
    const wrapper = mount(ProviderCard, {
      props: {
        provider: providerFactory({ payment_mode: 'redirect' }),
        enabled: true,
        availableTypes: [],
      },
      global: {
        stubs: {
          ToggleSwitch: toggleSwitchStub,
          Icon: iconStub,
        },
      },
    })

    expect(wrapper.text()).toContain('Redirect')
  })
})
