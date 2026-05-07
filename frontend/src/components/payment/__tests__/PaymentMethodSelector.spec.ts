import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'

const messages: Record<string, string> = {
  'payment.paymentMethod': 'Payment Method',
  'payment.fee': 'Fee',
  'payment.methods.alipay': 'Alipay',
  'payment.methods.wxpay': 'WeChat Pay',
}

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => messages[key] ?? fallback ?? key,
  }),
}))

describe('PaymentMethodSelector', () => {
  it('renders alias payment methods with canonical display labels', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        methods: [
          { type: 'wxpay_direct', fee_rate: 0, available: true },
          { type: 'alipay_direct', fee_rate: 1.5, available: true },
        ],
        selected: 'alipay_direct',
      },
    })

    expect(wrapper.text()).toContain('Alipay')
    expect(wrapper.text()).toContain('WeChat Pay')
    expect(wrapper.text()).toContain('Fee 1.5%')
  })
})
