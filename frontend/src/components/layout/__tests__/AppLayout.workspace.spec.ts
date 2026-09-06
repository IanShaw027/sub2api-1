import { mount } from '@vue/test-utils'
import { createPinia } from 'pinia'
import { describe, expect, it, vi } from 'vitest'
import AppLayout from '../AppLayout.vue'

vi.mock('@/composables/useOnboardingTour', () => ({ useOnboardingTour: () => ({ replayTour: vi.fn() }) }))
vi.mock('../AppSidebar.vue', () => ({ default: { template: '<aside data-shared-sidebar />' } }))
vi.mock('../AppHeader.vue', () => ({ default: { template: '<header class="app-header" />' } }))

describe('shared layout workspace sizing', () => {
  it('keeps normal modules in the document flow by default', () => {
    const page = mount(AppLayout, { global: { plugins: [createPinia()] }, slots: { default: '<div>Content</div>' } })
    expect(page.get('.app-shell-main').classes()).not.toContain('is-workspace')
    expect(page.get('.app-shell-content').text()).toBe('Content')
    page.unmount()
  })

  it('fits a workspace below the shared header without creating a second shell', async () => {
    const page = mount(AppLayout, { props: { fillHeight: true }, global: { plugins: [createPinia()] } })
    expect(page.get('.app-shell-main').classes()).toContain('is-workspace')
    expect(page.findAll('.app-header')).toHaveLength(1)
    expect(page.findAll('[data-shared-sidebar]')).toHaveLength(1)
    await page.setProps({ fillHeight: false })
    expect(page.get('.app-shell-main').classes()).not.toContain('is-workspace')
    page.unmount()
  })
})
