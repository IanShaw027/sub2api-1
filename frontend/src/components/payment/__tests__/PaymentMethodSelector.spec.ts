import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PaymentMethodSelector from '@/components/payment/PaymentMethodSelector.vue'

const componentSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../PaymentMethodSelector.vue'),
  'utf8',
)

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}))

describe('PaymentMethodSelector', () => {
  it('renders brand buttons as a wrapping 32px-tall pill row (glass-page-templates PaymentFlow spec)', () => {
    const methods = Array.from({ length: 12 }, (_, index) => ({
      type: `custom_${index}`,
      display_name: `CUSTOM_PAYMENT_METHOD_${index}`,
      fee_rate: 0,
      available: true,
    }))

    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'custom_0',
        methods,
      },
    })

    const row = wrapper.get('[data-testid="payment-method-grid"]')
    expect(row.classes()).toContain('method-row')

    const rowRuleMatch = componentSource.match(/\.method-row\s*\{([^}]*)\}/)
    expect(rowRuleMatch?.[1]).toMatch(/display:\s*flex/)
    expect(rowRuleMatch?.[1]).toMatch(/flex-wrap:\s*wrap/)

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(methods.length)
    expect(buttons.every(button => button.classes().includes('method-option'))).toBe(true)

    // Spec: "支付品牌按钮 32px" — brand buttons are 32px tall pills, not the
    // previous 58px stacked cards.
    const optionRuleMatch = componentSource.match(/\.method-option\s*\{([^}]*)\}/)
    expect(optionRuleMatch?.[1]).toMatch(/height:\s*32px/)

    expect(buttons.every((button, index) => button.attributes('title') === methods[index].display_name)).toBe(true)

    const labels = wrapper.findAll('[data-testid="payment-method-label"]')
    expect(labels.every(label => label.classes().includes('method-option-label'))).toBe(true)
  })

  it('shows the configured display name for custom EasyPay methods', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'ldc',
        methods: [{ type: 'ldc', display_name: 'LDC Pay', fee_rate: 0, available: true }],
      },
    })

    expect(wrapper.text()).toContain('LDC Pay')
    expect(wrapper.text()).not.toContain('ldc')
    expect(wrapper.text()).not.toContain('payment.methods.ldc')
  })

  it('includes the fee rate in the title when a method charges a fee', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'stripe',
        methods: [{ type: 'stripe', display_name: 'Stripe', fee_rate: 2.9, available: true }],
      },
    })

    const button = wrapper.get('button')
    expect(button.attributes('title')).toBe('Stripe · payment.fee 2.9%')
    expect(wrapper.get('.method-option-fee').text()).toContain('2.9%')
  })

  it('uses the generic selected style for custom methods that contain built-in names', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'card_alipay',
        methods: [{ type: 'card_alipay', display_name: 'Card Pay', fee_rate: 0, available: true }],
      },
    })

    // One generic selected style (`method-option--active`) is applied to every
    // method regardless of its `type`, so a custom method whose type merely
    // contains a built-in provider name (e.g. "card_alipay") can no longer be
    // mistaken for the real Alipay method and painted with its brand color.
    const button = wrapper.get('button')
    expect(button.classes()).toContain('method-option--active')
    expect(button.classes().some((cls) => /^border-\[#/.test(cls))).toBe(false)
  })
})
