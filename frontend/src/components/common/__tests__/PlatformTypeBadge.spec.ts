import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const translations: Record<string, string> = {
  'admin.accounts.types.oauth': '__OAUTH__',
  'admin.accounts.vertexLabel': '__VERTEX__',
  'admin.accounts.badges.private': '__PRIVATE__',
  'admin.accounts.badges.cf': '__CF__',
  'admin.accounts.badges.fail': '__FAIL__'
}

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => translations[key] ?? key
    })
  }
})

import PlatformTypeBadge from '../PlatformTypeBadge.vue'

function mountBadge(props: Record<string, unknown>) {
  return mount(PlatformTypeBadge, {
    props,
    global: {
      stubs: {
        PlatformIcon: {
          template: '<span data-testid="platform-icon" />'
        },
        Icon: {
          template: '<span data-testid="icon" />'
        }
      }
    }
  })
}

describe('PlatformTypeBadge', () => {
  it('renders localized type labels for oauth and vertex badges', () => {
    const oauthWrapper = mountBadge({ platform: 'openai', type: 'oauth' })
    expect(oauthWrapper.text()).toContain('__OAUTH__')

    const vertexWrapper = mountBadge({ platform: 'gemini', type: 'service_account' })
    expect(vertexWrapper.text()).toContain('__VERTEX__')
  })

  it('shows normalized Gemini tier labels instead of raw auth type labels', () => {
    const wrapper = mountBadge({
      platform: 'gemini',
      type: 'oauth',
      planType: 'free'
    })

    expect(wrapper.text()).toContain('Free')
    expect(wrapper.text()).not.toContain('__OAUTH__')
  })

  it('marks OpenAI team leaders in platform type badges', () => {
    const wrapper = mountBadge({
      platform: 'openai',
      type: 'oauth',
      organizationRole: 'owner'
    })

    expect(wrapper.text()).toContain('队长')
  })

  it('renders localized privacy labels instead of hardcoded english labels', () => {
    const privateWrapper = mountBadge({
      platform: 'openai',
      type: 'oauth',
      privacyMode: 'training_off'
    })
    expect(privateWrapper.text()).toContain('__PRIVATE__')

    const cfWrapper = mountBadge({
      platform: 'openai',
      type: 'oauth',
      privacyMode: 'training_set_cf_blocked'
    })
    expect(cfWrapper.text()).toContain('__CF__')

    const failWrapper = mountBadge({
      platform: 'antigravity',
      type: 'oauth',
      privacyMode: 'privacy_set_failed'
    })
    expect(failWrapper.text()).toContain('__FAIL__')
  })
})
