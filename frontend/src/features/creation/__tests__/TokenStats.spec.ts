import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string | number>) => {
        if (!params) return key
        const rendered = Object.entries(params)
          .map(([token, value]) => `${token}=${value}`)
          .join(',')
        return `${key}(${rendered})`
      },
    }),
  }
})

import TokenStats from '../components/TokenStats.vue'

describe('TokenStats', () => {
  it('renders nothing when no token data is available', () => {
    const wrapper = mount(TokenStats, { props: {} })
    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)
    expect(wrapper.html()).toBe('<!--v-if-->')
  })

  it('renders nothing when tokens are explicitly null', () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: null, outputTokens: null } })
    expect(wrapper.find('.studio-token-stats').exists()).toBe(false)
  })

  it('shows the collapsed total by default', () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 123, outputTokens: 456 } })
    const el = wrapper.get('.studio-token-stats')
    expect(el.text()).toContain('studio.tokens.total(total=579)')
    expect(el.attributes('aria-expanded')).toBe('false')
  })

  it('does not present a partial count as a complete total', () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 100, outputTokens: null } })
    expect(wrapper.get('.studio-token-stats').text()).toContain('studio.tokens.total(total=common.unknown)')
  })

  it('shows unknown rather than zero in the summary card without usage', () => {
    const wrapper = mount(TokenStats, { props: { variant: 'card', inputTokens: null, outputTokens: null, taskCount: 3 } })
    expect(wrapper.text()).toContain('common.unknown')
    expect(wrapper.text()).toContain('count=3')
  })

  it('expands to show the input/output breakdown on click', async () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 123, outputTokens: 456 } })
    const el = wrapper.get('.studio-token-stats')

    await el.trigger('click')

    expect(el.attributes('aria-expanded')).toBe('true')
    expect(el.text()).toContain('studio.tokens.detail(input=123,output=456)')
  })

  it('expands on hover and collapses on mouseleave', async () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 10, outputTokens: 20 } })
    const el = wrapper.get('.studio-token-stats')

    await el.trigger('mouseenter')
    expect(el.attributes('aria-expanded')).toBe('true')

    await el.trigger('mouseleave')
    expect(el.attributes('aria-expanded')).toBe('false')
  })

  it('collapses on blur even when pinned by a click', async () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 10, outputTokens: 20 } })
    const el = wrapper.get('.studio-token-stats')

    await el.trigger('click')
    expect(el.attributes('aria-expanded')).toBe('true')

    await el.trigger('blur')
    expect(el.attributes('aria-expanded')).toBe('false')
  })

  it('exposes an accessible label with the full breakdown regardless of expanded state', () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 7, outputTokens: 3 } })
    const el = wrapper.get('.studio-token-stats')
    expect(el.attributes('aria-label')).toBe('studio.tokens.ariaLabel(input=7,output=3,total=10)')
  })

  it('applies muted, low-emphasis text styling', () => {
    const wrapper = mount(TokenStats, { props: { inputTokens: 1, outputTokens: 2 } })
    const el = wrapper.get('.studio-token-stats')
    expect(el.classes()).toContain('text-xs')
    expect(el.classes()).toContain('text-muted')
  })
})
