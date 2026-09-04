/**
 * Real-world payment provider brand colours (Alipay blue, WeChat Pay green,
 * Stripe purple/indigo gradient).
 *
 * These are literal hex values on purpose: a design token cannot represent a
 * fixed third-party brand identity, so they live in this dedicated file
 * instead of being inlined in consumer components. `scripts/ui-lint.mjs`
 * whitelists `components/payment/*Brand*` for exactly this reason — keep the
 * literal colours contained here so consumer files stay token-only.
 */
export const ALIPAY_BRAND_COLOR = '#00AEEF'
export const WECHAT_BRAND_COLOR = '#2BB741'
export const STRIPE_BRAND_FROM = '#635bff'
export const STRIPE_BRAND_TO = '#4f46e5'
export const STRIPE_BRAND_GRADIENT = `linear-gradient(to bottom right, ${STRIPE_BRAND_FROM}, ${STRIPE_BRAND_TO})`
