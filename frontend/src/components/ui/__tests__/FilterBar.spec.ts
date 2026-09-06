import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import { afterEach, describe, expect, it } from 'vitest'
import FilterBar from '../FilterBar.vue'
import { readUi, uiDir } from './source'
import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'

const originalMatchMedia = window.matchMedia

function mockTabletUp(matches: boolean) {
  const subscriptions: Array<{ media: { matches: boolean }; notify: () => void }> = []
  window.matchMedia = ((query: string) => {
    const media = {
      matches: query.includes('min-width: 768px') ? matches : false,
      media: query,
      onchange: null,
      addListener: () => undefined,
      removeListener: () => undefined,
      addEventListener: (_event: string, notify: () => void) => {
        if (query.includes('min-width: 768px')) subscriptions.push({ media, notify })
      },
      removeEventListener: () => undefined,
      dispatchEvent: () => true
    }
    return media
  }) as unknown as typeof window.matchMedia
  return (tabletUp: boolean) => {
    for (const subscription of subscriptions) {
      subscription.media.matches = tabletUp
      subscription.notify()
    }
  }
}

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe('FilterBar', () => {
  it.each([
    'admin/PromoCodesView.vue', 'admin/ProxiesView.vue', 'admin/AnnouncementsView.vue',
    'admin/AuditLogView.vue', 'admin/affiliates/AdminAffiliateRecordsTable.vue',
    'admin/ChannelsView.vue', 'admin/SubscriptionsView.vue',
    'user/KeysView.vue', 'user/UserInvoicesView.vue'
  ])('uses the shared mobile filter panel without a duplicate parent panel in %s', (file) => {
    const source = readFileSync(resolve(uiDir, '../../views', file), 'utf8')
    expect(source).toContain('<FilterBar')
    expect(source).toContain('<template #filters>')
    expect(source).not.toContain('showMobileFilters')
  })

  it('renders search and filter slots on desktop', async () => {
    mockTabletUp(true)
    const wrapper = mount(FilterBar, {
      props: { search: 'key', searchPlaceholder: 'Search' },
      slots: { filters: '<div class="filter">Group</div>', trailing: '<span>3 selected</span>' }
    })
    expect(wrapper.find('input.field').exists()).toBe(true)
    expect(wrapper.find('.filter').exists()).toBe(true)
    expect(wrapper.text()).toContain('3 selected')
    await wrapper.find('input.field').setValue('next')
    expect(wrapper.emitted('update:search')?.[0]).toEqual(['next'])
    expect(wrapper.find('.ui-filter-bar-toggle').exists()).toBe(false)
  })

  it('collapses filters to a 44px button on mobile', () => {
    mockTabletUp(false)
    const wrapper = mount(FilterBar, {
      slots: { filters: '<div class="filter">Group</div>' }
    })
    expect(wrapper.find('.ui-filter-bar-toggle').exists()).toBe(true)
    expect(wrapper.find('.ui-filter-bar-filters').classes()).toContain('is-mobile-hidden')
    expect(readUi('FilterBar.vue')).toContain('height: 44px')
  })

  it('opens usable filters without a parent handler and preserves values when collapsed', async () => {
    mockTabletUp(false)
    const wrapper = mount(FilterBar, {
      slots: { filters: '<select aria-label="Status"><option>All</option><option>Active</option></select>' }
    })
    const toggle = wrapper.get('.ui-filter-bar-toggle')
    const filters = wrapper.get('.ui-filter-bar-filters')
    expect(toggle.attributes('aria-expanded')).toBe('false')
    expect(toggle.attributes('aria-controls')).toBe(filters.attributes('id'))

    await toggle.trigger('click')
    expect(toggle.attributes('aria-expanded')).toBe('true')
    expect(filters.classes()).not.toContain('is-mobile-hidden')
    expect(wrapper.emitted('open-filters')).toHaveLength(1)
    await wrapper.get('select').setValue('Active')

    await toggle.trigger('click')
    expect(filters.classes()).toContain('is-mobile-hidden')
    await toggle.trigger('click')
    expect((wrapper.get('select').element as HTMLSelectElement).value).toBe('Active')
    wrapper.unmount()
  })

  it('always exposes filters on desktop after a mobile collapse', async () => {
    const resize = mockTabletUp(false)
    const wrapper = mount(FilterBar, { slots: { filters: '<button>Status</button>' } })
    expect(wrapper.get('.ui-filter-bar-filters').classes()).toContain('is-mobile-hidden')
    resize(true)
    await nextTick()
    expect(wrapper.get('.ui-filter-bar-filters').classes()).not.toContain('is-mobile-hidden')
    expect(wrapper.find('.ui-filter-bar-toggle').exists()).toBe(false)
    wrapper.unmount()
  })
})
