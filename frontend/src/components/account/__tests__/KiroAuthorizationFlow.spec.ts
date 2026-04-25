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
})
