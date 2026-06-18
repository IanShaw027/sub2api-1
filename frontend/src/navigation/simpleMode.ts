const SIMPLE_MODE_RESTRICTED_PREFIXES = [
  '/usage',
  '/tickets',
  '/available-channels',
  '/subscriptions',
  '/purchase',
  '/orders',
  '/redeem',
  '/affiliate',
  '/admin/users',
  '/admin/groups',
  '/admin/channels',
  '/admin/subscriptions',
  '/admin/ai',
  '/admin/skills',
  '/admin/risk-control',
  '/admin/redeem',
  '/admin/promo-codes',
  '/admin/affiliates',
  '/admin/orders',
] as const

function normalizePath(path: string): string {
  const clean = (path || '').split(/[?#]/, 1)[0] || '/'
  return clean.length > 1 && clean.endsWith('/') ? clean.slice(0, -1) : clean
}

export function isSimpleModeRouteRestricted(path: string): boolean {
  const cleanPath = normalizePath(path)
  return SIMPLE_MODE_RESTRICTED_PREFIXES.some((prefix) => (
    cleanPath === prefix || cleanPath.startsWith(`${prefix}/`)
  ))
}
