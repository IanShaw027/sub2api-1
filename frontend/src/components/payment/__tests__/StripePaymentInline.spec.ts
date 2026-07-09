import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const routerResolve = vi.hoisted(() => vi.fn())
const loadStripe = vi.hoisted(() => vi.fn())
const cancelOrder = vi.hoisted(() => vi.fn())
const showError = vi.hoisted(() => vi.fn())
const windowOpen = vi.hoisted(() => vi.fn())

const stripeElements = vi.hoisted(() => ({
  create: vi.fn(),
}))
const paymentElement = vi.hoisted(() => ({
  mount: vi.fn(),
  on: vi.fn(),
}))
const stripeInstance = vi.hoisted(() => ({
  elements: vi.fn(),
  confirmPayment: vi.fn(),
}))

vi.mock('vue-router', async () => {
  const actual = await vi.importActual<typeof import('vue-router')>('vue-router')
  return {
    ...actual,
    useRouter: () => ({
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

vi.mock('@/stores', () => ({
  useAppStore: () => ({
    showError,
  }),
}))

vi.mock('@/api/payment', () => ({
  paymentAPI: {
    cancelOrder,
  },
}))

vi.mock('@/components/payment/providerConfig', () => ({
  getPaymentPopupFeatures: () => 'popup-features',
}))

vi.mock('@stripe/stripe-js', () => ({
  loadStripe,
}))

import StripePaymentInline from '../StripePaymentInline.vue'

function mountView() {
  return mount(StripePaymentInline, {
    props: {
      orderId: 42,
      amount: 100,
      clientSecret: 'pi_secret_42',
      publishableKey: 'pk_test',
      payAmount: 103,
      currency: 'HKD',
    },
    global: {
      stubs: {
        Icon: { template: '<span />' },
      },
    },
  })
}

describe('StripePaymentInline', () => {
  beforeEach(() => {
    routerResolve.mockReset().mockReturnValue({
      href: '/payment/stripe-popup?mock=1',
    })
    cancelOrder.mockReset()
    showError.mockReset()
    windowOpen.mockReset().mockReturnValue({
      postMessage: vi.fn(),
    })
    loadStripe.mockReset().mockResolvedValue(stripeInstance)
    stripeInstance.elements.mockReset().mockReturnValue(stripeElements)
    stripeInstance.confirmPayment.mockReset()
    stripeElements.create.mockReset().mockReturnValue(paymentElement)
    paymentElement.mount.mockReset()
    paymentElement.on.mockReset().mockImplementation((event: string, callback: (payload?: { value: { type: string } }) => void) => {
      if (event === 'ready') {
        callback()
      }
      if (event === 'change') {
        callback({ value: { type: 'alipay' } })
      }
    })
    vi.spyOn(window, 'open').mockImplementation(windowOpen)
  })

  it('includes currency in the stripe popup route query', async () => {
    const wrapper = mountView()
    await flushPromises()

    await wrapper.get('button.btn-stripe').trigger('click')

    expect(routerResolve).toHaveBeenCalledWith({
      path: '/payment/stripe-popup',
      query: {
        order_id: '42',
        method: 'alipay',
        amount: '103',
        currency: 'HKD',
      },
    })
    expect(windowOpen).toHaveBeenCalled()
  })

  it('formats payAmount with the payment currency fraction digits', async () => {
    const wrapper = mount(StripePaymentInline, {
      props: {
        orderId: 42,
        amount: 100,
        clientSecret: 'pi_secret_42',
        publishableKey: 'pk_test',
        payAmount: 108,
        currency: 'JPY',
      },
      global: {
        stubs: {
          Icon: { template: '<span />' },
        },
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('¥108')
    expect(wrapper.text()).not.toContain('¥108.00')
  })
})
