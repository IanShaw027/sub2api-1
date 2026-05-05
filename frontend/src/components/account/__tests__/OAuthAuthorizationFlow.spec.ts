import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

const copyToClipboardMock = vi.fn()

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copied: { value: false },
    copyToClipboard: copyToClipboardMock
  })
}))

import OAuthAuthorizationFlow from '../OAuthAuthorizationFlow.vue'

function mountComponent(props: Record<string, unknown> = {}) {
  return mount(OAuthAuthorizationFlow, {
    props: {
      addMethod: 'oauth',
      platform: 'gemini',
      ...props
    },
    global: {
      stubs: {
        Icon: true
      }
    }
  })
}

describe('OAuthAuthorizationFlow', () => {
  it('shows Gemini project preparation tip', () => {
    const wrapper = mountComponent()

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectTipTitle')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectTipStepIam')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectApiLink')
  })

  it('reveals project id recovery step only after Gemini error in recovery mode', async () => {
    const wrapper = mountComponent({
      showProjectId: false,
      showProjectIdRecovery: true,
      error: 'admin.accounts.oauth.gemini.missingProjectId'
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')

    const projectInput = wrapper.get('input[placeholder="admin.accounts.oauth.gemini.projectIdPlaceholder"]')
    await projectInput.setValue('personal-project-123')
    const generateButtons = wrapper.findAll('button').filter((candidate) =>
      candidate.text().includes('admin.accounts.oauth.gemini.generateAuthUrl')
    )
    expect(generateButtons.length).toBeGreaterThan(0)
    await generateButtons[generateButtons.length - 1].trigger('click')

    expect(wrapper.emitted('generate-url')).toEqual([[]])
  })

  it('does not reveal project id recovery for unrelated Gemini errors', () => {
    const wrapper = mountComponent({
      showProjectId: false,
      showProjectIdRecovery: true,
      error: 'admin.accounts.oauth.gemini.failedToExchangeCode'
    })

    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
  })

  it('clears parsed oauth state when regenerating a Gemini auth URL', async () => {
    const wrapper = mountComponent({
      authUrl: 'https://example.com/oauth'
    })

    await wrapper.get('textarea').setValue('http://localhost/callback?code=abc&state=stale-state')
    expect((wrapper.vm as any).$?.exposed?.oauthState?.value).toBe('stale-state')

    const regenerateButton = wrapper.findAll('button').find((candidate) =>
      candidate.text().includes('admin.accounts.oauth.regenerate')
    )
    expect(regenerateButton).toBeTruthy()
    await regenerateButton!.trigger('click')

    expect((wrapper.vm as any).$?.exposed?.oauthState?.value).toBe('')
    expect(wrapper.emitted('generate-url')).toEqual([[]])
  })
})
