import type {
  CreateOrderRequest,
  CreateOrderResult,
  MethodLimit,
  OrderType,
  WechatJSAPIPayload,
  WechatOAuthInfo,
} from '@/types/payment'

export const PAYMENT_RECOVERY_STORAGE_KEY = 'payment.recovery.current'
export const PAYMENT_SESSION_RECOVERY_STORAGE_KEY = 'payment.recovery.session.current'

const VISIBLE_METHOD_ALIASES = {
  alipay: 'alipay',
  alipay_direct: 'alipay',
  wxpay: 'wxpay',
  wxpay_direct: 'wxpay',
  stripe: 'stripe',
  airwallex: 'airwallex',
} as const

export type VisiblePaymentMethod = 'alipay' | 'wxpay' | 'stripe' | 'airwallex'
export type StripeVisibleMethod = 'alipay' | 'wechat_pay'
export type PaymentLaunchKind =
  | 'qr_waiting'
  | 'redirect_waiting'
  | 'stripe_popup'
  | 'stripe_route'
  | 'airwallex_route'
  | 'wechat_oauth'
  | 'wechat_jsapi'
  | 'unhandled'

export interface PaymentRecoverySnapshot {
  orderId: number
  amount: number
  qrCode: string
  expiresAt: string
  paymentType: string
  payUrl: string
  outTradeNo: string
  clientSecret: string
  intentId: string
  currency: string
  countryCode: string
  paymentEnv: string
  payAmount: number
  orderType: OrderType | ''
  paymentMode: string
  resumeToken: string
  launchKind?: PaymentLaunchKind
  redirected?: boolean
  createdAt: number
}

export interface PaymentLaunchContext {
  visibleMethod: string
  orderType: OrderType
  isMobile: boolean
  isWechatBrowser?: boolean
  /** When true, Alipay payments always use QR code regardless of device type */
  forceQRCode?: boolean
  now?: number
  stripePopupUrl?: string
  stripeRouteUrl?: string
  airwallexRouteUrl?: string
}

export interface PaymentLaunchDecision {
  kind: PaymentLaunchKind
  paymentState: PaymentRecoverySnapshot
  recovery: PaymentRecoverySnapshot
  stripeMethod?: StripeVisibleMethod
  oauth?: WechatOAuthInfo
  jsapi?: WechatJSAPIPayload
}

export interface BuildCreateOrderPayloadInput {
  amount: number
  paymentType: string
  orderType: OrderType
  planId?: number
  origin?: string
  isMobile: boolean
  isWechatBrowser: boolean
  /** When true, Alipay payments always use QR code (passes is_mobile: false to backend) */
  forceQRCode?: boolean
}

type CreateOrderFlowResult = CreateOrderResult & {
  resume_token?: string
}

type StorageWriter = Pick<Storage, 'removeItem' | 'setItem'>

interface PaymentSessionRecoveryOptions {
  orderId: number
  resumeToken?: string
  outTradeNo?: string
  now?: number
}

export function normalizeVisibleMethod(method: string): VisiblePaymentMethod | '' {
  const normalized = VISIBLE_METHOD_ALIASES[method.trim() as keyof typeof VISIBLE_METHOD_ALIASES]
  return normalized ?? ''
}

/**
 * Payment launch URLs may be absolute http(s) URLs or root-relative routes on
 * the current origin. Reject javascript:/data:/, protocol-relative URLs and
 * backslash-based browser parsing ambiguities before window.open/location.href.
 */
export function assertPaymentLaunchUrl(url: string): string {
  const trimmed = (url || '').trim()
  if (!trimmed) return ''
  if (Array.from(trimmed).some((character) => {
    const codePoint = character.codePointAt(0) ?? 0
    return codePoint < 32 || codePoint === 127
  })) return ''

  if (trimmed.startsWith('/')) {
    if (trimmed.startsWith('//') || trimmed.startsWith('/\\') || trimmed.includes('\\')) return ''
    try {
      const baseOrigin = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
      const parsed = new URL(trimmed, baseOrigin)
      if (parsed.origin !== baseOrigin) return ''
      return `${parsed.pathname}${parsed.search}${parsed.hash}`
    } catch {
      return ''
    }
  }

  if (!/^https?:\/\//i.test(trimmed)) return ''
  try {
    const parsed = new URL(trimmed)
    const protocol = parsed.protocol.toLowerCase()
    if (protocol !== 'http:' && protocol !== 'https:') return ''
    return parsed.toString()
  } catch {
    return ''
  }
}

export function getVisibleMethods(methods: Record<string, MethodLimit>): Record<string, MethodLimit> {
  const visible: Record<string, MethodLimit> = {}

  Object.entries(methods).forEach(([type, limit]) => {
    const normalized = normalizeVisibleMethod(type) || type.trim()
    if (!normalized) return

    const isCanonical = type === normalized
    const existing = visible[normalized]
    if (!existing || isCanonical) {
      visible[normalized] = { ...limit }
    }
  })

  return visible
}

export function buildCreateOrderPayload(input: BuildCreateOrderPayloadInput): CreateOrderRequest {
  const visibleMethod = normalizeVisibleMethod(input.paymentType) || input.paymentType.trim()
  const normalizedOrigin = (input.origin || '').trim().replace(/\/+$/, '')
  // When forceQRCode is enabled for alipay, always tell the backend this is not a mobile
  // request so it generates a QR code instead of a mobile-redirect URL.
  const effectiveMobile = (input.forceQRCode && visibleMethod === 'alipay')
    ? false
    : input.isMobile
  const payload: CreateOrderRequest = {
    amount: input.amount,
    payment_type: visibleMethod,
    order_type: input.orderType,
    is_mobile: effectiveMobile,
    payment_source: visibleMethod === 'wxpay' && input.isWechatBrowser
      ? 'wechat_in_app_resume'
      : 'hosted_redirect',
  }

  if (input.planId) {
    payload.plan_id = input.planId
  }
  if (normalizedOrigin) {
    payload.return_url = `${normalizedOrigin}/payment/result`
  }

  return payload
}

export function decidePaymentLaunch(
  result: CreateOrderFlowResult,
  context: PaymentLaunchContext,
): PaymentLaunchDecision {
  const visibleMethod = normalizeVisibleMethod(context.visibleMethod) || context.visibleMethod
  const baseState = createPaymentRecoverySnapshot({
    orderId: result.order_id,
    amount: result.amount,
    qrCode: result.qr_code || '',
    expiresAt: result.expires_at || '',
    paymentType: visibleMethod,
    payUrl: assertPaymentLaunchUrl(result.pay_url || ''),
    outTradeNo: result.out_trade_no || '',
    clientSecret: result.client_secret || '',
    intentId: result.intent_id || '',
    currency: result.currency || '',
    countryCode: result.country_code || '',
    paymentEnv: result.payment_env || '',
    payAmount: result.pay_amount,
    orderType: context.orderType,
    paymentMode: (result.payment_mode || '').trim(),
    resumeToken: result.resume_token || '',
  }, context.now)

  if (visibleMethod === 'airwallex' && baseState.clientSecret && baseState.intentId) {
    if (!context.airwallexRouteUrl) {
      return { kind: 'unhandled', paymentState: baseState, recovery: baseState }
    }
    const paymentState = {
      ...baseState,
      payUrl: context.airwallexRouteUrl || '',
      launchKind: 'airwallex_route' as PaymentLaunchKind,
    }
    return { kind: 'airwallex_route', paymentState, recovery: paymentState }
  }

  if (baseState.clientSecret) {
    // visibleMethod === 'stripe' means the user clicked the dedicated Stripe button
    // and should land on the full Payment Element to choose a sub-method themselves.
    const isStripeButton = visibleMethod === 'stripe'
    const stripeMethod: StripeVisibleMethod | undefined = isStripeButton
      ? undefined
      : visibleMethod === 'wxpay' ? 'wechat_pay' : 'alipay'
    const kind: PaymentLaunchKind = stripeMethod === 'alipay' && !context.isMobile
      ? 'stripe_popup'
      : 'stripe_route'
    const payUrl = kind === 'stripe_popup'
      ? context.stripePopupUrl || context.stripeRouteUrl || ''
      : context.stripeRouteUrl || context.stripePopupUrl || ''
    const paymentState = { ...baseState, payUrl, launchKind: kind }
    return { kind, paymentState, recovery: paymentState, stripeMethod }
  }

  if (result.result_type === 'oauth_required' && result.oauth?.authorize_url) {
    const paymentState = { ...baseState, launchKind: 'wechat_oauth' as PaymentLaunchKind }
    return { kind: 'wechat_oauth', paymentState, recovery: paymentState, oauth: result.oauth }
  }

  const jsapiPayload = result.jsapi ?? result.jsapi_payload
  if (result.result_type === 'jsapi_ready' && jsapiPayload) {
    const paymentState = { ...baseState, launchKind: 'wechat_jsapi' as PaymentLaunchKind }
    return { kind: 'wechat_jsapi', paymentState, recovery: paymentState, jsapi: jsapiPayload }
  }

  const normalizedPaymentMode = baseState.paymentMode.trim().toLowerCase()
  // When forceQRCode is on for alipay, treat the device as desktop so the mobile-redirect
  // branch is bypassed and we fall through to qr_waiting.
  const effectiveMobile = (context.forceQRCode && visibleMethod === 'alipay')
    ? false
    : context.isMobile
  const prefersRedirect = normalizedPaymentMode === 'redirect'
    || normalizedPaymentMode === 'popup'
    || (effectiveMobile && !!baseState.payUrl)
  const prefersQr = normalizedPaymentMode === 'qrcode'
    || normalizedPaymentMode === 'native'
    || (!prefersRedirect && !!baseState.qrCode)

  if (visibleMethod === 'wxpay' && context.isWechatBrowser && baseState.payUrl && !baseState.qrCode) {
    const paymentState = { ...baseState, launchKind: 'redirect_waiting' as PaymentLaunchKind }
    return { kind: 'redirect_waiting', paymentState, recovery: paymentState }
  }

  if (prefersRedirect && baseState.payUrl) {
    const paymentState = { ...baseState, launchKind: 'redirect_waiting' as PaymentLaunchKind }
    return { kind: 'redirect_waiting', paymentState, recovery: paymentState }
  }

  if (prefersQr && baseState.qrCode) {
    const paymentState = { ...baseState, launchKind: 'qr_waiting' as PaymentLaunchKind }
    return { kind: 'qr_waiting', paymentState, recovery: paymentState }
  }

  if (baseState.payUrl) {
    const paymentState = { ...baseState, launchKind: 'redirect_waiting' as PaymentLaunchKind }
    return { kind: 'redirect_waiting', paymentState, recovery: paymentState }
  }

  return { kind: 'unhandled', paymentState: baseState, recovery: baseState }
}

export function createPaymentRecoverySnapshot(
  state: Omit<PaymentRecoverySnapshot, 'createdAt'>,
  now = Date.now(),
): PaymentRecoverySnapshot {
  return {
    ...state,
    createdAt: now,
  }
}

export function writePaymentRecoverySnapshot(
  storage: StorageWriter,
  snapshot: PaymentRecoverySnapshot,
  key = PAYMENT_RECOVERY_STORAGE_KEY,
): void {
  // Never persist client secrets in localStorage (XSS / shared-device risk).
  // Stripe/Airwallex secrets stay in memory or sessionStorage via caller if needed.
  const safe: PaymentRecoverySnapshot = {
    ...snapshot,
    payUrl: assertPaymentLaunchUrl(snapshot.payUrl),
    clientSecret: '',
  }
  storage.setItem(key, JSON.stringify(safe))
}

export function writePaymentSessionRecoverySnapshot(
  storage: Pick<Storage, 'setItem'>,
  snapshot: PaymentRecoverySnapshot,
  key = PAYMENT_SESSION_RECOVERY_STORAGE_KEY,
): void {
  const sessionSnapshot: PaymentRecoverySnapshot = {
    ...snapshot,
    payUrl: assertPaymentLaunchUrl(snapshot.payUrl),
  }
  storage.setItem(key, JSON.stringify(sessionSnapshot))
}

export function clearPaymentRecoverySnapshot(
  storage: Pick<Storage, 'removeItem'>,
  key = PAYMENT_RECOVERY_STORAGE_KEY,
): void {
  storage.removeItem(key)
}

export function readPaymentSessionRecoverySnapshot(
  raw: string | null | undefined,
  options: PaymentSessionRecoveryOptions,
): PaymentRecoverySnapshot | null {
  const parsed = parsePaymentRecoverySnapshot(raw, options.now ?? Date.now(), true)
  if (!parsed || parsed.orderId !== options.orderId) {
    return null
  }
  if (options.resumeToken && parsed.resumeToken !== options.resumeToken) {
    return null
  }
  if (options.outTradeNo && parsed.outTradeNo !== options.outTradeNo) {
    return null
  }
  return parsed
}

export function readPaymentRecoverySnapshot(
  raw: string | null | undefined,
  options: { now?: number; resumeToken?: string } = {},
): PaymentRecoverySnapshot | null {
  const parsed = parsePaymentRecoverySnapshot(raw, options.now ?? Date.now(), false)
  if (!parsed) {
    return null
  }
  if (options.resumeToken && parsed.resumeToken !== options.resumeToken) {
    return null
  }
  return parsed
}

function parsePaymentRecoverySnapshot(
  raw: string | null | undefined,
  now: number,
  preserveClientSecret: boolean,
): PaymentRecoverySnapshot | null {
  if (!raw) return null

  try {
    const parsed = JSON.parse(raw) as Partial<PaymentRecoverySnapshot>
    if (
      typeof parsed.orderId !== 'number'
      || typeof parsed.amount !== 'number'
      || typeof parsed.qrCode !== 'string'
      || typeof parsed.expiresAt !== 'string'
      || typeof parsed.paymentType !== 'string'
      || typeof parsed.payUrl !== 'string'
      || (parsed.outTradeNo != null && typeof parsed.outTradeNo !== 'string')
      || typeof parsed.clientSecret !== 'string'
      || (parsed.intentId != null && typeof parsed.intentId !== 'string')
      || (parsed.currency != null && typeof parsed.currency !== 'string')
      || (parsed.countryCode != null && typeof parsed.countryCode !== 'string')
      || (parsed.paymentEnv != null && typeof parsed.paymentEnv !== 'string')
      || typeof parsed.payAmount !== 'number'
      || typeof parsed.paymentMode !== 'string'
      || typeof parsed.resumeToken !== 'string'
      || typeof parsed.createdAt !== 'number'
    ) {
      return null
    }

    const expiresAt = Date.parse(parsed.expiresAt)
    if (Number.isFinite(expiresAt) && expiresAt <= now) {
      return null
    }

    return {
      orderId: parsed.orderId,
      amount: parsed.amount,
      qrCode: parsed.qrCode,
      expiresAt: parsed.expiresAt,
      paymentType: parsed.paymentType,
      payUrl: assertPaymentLaunchUrl(parsed.payUrl),
      outTradeNo: parsed.outTradeNo || '',
      // Long-lived recovery snapshots never expose historically stored secrets.
      clientSecret: preserveClientSecret ? parsed.clientSecret : '',
      intentId: parsed.intentId || '',
      currency: parsed.currency || '',
      countryCode: parsed.countryCode || '',
      paymentEnv: parsed.paymentEnv || '',
      payAmount: parsed.payAmount,
      orderType: parsed.orderType === 'subscription' ? 'subscription' : 'balance',
      paymentMode: parsed.paymentMode,
      resumeToken: parsed.resumeToken,
      launchKind: isPaymentLaunchKind(parsed.launchKind) ? parsed.launchKind : undefined,
      redirected: parsed.redirected === true,
      createdAt: parsed.createdAt,
    }
  } catch {
    return null
  }
}

function isPaymentLaunchKind(value: unknown): value is PaymentLaunchKind {
  return value === 'qr_waiting'
    || value === 'redirect_waiting'
    || value === 'stripe_popup'
    || value === 'stripe_route'
    || value === 'airwallex_route'
    || value === 'wechat_oauth'
    || value === 'wechat_jsapi'
    || value === 'unhandled'
}
