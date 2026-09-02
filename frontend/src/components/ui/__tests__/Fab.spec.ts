import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import Fab from '../Fab.vue'
import { readUi } from './source'

describe('Fab', () => {
  it('uses 52px height and safe-area offset', async () => {
    const wrapper = mount(Fab, { props: { label: 'Create key' }, slots: { default: 'Create' } })
    expect(wrapper.classes()).toContain('btn-glass-primary')
    expect(wrapper.attributes('aria-label')).toBe('Create key')
    await wrapper.trigger('click')
    expect(wrapper.emitted('click')).toHaveLength(1)
    const src = readUi('Fab.vue')
    expect(src).toMatch(/height:\s*52px/)
    expect(src).toContain('right: 16px')
    expect(src).toContain('28px')
    expect(src).toContain('safe-area-inset-bottom')
    expect(src).toContain('border-radius: 16px')
  })
})
