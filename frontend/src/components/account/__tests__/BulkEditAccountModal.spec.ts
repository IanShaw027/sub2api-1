import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import BulkEditAccountModal from '../BulkEditAccountModal.vue'
import ModelWhitelistSelector from '../ModelWhitelistSelector.vue'
import { adminAPI } from '@/api/admin'

const {
  listTlsFingerprintProfilesMock,
  listTlsFingerprintRoutersMock
} = vi.hoisted(() => ({
  listTlsFingerprintProfilesMock: vi.fn(),
  listTlsFingerprintRoutersMock: vi.fn()
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      bulkUpdate: vi.fn(),
      checkMixedChannelRisk: vi.fn()
    }
  }
}))

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

vi.mock('@/api/admin/tlsFingerprintProfile', () => ({
  list: listTlsFingerprintProfilesMock
}))

vi.mock('@/api/admin/tlsFingerprintRouter', () => ({
  list: listTlsFingerprintRoutersMock
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountModal(extraProps: Record<string, unknown> = {}) {
  return mount(BulkEditAccountModal, {
    props: {
      show: true,
      accountIds: [1, 2],
      selectedPlatforms: ['antigravity'],
      selectedTypes: ['apikey'],
      proxies: [],
      groups: [],
      ...extraProps
    } as any,
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        ConfirmDialog: true,
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue'],
          template: `
            <select
              v-bind="$attrs"
              :value="modelValue"
              @change="$emit('update:modelValue', $event.target.value)"
            >
              <option v-for="option in options" :key="option.value" :value="option.value">
                {{ option.label }}
              </option>
            </select>
          `
        },
        ProxySelector: true,
        GroupSelector: true,
        Icon: true
      }
    }
  })
}

describe('BulkEditAccountModal', () => {
  beforeEach(() => {
    vi.mocked(adminAPI.accounts.bulkUpdate).mockReset()
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockReset()
    listTlsFingerprintProfilesMock.mockReset()
    listTlsFingerprintRoutersMock.mockReset()

    vi.mocked(adminAPI.accounts.bulkUpdate).mockResolvedValue({
      success: 2,
      failed: 0,
      results: []
    } as any)
    vi.mocked(adminAPI.accounts.checkMixedChannelRisk).mockResolvedValue({
      has_risk: false
    } as any)
    listTlsFingerprintProfilesMock.mockResolvedValue([])
    listTlsFingerprintRoutersMock.mockResolvedValue([{ id: 9, name: 'UA Router', enabled: true }])
  })

  it('antigravity 白名单包含 Gemini 图片模型且过滤掉普通 GPT 模型', async () => {
    const wrapper = mountModal()
    const selector = wrapper.findComponent(ModelWhitelistSelector)
    expect(selector.exists()).toBe(true)

    await selector.find('div.cursor-pointer').trigger('click')

    expect(wrapper.text()).toContain('gemini-3.1-flash-image')
    expect(wrapper.text()).toContain('gemini-2.5-flash-image')
    expect(wrapper.text()).not.toContain('gpt-5.3-codex')
  })

  it('antigravity 映射预设包含图片映射并过滤 OpenAI 预设', async () => {
    const wrapper = mountModal()

    const mappingTab = wrapper.findAll('button').find((btn) => btn.text().includes('admin.accounts.modelMapping'))
    expect(mappingTab).toBeTruthy()
    await mappingTab!.trigger('click')

    expect(wrapper.text()).toContain('3.1-Flash-Image透传')
    expect(wrapper.text()).toContain('3-Pro-Image→3.1')
    expect(wrapper.text()).not.toContain('GPT-5.3 Codex Spark')
  })

  it('仅勾选模型限制且白名单留空时，应提交空 model_mapping 以支持所有模型', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      credentials: {
        model_mapping: {}
      }
    })
  })

  it('OpenAI 账号批量编辑可开启自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
  })

  it('Grok OAuth 批量编辑不显示 OpenAI 图片生成开关', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })

    expect(wrapper.find('#bulk-edit-openai-image-generation-enabled').exists()).toBe(false)
    expect(wrapper.find('#bulk-edit-openai-image-generation-toggle').exists()).toBe(false)
  })

  it('Bulk TLS router selector only appears for OpenAI and Grok targets', async () => {
    const openAIWrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    ;(openAIWrapper.vm as any).tlsFingerprintEnabled = true
    await openAIWrapper.vm.$nextTick()
    expect(openAIWrapper.text()).toContain('admin.accounts.quotaControl.tlsFingerprint.routerLabel')

    const grokWrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    ;(grokWrapper.vm as any).tlsFingerprintEnabled = true
    await grokWrapper.vm.$nextTick()
    expect(grokWrapper.text()).toContain('admin.accounts.quotaControl.tlsFingerprint.routerLabel')

    const anthropicWrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    ;(anthropicWrapper.vm as any).tlsFingerprintEnabled = true
    await anthropicWrapper.vm.$nextTick()
    expect(anthropicWrapper.text()).not.toContain('admin.accounts.quotaControl.tlsFingerprint.routerLabel')

    const kiroWrapper = mountModal({
      selectedPlatforms: ['kiro'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    ;(kiroWrapper.vm as any).tlsFingerprintEnabled = true
    await kiroWrapper.vm.$nextTick()
    expect(kiroWrapper.text()).not.toContain('admin.accounts.quotaControl.tlsFingerprint.routerLabel')
  })

  it('Bulk TLS router is submitted for Grok but not Anthropic or Kiro targets', async () => {
    const grokWrapper = mountModal({
      selectedPlatforms: ['grok'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    await grokWrapper.get('#bulk-edit-tls-fingerprint-enabled').setValue(true)
    ;(grokWrapper.vm as any).tlsFingerprintEnabled = true
    ;(grokWrapper.vm as any).tlsFingerprintProfileId = 15
    ;(grokWrapper.vm as any).tlsFingerprintRouterId = 9
    await grokWrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenLastCalledWith([1, 2], {
      extra: {
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 15,
        tls_fingerprint_router_id: 9,
        tls_fingerprint_bindings: null,
        tls_fingerprint_default_os: null
      }
    })

    vi.mocked(adminAPI.accounts.bulkUpdate).mockClear()
    const kiroWrapper = mountModal({
      selectedPlatforms: ['kiro'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    await kiroWrapper.get('#bulk-edit-tls-fingerprint-enabled').setValue(true)
    ;(kiroWrapper.vm as any).tlsFingerprintEnabled = true
    ;(kiroWrapper.vm as any).tlsFingerprintProfileId = 15
    ;(kiroWrapper.vm as any).tlsFingerprintRouterId = 9
    await kiroWrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenLastCalledWith([1, 2], {
      extra: {
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 15,
        tls_fingerprint_router_id: null,
        tls_fingerprint_bindings: null,
        tls_fingerprint_default_os: null
      }
    })

    vi.mocked(adminAPI.accounts.bulkUpdate).mockClear()
    const anthropicWrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    await anthropicWrapper.get('#bulk-edit-tls-fingerprint-enabled').setValue(true)
    ;(anthropicWrapper.vm as any).tlsFingerprintEnabled = true
    ;(anthropicWrapper.vm as any).tlsFingerprintProfileId = 15
    ;(anthropicWrapper.vm as any).tlsFingerprintRouterId = 9
    await anthropicWrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenLastCalledWith([1, 2], {
      extra: {
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 15,
        tls_fingerprint_router_id: null,
        tls_fingerprint_bindings: null,
        tls_fingerprint_default_os: null
      }
    })
  })

  it('does not offer TLS fingerprint bulk edit for unsupported OpenAI account types', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['setup-token']
    })

    expect(wrapper.find('#bulk-edit-tls-fingerprint-enabled').exists()).toBe(false)
  })

  it('Bulk TLS enable clears stale matrix and default OS fields so the selected profile applies', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    await wrapper.get('#bulk-edit-tls-fingerprint-enabled').setValue(true)
    ;(wrapper.vm as any).tlsFingerprintEnabled = true
    ;(wrapper.vm as any).tlsFingerprintProfileId = 15
    ;(wrapper.vm as any).tlsFingerprintRouterId = 9
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenLastCalledWith([1, 2], {
      extra: {
        enable_tls_fingerprint: true,
        tls_fingerprint_profile_id: 15,
        tls_fingerprint_router_id: 9,
        tls_fingerprint_bindings: null,
        tls_fingerprint_default_os: null
      }
    })
  })

  it('Bulk TLS disable clears stale matrix and default OS fields', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    await wrapper.get('#bulk-edit-tls-fingerprint-enabled').setValue(true)
    ;(wrapper.vm as any).tlsFingerprintEnabled = false
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenLastCalledWith([1, 2], {
      extra: {
        enable_tls_fingerprint: false,
        tls_fingerprint_profile_id: null,
        tls_fingerprint_router_id: null,
        tls_fingerprint_bindings: null,
        tls_fingerprint_default_os: null
      }
    })
  })

  it('resets bulk TLS state when modal closes', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })
    await flushPromises()
    ;(wrapper.vm as any).enableTLSFingerprint = true
    ;(wrapper.vm as any).tlsFingerprintEnabled = true
    ;(wrapper.vm as any).tlsFingerprintProfileId = 15
    ;(wrapper.vm as any).tlsFingerprintRouterId = 9

    await wrapper.setProps({ show: false })
    await flushPromises()

    expect((wrapper.vm as any).enableTLSFingerprint).toBe(false)
    expect((wrapper.vm as any).tlsFingerprintEnabled).toBe(false)
    expect((wrapper.vm as any).tlsFingerprintProfileId).toBeNull()
    expect((wrapper.vm as any).tlsFingerprintRouterId).toBeNull()
  })

  it('OpenAI API Key 仅支持 embeddings 时不显示文本端点自动转换开关', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey'],
      target: {
        mode: 'selected',
        accountIds: [1, 2],
        selectedPlatforms: ['openai'],
        selectedTypes: ['apikey'],
        textEndpointAutoRouteConfigurable: false
      }
    })

    expect(wrapper.find('#bulk-edit-text-endpoint-auto-route-enabled').exists()).toBe(false)
  })

  it('OpenAI OAuth 批量编辑应提交 OAuth 专属 WS mode 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-ws-mode-select"]').setValue('passthrough')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_oauth_responses_websockets_v2_mode: 'passthrough',
        openai_oauth_responses_websockets_v2_enabled: true
      }
    })
  })

  it('OpenAI API Key 批量编辑不显示 WS mode 入口', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    expect(wrapper.find('#bulk-edit-openai-ws-mode-enabled').exists()).toBe(false)
  })

  it('OpenAI OAuth 批量编辑应提交 codex_cli_only 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true
      }
    })
  })

  it('OpenAI OAuth 批量编辑应提交 codex_cli_only_allow_app_server 字段（需同时开启父开关）', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    // 子开关从属于 codex_cli_only：必须同时批量开启父开关才写入
    await wrapper.get('#bulk-edit-openai-codex-cli-only-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-cli-only-toggle').trigger('click')
    await wrapper.get('#bulk-edit-openai-codex-app-server-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-app-server-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        codex_cli_only: true,
        codex_cli_only_allow_app_server: true
      }
    })
  })

  it('未同时开启父开关时不应写入 codex_cli_only_allow_app_server', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    // 仅开启子开关、不批量设置父开关 codex_cli_only：不应写入孤立字段，也不应调用接口
    await wrapper.get('#bulk-edit-openai-codex-app-server-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-codex-app-server-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
  })

  it('OpenAI API Key 批量编辑应提交 API Key 专属 WS mode 字段', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-openai-apikey-ws-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-apikey-ws-mode-select"]').setValue('ctx_pool')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_apikey_responses_websockets_v2_mode: 'ctx_pool',
        openai_apikey_responses_websockets_v2_enabled: true
      }
    })
  })

  it('批量编辑应提交账号级文本端点自动转换开关', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['anthropic'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-text-endpoint-auto-route-enabled').setValue(true)
    await wrapper.get('#bulk-edit-text-endpoint-auto-route-toggle').trigger('click')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        text_endpoint_auto_route: true
      }
    })
  })

  it('批量编辑应提交关闭文本端点自动转换开关的 false 值', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-text-endpoint-auto-route-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        text_endpoint_auto_route: false
      }
    })
  })

  it('批量编辑不向非文本转换平台显示文本端点自动转换开关', () => {
    const wrapper = mountModal({
      selectedPlatforms: ['sora'],
      selectedTypes: ['apikey']
    })

    expect(wrapper.find('#bulk-edit-text-endpoint-auto-route-enabled').exists()).toBe(false)
    expect(wrapper.find('#bulk-edit-text-endpoint-auto-route-toggle').exists()).toBe(false)
  })

  it('批量编辑保存前拒绝小数 custom_error_codes', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    ;(wrapper.vm as any).enableCustomErrorCodes = true
    ;(wrapper.vm as any).selectedErrorCodes = [502.5]

    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).not.toHaveBeenCalled()
  })

  it('筛选 OpenAI 账号批量编辑应提交 Compact 模式和专属模型映射', async () => {
    const wrapper = mountModal({
      accountIds: [],
      selectedPlatforms: [],
      selectedTypes: [],
      target: {
        mode: 'filtered',
        filters: { platform: 'openai' },
        previewCount: 12,
        selectedPlatforms: ['openai'],
        selectedTypes: ['oauth', 'apikey']
      }
    })

    await wrapper.get('#bulk-edit-openai-compact-mode-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-compact-mode-select"]').setValue('force_on')
    await wrapper.get('#bulk-edit-openai-compact-model-mapping-enabled').setValue(true)
    await wrapper.get('[data-testid="bulk-edit-openai-compact-model-mapping-add"]').trigger('click')
    const inputs = wrapper.findAll('[data-testid="bulk-edit-openai-compact-model-mapping-input"]')
    await inputs[0].setValue('gpt-5.4')
    await inputs[1].setValue('gpt-5.4-openai-compact')
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: { platform: 'openai' },
      extra: {
        openai_compact_mode: 'force_on'
      },
      credentials: {
        compact_model_mapping: {
          'gpt-5.4': 'gpt-5.4-openai-compact'
        }
      }
    })
  })

  it('OpenAI 账号批量编辑可关闭自动透传', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['apikey']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: false,
        openai_oauth_passthrough: false
      }
    })
  })

  it('开启 OpenAI 自动透传时不再同时提交模型限制', async () => {
    const wrapper = mountModal({
      selectedPlatforms: ['openai'],
      selectedTypes: ['oauth']
    })

    await wrapper.get('#bulk-edit-openai-passthrough-enabled').setValue(true)
    await wrapper.get('#bulk-edit-openai-passthrough-toggle').trigger('click')
    await wrapper.get('#bulk-edit-model-restriction-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith([1, 2], {
      extra: {
        openai_passthrough: true
      }
    })
    expect(wrapper.text()).toContain('admin.accounts.openai.modelRestrictionDisabledByPassthrough')
  })

  it('filtered-results 模式下应提交 filters 而不是 account_ids', async () => {
    const wrapper = mountModal({
      accountIds: [],
      target: {
        mode: 'filtered',
        filters: {
          platform: 'openai',
          type: 'oauth',
          status: 'active',
          group: '12',
          search: 'bulk-target',
          privacy_mode: 'training_set_cf_blocked'
        },
        previewCount: 5,
        selectedPlatforms: ['openai'],
        selectedTypes: ['oauth']
      }
    })

    await wrapper.get('#bulk-edit-status-enabled').setValue(true)
    await wrapper.get('#bulk-edit-account-form').trigger('submit.prevent')
    await flushPromises()

    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledTimes(1)
    expect(adminAPI.accounts.bulkUpdate).toHaveBeenCalledWith({
      filters: {
        platform: 'openai',
        type: 'oauth',
        status: 'active',
        group: '12',
        search: 'bulk-target',
        privacy_mode: 'training_set_cf_blocked'
      },
      status: 'active'
    })
  })
})
