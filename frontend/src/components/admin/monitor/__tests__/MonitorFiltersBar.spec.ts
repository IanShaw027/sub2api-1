import { mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { describe, expect, it, vi } from 'vitest'
import MonitorFiltersBar from '../MonitorFiltersBar.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: String,
      default: '',
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue', 'change'],
  template: `
    <select
      class="select-stub"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value); $emit('change', $event.target.value, null)"
    >
      <option v-for="option in options" :key="String(option.value)" :value="String(option.value)">
        {{ option.label }}
      </option>
    </select>
  `,
})

describe('MonitorFiltersBar', () => {
  it('includes Kiro in provider filter options and emits filter changes', async () => {
    const wrapper = mount(MonitorFiltersBar, {
      props: {
        loading: false,
        search: '',
        provider: '',
        enabled: '',
      },
      global: {
        stubs: {
          Select: SelectStub,
          Icon: true,
        },
      },
    })

    const selects = wrapper.findAll('select.select-stub')
    expect(selects).toHaveLength(2)
    const providerOptions = selects[0].findAll('option').map((option) => option.attributes('value'))
    expect(providerOptions).toContain('kiro')

    await selects[0].setValue('kiro')
    expect(wrapper.emitted('update:provider')?.[0]).toEqual(['kiro'])
    expect(wrapper.emitted('filter-change')).toHaveLength(1)
  })
})
