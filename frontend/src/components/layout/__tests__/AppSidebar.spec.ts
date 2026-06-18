import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar skill navigation', () => {
  it('guards user skill entries behind the ai studio feature flag', () => {
    expect(componentSource).toContain(
      "{ path: '/skills', label: t('nav.skillsCenter', '技能中心'), icon: SkillCenterIcon, featureFlag: flagAiStudio }"
    )
    expect(componentSource).toContain(
      "{ path: '/skills/installed', label: t('nav.installedSkills', '已安装技能'), icon: SkillCenterIcon, featureFlag: flagAiStudio }"
    )
  })

  it('guards the admin skill governance group behind the ai studio feature flag', () => {
    expect(componentSource).toContain("path: '/admin/skills'")
    expect(componentSource).toContain("t('nav.skillGovernance', '技能治理')")
    expect(componentSource).toContain('featureFlag: flagAiStudio')
    expect(componentSource).toContain("path: '/admin/skills/review'")
    expect(componentSource).toContain("path: '/admin/skills/governance'")
    expect(componentSource).toContain("path: '/admin/skills/runtime'")
    expect(componentSource).toContain("path: '/admin/skills/settlements'")
  })
})

describe('AppSidebar affiliate admin navigation', () => {
  it('keeps a single admin affiliates root entry and exposes the records as its children', () => {
    const affiliateRootEntries = componentSource.match(/path: '\/admin\/affiliates'/g) ?? []

    expect(affiliateRootEntries).toHaveLength(1)
    expect(componentSource).toContain("label: t('nav.affiliateManagement')")
    expect(componentSource).toContain("path: '/admin/affiliates/invites'")
    expect(componentSource).toContain("path: '/admin/affiliates/rebates'")
    expect(componentSource).toContain("path: '/admin/affiliates/transfers'")
  })
})

describe('AppSidebar custom menu dedupe', () => {
  it('does not dedupe custom items by label text', () => {
    expect(componentSource).toContain('const seenPaths = new Set<string>()')
    expect(componentSource).not.toContain('const seenLabels = new Set<string>()')
    expect(componentSource).not.toContain('seenLabels.has')
    expect(componentSource).not.toContain('seenLabels.add')
  })
})

describe('AppSidebar simple mode navigation', () => {
  it('uses the shared simple mode route restriction helper', () => {
    expect(componentSource).toContain("import { isSimpleModeRouteRestricted } from '@/navigation/simpleMode'")
    expect(componentSource).toContain('!isSimpleModeRouteRestricted(item.path)')
  })
})
