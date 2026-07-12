import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import ModelIcon from '../ModelIcon.vue'
import PlatformIcon from '../PlatformIcon.vue'
import ProviderIcon from '@/components/user/monitor/ProviderIcon.vue'
import { GROK_BRAND_ICON, KIRO_BRAND_ICON } from '../platformBrandIcons'

describe('platform brand icons', () => {
  it.each([
    ['kiro', KIRO_BRAND_ICON],
    ['grok', GROK_BRAND_ICON]
  ] as const)('renders the shared %s mark in platform badges', (platform, icon) => {
    const wrapper = mount(PlatformIcon, { props: { platform } })
    const svg = wrapper.get('svg')

    expect(svg.attributes('viewBox')).toBe(icon.viewBox)
    expect(wrapper.get('path').attributes('d')).toBe(icon.path)
    expect(svg.attributes('fill')).toBe('currentColor')
  })

  it.each([
    ['kiro', KIRO_BRAND_ICON],
    ['grok', GROK_BRAND_ICON]
  ] as const)('renders the shared %s mark in channel monitoring', (provider, icon) => {
    const wrapper = mount(ProviderIcon, { props: { provider } })

    expect(wrapper.attributes('viewBox')).toBe(icon.viewBox)
    expect(wrapper.get('path').attributes('d')).toBe(icon.path)
  })

  it('uses the supplied Grok mark for Grok models', () => {
    const wrapper = mount(ModelIcon, { props: { model: 'grok-4.5' } })

    expect(wrapper.attributes('viewBox')).toBe(GROK_BRAND_ICON.viewBox)
    expect(wrapper.get('path').attributes('d')).toBe(GROK_BRAND_ICON.path)
  })
})
