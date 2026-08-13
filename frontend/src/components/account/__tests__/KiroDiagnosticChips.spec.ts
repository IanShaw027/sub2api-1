import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

import KiroDiagnosticChips from '../KiroDiagnosticChips.vue'

describe('KiroDiagnosticChips', () => {
  it('reads persisted diagnostics from credentials before legacy extra fields', () => {
    const wrapper = mount(KiroDiagnosticChips, {
      props: {
        credentials: {
          profile_id: 'PROFILE-CREDENTIALS',
          login_provider: 'google',
          status_reason: 'FEATURE_NOT_SUPPORTED'
        },
        extra: {
          profile_id: 'PROFILE-LEGACY',
          login_provider: 'legacy',
          kiro_status_reason: 'LEGACY_REASON'
        }
      }
    })

    expect(wrapper.text()).toContain('PROFILE-CREDENTIALS')
    expect(wrapper.text()).toContain('google')
    expect(wrapper.text()).toContain('FEATURE_NOT_SUPPORTED')
    expect(wrapper.text()).not.toContain('PROFILE-LEGACY')
    expect(wrapper.text()).not.toContain('LEGACY_REASON')
  })
})
