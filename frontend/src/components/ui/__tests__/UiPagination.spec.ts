import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'
import UiPagination from '../UiPagination.vue'
import { readUi } from './source'

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

describe('UiPagination', () => {
  it('wraps common Pagination and restyles fields to 36px', async () => {
    const wrapper = mount(UiPagination, {
      props: { page: 1, pageSize: 20, total: 40 }
    })
    expect(wrapper.classes()).toContain('ui-pagination')
    const next = wrapper.find('[aria-label="pagination.next"]')
    expect(next.exists()).toBe(true)
    await next.trigger('click')
    expect(wrapper.emitted('update:page')?.[0]).toEqual([2])
    const src = readUi('UiPagination.vue')
    expect(src).toContain("from '@/components/common/Pagination.vue'")
    expect(src).toContain('height: 36px')
    expect(src).toContain('var(--radius-field)')
  })
})
