import { defineComponent, ref } from 'vue'
import { afterEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, mount } from '@vue/test-utils'
import CreateAccountLimitsFieldsSection from '../create/CreateAccountLimitsFieldsSection.vue'
import ConcurrencyPrioritySection from '../bulk/ConcurrencyPrioritySection.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key })
}))

enableAutoUnmount(afterEach)

function mountFields(mode: 'create' | 'bulk') {
  return mount(defineComponent({
    components: { CreateAccountLimitsFieldsSection, ConcurrencyPrioritySection },
    setup: () => ({ concurrency: ref(3), loadFactor: ref<number | null>(null) }),
    template: mode === 'create'
      ? '<CreateAccountLimitsFieldsSection v-model:concurrency="concurrency" v-model:load-factor="loadFactor" :priority="1" :rate-multiplier="1" />'
      : '<ConcurrencyPrioritySection v-model:concurrency="concurrency" v-model:load-factor="loadFactor" :enable-concurrency="true" :enable-load-factor="true" :enable-priority="false" :priority="null" :enable-rate-multiplier="false" :rate-multiplier="null" />'
  }))
}

describe.each(['create', 'bulk'] as const)('%s account scheduling fields', (mode) => {
  it('keeps native input edits in the parent-controlled models', async () => {
    const wrapper = mountFields(mode)
    const [concurrency, loadFactor] = wrapper.findAll('input[type="number"]')

    await concurrency.setValue('8')
    await loadFactor.setValue('6')
    expect(wrapper.vm.concurrency).toBe(8)
    expect(wrapper.vm.loadFactor).toBe(6)

    await concurrency.setValue('12')
    await loadFactor.setValue('9')
    expect(wrapper.vm.concurrency).toBe(12)
    expect(wrapper.vm.loadFactor).toBe(9)
  })

  it('normalizes cleared and invalid values in both model and input', async () => {
    const wrapper = mountFields(mode)
    const [concurrency, loadFactor] = wrapper.findAll('input[type="number"]')

    await concurrency.setValue('')
    expect(wrapper.vm.concurrency).toBe(1)
    expect((concurrency.element as HTMLInputElement).value).toBe('1')
    await concurrency.setValue('0')
    expect(wrapper.vm.concurrency).toBe(1)
    expect((concurrency.element as HTMLInputElement).value).toBe('1')

    await loadFactor.setValue('6')
    await loadFactor.setValue('')
    expect(wrapper.vm.loadFactor).toBeNull()
    await loadFactor.setValue('0')
    expect(wrapper.vm.loadFactor).toBeNull()
    expect((loadFactor.element as HTMLInputElement).value).toBe('')
  })
})
