import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import TLSFingerprintBindingMatrix from '../TLSFingerprintBindingMatrix.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

describe('TLSFingerprintBindingMatrix', () => {
  it('does not allow duplicate os/clientType dimensions to overwrite earlier bindings', async () => {
    const wrapper = mount(TLSFingerprintBindingMatrix, {
      props: {
        modelValue: {},
        profiles: [
          { id: 11, name: 'Chrome A' },
          { id: 12, name: 'Chrome B' }
        ],
        platform: 'openai',
        withClientType: true
      } as any,
      global: {
        stubs: {
          Icon: true
        }
      }
    })

    const addButton = wrapper.findAll('button').find(button =>
      button.text().includes('admin.accounts.quotaControl.tlsFingerprint.addBinding')
    )
    expect(addButton).toBeDefined()
    await addButton!.trigger('click')
    await addButton!.trigger('click')

    const selects = wrapper.findAll('select')
    const clientTypeInputs = wrapper.findAll('input')
    await clientTypeInputs[0].setValue('codex')
    await clientTypeInputs[1].setValue('codex')
    await selects[1].setValue('11')
    await selects[3].setValue('12')

    const updates = wrapper.emitted('update:modelValue') as Array<[Record<string, number>]>
    expect(updates.at(-1)?.[0]).toEqual({ 'windows/codex': 11 })
    expect(wrapper.text()).toContain('Duplicate TLS binding dimensions are ignored')
  })
})
