import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { nextTick } from 'vue'
import UiSelect from '../UiSelect.vue'
import { readUi, tokensCss } from './source'

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

afterEach(() => {
  document.body.innerHTML = ''
})

describe('UiSelect', () => {
  it('delegates to common Select and uses field dimensions', () => {
    const wrapper = mount(UiSelect, {
      props: {
        modelValue: 'a',
        options: [
          { value: 'a', label: 'Alpha' },
          { value: 'b', label: 'Beta' }
        ]
      }
    })
    expect(wrapper.find('.ui-select').exists()).toBe(true)
    expect(wrapper.find('.select-trigger').exists()).toBe(true)
    const src = readUi('UiSelect.vue')
    expect(src).toContain("from '@/components/common/Select.vue'")
    expect(src).toContain('height: 36px')
    expect(src).toContain('var(--radius-field)')
    expect(tokensCss).toContain('--radius-field: 12px')
  })

  it('forwards model updates', async () => {
    const wrapper = mount(UiSelect, {
      props: {
        modelValue: null,
        options: [{ value: 'a', label: 'Alpha' }]
      }
    })
    await wrapper.get('button').trigger('click')
    await nextTick()
    const option = document.body.querySelector('.select-option')
    expect(option).not.toBeNull()
    option?.dispatchEvent(new MouseEvent('click', { bubbles: true }))
    expect(wrapper.emitted('update:modelValue')?.[0]).toEqual(['a'])
  })
})
