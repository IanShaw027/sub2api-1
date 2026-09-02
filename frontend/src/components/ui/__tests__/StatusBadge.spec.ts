import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusBadge from '../StatusBadge.vue'
import { styleCss } from './source'

describe('StatusBadge', () => {
  it('applies tone class and optional dot', () => {
    const wrapper = mount(StatusBadge, {
      props: { tone: 'success', label: 'OK', dot: true }
    })
    expect(wrapper.classes()).toContain('badge-tone-success')
    expect(wrapper.find('.ui-status-badge-dot-mark').exists()).toBe(true)
    expect(wrapper.text()).toContain('OK')
  })

  it('covers every tone and badge height token', () => {
    for (const tone of ['success', 'warning', 'danger', 'muted', 'accent'] as const) {
      expect(mount(StatusBadge, { props: { tone, label: tone } }).classes()).toContain(`badge-tone-${tone}`)
    }
    expect(styleCss).toMatch(/\.badge-tone-success[\s\S]*?height:\s*22px/)
    expect(styleCss).toContain('border-radius: 999px')
  })
})
