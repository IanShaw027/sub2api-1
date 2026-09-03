export interface NavItem {
  path: string
  label: string
  icon: unknown
  iconSvg?: string
  hideInSimpleMode?: boolean
  badge?: number
  children?: NavItem[]
  /**
   * When true, the parent item only toggles the expand/collapse state and
   * does NOT navigate to its `path`. The `path` is purely a stable key.
   */
  expandOnly?: boolean
  /**
   * Optional feature-flag getter. `false` hides the item; undefined/true shows it.
   */
  featureFlag?: () => boolean | undefined
  section?: string
}

export interface NavSection {
  key: string
  titleKey: string
  items: NavItem[]
}

export const ADMIN_SECTION_ORDER = [
  'overview',
  'usersResources',
  'channels',
  'operations',
  'securityAudit',
  'system'
] as const

export const USER_SECTION_ORDER = ['workspace', 'billing', 'support'] as const

export function adminSectionKey(item: NavItem): string {
  const path = item.path
  if (path === '/admin/dashboard' || path === '/admin/ops') return 'overview'
  if (
    path === '/admin/users' ||
    path === '/admin/groups' ||
    path === '/admin/subscriptions' ||
    path === '/admin/accounts' ||
    path === '/admin/proxies'
  ) {
    return 'usersResources'
  }
  if (path === '/admin/channels' || path.startsWith('/admin/channels/') || path === '/admin/plugins') {
    return 'channels'
  }
  if (
    path === '/admin/announcements' ||
    path === '/admin/redeem' ||
    path === '/admin/promo-codes' ||
    path === '/admin/affiliates' ||
    path === '/admin/orders' ||
    path === '/admin/tickets'
  ) {
    return 'operations'
  }
  if (
    path === '/admin/security-audit' ||
    path === '/admin/risk-control' ||
    path === '/admin/prompt-audit' ||
    path === '/admin/usage' ||
    path === '/admin/audit-logs'
  ) {
    return 'securityAudit'
  }
  return 'system'
}

export function userSectionKey(item: NavItem): string {
  const path = item.path
  if (
    path === '/dashboard' ||
    path === '/keys' ||
    path === '/studio' ||
    path === '/batch-image' ||
    path === '/usage' ||
    path === '/available-channels' ||
    path === '/monitor'
  ) {
    return 'workspace'
  }
  if (
    path === '/subscriptions' ||
    path === '/purchase' ||
    path === '/orders' ||
    path === '/invoices' ||
    path === '/redeem'
  ) {
    return 'billing'
  }
  return 'support'
}

function groupBySection(items: NavItem[], order: readonly string[], keyOf: (item: NavItem) => string): NavSection[] {
  const buckets = new Map<string, NavItem[]>()
  for (const key of order) {
    buckets.set(key, [])
  }
  for (const item of items) {
    const key = item.section || keyOf(item)
    if (!buckets.has(key)) {
      buckets.set(key, [])
    }
    buckets.get(key)!.push(item)
  }
  return order
    .map((key) => ({
      key,
      titleKey: `nav.section.${key}`,
      items: buckets.get(key) || []
    }))
    .filter((section) => section.items.length > 0)
}

export function groupAdminNav(adminItems: NavItem[], personalItems: NavItem[]): NavSection[] {
  const sections = groupBySection(adminItems, ADMIN_SECTION_ORDER, adminSectionKey)
  if (personalItems.length > 0) {
    sections.push({
      key: 'myAccount',
      titleKey: 'nav.section.myAccount',
      items: personalItems
    })
  }
  return sections
}

export function groupUserNav(userItems: NavItem[]): NavSection[] {
  return groupBySection(userItems, USER_SECTION_ORDER, userSectionKey)
}

export function itemDomId(path: string): string | undefined {
  if (path === '/admin/accounts') return 'sidebar-channel-manage'
  if (path === '/admin/groups') return 'sidebar-group-manage'
  if (path === '/admin/redeem') return 'sidebar-wallet'
  return undefined
}

export function itemTourAttr(path: string): string | undefined {
  return path === '/keys' ? 'sidebar-my-keys' : undefined
}

export { sectionKeyForSelector } from '@/constants/sidebar'

export function sectionContainsPath(section: NavSection, routePath: string): boolean {
  return section.items.some((item) => itemMatchesPath(item, routePath))
}

export function itemMatchesPath(item: NavItem, routePath: string): boolean {
  if (routePath === item.path || routePath.startsWith(item.path + '/')) return true
  return Boolean(item.children?.some((child) => routePath === child.path || routePath.startsWith(child.path + '/')))
}
