import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'
import PaymentView from '../PaymentView.vue'
import { PAYMENT_RECOVERY_STORAGE_KEY } from '@/components/payment/paymentFlow'
import { formatPaymentAmount } from '@/components/payment/currency'
import type { CheckoutInfoResponse, MethodLimit, SubscriptionPlan } from '@/types/payment'

const routeState = vi.hoisted(() => ({
  path: '/purchase',
  query: {} as Record<string, unknown>,
}))

const routerReplace = vi.hoisted(() => vi.fn())
const routerPush = vi.hoisted(() => vi.fn())
const routerResolve = vi.hoisted(() => vi.fn(() => ({ href: '/payment/stripe?mock=1' })))
const createOrder = vi.hoisted(() => vi.fn())
const refreshUser = vi.hoisted(() => vi.fn())
const fetchActiveSubscriptions = vi.hoisted(() => vi.fn().mockResolvedValue(undefined))
const showError = vi.hoisted(() => vi.fn())
const showInfo = vi.hoisted(() => vi.fn())
const showWarning = vi.hoisted(() => vi.fn())
const getCheckoutInfo = vi.hoisted(() => vi.fn())
const bridgeInvoke = vi.hoisted(() => vi.fn())

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({
      replace: routerReplace,
      push: routerPush,
      resolve: routerResolve,
    }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    user: {
      username: 'demo-user',
      balance: 0,
    },
    refreshUser,
  }),
}))

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => ({
    createOrder,
  }),
}))

vi.mock('@/stores/subscriptions', () => ({
  useSubscriptionStore: () => ({
    activeSubscriptions: [],
    fetchActiveSubscriptions,
  }),
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
    showInfo,
    showWarning,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getCheckoutInfo,
  },
}))

vi.mock('@/utils/device', () => ({
  isMobileDevice: () => true,
}))

function checkoutInfoFixture(overrides: Partial<CheckoutInfoResponse> = {}) {
  const wxpayMethod: MethodLimit = {
    daily_limit: 0,
    daily_used: 0,
    daily_remaining: 0,
    single_min: 0,
    single_max: 0,
    fee_rate: 0,
    available: true,
  }
  const data: CheckoutInfoResponse = {
    methods: {
      wxpay: wxpayMethod,
    },
    global_min: 0,
    global_max: 0,
    plans: [],
    balance_disabled: false,
    balance_recharge_multiplier: 1,
    subscription_usd_to_cny_rate: 0,
    recharge_fee_rate: 0,
    help_text: '',
    help_image_url: '',
    stripe_publishable_key: '',
  }

  return {
    data: { ...data, ...overrides },
  }
}

function checkoutInfoWithPlansFixture(options: {
  checkout?: Partial<CheckoutInfoResponse>
  method?: Partial<MethodLimit>
  plan?: Partial<SubscriptionPlan>
} = {}) {
  const base = checkoutInfoFixture(options.checkout).data
  const plan: SubscriptionPlan = {
    id: 7,
    group_id: 3,
    name: 'Starter',
    description: '',
    price: 128,
    original_price: 0,
    validity_days: 30,
    validity_unit: 'day',
    rate_multiplier: 1,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    features: [],
    group_platform: 'openai',
    sort_order: 1,
    for_sale: true,
    group_name: 'OpenAI',
    ...options.plan,
  }

  return {
    data: {
      ...base,
      methods: {
        ...base.methods,
        wxpay: {
          ...base.methods.wxpay,
          ...options.method,
        },
      },
      plans: [plan],
    },
  }
}

function jsapiOrderFixture(resumeToken: string) {
  return {
    order_id: 123,
    amount: 88,
    pay_amount: 88,
    fee_rate: 0,
    expires_at: '2099-01-01T00:10:00.000Z',
    payment_type: 'wxpay',
    out_trade_no: 'sub2_jsapi_123',
    result_type: 'jsapi_ready' as const,
    resume_token: resumeToken,
    jsapi: {
      appId: 'wx123',
      timeStamp: '1712345678',
      nonceStr: 'nonce',
      package: 'prepay_id=wx123',
      signType: 'RSA',
      paySign: 'signed',
    },
  }
}

function oauthOrderFixture() {
  return {
    order_id: 456,
    amount: 128,
    pay_amount: 128,
    fee_rate: 0,
    expires_at: '2099-01-01T00:10:00.000Z',
    payment_type: 'wxpay',
    result_type: 'oauth_required' as const,
    oauth: {
      authorize_url: '/api/v1/auth/oauth/wechat/payment/start?payment_type=wxpay&redirect=%2Fpurchase%3Ffrom%3Dwechat',
      appid: 'wx123',
      scope: 'snsapi_base',
      redirect_url: '/auth/wechat/payment/callback',
    },
  }
}

async function mountSubscriptionConfirm(options: Parameters<typeof checkoutInfoWithPlansFixture>[0] = {}) {
  vi.useRealTimers()
  routeState.path = '/purchase'
  routeState.query = {
    tab: 'subscription',
    group: '3',
  }
  routerReplace.mockReset().mockResolvedValue(undefined)
  routerPush.mockReset().mockResolvedValue(undefined)
  routerResolve.mockClear()
  createOrder.mockReset()
  refreshUser.mockReset()
  fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
  showError.mockReset()
  showInfo.mockReset()
  showWarning.mockReset()
  getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoWithPlansFixture(options))
  bridgeInvoke.mockReset()
  window.localStorage.clear()
  ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = undefined

  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: {
          template: '<div><slot /></div>',
        },
        Teleport: true,
        Transition: false,
      },
    },
  })
  await flushPromises()
  await flushPromises()
  return wrapper
}

async function mountRecharge(options: {
  checkout?: Partial<CheckoutInfoResponse>
  method?: Partial<MethodLimit>
  amount?: number
} = {}) {
  vi.useRealTimers()
  routeState.path = '/purchase'
  routeState.query = {}
  routerReplace.mockReset().mockResolvedValue(undefined)
  routerPush.mockReset().mockResolvedValue(undefined)
  routerResolve.mockClear()
  createOrder.mockReset()
  refreshUser.mockReset()
  fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
  showError.mockReset()
  showInfo.mockReset()
  showWarning.mockReset()
  getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoFixture({
    ...options.checkout,
    methods: {
      wxpay: {
        ...checkoutInfoFixture().data.methods.wxpay,
        ...options.method,
      },
    },
  }))
  bridgeInvoke.mockReset()
  window.localStorage.clear()
  ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = undefined

  const wrapper = shallowMount(PaymentView, {
    global: {
      stubs: {
        AppLayout: {
          template: '<div><slot /></div>',
        },
        Teleport: true,
        Transition: false,
      },
    },
  })
  await flushPromises()
  await flushPromises()
  if (options.amount != null) {
    wrapper.findComponent({ name: 'AmountInput' }).vm.$emit('update:modelValue', options.amount)
    await flushPromises()
  }
  return wrapper
}

describe('PaymentView recharge confirmation amounts', () => {
  it('passes the effective checkout fee rate to the payment method display', async () => {
    const wrapper = await mountRecharge({
      checkout: {
        recharge_fee_rate: 2.5,
      },
      method: {
        currency: 'CNY',
        fee_rate: 0,
      },
      amount: 100,
    })

    const selector = wrapper.findComponent({ name: 'PaymentMethodSelector' })
    const methods = selector.props('methods') as Array<{ type: string; fee_rate: number }>
    expect(methods.find((method) => method.type === 'wxpay')?.fee_rate).toBe(2.5)
  })

  it('marks recharge payment method unavailable when fee-inclusive total exceeds method limit', async () => {
    const wrapper = await mountRecharge({
      checkout: {
        recharge_fee_rate: 2.5,
      },
      method: {
        currency: 'CNY',
        single_max: 102,
      },
      amount: 100,
    })

    const selector = wrapper.findComponent({ name: 'PaymentMethodSelector' })
    const methods = selector.props('methods') as Array<{ type: string; available: boolean }>
    expect(methods.find((method) => method.type === 'wxpay')?.available).toBe(false)
  })
})

describe('PaymentView subscription confirmation amounts', () => {
  it('rechecks method availability inside the subscription submit handler', async () => {
    const wrapper = await mountSubscriptionConfirm({
      method: { available: false },
    })

    await (wrapper.vm as unknown as { confirmSubscribe: () => Promise<void> }).confirmSubscribe()

    expect(createOrder).not.toHaveBeenCalled()
  })

  it('keeps subscription plan price independent from balance recharge multiplier', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        balance_recharge_multiplier: 4,
      },
      method: {
        currency: 'CNY',
      },
      plan: {
        price: 200,
        original_price: 300,
      },
    })

    const text = wrapper.text()
    const planPrice = formatPaymentAmount(200, 'CNY')
    const originalPrice = formatPaymentAmount(300, 'CNY')
    const convertedByRechargeMultiplier = formatPaymentAmount(50, 'CNY')

    expect(text).toContain(planPrice)
    expect(text).toContain(originalPrice)
    expect(text).not.toContain(convertedByRechargeMultiplier)
    expect(wrapper.findAll('button').some(button => button.text().includes(planPrice))).toBe(true)
  })

  it('validates subscription method limits against direct pay amount, not multiplier-derived balance credit', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        balance_recharge_multiplier: 0.14,
      },
      method: {
        currency: 'CNY',
        single_max: 10,
      },
      plan: {
        price: 7.99,
      },
    })

    const createButton = wrapper.findAll('button').find(button => button.text().includes('payment.createOrder'))

    expect(createButton?.attributes('disabled')).toBeUndefined()
    expect(createButton?.text()).toContain(formatPaymentAmount(7.99, 'CNY'))
  })

  it('keeps plan price when multiplier is not configured or payment currency is not CNY', async () => {
    const cnyWrapper = await mountSubscriptionConfirm({
      checkout: {
        balance_recharge_multiplier: 0,
      },
      method: {
        currency: 'CNY',
      },
      plan: {
        price: 7.99,
      },
    })

    expect(cnyWrapper.text()).toContain(formatPaymentAmount(7.99, 'CNY'))
    expect(cnyWrapper.text()).not.toContain(formatPaymentAmount(57.07, 'CNY'))

    const usdWrapper = await mountSubscriptionConfirm({
      checkout: {
        balance_recharge_multiplier: 0.14,
      },
      method: {
        currency: 'USD',
      },
      plan: {
        price: 7.99,
        original_price: 9.99,
      },
    })

    expect(usdWrapper.text()).toContain(formatPaymentAmount(7.99, 'USD'))
    expect(usdWrapper.text()).toContain(formatPaymentAmount(9.99, 'USD'))
  })

  it('converts subscription USD price to CNY gateway amount when subscription exchange rate is configured', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        subscription_usd_to_cny_rate: 7.15,
      },
      method: {
        currency: 'CNY',
      },
      plan: {
        price: 7.99,
        original_price: 9.99,
      },
    })

    const text = wrapper.text()
    const convertedPrice = formatPaymentAmount(57.13, 'CNY')
    const convertedOriginalPrice = formatPaymentAmount(71.43, 'CNY')

    expect(text).toContain(convertedPrice)
    expect(text).toContain(convertedOriginalPrice)
    expect(text).toContain(`${formatPaymentAmount(7.99, 'USD')} × 7.15`)
    expect(wrapper.findAll('button').some(button => button.text().includes(convertedPrice))).toBe(true)
  })

  it('validates subscription method limits against exchange-rate converted CNY pay amount', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        subscription_usd_to_cny_rate: 7.15,
      },
      method: {
        currency: 'CNY',
        single_max: 57,
      },
      plan: {
        price: 7.99,
      },
    })

    const selector = wrapper.findComponent({ name: 'PaymentMethodSelector' })
    const methods = selector.props('methods') as Array<{ type: string; available: boolean }>
    expect(methods.find((method) => method.type === 'wxpay')?.available).toBe(false)
  })

  it('adds fee rate to the direct subscription price without applying balance multiplier', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        balance_recharge_multiplier: 4,
        recharge_fee_rate: 2.5,
      },
      method: {
        currency: 'CNY',
      },
      plan: {
        price: 7.99,
      },
    })

    const text = wrapper.text()
    const planPrice = formatPaymentAmount(7.99, 'CNY')
    const fee = formatPaymentAmount(0.2, 'CNY')
    const total = formatPaymentAmount(8.19, 'CNY')

    expect(text).toContain(planPrice)
    expect(text).toContain(fee)
    expect(text).toContain(total)
    expect(text).not.toContain(formatPaymentAmount(57.07, 'CNY'))
    expect(wrapper.findAll('button').some(button => button.text().includes(total))).toBe(true)
  })

  it('passes the checkout fee rate to the selected payment method display', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        recharge_fee_rate: 2.5,
      },
      method: {
        currency: 'CNY',
        fee_rate: 0,
      },
      plan: {
        price: 7.99,
      },
    })

    const selector = wrapper.findComponent({ name: 'PaymentMethodSelector' })
    const methods = selector.props('methods') as Array<{ type: string; fee_rate: number }>
    expect(methods.find((method) => method.type === 'wxpay')?.fee_rate).toBe(2.5)
  })

  it('marks subscription payment method unavailable when fee-inclusive total exceeds method limit', async () => {
    const wrapper = await mountSubscriptionConfirm({
      checkout: {
        recharge_fee_rate: 2.5,
      },
      method: {
        currency: 'CNY',
        single_max: 8.1,
      },
      plan: {
        price: 7.99,
      },
    })

    const selector = wrapper.findComponent({ name: 'PaymentMethodSelector' })
    const methods = selector.props('methods') as Array<{ type: string; available: boolean }>
    expect(methods.find((method) => method.type === 'wxpay')?.available).toBe(false)
  })
})

describe('PaymentView WeChat JSAPI flow', () => {
  beforeEach(() => {
    routeState.path = '/purchase'
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-123',
    }
    routerReplace.mockReset().mockResolvedValue(undefined)
    routerPush.mockReset().mockResolvedValue(undefined)
    routerResolve.mockClear()
    createOrder.mockReset()
    refreshUser.mockReset()
    fetchActiveSubscriptions.mockReset().mockResolvedValue(undefined)
    showError.mockReset()
    showInfo.mockReset()
    showWarning.mockReset()
    getCheckoutInfo.mockReset().mockResolvedValue(checkoutInfoFixture())
    bridgeInvoke.mockReset()
    window.localStorage.clear()
    ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = {
      invoke: bridgeInvoke,
    }
  })

  it('resets payment state and redirects to /payment/result after JSAPI reports success', async () => {
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-123'))
    bridgeInvoke.mockImplementation((_action, _payload, callback) => {
      callback({ err_msg: 'get_brand_wcpay_request:ok' })
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ path: '/purchase', query: {} })
    expect(routerPush).toHaveBeenCalledWith({
      path: '/payment/result',
      query: {
        order_id: '123',
        out_trade_no: 'sub2_jsapi_123',
        resume_token: 'resume-token-123',
      },
    })
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('resets payment state when JSAPI reports cancellation', async () => {
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-cancel'))
    bridgeInvoke.mockImplementation((_action, _payload, callback) => {
      callback({ err_msg: 'get_brand_wcpay_request:cancel' })
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(showInfo).toHaveBeenCalledWith('payment.qr.cancelled')
    expect(routerPush).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('clears stale recovery state when JSAPI never becomes available', async () => {
    vi.useFakeTimers()
    createOrder.mockResolvedValue(jsapiOrderFixture('resume-token-missing-bridge'))
    ;(window as Window & { WeixinJSBridge?: { invoke: typeof bridgeInvoke } }).WeixinJSBridge = undefined

    const wrapper = shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })

    await flushPromises()
    await vi.advanceTimersByTimeAsync(4000)
    await flushPromises()
    await flushPromises()

    expect(showError).toHaveBeenCalledWith(
      'payment.errors.wechatJsapiUnavailable payment.errors.wechatOpenInWeChatHint',
    )
    expect(routerPush).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
    expect(wrapper.html()).not.toContain('payment-status-panel-stub')
  })

  it('clears a stale recovery snapshot before a later unrelated session can restore it', async () => {
    routeState.query = {}
    window.localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify({
      orderId: 999,
      amount: 66,
      qrCode: 'stale-qr',
      expiresAt: '2099-01-01T00:10:00.000Z',
      paymentType: 'alipay',
      payUrl: 'https://pay.example.com/stale',
      outTradeNo: 'stale-out-trade-no',
      clientSecret: '',
      intentId: '',
      currency: '',
      countryCode: '',
      paymentEnv: '',
      payAmount: 66,
      orderType: 'balance',
      paymentMode: 'popup',
      resumeToken: '',
      createdAt: Date.UTC(2099, 0, 1, 0, 0, 0),
    }))

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(createOrder).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toBeNull()
  })

  it('keeps subscription resume context for token-only WeChat callbacks', async () => {
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-subscription-7',
      payment_type: 'wxpay_direct',
      order_type: 'subscription',
      plan_id: '7',
    }
    getCheckoutInfo.mockResolvedValue(checkoutInfoWithPlansFixture())
    createOrder.mockResolvedValue(oauthOrderFixture())

    const originalLocation = window.location
    const locationState = {
      href: 'http://localhost/purchase',
      origin: 'http://localhost',
    }
    Object.defineProperty(window, 'location', {
      configurable: true,
      value: locationState,
    })

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(routerReplace).toHaveBeenCalledWith({ path: '/purchase', query: {} })
    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      payment_type: 'wxpay',
      order_type: 'subscription',
      plan_id: 7,
      wechat_resume_token: 'resume-subscription-7',
    }))
    expect(locationState.href).toContain('/api/v1/auth/oauth/wechat/payment/start?')
    expect(new URL(locationState.href, 'http://localhost').searchParams.get('redirect')).toBe(
      '/purchase?from=wechat&payment_type=wxpay&order_type=subscription&plan_id=7',
    )

    Object.defineProperty(window, 'location', {
      configurable: true,
      value: originalLocation,
    })
  })

  it('keeps a matching recovery snapshot when token-only resume retry fails', async () => {
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-keep',
      payment_type: 'wxpay_direct',
    }
    createOrder.mockRejectedValueOnce(new Error('resume failed'))
    window.localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify({
      orderId: 321,
      amount: 88,
      qrCode: 'weixin://wxpay/bizpayurl?pr=resume',
      expiresAt: '2099-01-01T00:10:00.000Z',
      paymentType: 'wxpay',
      payUrl: '',
      outTradeNo: 'sub2_qr_321',
      clientSecret: '',
      payAmount: 88,
      orderType: 'balance',
      paymentMode: 'native',
      resumeToken: 'resume-token-keep',
      createdAt: Date.UTC(2099, 0, 1, 0, 0, 0),
    }))

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(createOrder).toHaveBeenCalledWith(expect.objectContaining({
      wechat_resume_token: 'resume-token-keep',
    }))
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toContain('"resumeToken":"resume-token-keep"')
  })

  it('falls back to QR flow when mobile WeChat payment is unavailable', async () => {
    routeState.query = {
      wechat_resume: '1',
      wechat_resume_token: 'resume-token-h5',
      payment_type: 'wxpay_direct',
    }
    createOrder
      .mockRejectedValueOnce({ reason: 'WECHAT_H5_NOT_AUTHORIZED' })
      .mockResolvedValueOnce({
        order_id: 778,
        amount: 88,
        pay_amount: 88,
        fee_rate: 0,
        expires_at: '2099-01-01T00:10:00.000Z',
        payment_type: 'wxpay',
        qr_code: 'weixin://wxpay/bizpayurl?pr=fallback-native',
        out_trade_no: 'sub2_qr_778',
      })
    window.localStorage.setItem(PAYMENT_RECOVERY_STORAGE_KEY, JSON.stringify({
      orderId: 777,
      amount: 88,
      qrCode: 'weixin://wxpay/bizpayurl?pr=resume-native',
      expiresAt: '2099-01-01T00:10:00.000Z',
      paymentType: 'wxpay',
      payUrl: '',
      outTradeNo: 'sub2_qr_777',
      clientSecret: '',
      payAmount: 88,
      orderType: 'balance',
      paymentMode: 'native',
      resumeToken: 'resume-token-h5',
      createdAt: Date.UTC(2099, 0, 1, 0, 0, 0),
    }))

    shallowMount(PaymentView, {
      global: {
        stubs: {
          Teleport: true,
          Transition: false,
        },
      },
    })
    await flushPromises()
    await flushPromises()

    expect(createOrder).toHaveBeenNthCalledWith(1, expect.objectContaining({
      amount: 88,
      payment_type: 'wxpay',
      is_mobile: true,
      wechat_resume_token: 'resume-token-h5',
    }))
    expect(createOrder).toHaveBeenNthCalledWith(2, expect.objectContaining({
      amount: 88,
      payment_type: 'wxpay',
      is_mobile: false,
      payment_source: 'hosted_redirect',
      wechat_resume_token: 'resume-token-h5',
    }))
    expect(showWarning).toHaveBeenCalledWith('payment.errors.mobilePaymentFallbackToQr')
    expect(showError).not.toHaveBeenCalled()
    expect(window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY)).toContain('weixin://wxpay/bizpayurl?pr=fallback-native')
  })
})
