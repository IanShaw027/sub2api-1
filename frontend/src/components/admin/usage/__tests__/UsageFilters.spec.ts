import { beforeEach, describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

import UsageFilters from '../UsageFilters.vue'

const { listGroupsMock, getModelStatsMock } = vi.hoisted(() => ({
  listGroupsMock: vi.fn(),
  getModelStatsMock: vi.fn(),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

vi.mock('@/api/admin', () => ({
  adminAPI: {
    usage: {
      searchUsers: vi.fn(),
      searchApiKeys: vi.fn(),
    },
    accounts: {
      list: vi.fn(),
    },
    groups: {
      list: listGroupsMock,
    },
    dashboard: {
      getModelStats: getModelStatsMock,
    },
  },
}))

const SelectStub = {
  props: ['modelValue', 'options', 'searchable'],
  emits: ['update:modelValue', 'change'],
  template: `
    <select>
      <option v-for="option in options" :key="String(option.value)" :value="option.value">
        {{ option.label }}
      </option>
    </select>
  `,
}

describe('admin UsageFilters', () => {
  beforeEach(() => {
    listGroupsMock.mockResolvedValue({ items: [] })
    getModelStatsMock.mockResolvedValue({ models: [] })
  })

  it('emits change when exclude admin is toggled', async () => {
    const modelValue = {
      exclude_admin: false,
      start_date: '2026-04-01',
      end_date: '2026-04-24',
    }

    const wrapper = mount(UsageFilters, {
      props: {
        modelValue,
        exporting: false,
        startDate: '2026-04-01',
        endDate: '2026-04-24',
      },
      global: {
        stubs: {
          Select: SelectStub,
        },
      },
    })

    await nextTick()
    await wrapper.get('[role="switch"]').trigger('click')

    expect(modelValue.exclude_admin).toBe(true)
    expect(wrapper.emitted('change')).toHaveLength(1)
  })

  it('offers video as a request type filter option', () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: {
          start_date: '2026-04-01',
          end_date: '2026-04-24',
        },
        exporting: false,
        startDate: '2026-04-01',
        endDate: '2026-04-24',
      },
      global: {
        stubs: {
          Select: SelectStub,
        },
      },
    })

    expect(wrapper.text()).toContain('usage.video')
  })

  it('shows error-specific filters and hides usage-only actions in errors mode', () => {
    const wrapper = mount(UsageFilters, {
      props: {
        modelValue: {
          start_date: '2026-04-01',
          end_date: '2026-04-24',
        },
        exporting: false,
        startDate: '2026-04-01',
        endDate: '2026-04-24',
        mode: 'errors',
      },
      global: { stubs: { Select: SelectStub } },
    })

    const text = wrapper.text()
    expect(text).toContain('admin.ops.errorLog.typeUpstream')
    expect(text).toContain('usage.errors.categories.rate_limit')
    expect(text).toContain('usage.errors.categories.other')
    expect(text).toContain('429')
    expect(text).not.toContain('usage.video')
    expect(text).not.toContain('admin.usage.cleanup.button')
    expect(text).not.toContain('usage.exportExcel')
  })
})
