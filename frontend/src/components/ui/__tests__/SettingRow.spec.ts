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

  it('preserves a label association when migrating a native field', () => {
    const wrapper = mount(SettingRow, {
      props: { label: 'SMTP host', labelFor: 'smtp-host' },
      slots: { default: '<input id="smtp-host" />' }
    })
    expect(wrapper.get('label').attributes('for')).toBe(wrapper.get('input').attributes('id'))
  })

  it('supports rich labels and descriptions without moving controls into them', async () => {
    const wrapper = mount(SettingRow, {
      slots: {
        label: '<label for="client-id">Client ID <span>*</span></label>',
        description: '<a href="/docs">Provider configuration</a>',
        default: '<input id="client-id" />'
      }
    })
    const control = wrapper.get('.ui-setting-row-control')
    expect(control.attributes('aria-labelledby')).toBe(wrapper.get('.ui-setting-row-title').attributes('id'))
    expect(wrapper.get('.ui-setting-row-description a').attributes('href')).toBe('/docs')
    await wrapper.get('input').setValue('configured-client')
    expect((wrapper.get('input').element as HTMLInputElement).value).toBe('configured-client')
  })
})
