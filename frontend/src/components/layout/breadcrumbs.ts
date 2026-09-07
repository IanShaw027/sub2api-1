import type { RouteLocationNormalizedLoaded, Router } from 'vue-router'

export interface Breadcrumb {
  label: string
  path?: string
}

export function parentBreadcrumbs(router: Router, route: RouteLocationNormalizedLoaded, t: (key: string) => string): Breadcrumb[] {
  // Routes are mostly flat. Only link ancestors that actually exist in the router.
  const records = router.getRoutes().filter(record =>
    record.path !== '/' && record.path !== '/admin' && record.path !== route.path &&
    !record.path.includes(':') && route.path.startsWith(record.path + '/')
  ).sort((a, b) => a.path.length - b.path.length)
  const crumbs: Breadcrumb[] = []
  for (const record of [...records, ...route.matched.slice(0, -1)]) {
    const label = record.meta.titleKey ? t(record.meta.titleKey as string) : record.meta.title as string | undefined
    if (!label || record.path === '/' || record.path === '/admin') continue
    const path = record.name
      ? router.resolve({ name: record.name, params: route.params }).path
      : record.path.includes(':') ? undefined : record.path
    if (path === route.path || crumbs.some(crumb => crumb.path === path && crumb.label === label)) continue
    crumbs.push({ label, path })
  }
  const parentKey = route.meta.parentTitleKey as string | undefined
  if (parentKey) {
    const label = t(parentKey)
    const parentPath = route.meta.parentPath as string | undefined
    const path = parentPath && router.getRoutes().some(record => record.path === parentPath) ? parentPath : undefined
    if (!crumbs.some(crumb => crumb.label === label)) crumbs.push({ label, path })
  }
  return crumbs
}
