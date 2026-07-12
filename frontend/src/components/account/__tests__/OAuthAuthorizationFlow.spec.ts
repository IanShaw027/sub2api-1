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
  it('shows Gemini Code Assist project preparation tip only when enabled', () => {
    const wrapper = mountComponent({
      showGeminiProjectBootstrapTip: true
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectTipTitle')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectTipStepIam')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectApiLink')
  })

  it('does not show the Gemini Code Assist bootstrap tip by default', () => {
    const wrapper = mountComponent()

    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectTipTitle')
  })

  it('reveals project id recovery for google one companion-project detection failures', () => {
    const wrapper = mountComponent({
      showProjectId: false,
      showProjectIdRecovery: true,
      errorCode: 'google_one_project_detection_failed'
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
  })

  it('reveals project id recovery step only after Gemini error in recovery mode', async () => {
    const wrapper = mountComponent({
      showProjectId: true,
      showProjectIdRecovery: true,
      errorCode: 'missing_project_id'
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

  it('keeps the project preparation tip visible during code-assist recovery even when the original input is hidden', () => {
    const wrapper = mountComponent({
      showProjectId: false,
      showProjectIdRecovery: true,
      showGeminiProjectBootstrapTip: true,
      errorCode: 'code_assist_user_defined_project_required'
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectTipTitle')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
  })

  it('does not reveal project id recovery for unrelated Gemini errors', () => {
    const wrapper = mountComponent({
      showProjectId: true,
      showProjectIdRecovery: true,
      errorCode: 'exchange_failed'
    })

    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
  })

  it('shows explicit Gemini age-eligibility guidance without revealing project recovery', () => {
    const wrapper = mountComponent({
      showProjectId: true,
      showProjectIdRecovery: true,
      errorCode: 'age_verification_required'
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.eligibilityGuidanceTitle')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.eligibilityAgeReason')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.eligibilityAgeNotGcpRelated')
    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
  })

  it('shows generic Gemini eligibility guidance for non-age failures', () => {
    const wrapper = mountComponent({
      showProjectId: true,
      showProjectIdRecovery: true,
      errorCode: 'ineligible_tier'
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.ineligibleTierGuidanceTitle')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.ineligibleTierReason')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.eligibilityGenericStepRetryAuth')
    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.eligibilityGenericStepTryOtherFlow')
    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.eligibilityAgeNotGcpRelated')
  })

  it('does not reveal project id recovery when recovery mode is disabled', () => {
    const wrapper = mountComponent({
      showProjectId: false,
      showProjectIdRecovery: false,
      errorCode: 'google_one_project_detection_failed'
    })

    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectIdRecoveryTitle')
    expect(wrapper.text()).not.toContain('admin.accounts.oauth.gemini.projectTipTitle')
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

  it('marks Gemini project recovery as required after project id changes under an existing session', async () => {
    const wrapper = mountComponent({
      authUrl: 'https://example.com/oauth',
      sessionId: 'session-1',
      showProjectId: true
    })

    expect((wrapper.vm as any).$?.exposed?.requiresProjectIdRecovery?.value).toBe(false)

    const projectInput = wrapper.get('input[placeholder="admin.accounts.oauth.gemini.projectIdPlaceholder"]')
    await projectInput.setValue('manual-project-id')

    expect(wrapper.text()).toContain('admin.accounts.oauth.gemini.projectIdChangedRegenerate')
    expect((wrapper.vm as any).$?.exposed?.requiresProjectIdRecovery?.value).toBe(true)
  })

  it('shows Grok manual SSO and password authorization methods when enabled', () => {
    const wrapper = mountComponent({
      platform: 'grok',
      showRefreshTokenOption: true,
      showSSOTokenOption: true,
      showEmailPasswordOption: true
    })

    expect(wrapper.text()).toContain('admin.accounts.oauth.grok.refreshTokenAuth')
    expect(wrapper.text()).toContain('admin.accounts.oauth.grok.ssoTokenAuth')
    expect(wrapper.text()).toContain('admin.accounts.oauth.grok.emailPasswordAuth')
  })

  it('emits validate-sso-token for Grok manual SSO input', async () => {
    const wrapper = mountComponent({
      platform: 'grok',
      showSSOTokenOption: true,
      allowMultiple: true
    })

    await wrapper.get('input[type="radio"][value="sso_token"]').setValue(true)
    await wrapper.get('textarea').setValue('sso-line-1\nsso-line-2')
    const validateButton = wrapper.findAll('button').find((candidate) =>
      candidate.text().includes('admin.accounts.oauth.grok.validateAndCreate')
    )
    expect(validateButton).toBeTruthy()
    await validateButton!.trigger('click')

    expect(wrapper.emitted('validate-sso-token')).toEqual([['sso-line-1\nsso-line-2']])
  })

  it('emits authorize-password for Grok manual email-password input', async () => {
    const wrapper = mountComponent({
      platform: 'grok',
      showEmailPasswordOption: true
    })

    await wrapper.get('input[type="radio"][value="email_password"]').setValue(true)
    const passwordInput = wrapper.get('input[placeholder="admin.accounts.oauth.grok.emailPasswordPlaceholder"]')
    await passwordInput.setValue(' user@example.com---- secret  ')
    const validateButton = wrapper.findAll('button').find((candidate) =>
      candidate.text().includes('admin.accounts.oauth.grok.validateAndCreate')
    )
    expect(validateButton).toBeTruthy()
    await validateButton!.trigger('click')

    expect(wrapper.emitted('authorize-password')).toEqual([[' user@example.com---- secret  ']])
  })

  it('uses single-line credential inputs and hides batch counts when multiple input is disabled', async () => {
    const wrapper = mountComponent({
      platform: 'grok',
      showRefreshTokenOption: true,
      allowMultiple: false
    })

    await wrapper.get('input[type="radio"][value="refresh_token"]').setValue(true)
    const input = wrapper.get('input[placeholder="admin.accounts.oauth.grok.refreshTokenPlaceholder"]')
    await input.setValue('single-token')

    expect(wrapper.find('textarea[placeholder="admin.accounts.oauth.grok.refreshTokenPlaceholder"]').exists()).toBe(false)
    expect(wrapper.text()).not.toContain('admin.accounts.oauth.batchCreateAccounts')
  })
})
