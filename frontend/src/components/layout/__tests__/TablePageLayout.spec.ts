import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import TablePageLayout from '../TablePageLayout.vue'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../TablePageLayout.vue')
const componentSource = readFileSync(componentPath, 'utf8')

describe('TablePageLayout responsive table scrolling', () => {
  it('keeps pagination in one footer inside the table container', () => {
    const wrapper = mount(TablePageLayout, {
      slots: { table: '<table />', pagination: '<nav aria-label="Pagination" />' }
    })
    expect(wrapper.findAll('.table-pagination-footer')).toHaveLength(1)
    expect(wrapper.get('.table-scroll-container > .table-pagination-footer nav').exists()).toBe(true)
    wrapper.unmount()
  })

  it('does not create an empty footer without a pagination slot', () => {
    const wrapper = mount(TablePageLayout, { slots: { table: '<table />' } })
    expect(wrapper.find('.table-pagination-footer').exists()).toBe(false)
    wrapper.unmount()
  })

  it('does not disable the table horizontal scroll container in mobile mode', () => {
    const tableWrapperBlocks = Array.from(
      componentSource.matchAll(/([^{}]*:deep\(\.table-wrapper\)[^{}]*)\{([^{}]*)\}/g)
    )

    expect(tableWrapperBlocks.length).toBeGreaterThan(0)

    const baseBlock = tableWrapperBlocks.find(([selector]) => !selector.includes('.mobile-mode'))
    const mobileBlocks = tableWrapperBlocks.filter(([selector]) => selector.includes('.mobile-mode'))

    expect(baseBlock?.[2]).toMatch(/overflow-x:\s*auto/)
    expect(
      mobileBlocks.every(
        ([, , declarations]) => !/overflow(-x)?:\s*visible/.test(declarations) && !declarations.includes('overflow-visible')
      )
    ).toBe(true)
  })
})
