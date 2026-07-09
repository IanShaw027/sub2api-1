import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { formatPaymentAmount } from '@/components/payment/currency'

const routeState = vi.hoisted(() => ({
  query: {} as Record<string, unknown>,
}))

const loadStripe = vi.hoisted(() => vi.fn())
const fetchMock = vi.hoisted(() => vi.fn())
const getOrder = vi.hoisted(() => vi.fn())
const openerPostMessage = vi.hoisted(() => vi.fn())
const windowClose = vi.hoisted(() => vi.fn())
const stripeInstance = vi.hoisted(() => ({
  confirmWechatPayPayment: vi.fn(),
  confirmAlipayPayment: vi.fn(),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRoute: () => routeState,
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

vi.mock('@stripe/stripe-js', () => ({
  loadStripe,
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    getOrder,
  },
}))

vi.mock('@/utils/device', () => ({
  isMobileDevice: () => false,
}))

import StripePopupView from '../StripePopupView.vue'

describe('StripePopupView', () => {
  beforeEach(() => {
    routeState.query = {
      order_id: '42',
      method: 'wechat_pay',
      amount: '88',
      currency: 'HKD',
    }
    loadStripe.mockReset().mockResolvedValue(stripeInstance)
    openerPostMessage.mockReset()
    windowClose.mockReset()
    stripeInstance.confirmWechatPayPayment.mockReset().mockResolvedValue({
      paymentIntent: {
        status: 'processing',
      },
    })
    stripeInstance.confirmAlipayPayment.mockReset()
    getOrder.mockReset().mockResolvedValue({
      data: { status: 'RECHARGING' },
    })
    fetchMock.mockReset().mockResolvedValue({
      ok: true,
      json: async () => ({ data: { status: 'RECHARGING' } }),
    })
    Object.defineProperty(window, 'opener', {
      configurable: true,
      value: {
        postMessage: openerPostMessage,
      },
    })
    vi.spyOn(window, 'close').mockImplementation(windowClose)
    vi.stubGlobal('fetch', fetchMock)
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
    vi.unstubAllGlobals()
    Object.defineProperty(window, 'opener', {
      configurable: true,
      value: null,
    })
  })

  it('posts the popup-ready handshake and closes after a successful confirmation', async () => {
    const wrapper = mount(StripePopupView)

    await flushPromises()
    expect(wrapper.text()).toContain(formatPaymentAmount(88, 'HKD'))
    expect(wrapper.text()).not.toContain('¥88')
    expect(openerPostMessage).toHaveBeenCalledWith(
      { type: 'STRIPE_POPUP_READY' },
      window.location.origin,
    )
    stripeInstance.confirmWechatPayPayment.mockResolvedValueOnce({
      paymentIntent: {
        status: 'succeeded',
      },
    })

    window.dispatchEvent(new MessageEvent('message', {
      origin: window.location.origin,
      data: {
        type: 'STRIPE_POPUP_INIT',
        clientSecret: 'pi_secret_42',
        publishableKey: 'pk_test',
      },
    }))
    await flushPromises()

    await vi.advanceTimersByTimeAsync(2000)
    await flushPromises()

    expect(wrapper.text()).toContain('payment.result.success')
    expect(windowClose).toHaveBeenCalled()
  })

  it('keeps polling while the popup order is still RECHARGING', async () => {
    const wrapper = mount(StripePopupView)

    await flushPromises()
    window.dispatchEvent(new MessageEvent('message', {
      origin: window.location.origin,
      data: {
        type: 'STRIPE_POPUP_INIT',
        clientSecret: 'pi_secret_42',
        publishableKey: 'pk_test',
      },
    }))
    await flushPromises()

    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()

    expect(getOrder).toHaveBeenCalledWith(42)
    expect(wrapper.text()).not.toContain('payment.result.success')
    expect(windowClose).not.toHaveBeenCalled()
  })
})
