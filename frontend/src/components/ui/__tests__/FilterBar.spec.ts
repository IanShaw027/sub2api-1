import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'
import FilterBar from '../FilterBar.vue'
import { readUi } from './source'

const originalMatchMedia = window.matchMedia

function mockTabletUp(matches: boolean) {
  window.matchMedia = ((query: string) => ({
    matches: query.includes('min-width: 768px') ? matches : false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => true
  })) as unknown as typeof window.matchMedia
}

afterEach(() => {
  window.matchMedia = originalMatchMedia
})

describe('FilterBar', () => {
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
})
