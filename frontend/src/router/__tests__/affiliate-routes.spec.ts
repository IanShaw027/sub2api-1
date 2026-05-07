import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const routerPath = resolve(dirname(fileURLToPath(import.meta.url)), '../index.ts')
const routerSource = readFileSync(routerPath, 'utf8')

describe('affiliate admin routes', () => {
  it('registers a single /admin/affiliates root and uses it as the records entry point', () => {
    const affiliateRootRoutes = routerSource.match(/path: '\/admin\/affiliates'/g) ?? []

    expect(affiliateRootRoutes).toHaveLength(1)
    expect(routerSource).toContain("redirect: '/admin/affiliates/invites'")
    expect(routerSource).not.toContain("name: 'AdminAffiliates'")
    expect(routerSource).not.toContain("component: () => import('@/views/admin/AffiliatesView.vue')")
  })
})
