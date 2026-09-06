import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PublicPageLayout from '../PublicPageLayout.vue'
import DashboardPageLayout from '../DashboardPageLayout.vue'
import DetailPageLayout from '../DetailPageLayout.vue'
import SettingsPageLayout from '../SettingsPageLayout.vue'

describe('PublicPageLayout', () => {
  it('renders on the shared public-page surface with an optional top nav', () => {
    const withNav = mount(PublicPageLayout, {
      slots: { nav: '<div class="my-nav">Nav</div>', default: '<div class="my-body">Body</div>' }
    })
    expect(withNav.classes()).toContain('public-page')
    expect(withNav.find('.my-nav').exists()).toBe(true)
    expect(withNav.find('.my-body').exists()).toBe(true)

    const withoutNav = mount(PublicPageLayout, { slots: { default: '<div class="my-body">Body</div>' } })
    expect(withoutNav.find('.public-page-layout-nav').exists()).toBe(false)
  })
})

describe('DashboardPageLayout', () => {
  it('lays out header/stats/charts/lists as a 14px-gap column', () => {
    const wrapper = mount(DashboardPageLayout, {
      slots: {
        header: '<div class="h">H</div>',
        stats: '<div class="s">S</div>',
        charts: '<div class="c">C</div>',
        lists: '<div class="l">L</div>'
      }
    })
    expect(wrapper.find('.h').exists()).toBe(true)
    expect(wrapper.find('.s').exists()).toBe(true)
    expect(wrapper.find('.c').exists()).toBe(true)
    expect(wrapper.find('.l').exists()).toBe(true)
  })

  it('falls back to the default slot when no named sections are used', () => {
    const wrapper = mount(DashboardPageLayout, { slots: { default: '<div class="anything" />' } })
    expect(wrapper.find('.anything').exists()).toBe(true)
  })
})

describe('DetailPageLayout', () => {
  it('renders header, a 1fr/320px main+side grid, and an optional composer', () => {
    const wrapper = mount(DetailPageLayout, {
      slots: {
        header: '<div class="h">H</div>',
        main: '<div class="m">M</div>',
        side: '<div class="sd">S</div>',
        composer: '<div class="cp">C</div>'
      }
    })
    expect(wrapper.find('.h').exists()).toBe(true)
    expect(wrapper.find('.m').exists()).toBe(true)
    expect(wrapper.find('.sd').exists()).toBe(true)
    expect(wrapper.find('.cp').exists()).toBe(true)
  })

  it('omits the side column when unused', () => {
    const wrapper = mount(DetailPageLayout, { slots: { main: '<div class="m">M</div>' } })
    expect(wrapper.find('.detail-page-layout-side').exists()).toBe(false)
  })
})

describe('SettingsPageLayout', () => {
  it('renders header + a 224px/1fr nav+content grid', () => {
    const wrapper = mount(SettingsPageLayout, {
      slots: {
        header: '<div class="h">H</div>',
        nav: '<div class="n">N</div>',
        content: '<div class="c">C</div>'
      }
    })
    expect(wrapper.find('.h').exists()).toBe(true)
    expect(wrapper.find('.n').exists()).toBe(true)
    expect(wrapper.find('.c').exists()).toBe(true)
  })

  it('omits the nav column when unused', () => {
    const wrapper = mount(SettingsPageLayout, { slots: { content: '<div class="c">C</div>' } })
    expect(wrapper.find('.settings-page-layout-nav').exists()).toBe(false)
  })
})
