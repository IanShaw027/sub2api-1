import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import AffiliateView from '../AffiliateView.vue'

const componentSource = readFileSync(
  resolve(dirname(fileURLToPath(import.meta.url)), '../AffiliateView.vue'),
  'utf8',
)

const { copyToClipboard, getAffiliateDetail } = vi.hoisted(() => ({
  copyToClipboard: vi.fn(),
  getAffiliateDetail: vi.fn(),
}))

vi.mock('@/api/user', () => ({
  default: {
    getAffiliateDetail,
    transferAffiliateQuota: vi.fn(),
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
  }),
}))

vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({
    refreshUser: vi.fn(),
  }),
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({ copyToClipboard }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

describe('AffiliateView', () => {
  const affiliateCode = 'affiliate-code-that-is-long-enough-to-overflow-a-mobile-viewport'

  beforeEach(() => {
    vi.clearAllMocks()
    copyToClipboard.mockResolvedValue(true)
    getAffiliateDetail.mockResolvedValue({
      user_id: 1,
      aff_code: affiliateCode,
      inviter_id: null,
      aff_count: 0,
      aff_quota: 0,
      aff_frozen_quota: 0,
      aff_history_quota: 0,
      effective_rebate_rate_percent: 10,
      invitees: [],
    })
  })

  it('stacks long values and copy controls on mobile while retaining desktop rows', async () => {
    const wrapper = mount(AffiliateView, {
      global: {
        stubs: {
          AppLayout: { template: '<main><slot /></main>' },
          Icon: true,
        },
      },
    })

    await flushPromises()

    // These mobile/desktop responsibilities moved from Tailwind utility
    // classes into scoped `.affiliate-copy-row`/`.affiliate-copy-value`/
    // `.affiliate-copy-btn` rules during the Glass redesign: the row stacks
    // (column, stretched) and the value wraps with `break-all` by default,
    // switching to a horizontal, truncating row only from the `sm` (640px)
    // breakpoint up.
    const values = wrapper.findAll('code')
    expect(values).toHaveLength(2)
    for (const value of values) {
      expect(value.classes()).toContain('affiliate-copy-value')
      expect(Array.from(value.element.parentElement?.classList ?? [])).toContain('affiliate-copy-row')
    }

    const valueRuleMatch = componentSource.match(/\.affiliate-copy-value\s*\{([^}]*)\}/)
    expect(valueRuleMatch?.[1]).toMatch(/word-break:\s*break-all/)
    const rowRuleMatch = componentSource.match(/\.affiliate-copy-row\s*\{([^}]*)\}/)
    expect(rowRuleMatch?.[1]).toMatch(/flex-direction:\s*column/)
    expect(rowRuleMatch?.[1]).toMatch(/align-items:\s*stretch/)

    const responsiveBlockMatch = componentSource.match(/@media \(min-width: 640px\)\s*\{([\s\S]*?)\n\}\n/)
    const responsiveBlock = responsiveBlockMatch?.[1] ?? ''
    expect(responsiveBlock).toMatch(/\.affiliate-copy-row\s*\{[^}]*flex-direction:\s*row/)
    expect(responsiveBlock).toMatch(/\.affiliate-copy-value\s*\{[^}]*white-space:\s*nowrap/)

    const copyButtons = wrapper.findAll('button').filter((button) =>
      ['affiliate.copyCode', 'affiliate.copyLink'].includes(button.text()),
    )
    expect(copyButtons).toHaveLength(2)
    for (const button of copyButtons) {
      expect(button.classes()).toContain('affiliate-copy-btn')
    }
    expect(responsiveBlock).toMatch(/\.affiliate-copy-btn\s*\{[^}]*width:\s*auto/)

    await copyButtons[0].trigger('click')
    await copyButtons[1].trigger('click')
    await flushPromises()

    expect(copyToClipboard).toHaveBeenNthCalledWith(1, affiliateCode, 'affiliate.codeCopied')
    expect(copyToClipboard).toHaveBeenNthCalledWith(
      2,
      `${window.location.origin}/register?aff=${encodeURIComponent(affiliateCode)}`,
      'affiliate.linkCopied',
    )
  })
})
