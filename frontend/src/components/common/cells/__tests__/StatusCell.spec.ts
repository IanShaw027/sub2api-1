import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import StatusCell from '../StatusCell.vue'
import { statusTone } from '../statusTone'

describe('statusTone', () => {
  it('maps common status vocabularies to a tone', () => {
    expect(statusTone('active')).toBe('success')
    expect(statusTone('disabled')).toBe('danger')
    expect(statusTone('pending')).toBe('warning')
    expect(statusTone('something-unknown')).toBe('muted')
    expect(statusTone(null)).toBe('muted')
  })
})

describe('StatusCell', () => {
  it('resolves tone from status automatically', () => {
    const wrapper = mount(StatusCell, { props: { status: 'active', label: 'Active' } })
    expect(wrapper.classes()).toContain('badge-tone-success')
    expect(wrapper.text()).toContain('Active')
  })

  it('honours an explicit tone override', () => {
    const wrapper = mount(StatusCell, { props: { status: 'active', label: 'Active', tone: 'danger' } })
    expect(wrapper.classes()).toContain('badge-tone-danger')
  })
})
