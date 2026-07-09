import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, shallowMount } from '@vue/test-utils'

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
}))
const routerPush = vi.hoisted(() => vi.fn())
const getOrder = vi.hoisted(() => vi.fn())
const resolveOrderPublicByResumeToken = vi.hoisted(() => vi.fn())
const paymentStore = vi.hoisted(() => ({
  config: { stripe_publishable_key: 'pk_test' } as { stripe_publishable_key?: string },
  fetchConfig: vi.fn(),
  pollOrderStatus: vi.fn(),
}))
const loadStripe = vi.hoisted(() => vi.fn())
const stripeElements = vi.hoisted(() => ({
  create: vi.fn(),
}))
const stripePaymentElement = vi.hoisted(() => ({
  mount: vi.fn(),
  on: vi.fn(),
}))
const stripeInstance = vi.hoisted(() => ({
  elements: vi.fn(),
  confirmPayment: vi.fn(),
  confirmAlipayPayment: vi.fn(),
  confirmWechatPayPayment: vi.fn(),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
    useRouter: () => ({ push: routerPush }),
  }
})

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'zh-CN' },
    }),
  }
})

vi.mock('@/stores/payment', () => ({
  usePaymentStore: () => paymentStore,
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getOrder,
    resolveOrderPublicByResumeToken,
  },
}))

vi.mock('@stripe/stripe-js', () => ({
  loadStripe,
}))

import StripePaymentView from '../StripePaymentView.vue'
import { formatPaymentAmount } from '@/components/payment/currency'
import type { PaymentOrder } from '@/types/payment'

function orderFactory(overrides: Partial<PaymentOrder> = {}): PaymentOrder {
  return {
    id: 42,
    user_id: 7,
    amount: 100,
    pay_amount: 103,
    currency: 'CNY',
    fee_rate: 0.03,
    payment_type: 'stripe',
    out_trade_no: 'sub2_stripe_42',
    status: 'PENDING',
    order_type: 'balance',
    created_at: '2026-04-20T12:00:00Z',
    expires_at: '2026-04-20T12:30:00Z',
    refund_amount: 0,
    ...overrides,
  }
}

function mountView() {
  return shallowMount(StripePaymentView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
      },
    },
  })
}

describe('StripePaymentView', () => {
  beforeEach(() => {
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
    }
    routerPush.mockReset()
    getOrder.mockReset()
    resolveOrderPublicByResumeToken.mockReset()
    paymentStore.config = { stripe_publishable_key: 'pk_test' }
    paymentStore.fetchConfig.mockReset().mockResolvedValue(undefined)
    paymentStore.pollOrderStatus.mockReset()
    loadStripe.mockReset().mockResolvedValue(stripeInstance)
    stripeElements.create.mockReset().mockReturnValue(stripePaymentElement)
    stripePaymentElement.mount.mockReset()
    stripePaymentElement.on.mockReset().mockImplementation((event: string, callback: () => void) => {
      if (event === 'ready') callback()
    })
    stripeInstance.elements.mockReset().mockReturnValue(stripeElements)
    stripeInstance.confirmPayment.mockReset()
    stripeInstance.confirmAlipayPayment.mockReset()
    stripeInstance.confirmWechatPayPayment.mockReset()
    window.localStorage.clear()
  })

  it('本地恢复快照缺失时使用订单接口返回的 Stripe 币种展示金额', async () => {
    getOrder.mockResolvedValue({
      data: orderFactory({ currency: 'HKD', pay_amount: 103 }),
    })

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(getOrder).toHaveBeenCalledWith(42)
    expect(loadStripe).toHaveBeenCalledWith('pk_test')
    expect(wrapper.text()).toContain(formatPaymentAmount(103, 'HKD', 'zh-CN'))
  })

  it('keeps polling while the Stripe order is still RECHARGING', async () => {
    vi.useFakeTimers()
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
      method: 'wechat_pay',
    }
    getOrder.mockResolvedValue({
      data: orderFactory(),
    })
    stripeInstance.confirmWechatPayPayment.mockResolvedValue({
      paymentIntent: {
        status: 'processing',
        next_action: {
          wechat_pay_display_qr_code: {
            image_data_url: 'data:image/png;base64,qr',
          },
        },
      },
    })
    paymentStore.pollOrderStatus.mockResolvedValue({
      ...orderFactory({ status: 'RECHARGING' }),
    })

    const wrapper = mountView()
    await flushPromises()

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(paymentStore.pollOrderStatus).toHaveBeenCalledWith(42)
    expect(wrapper.text()).not.toContain('payment.result.success')

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(routerPush).not.toHaveBeenCalled()
    vi.useRealTimers()
  })

  it('falls back to public resume-token order resolution when authenticated order lookup fails', async () => {
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
      resume_token: 'resume-42',
    }
    getOrder.mockRejectedValueOnce(new Error('auth required'))
    resolveOrderPublicByResumeToken.mockResolvedValueOnce({
      data: orderFactory({ currency: 'HKD', pay_amount: 103 }),
    })

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(getOrder).toHaveBeenCalledWith(42)
    expect(resolveOrderPublicByResumeToken).toHaveBeenCalledWith('resume-42')
    expect(loadStripe).toHaveBeenCalledWith('pk_test')
    expect(wrapper.text()).toContain(formatPaymentAmount(103, 'HKD', 'zh-CN'))
    expect(wrapper.text()).not.toContain('payment.stripeLoadFailed')
  })

  it('does not fall back to public resume-token order resolution for non-auth order lookup failures', async () => {
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
      resume_token: 'resume-42',
    }
    getOrder.mockRejectedValueOnce({ status: 500, message: 'server exploded' })

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(getOrder).toHaveBeenCalledWith(42)
    expect(resolveOrderPublicByResumeToken).not.toHaveBeenCalled()
    expect(loadStripe).not.toHaveBeenCalled()
    expect(wrapper.text()).toContain('server exploded')
  })

  it('includes resume-token and out-trade-no in stripe return URLs', async () => {
    vi.useFakeTimers()
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
      method: 'alipay',
      resume_token: 'resume-42',
      out_trade_no: 'sub2_stripe_42',
    }
    getOrder.mockResolvedValue({
      data: orderFactory(),
    })
    stripeInstance.confirmAlipayPayment.mockResolvedValueOnce({})

    mountView()
    await flushPromises()

    expect(stripeInstance.confirmAlipayPayment).toHaveBeenCalledTimes(1)
    const returnUrl = stripeInstance.confirmAlipayPayment.mock.calls[0]?.[1]?.return_url as string
    expect(returnUrl).toContain('/payment/result?')
    expect(returnUrl).toContain('order_id=42')
    expect(returnUrl).toContain('resume_token=resume-42')
    expect(returnUrl).toContain('out_trade_no=sub2_stripe_42')
    vi.useRealTimers()
  })

  it('uses publishable_key from route query when payment config lookup is unavailable', async () => {
    routeState.query = {
      order_id: '42',
      client_secret: 'pi_secret_42',
      publishable_key: 'pk_live_from_query',
    }
    getOrder.mockResolvedValue({
      data: orderFactory(),
    })
    paymentStore.config = {}
    paymentStore.fetchConfig.mockRejectedValueOnce(new Error('auth required'))

    const wrapper = mountView()
    await flushPromises()
    await flushPromises()

    expect(loadStripe).toHaveBeenCalledWith('pk_live_from_query')
    expect(wrapper.text()).not.toContain('payment.stripeLoadFailed')
  })
})
