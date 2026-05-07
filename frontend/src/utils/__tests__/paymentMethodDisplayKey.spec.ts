import { describe, expect, it } from 'vitest'
import { paymentMethodDisplayKey } from '../i18n'

describe('paymentMethodDisplayKey', () => {
  it('normalizes display aliases to canonical payment keys', () => {
    expect(paymentMethodDisplayKey('alipay_direct')).toBe('payment.methods.alipay')
    expect(paymentMethodDisplayKey('wechat_pay')).toBe('payment.methods.wxpay')
  })

  it('keeps canonical methods unchanged', () => {
    expect(paymentMethodDisplayKey('stripe')).toBe('payment.methods.stripe')
  })
})
