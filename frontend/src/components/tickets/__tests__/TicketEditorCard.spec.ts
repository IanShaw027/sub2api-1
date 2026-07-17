import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import TicketEditorCard from '../TicketEditorCard.vue'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('TicketEditorCard', () => {
  it('strips hidden rate-apply group ids before submit', async () => {
    const wrapper = mount(TicketEditorCard, {
      props: {
        category: 'rate_apply',
        title: 'Rate request',
        payload: {
          group_ids: [88, 99],
          current_group_rates: [
            {
              group_id: 88,
              group_name: 'Hidden Group',
              base_rate: 2,
              special_rate: null,
              effective_rate: 2,
            },
          ],
          target_rate: '1.2',
          usage_scenario: 'test',
        },
        submitLabel: 'Submit',
        availableGroups: [],
      },
      global: {
        stubs: {
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue'],
            template: '<div class="select-stub" />',
          },
        },
      },
    })

    await wrapper.get('.btn.btn-primary').trigger('click')

    expect(wrapper.emitted('submit')).toEqual([[
      {
        category: 'rate_apply',
        title: 'Rate request',
        form_payload: expect.objectContaining({
          group_ids: [],
          current_group_rates: [],
          target_rate: '1.2',
          usage_scenario: 'test',
        }),
      },
    ]])
  })

  it('uses a frameless embedded layout when rendered inside a dialog', () => {
    const wrapper = mount(TicketEditorCard, {
      props: {
        category: 'consult',
        title: '',
        payload: {},
        submitLabel: 'Submit',
        embedded: true,
      },
      global: {
        stubs: {
          Select: {
            props: ['modelValue', 'options'],
            emits: ['update:modelValue'],
            template: '<div class="select-stub" />',
          },
        },
      },
    })

    const rootClasses = wrapper.classes()
    expect(rootClasses).toContain('space-y-5')
    expect(rootClasses).not.toContain('rounded-card')
    expect(rootClasses).not.toContain('border')
  })
})
