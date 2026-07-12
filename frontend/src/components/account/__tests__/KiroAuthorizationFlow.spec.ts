import { beforeEach, describe, expect, it, vi } from 'vitest'
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

import KiroAuthorizationFlow from '../KiroAuthorizationFlow.vue'

function mountComponent(props: Record<string, unknown> = {}) {
  return mount(KiroAuthorizationFlow, {
    props,
    global: {
      stubs: {
        Icon: true
      }
    }
  })
}

function findButtonByText(wrapper: ReturnType<typeof mountComponent>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text().includes(text))
  expect(button, `button containing "${text}" should exist`).toBeTruthy()
  return button!
}

describe('KiroAuthorizationFlow', () => {
  beforeEach(() => {
    copyToClipboardMock.mockReset()
  })

  it('keeps create manual refresh-token submission behavior', async () => {
    const wrapper = mountComponent({ mode: 'create' })

    expect(wrapper.get('input[value="oauth"]').exists()).toBe(true)
    expect(wrapper.get('input[value="refresh_token"]').exists()).toBe(true)

    await wrapper.get('input[value="refresh_token"]').setValue()
    await wrapper
      .get('textarea[placeholder="admin.accounts.kiro.refreshTokenPlaceholderBatch"]')
      .setValue('  rt-create  ')

    await findButtonByText(wrapper, 'admin.accounts.kiro.validateAndCreate').trigger('click')

    expect(wrapper.emitted('submit-refresh-token')).toEqual([[
      {
        credentials: {
          refresh_token: 'rt-create',
          auth_method: 'social',
          region: 'us-east-1'
        },
        extra: {}
      }
    ]])
    expect(wrapper.emitted('submit')).toBeUndefined()
  })

  it('allows manual refresh-token mode in reauth and reuses existing credential defaults', async () => {
    const wrapper = mountComponent({
      mode: 'reauth',
      initialCredentials: {
        auth_method: 'idc',
        region: 'eu-west-1',
        client_id: 'saved-client'
      }
    })

    expect(wrapper.get('input[value="oauth"]').exists()).toBe(true)
    expect(wrapper.get('input[value="refresh_token"]').exists()).toBe(true)

    await wrapper.get('input[value="refresh_token"]').setValue()
    await wrapper
      .get('textarea[placeholder="admin.accounts.kiro.refreshTokenPlaceholderBatch"]')
      .setValue('rt-reauth')
    await wrapper
      .get('input[placeholder="admin.accounts.kiro.clientSecretPlaceholder"]')
      .setValue('new-secret')

    await findButtonByText(wrapper, 'admin.accounts.reAuthorize').trigger('click')

    expect(wrapper.emitted('submit-refresh-token')).toEqual([[
      {
        credentials: {
          refresh_token: 'rt-reauth',
          auth_method: 'idc',
          region: 'eu-west-1',
          client_id: 'saved-client',
          client_secret: 'new-secret'
        },
        extra: {}
      }
    ]])
    expect(wrapper.emitted('submit')).toBeUndefined()
  })

  it('emits ExternalIdp metadata for manual refresh-token account creation', async () => {
    const wrapper = mountComponent({
      mode: 'create',
      initialCredentials: {
        auth_method: 'external_idp',
        client_id: 'saved-client',
        issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
        token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token',
        scopes: 'scope-a scope-b',
        login_hint: 'user@example.com',
        profile_arn: 'arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD'
      }
    })

    await wrapper.get('input[value="refresh_token"]').setValue()
    await wrapper
      .get('textarea[placeholder="admin.accounts.kiro.refreshTokenPlaceholderBatch"]')
      .setValue('rt-external')

    await findButtonByText(wrapper, 'admin.accounts.kiro.validateAndCreate').trigger('click')

    expect(wrapper.emitted('submit-refresh-token')).toEqual([[
      {
        credentials: {
          refresh_token: 'rt-external',
          auth_method: 'external_idp',
          region: 'us-east-1',
          client_id: 'saved-client',
          issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
          token_endpoint: 'https://login.microsoftonline.com/tenant/oauth2/v2.0/token',
          scopes: 'scope-a scope-b',
          login_hint: 'user@example.com',
          profile_arn: 'arn:aws:codewhisperer:us-east-1:904962390873:profile/CQRAXYDP9YVD'
        },
        extra: {}
      }
    ]])
  })

  it('allows multiline manual refresh-token input during reauth', async () => {
    const wrapper = mountComponent({ mode: 'reauth' })

    await wrapper.get('input[value="refresh_token"]').setValue()
    await wrapper
      .get('textarea[placeholder="admin.accounts.kiro.refreshTokenPlaceholderBatch"]')
      .setValue('rt-one\nrt-two')

    await findButtonByText(wrapper, 'admin.accounts.reAuthorize').trigger('click')

    expect(wrapper.emitted('submit-refresh-token')).toEqual([[
      {
        credentials: {
          refresh_token: 'rt-one\nrt-two',
          auth_method: 'social',
          region: 'us-east-1'
        },
        extra: {}
      }
    ]])
  })

  it('uses i18n keys for copy feedback and runtime-managed hint', async () => {
    const wrapper = mountComponent({
      mode: 'create',
      authUrl: 'https://example.com/kiro/oauth'
    })

    const copyButton = wrapper.get('button[title="common.copy"]')
    await copyButton.trigger('click')

    expect(copyToClipboardMock).toHaveBeenCalledWith('https://example.com/kiro/oauth', 'common.copiedToClipboard')
    expect(wrapper.text()).toContain('admin.accounts.kiro.runtimeManagedHint')
  })

  it('shows Microsoft External IdP authorization details and keeps final callback input available', async () => {
    const authUrl = 'https://login.microsoftonline.com/tenant/oauth2/v2.0/authorize?client_id=client-1'
    const wrapper = mountComponent({
      mode: 'create',
      authUrl: 'https://app.kiro.dev/signin?state=state-1',
      externalIDPAuthorization: {
        session_id: 'session-1',
        status: 'authorization_required',
        auth_method: 'external_idp',
        login_option: 'external_idp',
        auth_url: authUrl,
        client_id: 'client-1',
        redirect_uri: 'http://localhost:3128/signin/callback?login_option=external_idp',
        issuer_url: 'https://login.microsoftonline.com/tenant/v2.0',
        scopes: ['scope-a', 'offline_access'],
        login_hint: 'user@example.com'
      }
    })

    expect(wrapper.text()).toContain('admin.accounts.kiro.externalIdpAuthorizationTitle')
    expect(wrapper.text()).toContain(authUrl)
    expect(wrapper.text()).toContain('client-1')
    expect(wrapper.text()).toContain('admin.accounts.kiro.externalIdpRedirectUri')
    expect(wrapper.get('textarea[placeholder="admin.accounts.kiro.callbackUrlPlaceholder"]').exists()).toBe(true)

    await wrapper.get(`a[href="${authUrl}"]`).trigger('click')
    await findButtonByText(wrapper, 'common.copy').trigger('click')
    expect(copyToClipboardMock).toHaveBeenCalledWith(authUrl, 'common.copiedToClipboard')
  })
})
