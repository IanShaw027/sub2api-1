import { ref, computed, onMounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePaymentStore } from '@/stores/payment'
import { useSubscriptionStore } from '@/stores/subscriptions'
import { useAppStore } from '@/stores'
import { paymentAPI } from '@/api/payment'
import { extractApiErrorMessage, extractI18nErrorMessage } from '@/utils/apiError'
import { isMobileDevice } from '@/utils/device'
import { hasPeakRate, formatPeakRateWindow, serverTimezoneLabel, type PeakRateFields } from '@/utils/peak-rate'
import type { SubscriptionPlan, CheckoutInfoResponse, CreateOrderResult, OrderType } from '@/types/payment'
import { METHOD_ORDER, getPaymentPopupFeatures, isBuiltInAlipayMethod, isBuiltInWxpayMethod } from '@/components/payment/providerConfig'
import {
  PAYMENT_RECOVERY_STORAGE_KEY,
  buildCreateOrderPayload,
  clearPaymentRecoverySnapshot,
  decidePaymentLaunch,
  getVisibleMethods,
  normalizeVisibleMethod,
  readPaymentRecoverySnapshot,
  type PaymentRecoverySnapshot,
  writePaymentRecoverySnapshot,
} from '@/components/payment/paymentFlow'
import { platformBadgeClass, platformTextClass } from '@/utils/platformColors'
import { DEFAULT_PAYMENT_CURRENCY, formatPaymentAmount, normalizePaymentCurrency } from '@/components/payment/currency'
import { planValiditySuffix as validitySuffixOf } from '@/components/payment/validity'
import type { PaymentMethodOption } from '@/components/payment/PaymentMethodSelector.vue'
import { buildPaymentErrorToastMessage, describePaymentScenarioError } from '../paymentUx'
import { hasWechatResumeQuery, parseWechatResumeRoute, stripWechatResumeQuery } from '../paymentWechatResume'

interface CreateOrderOptions {
  openid?: string
  wechatResumeToken?: string
  paymentType?: string
  isResume?: boolean
  mobileQrFallbackAttempted?: boolean
}

interface WeixinJSBridgeLike {
  invoke(
    action: string,
    payload: Record<string, unknown>,
    callback: (result: Record<string, unknown>) => void,
  ): void
}

interface MobileQrFallbackContext {
  orderAmount: number
  orderType: OrderType
  planId?: number
  paymentType: string
  attempted: boolean
}

function emptyPaymentState(): PaymentRecoverySnapshot {
  return {
    orderId: 0,
    amount: 0,
    qrCode: '',
    expiresAt: '',
    paymentType: '',
    payUrl: '',
    outTradeNo: '',
    clientSecret: '',
    intentId: '',
    currency: '',
    countryCode: '',
    paymentEnv: '',
    payAmount: 0,
    orderType: '',
    paymentMode: '',
    resumeToken: '',
    alipayMobilePrecreateDeepLink: false,
    createdAt: 0,
  }
}

function getWeixinJSBridge(): WeixinJSBridgeLike | undefined {
  return (window as Window & { WeixinJSBridge?: WeixinJSBridgeLike }).WeixinJSBridge
}

function waitForWeixinJSBridge(timeoutMs = 4000): Promise<WeixinJSBridgeLike | null> {
  const existing = getWeixinJSBridge()
  if (existing) return Promise.resolve(existing)

  return new Promise((resolve) => {
    let settled = false
    const finish = (bridge: WeixinJSBridgeLike | null) => {
      if (settled) return
      settled = true
      document.removeEventListener('WeixinJSBridgeReady', handleReady)
      document.removeEventListener('onWeixinJSBridgeReady', handleReady)
      window.clearTimeout(timer)
      resolve(bridge)
    }
    const handleReady = () => finish(getWeixinJSBridge() ?? null)
    const timer = window.setTimeout(() => finish(getWeixinJSBridge() ?? null), timeoutMs)
    document.addEventListener('WeixinJSBridgeReady', handleReady, false)
    document.addEventListener('onWeixinJSBridgeReady', handleReady, false)
  })
}

async function invokeWechatJsapiPayment(payload: Record<string, unknown>): Promise<Record<string, unknown>> {
  const bridge = await waitForWeixinJSBridge()
  if (!bridge) {
    throw new Error('WECHAT_JSAPI_UNAVAILABLE')
  }
  return new Promise((resolve) => {
    bridge.invoke('getBrandWCPayRequest', payload, (result) => resolve(result || {}))
  })
}

export function usePurchaseFlow() {
  const i18n = useI18n()
  const { t } = i18n
  const route = useRoute()
  const router = useRouter()
  const authStore = useAuthStore()
  const paymentStore = usePaymentStore()
  const subscriptionStore = useSubscriptionStore()
  const appStore = useAppStore()

  const user = computed(() => authStore.user)
  const activeSubscriptions = computed(() => subscriptionStore.activeSubscriptions)

  function getDaysRemaining(expiresAt: string): number {
    const diff = new Date(expiresAt).getTime() - Date.now()
    return Math.max(0, Math.ceil(diff / (1000 * 60 * 60 * 24)))
  }

  function subscriptionHasPeakRate(sub: { group?: PeakRateFields | null }): boolean {
    return hasPeakRate(sub.group)
  }

  function subscriptionPeakRateLabel(sub: { group?: PeakRateFields | null }): string {
    return formatPeakRateWindow(sub.group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
  }

  const loading = ref(true)
  const submitting = ref(false)
  const errorMessage = ref('')
  const errorHintMessage = ref('')
  const activeTab = ref<'recharge' | 'subscription'>('recharge')
  const amount = ref<number | null>(null)
  const selectedMethod = ref('')
  const selectedPlan = ref<SubscriptionPlan | null>(null)
  const previewImage = ref('')

  const paymentPhase = ref<'select' | 'paying'>('select')
  const paymentState = ref<PaymentRecoverySnapshot>(emptyPaymentState())

  function persistRecoverySnapshot(snapshot: PaymentRecoverySnapshot) {
    if (typeof window === 'undefined' || !snapshot.orderId) return
    writePaymentRecoverySnapshot(window.localStorage, snapshot, PAYMENT_RECOVERY_STORAGE_KEY)
  }

  function removeRecoverySnapshot() {
    if (typeof window === 'undefined') return
    clearPaymentRecoverySnapshot(window.localStorage, PAYMENT_RECOVERY_STORAGE_KEY)
  }

  function resetPayment() {
    paymentPhase.value = 'select'
    paymentState.value = emptyPaymentState()
    removeRecoverySnapshot()
  }

  async function redirectToPaymentResult(state: PaymentRecoverySnapshot): Promise<void> {
    const query: Record<string, string | undefined> = {}
    if (state.orderId > 0) {
      query.order_id = String(state.orderId)
    }
    if (state.outTradeNo) {
      query.out_trade_no = state.outTradeNo
    }
    if (state.resumeToken) {
      query.resume_token = state.resumeToken
    }
    await router.push({
      path: '/payment/result',
      query,
    })
  }

  function buildWechatOAuthAuthorizeUrl(
    authorizeUrl: string,
    context: { paymentType: string; orderType: OrderType; planId?: number; orderAmount: number },
  ): string {
    const normalizedUrl = authorizeUrl.trim()
    if (!normalizedUrl || typeof window === 'undefined') {
      return normalizedUrl
    }

    try {
      const targetUrl = new URL(normalizedUrl, window.location.origin)
      const redirectPath = targetUrl.searchParams.get('redirect') || '/purchase'
      const redirectUrl = new URL(redirectPath, window.location.origin)
      const paymentType = normalizeVisibleMethod(context.paymentType) || context.paymentType.trim() || 'wxpay'

      redirectUrl.searchParams.set('payment_type', paymentType)
      redirectUrl.searchParams.set('order_type', context.orderType)

      if (context.planId) {
        redirectUrl.searchParams.set('plan_id', String(context.planId))
      } else {
        redirectUrl.searchParams.delete('plan_id')
      }

      if (context.orderAmount > 0) {
        redirectUrl.searchParams.set('amount', String(context.orderAmount))
      } else {
        redirectUrl.searchParams.delete('amount')
      }

      targetUrl.searchParams.set('redirect', `${redirectUrl.pathname}${redirectUrl.search}`)
      return targetUrl.toString()
    } catch {
      return normalizedUrl
    }
  }

  function onPaymentDone() {
    const wasSubscription = paymentState.value.orderType === 'subscription'
    resetPayment()
    selectedPlan.value = null
    if (wasSubscription) {
      subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
    }
  }

  async function onPaymentSuccess() {
    const completedPayment = { ...paymentState.value }
    removeRecoverySnapshot()
    authStore.refreshUser()
    if (paymentState.value.orderType === 'subscription') {
      subscriptionStore.fetchActiveSubscriptions(true).catch(() => {})
    }
    await redirectToPaymentResult(completedPayment)
  }

  function onPaymentSettled() {
    removeRecoverySnapshot()
  }

  const checkout = ref<CheckoutInfoResponse>({
    methods: {}, global_min: 0, global_max: 0,
    plans: [], balance_disabled: false, balance_recharge_multiplier: 1, subscription_usd_to_cny_rate: 0, recharge_fee_rate: 0, help_text: '', help_image_url: '', stripe_publishable_key: '',
  })

  const tabs = computed(() => {
    const result: { key: 'recharge' | 'subscription'; label: string }[] = []
    if (!checkout.value.balance_disabled) result.push({ key: 'recharge', label: t('payment.tabTopUp') })
    result.push({ key: 'subscription', label: t('payment.tabSubscribe') })
    return result
  })

  const visibleMethods = computed(() => getVisibleMethods(checkout.value.methods))
  const enabledMethods = computed(() => Object.keys(visibleMethods.value))
  const validAmount = computed(() => amount.value ?? 0)
  const balanceRechargeMultiplier = computed(() => {
    const multiplier = checkout.value.balance_recharge_multiplier
    return Number.isFinite(multiplier) && multiplier > 0 ? multiplier : 1
  })
  const subscriptionUsdToCnyRate = computed(() => {
    const rate = checkout.value.subscription_usd_to_cny_rate
    return Number.isFinite(rate) && rate > 0 ? rate : 0
  })
  const creditedAmount = computed(() => Math.round((validAmount.value * balanceRechargeMultiplier.value) * 100) / 100)

  const planGridClass = computed(() => {
    const n = checkout.value.plans.length
    if (n <= 2) return 'grid grid-cols-1 gap-5 sm:grid-cols-2'
    return 'grid grid-cols-1 gap-5 sm:grid-cols-2 lg:grid-cols-3'
  })

  function amountFitsMethod(amt: number, methodType: string): boolean {
    if (amt <= 0) return true
    const ml = visibleMethods.value[methodType]
    if (!ml) return false
    if (ml.single_min > 0 && amt < ml.single_min) return false
    if (ml.single_max > 0 && amt > ml.single_max) return false
    return true
  }

  const globalMinAmount = computed(() => {
    const limits = Object.values(visibleMethods.value)
    if (limits.length === 0) return 0
    if (limits.some(limit => limit.single_min <= 0)) return 0
    return Math.min(...limits.map(limit => limit.single_min))
  })
  const globalMaxAmount = computed(() => {
    const limits = Object.values(visibleMethods.value)
    if (limits.length === 0) return 0
    if (limits.some(limit => limit.single_max <= 0)) return 0
    return Math.max(...limits.map(limit => limit.single_max))
  })

  const selectedLimit = computed(() => visibleMethods.value[selectedMethod.value])
  const selectedCurrency = computed(() => normalizePaymentCurrency(selectedLimit.value?.currency))
  const localeCode = computed(() => {
    const raw = i18n.locale as unknown
    if (typeof raw === 'string') return raw
    if (raw && typeof raw === 'object' && 'value' in raw) {
      return String((raw as { value?: string }).value || '')
    }
    return undefined
  })

  function currencyFractionDigits(currency: string): number {
    try {
      return new Intl.NumberFormat(undefined, {
        style: 'currency',
        currency,
      }).resolvedOptions().maximumFractionDigits ?? 2
    } catch {
      return 2
    }
  }

  function roundPaymentAmount(value: number, currency: string): number {
    if (!Number.isFinite(value)) return 0
    const factor = 10 ** currencyFractionDigits(currency)
    return Math.round(value * factor) / factor
  }

  function ceilPaymentAmount(value: number, currency: string): number {
    if (!Number.isFinite(value)) return 0
    const factor = 10 ** currencyFractionDigits(currency)
    return Math.ceil(value * factor) / factor
  }

  function subscriptionPaymentAmountForCurrency(value: number, currency: string): number {
    const rate = subscriptionUsdToCnyRate.value
    if (rate <= 0 || currency !== DEFAULT_PAYMENT_CURRENCY) return roundPaymentAmount(value, currency)
    return roundPaymentAmount(value * rate, currency)
  }

  function formatSelectedPaymentAmount(value: number): string {
    return formatPaymentAmount(value, selectedCurrency.value, localeCode.value)
  }

  function formatSelectedSubscriptionPaymentAmount(value: number): string {
    return formatSelectedPaymentAmount(subscriptionPaymentAmountForCurrency(value, selectedCurrency.value))
  }

  const methodOptions = computed<PaymentMethodOption[]>(() =>
    enabledMethods.value.map((type) => {
      const ml = visibleMethods.value[type]
      return {
        type,
        display_name: ml?.display_name,
        fee_rate: ml?.fee_rate ?? 0,
        available: ml?.available !== false && amountFitsMethod(validAmount.value, type),
      }
    })
  )

  const feeRate = computed(() => checkout.value?.recharge_fee_rate ?? 0)
  const feeAmount = computed(() =>
    feeRate.value > 0 && validAmount.value > 0
      ? Math.ceil(((validAmount.value * feeRate.value) / 100) * 100) / 100
      : 0
  )
  const totalAmount = computed(() =>
    feeRate.value > 0 && validAmount.value > 0
      ? Math.round((validAmount.value + feeAmount.value) * 100) / 100
      : validAmount.value
  )

  const amountError = computed(() => {
    if (validAmount.value <= 0) return ''
    if (!enabledMethods.value.some((m) => amountFitsMethod(validAmount.value, m))) {
      return t('payment.amountNoMethod')
    }
    const ml = selectedLimit.value
    if (ml) {
      if (ml.single_min > 0 && validAmount.value < ml.single_min) return t('payment.amountTooLow', { min: formatSelectedPaymentAmount(ml.single_min) })
      if (ml.single_max > 0 && validAmount.value > ml.single_max) return t('payment.amountTooHigh', { max: formatSelectedPaymentAmount(ml.single_max) })
    }
    return ''
  })

  const canSubmit = computed(() =>
    validAmount.value > 0
      && amountFitsMethod(validAmount.value, selectedMethod.value)
      && selectedLimit.value?.available !== false
  )

  const subPaymentAmount = computed(() => {
    const price = selectedPlan.value?.price ?? 0
    return subscriptionPaymentAmountForCurrency(price, selectedCurrency.value)
  })

  const subFeeAmount = computed(() => {
    if (feeRate.value <= 0 || subPaymentAmount.value <= 0) return 0
    return ceilPaymentAmount((subPaymentAmount.value * feeRate.value) / 100, selectedCurrency.value)
  })

  const subTotalAmount = computed(() => {
    if (feeRate.value <= 0 || subPaymentAmount.value <= 0) return subPaymentAmount.value
    return roundPaymentAmount(subPaymentAmount.value + subFeeAmount.value, selectedCurrency.value)
  })

  function subscriptionTotalAmountForCurrency(value: number, currency: string): number {
    const paymentAmount = subscriptionPaymentAmountForCurrency(value, currency)
    if (feeRate.value <= 0 || paymentAmount <= 0) return paymentAmount
    const fee = ceilPaymentAmount((paymentAmount * feeRate.value) / 100, currency)
    return roundPaymentAmount(paymentAmount + fee, currency)
  }

  const subMethodOptions = computed<PaymentMethodOption[]>(() => {
    const price = selectedPlan.value?.price ?? 0
    return enabledMethods.value.map((type) => {
      const ml = visibleMethods.value[type]
      const currency = normalizePaymentCurrency(ml?.currency)
      return {
        type,
        display_name: ml?.display_name,
        fee_rate: ml?.fee_rate ?? 0,
        available: ml?.available !== false && amountFitsMethod(subscriptionTotalAmountForCurrency(price, currency), type),
      }
    })
  })

  const canSubmitSubscription = computed(() =>
    selectedPlan.value !== null
      && amountFitsMethod(subTotalAmount.value, selectedMethod.value)
      && selectedLimit.value?.available !== false
  )

  watch(() => [validAmount.value, selectedMethod.value] as const, ([amt, method]) => {
    if (amt <= 0 || amountFitsMethod(amt, method)) return
    const available = enabledMethods.value.find((m) => amountFitsMethod(amt, m))
    if (available) selectedMethod.value = available
  })

  const paymentButtonClass = computed(() => {
    const m = selectedMethod.value
    if (!m) return 'btn-primary'
    if (isBuiltInAlipayMethod(m)) return 'btn-alipay'
    if (isBuiltInWxpayMethod(m)) return 'btn-wxpay'
    if (m === 'stripe') return 'btn-stripe'
    if (m === 'airwallex') return 'btn-airwallex'
    return 'btn-primary'
  })

  const planBadgeClass = computed(() => platformBadgeClass(selectedPlan.value?.group_platform || ''))
  const planTextClass = computed(() => platformTextClass(selectedPlan.value?.group_platform || ''))

  const showRenewalModal = ref(false)
  const renewGroupId = ref<number | null>(null)
  const renewalPlans = computed(() => {
    if (renewGroupId.value == null) return []
    return checkout.value.plans.filter(p => p.group_id === renewGroupId.value)
  })

  const planValiditySuffix = computed(() => {
    if (!selectedPlan.value) return ''
    return validitySuffixOf(selectedPlan.value, t)
  })

  function planHasPeakRate(plan: SubscriptionPlan): boolean {
    return hasPeakRate(plan)
  }

  function planPeakRateLabel(plan: SubscriptionPlan): string {
    return formatPeakRateWindow(plan, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
  }

  function selectPlan(plan: SubscriptionPlan) {
    selectedPlan.value = plan
    errorMessage.value = ''
  }

  function selectPlanFromModal(plan: SubscriptionPlan) {
    showRenewalModal.value = false
    renewGroupId.value = null
    selectedPlan.value = plan
    errorMessage.value = ''
  }

  function closeRenewalModal() {
    showRenewalModal.value = false
    renewGroupId.value = null
  }

  async function handleSubmitRecharge() {
    if (!canSubmit.value || submitting.value) return
    await createOrder(validAmount.value, 'balance')
  }

  async function confirmSubscribe() {
    if (!selectedPlan.value || submitting.value) return
    await createOrder(selectedPlan.value.price, 'subscription', selectedPlan.value.id)
  }

  async function createOrder(orderAmount: number, orderType: OrderType, planId?: number, options: CreateOrderOptions = {}) {
    submitting.value = true
    errorMessage.value = ''
    errorHintMessage.value = ''
    const requestType = normalizeVisibleMethod(options.paymentType || selectedMethod.value) || options.paymentType || selectedMethod.value
    try {
      const payload = buildCreateOrderPayload({
        amount: orderAmount,
        paymentType: requestType,
        orderType,
        planId,
        origin: typeof window !== 'undefined' ? window.location.origin : '',
        isMobile: isMobileDevice(),
        isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
        forceQRCode: !!(checkout.value.alipay_force_qrcode && normalizeVisibleMethod(requestType) === 'alipay'),
        mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
      })
      if (options.openid) {
        payload.openid = options.openid
      }
      if (options.wechatResumeToken) {
        payload.wechat_resume_token = options.wechatResumeToken
      }

      const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
      const openWindow = (url: string) => {
        const win = window.open(url, 'paymentPopup', getPaymentPopupFeatures())
        if (!win || win.closed) {
          window.location.href = url
        }
      }
      const visibleMethod = normalizeVisibleMethod(requestType) || requestType
      const stripeMethod = visibleMethod === 'stripe'
        ? ''
        : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
      const stripeRouteUrl = result.client_secret && visibleMethod !== 'airwallex'
        ? router.resolve({
          path: '/payment/stripe',
          query: {
            order_id: String(result.order_id),
            client_secret: result.client_secret,
            method: stripeMethod || undefined,
            resume_token: result.resume_token || undefined,
          },
        }).href
        : ''
      const airwallexRouteUrl = result.client_secret && result.intent_id
        ? router.resolve({
          path: '/payment/airwallex',
          query: {
            order_id: String(result.order_id),
            out_trade_no: result.out_trade_no || undefined,
            resume_token: result.resume_token || undefined,
          },
        }).href
        : ''
      const decision = decidePaymentLaunch(result, {
        visibleMethod,
        orderType,
        isMobile: isMobileDevice(),
        isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
        forceQRCode: !!(checkout.value.alipay_force_qrcode && visibleMethod === 'alipay'),
        mobilePrecreateDeepLink: checkout.value.alipay_mobile_precreate_deep_link === true,
        stripePopupUrl: stripeRouteUrl,
        stripeRouteUrl,
        airwallexRouteUrl,
      })

      if (decision.kind === 'wechat_oauth' && decision.oauth?.authorize_url) {
        window.location.href = buildWechatOAuthAuthorizeUrl(decision.oauth.authorize_url, {
          paymentType: visibleMethod,
          orderType,
          planId,
          orderAmount,
        })
        return
      }

      if (decision.kind === 'unhandled') {
        applyScenarioError({ reason: 'UNHANDLED_PAYMENT_SCENARIO' }, visibleMethod)
        return
      }

      paymentState.value = decision.paymentState
      paymentPhase.value = 'paying'
      persistRecoverySnapshot(decision.recovery)

      if (decision.kind === 'stripe_popup') {
        openWindow(decision.paymentState.payUrl)
        return
      }
      if (decision.kind === 'stripe_route') {
        window.location.href = decision.paymentState.payUrl
        return
      }
      if (decision.kind === 'airwallex_route') {
        window.location.href = decision.paymentState.payUrl
        return
      }
      if (decision.kind === 'wechat_jsapi' && decision.jsapi) {
        try {
          const jsapiResult = await invokeWechatJsapiPayment(decision.jsapi as Record<string, unknown>)
          const errMsg = String(jsapiResult.err_msg || '').toLowerCase()
          if (errMsg.includes('cancel')) {
            appStore.showInfo(t('payment.qr.cancelled'))
            resetPayment()
          } else if (errMsg && !errMsg.includes('ok')) {
            resetPayment()
            const fallbackApplied = await attemptMobileQrFallback(
              { reason: 'WECHAT_JSAPI_FAILED', message: errMsg },
              {
                orderAmount,
                orderType,
                planId,
                paymentType: visibleMethod,
                attempted: options.mobileQrFallbackAttempted === true,
              },
            )
            if (!fallbackApplied) {
              applyScenarioError({ reason: 'WECHAT_JSAPI_FAILED', message: errMsg }, visibleMethod)
            }
          } else {
            const resultState = { ...decision.paymentState }
            resetPayment()
            await redirectToPaymentResult(resultState)
          }
        } catch (err: unknown) {
          resetPayment()
          const fallbackApplied = await attemptMobileQrFallback(err, {
            orderAmount,
            orderType,
            planId,
            paymentType: visibleMethod,
            attempted: options.mobileQrFallbackAttempted === true,
          })
          if (!fallbackApplied) {
            throw err
          }
        }
        return
      }
      if (decision.kind === 'redirect_waiting' && decision.paymentState.payUrl) {
        if (isMobileDevice()) {
          window.location.href = decision.paymentState.payUrl
          return
        }
        openWindow(decision.paymentState.payUrl)
      }
    } catch (err: unknown) {
      const apiErr = err as Record<string, unknown>
      if (apiErr.reason === 'TOO_MANY_PENDING') {
        const metadata = apiErr.metadata as Record<string, unknown> | undefined
        errorMessage.value = t('payment.errors.tooManyPending', { max: metadata?.max || '' })
        errorHintMessage.value = ''
      } else if (apiErr.reason === 'CANCEL_RATE_LIMITED') {
        errorMessage.value = t('payment.errors.cancelRateLimited')
        errorHintMessage.value = ''
      } else if (await attemptMobileQrFallback(err, {
        orderAmount,
        orderType,
        planId,
        paymentType: requestType,
        attempted: options.mobileQrFallbackAttempted === true,
      })) {
        return
      } else {
        const handled = applyScenarioError(
          err,
          normalizeVisibleMethod(options.paymentType || selectedMethod.value) || selectedMethod.value,
        )
        if (!handled) {
          errorMessage.value = extractI18nErrorMessage(err, t, 'payment.errors', extractApiErrorMessage(err, t('payment.result.failed')))
          errorHintMessage.value = ''
        }
        if (handled) {
          return
        }
      }
      appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
    } finally {
      submitting.value = false
    }
  }

  function shouldFallbackToDesktopQr(err: unknown, paymentMethod: string, attempted: boolean): boolean {
    if (attempted || !isMobileDevice()) {
      return false
    }

    const normalizedMethod = normalizeVisibleMethod(paymentMethod) || paymentMethod
    const reason = typeof err === 'object' && err && 'reason' in err && typeof err.reason === 'string'
      ? err.reason
      : ''
    const message = err instanceof Error
      ? err.message
      : (typeof err === 'object' && err && 'message' in err && typeof err.message === 'string'
        ? err.message
        : '')
    const normalizedMessage = message.toLowerCase()

    if (normalizedMethod === 'wxpay') {
      return reason === 'WECHAT_H5_NOT_AUTHORIZED'
        || reason === 'WECHAT_PAYMENT_MP_NOT_CONFIGURED'
        || reason === 'WECHAT_JSAPI_FAILED'
        || reason === 'PAYMENT_GATEWAY_ERROR'
        || reason === 'UNHANDLED_PAYMENT_SCENARIO'
        || normalizedMessage.includes('weixinjsbridge is unavailable')
        || normalizedMessage.includes('wechat_jsapi_unavailable')
    }

    if (normalizedMethod === 'alipay') {
      return reason === 'PAYMENT_GATEWAY_ERROR' || reason === 'UNHANDLED_PAYMENT_SCENARIO'
    }

    return false
  }

  async function attemptMobileQrFallback(err: unknown, context: MobileQrFallbackContext): Promise<boolean> {
    if (!shouldFallbackToDesktopQr(err, context.paymentType, context.attempted)) {
      return false
    }

    try {
      const visibleMethod = normalizeVisibleMethod(context.paymentType) || context.paymentType
      const payload = buildCreateOrderPayload({
        amount: context.orderAmount,
        paymentType: visibleMethod,
        orderType: context.orderType,
        planId: context.planId,
        origin: typeof window !== 'undefined' ? window.location.origin : '',
        isMobile: false,
        isWechatBrowser: false,
      })
      const result = await paymentStore.createOrder(payload) as CreateOrderResult & { resume_token?: string }
      const stripeMethod = visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
      const stripeRouteUrl = result.client_secret
        ? router.resolve({
          path: '/payment/stripe',
          query: {
            order_id: String(result.order_id),
            client_secret: result.client_secret,
            method: stripeMethod,
            resume_token: result.resume_token || undefined,
          },
        }).href
        : ''
      const decision = decidePaymentLaunch(result, {
        visibleMethod,
        orderType: context.orderType,
        isMobile: false,
        isWechatBrowser: false,
        stripePopupUrl: stripeRouteUrl,
        stripeRouteUrl,
      })

      if (decision.kind !== 'qr_waiting' || !decision.paymentState.qrCode) {
        return false
      }

      errorMessage.value = ''
      errorHintMessage.value = ''
      paymentState.value = decision.paymentState
      paymentPhase.value = 'paying'
      persistRecoverySnapshot(decision.recovery)
      appStore.showWarning(t('payment.errors.mobilePaymentFallbackToQr'))
      return true
    } catch {
      return false
    }
  }

  function applyScenarioError(err: unknown, paymentMethod: string): boolean {
    const descriptor = describePaymentScenarioError(err, {
      paymentMethod,
      isMobile: isMobileDevice(),
      isWechatBrowser: typeof window !== 'undefined' && /MicroMessenger/i.test(window.navigator.userAgent),
    })
    if (!descriptor) {
      errorMessage.value = ''
      errorHintMessage.value = ''
      return false
    }
    errorMessage.value = t(descriptor.messageKey)
    errorHintMessage.value = descriptor.hintKey ? t(descriptor.hintKey) : ''
    appStore.showError(buildPaymentErrorToastMessage(errorMessage.value, errorHintMessage.value))
    return true
  }

  async function resumeWechatPaymentFromQuery() {
    const resume = parseWechatResumeRoute(route.query, checkout.value.plans, validAmount.value)
    if (!resume) {
      return
    }

    selectedMethod.value = resume.paymentType
    if (resume.orderType === 'balance' && resume.orderAmount > 0) {
      amount.value = resume.orderAmount
    }
    if (resume.orderType === 'subscription' && resume.planId) {
      selectedPlan.value = checkout.value.plans.find(plan => plan.id === resume.planId) ?? null
    }

    await router.replace({ path: route.path, query: stripWechatResumeQuery(route.query) })

    if (resume.wechatResumeToken) {
      await createOrder(0, resume.orderType, resume.planId, {
        wechatResumeToken: resume.wechatResumeToken,
        paymentType: resume.paymentType,
        isResume: true,
      })
      return
    }

    if (resume.orderAmount > 0 && resume.openid) {
      await createOrder(resume.orderAmount, resume.orderType, resume.planId, {
        openid: resume.openid,
        paymentType: resume.paymentType,
        isResume: true,
      })
    }
  }

  onMounted(async () => {
    try {
      const res = await paymentAPI.getCheckoutInfo()
      checkout.value = res.data
      if (enabledMethods.value.length) {
        const order: readonly string[] = METHOD_ORDER
        const sorted = [...enabledMethods.value].sort((a, b) => {
          const ai = order.indexOf(a)
          const bi = order.indexOf(b)
          return (ai === -1 ? 999 : ai) - (bi === -1 ? 999 : bi)
        })
        selectedMethod.value = sorted[0]
      }
      if (typeof window !== 'undefined') {
        if (hasWechatResumeQuery(route.query)) {
          removeRecoverySnapshot()
        }
        const routeResumeToken = typeof route.query.resume_token === 'string'
          ? route.query.resume_token
          : typeof route.query.wechat_resume_token === 'string'
            ? route.query.wechat_resume_token
            : undefined
        const restored = readPaymentRecoverySnapshot(
          window.localStorage.getItem(PAYMENT_RECOVERY_STORAGE_KEY),
          { resumeToken: routeResumeToken },
        )
        if (restored) {
          paymentState.value = restored
          paymentPhase.value = 'paying'
          const restoredMethod = normalizeVisibleMethod(restored.paymentType)
            || (visibleMethods.value[restored.paymentType] ? restored.paymentType : '')
          if (restoredMethod) {
            selectedMethod.value = restoredMethod
          }
        } else {
          removeRecoverySnapshot()
        }
      }
      await resumeWechatPaymentFromQuery()
      if (checkout.value.balance_disabled) {
        activeTab.value = 'subscription'
      }
      if (route.query.tab === 'subscription') {
        activeTab.value = 'subscription'
        if (route.query.group) {
          const groupId = Number(route.query.group)
          const groupPlans = checkout.value.plans.filter(p => p.group_id === groupId)
          if (groupPlans.length === 1) {
            selectedPlan.value = groupPlans[0]
          } else if (groupPlans.length > 1) {
            renewGroupId.value = groupId
            showRenewalModal.value = true
          }
        }
      }
    } catch (err: unknown) { appStore.showError(extractI18nErrorMessage(err, t, 'payment.errors', t('common.error'))) }
    finally { loading.value = false }
    subscriptionStore.fetchActiveSubscriptions().catch(() => {})
  })

  return {
    t,
    user,
    activeSubscriptions,
    getDaysRemaining,
    subscriptionHasPeakRate,
    subscriptionPeakRateLabel,
    loading,
    submitting,
    activeTab,
    amount,
    selectedMethod,
    selectedPlan,
    previewImage,
    paymentPhase,
    paymentState,
    onPaymentDone,
    onPaymentSuccess,
    onPaymentSettled,
    checkout,
    tabs,
    enabledMethods,
    validAmount,
    balanceRechargeMultiplier,
    creditedAmount,
    planGridClass,
    globalMinAmount,
    globalMaxAmount,
    selectedCurrency,
    formatSelectedPaymentAmount,
    formatSelectedSubscriptionPaymentAmount,
    methodOptions,
    feeRate,
    feeAmount,
    totalAmount,
    amountError,
    canSubmit,
    subPaymentAmount,
    subFeeAmount,
    subTotalAmount,
    subMethodOptions,
    canSubmitSubscription,
    paymentButtonClass,
    planBadgeClass,
    planTextClass,
    showRenewalModal,
    renewalPlans,
    planValiditySuffix,
    planHasPeakRate,
    planPeakRateLabel,
    selectPlan,
    selectPlanFromModal,
    closeRenewalModal,
    handleSubmitRecharge,
    confirmSubscribe,
  }
}
