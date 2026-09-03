import { nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { createRouter, createMemoryHistory } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import SidebarItem from '../sidebar/SidebarItem.vue'
import { FolderIcon } from '@/components/icons'

const originalMatchMedia = window.matchMedia

function mockTabletViewport() {
  Object.defineProperty(window, 'matchMedia', {
    writable: true,
    value: vi.fn().mockImplementation((query: string) => ({
      matches: query.includes('1024px') ? false : query.includes('768px'),
      media: query,
      onchange: null,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      addListener: vi.fn(),
      removeListener: vi.fn(),
      dispatchEvent: vi.fn()
    }))
  })
}

describe('SidebarItem tablet group flyout', () => {
  beforeEach(() => {
    mockTabletViewport()
  })

  afterEach(() => {
    window.matchMedia = originalMatchMedia
    document.body.querySelectorAll('.sidebar-group-flyout').forEach((node) => node.remove())
  })

  it('opens a flyout for nested items on collapsed tablet rail', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/admin/channels/pricing', component: { template: '<div />' } }]
    })
    await router.push('/admin/channels/pricing')

    const wrapper = mount(SidebarItem, {
      attachTo: document.body,
      props: {
        item: {
          path: '/admin/channels',
          label: 'Channels',
          icon: FolderIcon,
          expandOnly: true,
          children: [
            { path: '/admin/channels/pricing', label: 'Pricing', icon: FolderIcon }
          ]
        },
        collapsed: true,
        isActive: false,
        isGroupActive: true,
        isExpanded: false,
        badgeCount: 0,
        routePath: '/admin/channels/pricing',
        omitTourAnchors: false
      },
      global: {
        plugins: [router]
      }
    })

    expect(document.body.querySelector('.sidebar-group-flyout')).toBeNull()
    await wrapper.find('button.sidebar-item').trigger('click')
    await nextTick()
    const flyout = document.body.querySelector('.sidebar-group-flyout') as HTMLElement | null
    expect(flyout).not.toBeNull()
    expect(flyout?.parentElement).toBe(document.body)
    expect(wrapper.find('.sidebar-group-root').element.contains(flyout)).toBe(false)
    expect(flyout?.style.position).toBe('fixed')
    expect(Number.parseInt(flyout?.style.zIndex || '0', 10)).toBeGreaterThan(40)
    expect(wrapper.emitted('group-click')).toBeUndefined()
    wrapper.unmount()
  })
})
