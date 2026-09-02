import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import SettingRow from '../SettingRow.vue'
import { readUi } from './source'

describe('SettingRow', () => {
  it('uses a 240px label column', () => {
    const wrapper = mount(SettingRow, {
      props: { label: 'Site name', description: 'Shown in emails' },
      slots: { default: '<input class="field" />' }
    })
    expect(wrapper.text()).toContain('Site name')
    expect(wrapper.text()).toContain('Shown in emails')
    expect(wrapper.find('input.field').exists()).toBe(true)
    expect(readUi('SettingRow.vue')).toContain('240px')
  })
})
