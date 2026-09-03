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
  it('wraps large custom method collections without letting labels widen the selector', () => {
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

    // Layout responsibilities moved from Tailwind utility classes to the
    // scoped `.method-grid` rule during the Glass redesign; assert the
    // grid still wraps into columns (never a flex/nowrap row) at every
    // breakpoint instead of matching now-removed utility class names.
    const grid = wrapper.get('[data-testid="payment-method-grid"]')
    expect(grid.classes()).toContain('method-grid')
    expect(grid.classes()).not.toContain('sm:flex')

    const gridRuleMatch = componentSource.match(/\.method-grid\s*\{([^}]*)\}/)
    expect(gridRuleMatch?.[1]).toMatch(/display:\s*grid/)
    const responsiveGridRules = Array.from(
      componentSource.matchAll(/@media[^{]*\{\s*\.method-grid\s*\{([^}]*)\}/g),
    )
    expect(responsiveGridRules.length).toBeGreaterThanOrEqual(2)
    expect(responsiveGridRules.every(([, declarations]) => /grid-template-columns/.test(declarations))).toBe(true)
    expect(componentSource).not.toMatch(/\.method-grid\s*\{[^}]*display:\s*flex/)

    const buttons = wrapper.findAll('button')
    expect(buttons).toHaveLength(methods.length)
    // `min-w-0` and `truncate` were folded into the scoped `.method-option`
    // and `.method-option-label` rules; assert the equivalent raw CSS instead
    // of the now-removed Tailwind utility class names.
    expect(buttons.every(button => button.classes().includes('method-option'))).toBe(true)
    const optionRuleMatch = componentSource.match(/\.method-option\s*\{([^}]*)\}/)
    expect(optionRuleMatch?.[1]).toMatch(/min-width:\s*0/)
    expect(buttons.every((button, index) => button.attributes('title') === methods[index].display_name)).toBe(true)

    const labels = wrapper.findAll('[data-testid="payment-method-label"]')
    expect(labels.every(label => label.classes().includes('method-option-label'))).toBe(true)
    const labelRuleMatch = componentSource.match(/\.method-option-label\s*\{([^}]*)\}/)
    expect(labelRuleMatch?.[1]).toMatch(/overflow:\s*hidden/)
    expect(labelRuleMatch?.[1]).toMatch(/text-overflow:\s*ellipsis/)
    expect(labelRuleMatch?.[1]).toMatch(/white-space:\s*nowrap/)
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

  it('uses the generic selected style for custom methods that contain built-in names', () => {
    const wrapper = mount(PaymentMethodSelector, {
      props: {
        selected: 'card_alipay',
        methods: [{ type: 'card_alipay', display_name: 'Card Pay', fee_rate: 0, available: true }],
      },
    })

    // The redesign dropped the old per-brand `methodSelectedClass()` branching
    // (border-[#...] arbitrary colors per provider) in favor of one generic
    // selected style (`glass-ring` + `method-option--active`) applied to every
    // method regardless of its `type`, so a custom method whose type merely
    // contains a built-in provider name (e.g. "card_alipay") can no longer be
    // mistaken for the real Alipay method and painted with its brand color.
    const button = wrapper.get('button')
    expect(button.classes()).toContain('glass-ring')
    expect(button.classes()).toContain('method-option--active')
    expect(button.classes().some((cls) => /^border-\[#/.test(cls))).toBe(false)
  })
})
