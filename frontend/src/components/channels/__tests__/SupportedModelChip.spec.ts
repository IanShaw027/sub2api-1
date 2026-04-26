import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'
import SupportedModelChip from '../SupportedModelChip.vue'
import { BILLING_MODE_IMAGE } from '@/constants/channel'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/components/common/PlatformIcon.vue', () => ({
  default: { template: '<span data-test="platform-icon" />' },
}))

describe('SupportedModelChip', () => {
  it('uses per_request_price as the primary image billing price', async () => {
    const wrapper = mount(SupportedModelChip, {
      attachTo: document.body,
      props: {
        model: {
          name: 'gpt-image-1',
          platform: 'openai',
          pricing: {
            billing_mode: BILLING_MODE_IMAGE,
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            image_output_price: 0.99,
            per_request_price: 0.04,
            intervals: [],
          },
        },
      },
    })

    await wrapper.get('[tabindex="0"]').trigger('mouseenter')
    await nextTick()

    const tooltip = document.body.querySelector('[role="tooltip"]')
    expect(tooltip?.textContent).toContain('$0.04')
    expect(tooltip?.textContent).not.toContain('$0.99')

    wrapper.unmount()
    document.body.innerHTML = ''
  })
})
