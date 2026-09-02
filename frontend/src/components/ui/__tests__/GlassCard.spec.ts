import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import GlassCard from '../GlassCard.vue'
import { readUi, styleCss, tokensCss } from './source'

describe('GlassCard', () => {
  it('renders glass variant, padding, and slots', () => {
    const wrapper = mount(GlassCard, {
      props: { hover: true, padding: 'md' },
      slots: {
        header: 'Header',
        default: 'Body',
        footer: 'Footer'
      }
    })
    expect(wrapper.classes()).toContain('glass-card')
    expect(wrapper.classes()).toContain('glass-card-hover')
    expect(wrapper.classes()).toContain('ui-glass-card-pad-md')
    expect(wrapper.text()).toContain('Header')
    expect(wrapper.text()).toContain('Body')
    expect(wrapper.text()).toContain('Footer')
  })

  it('switches solid and transparent variants', () => {
    const solid = mount(GlassCard, { props: { variant: 'solid' } })
    expect(solid.classes()).toContain('glass-card-solid')
    const transparent = mount(GlassCard, { props: { variant: 'transparent' } })
    expect(transparent.classes()).toContain('ui-glass-card-transparent')
  })

  it('uses token radius and documented paddings', () => {
    expect(styleCss).toMatch(/\.glass-card[\s\S]*?border-radius:\s*var\(--radius-card\)/)
    expect(tokensCss).toContain('--radius-card: 14px')
    const src = readUi('GlassCard.vue')
    expect(src).toContain('padding: 12px')
    expect(src).toContain('padding: 16px')
    expect(src).toContain('padding: 20px')
  })
})
