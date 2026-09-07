import { describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import { parentBreadcrumbs } from '../breadcrumbs'

const component = { template: '<div />' }
const t = (key: string) => key

describe('parent breadcrumbs', () => {
  it('links registered flat-route parents in hierarchy order', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/admin/orders', component, meta: { titleKey: 'orders' } },
      { path: '/admin/orders/invoices', component, meta: { titleKey: 'invoices' } },
      { path: '/admin/orders/invoices/:id', component, meta: { titleKey: 'detail' } }
    ] })
    await router.push('/admin/orders/invoices/12')
    expect(parentBreadcrumbs(router, router.currentRoute.value, t)).toEqual([
      { label: 'orders', path: '/admin/orders' },
      { label: 'invoices', path: '/admin/orders/invoices' }
    ])
  })

  it('resolves dynamic matched parents and does not repeat them', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/teams/:id', name: 'team', component, meta: { titleKey: 'team' }, children: [
        { path: 'settings', component, meta: { titleKey: 'settings' } }
      ] }
    ] })
    await router.push('/teams/12/settings')
    expect(parentBreadcrumbs(router, router.currentRoute.value, t)).toEqual([{ label: 'team', path: '/teams/12' }])
  })

  it('keeps a non-routed group as text instead of inventing a link', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/admin/settings', component, meta: { titleKey: 'settings', parentTitleKey: 'system', parentPath: '/missing' } }
    ] })
    await router.push('/admin/settings')
    expect(parentBreadcrumbs(router, router.currentRoute.value, t)).toEqual([{ label: 'system', path: undefined }])
  })

  it('links an explicit parent with a real redirect route', async () => {
    const router = createRouter({ history: createMemoryHistory(), routes: [
      { path: '/studio', redirect: '/studio/chat' },
      { path: '/studio/chat', component, meta: { parentTitleKey: 'studio', parentPath: '/studio' } }
    ] })
    await router.push('/studio/chat')
    expect(parentBreadcrumbs(router, router.currentRoute.value, t)).toEqual([{ label: 'studio', path: '/studio' }])
  })
})
