import { defineComponent } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PaymentProviderList from '@/components/payment/PaymentProviderList.vue'
import type { ProviderInstance } from '@/types/payment'

const messages: Record<string, string> = {
  'admin.settings.payment.providerManagement': 'Provider Management',
  'admin.settings.payment.providerManagementDesc': 'Manage providers',
  'admin.settings.payment.createProvider': 'Create Provider',
  'common.refresh': 'Refresh',
  'payment.methods.card': 'Bank Card',
  'payment.methods.link': 'Link',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
  }),
}))

function createProvider(): ProviderInstance {
  return {
    id: 1,
    provider_key: 'stripe',
    name: 'Stripe Primary',
    config: {},
    supported_types: ['card', 'link'],
    enabled: true,
    payment_mode: 'popup',
    refund_enabled: false,
    allow_user_refund: false,
    limits: '',
    sort_order: 0,
  }
}

describe('PaymentProviderList', () => {
  it('falls back to translated labels for provider available types', () => {
    const wrapper = mount(PaymentProviderList, {
      props: {
        providers: [createProvider()],
        loading: false,
        canCreate: true,
        enabledPaymentTypes: ['stripe'],
        allPaymentTypes: [
          { value: 'alipay', label: 'Alipay' },
          { value: 'wxpay', label: 'WeChat Pay' },
        ],
        redirectLabel: 'Redirect',
      },
      global: {
        stubs: {
          Icon: true,
          VueDraggable: {
            template: '<div><slot /></div>',
          },
          ProviderCard: defineComponent({
            name: 'ProviderCardStub',
            props: {
              availableTypes: { type: Array, required: true },
            },
            template: '<div>{{ availableTypes.map((item) => item.label).join(",") }}</div>',
          }),
        },
      },
    })

    expect(wrapper.text()).toContain('Bank Card')
    expect(wrapper.text()).toContain('Link')
  })
})
