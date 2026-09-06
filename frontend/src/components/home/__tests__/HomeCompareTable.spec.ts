import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

import HomeCompareTable, { type CompareRow } from '../HomeCompareTable.vue'

const rows: CompareRow[] = [
  { feature: 'home.comparison.rows.multiAccount', official: 'home.comparison.no', us: 'home.comparison.yes' },
  { feature: 'home.comparison.rows.balance', official: 'home.comparison.no', us: 'home.comparison.yes' }
]

describe('HomeCompareTable', () => {
  it('renders a header row and one row per comparison feature', () => {
    const wrapper = mount(HomeCompareTable, { props: { rows } })
    expect(wrapper.find('.compare-head').exists()).toBe(true)
    expect(wrapper.findAll('.compare-row')).toHaveLength(2)
  })

  it('renders the feature/official/us cell content for each row', () => {
    const wrapper = mount(HomeCompareTable, { props: { rows } })
    const firstRow = wrapper.findAll('.compare-row')[0]!
    expect(firstRow.find('.compare-feature').text()).toBe(rows[0]!.feature)
    expect(firstRow.text()).toContain(rows[0]!.official)
    expect(firstRow.find('.compare-us').text()).toContain(rows[0]!.us)
  })

  it('renders nothing when given an empty rows array', () => {
    const wrapper = mount(HomeCompareTable, { props: { rows: [] } })
    expect(wrapper.findAll('.compare-row')).toHaveLength(0)
  })
})
