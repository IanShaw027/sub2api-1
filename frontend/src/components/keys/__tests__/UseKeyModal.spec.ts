import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import { nextTick } from 'vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

vi.mock('@/composables/useClipboard', () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn().mockResolvedValue(true)
  })
}))

import UseKeyModal from '../UseKeyModal.vue'

describe('UseKeyModal', () => {
  it('renders minimal API-key Codex config through experimental_bearer_token', () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('model_provider = "sub2api"'))
    const allCode = codeBlocks.join('\n')

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('[model_providers.sub2api]')
    expect(configToml).toContain('base_url = "https://example.com/v1"')
    expect(configToml).toContain('experimental_bearer_token = "sk-test"')
    expect(configToml).toContain('wire_api = "responses"')
    expect(configToml).toContain('requires_openai_auth = true')
    expect(configToml).toContain('# review_model = "gpt-5.5"')
    expect(configToml).toContain('# model_reasoning_effort = "xhigh"')
    expect(configToml).toContain('# goals = true')
    expect(configToml).not.toMatch(/^review_model = "gpt-5\.5"$/m)
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(allCode).not.toContain('OPENAI_API_KEY')
  })

  it('normalizes root base URL to /v1 for OpenAI Codex config', () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const configToml = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('[model_providers.sub2api]'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('base_url = "https://example.com/v1"')
  })

  it('normalizes /v1 base URL back to root for Anthropic configs', () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-claude',
        baseUrl: 'https://example.com/v1',
        platform: 'anthropic'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const allCode = wrapper.findAll('pre code').map((code) => code.text()).join('\n')

    expect(allCode).toContain('ANTHROPIC_BASE_URL="https://example.com"')
    expect(allCode).toContain('"ANTHROPIC_BASE_URL": "https://example.com"')
    expect(allCode).not.toContain('https://example.com/v1')
  })

  it('renders minimal API-key Codex WebSocket config through experimental_bearer_token', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const wsTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.codexCliWs')
    )

    expect(wsTab).toBeDefined()
    await wsTab!.trigger('click')
    await nextTick()

    const codeBlocks = wrapper.findAll('pre code').map((code) => code.text())
    const configToml = codeBlocks.find((content) => content.includes('supports_websockets = true'))

    expect(configToml).toBeDefined()
    expect(configToml).toContain('model = "gpt-5.5"')
    expect(configToml).toContain('model_provider = "sub2api"')
    expect(configToml).toContain('[model_providers.sub2api]')
    expect(configToml).toContain('experimental_bearer_token = "sk-test"')
    expect(configToml).toContain('supports_websockets = true')
    expect(configToml).toContain('# review_model = "gpt-5.5"')
    expect(configToml).toContain('# goals = true')
    expect(configToml).not.toMatch(/^review_model = "gpt-5\.5"$/m)
    expect(configToml).not.toContain('model = "gpt-5.4"')
    expect(configToml).not.toContain('model_context_window')
    expect(configToml).not.toContain('model_auto_compact_token_limit')
    expect(configToml).toContain('[features]\nresponses_websockets_v2 = true\n# goals = true')
  })

  it('renders GPT-5.4 mini entry in OpenCode config', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'openai'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const codeBlock = wrapper.find('pre code')
    expect(codeBlock.exists()).toBe(true)
    expect(codeBlock.text()).toContain('"name": "GPT-5.4 Mini"')
    expect(codeBlock.text()).not.toContain('"name": "GPT-5.4 Nano"')
  })

  it('renders Claude Fable 5 OpenCode config with adaptive thinking', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-test',
        baseUrl: 'https://example.com/v1',
        platform: 'antigravity'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const opencodeTab = wrapper.findAll('button').find((button) =>
      button.text().includes('keys.useKeyModal.cliTabs.opencode')
    )

    expect(opencodeTab).toBeDefined()
    await opencodeTab!.trigger('click')
    await nextTick()

    const claudeConfig = wrapper.findAll('pre code')
      .map((code) => code.text())
      .find((content) => content.includes('"antigravity-claude"'))

    expect(claudeConfig).toBeDefined()
    const parsed = JSON.parse(claudeConfig!)
    const fable = parsed.provider['antigravity-claude'].models['claude-fable-5']

    expect(fable.name).toBe('Claude Fable 5')
    expect(fable.limit).toEqual({ context: 1048576, output: 128000 })
    expect(fable.options.thinking).toEqual({ type: 'adaptive' })
    expect(fable.options.thinking).not.toHaveProperty('budgetTokens')
  })



  it('uses Windows Grok config paths for cmd and powershell tabs', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-grok',
        baseUrl: 'https://example.com/v1',
        platform: 'grok'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    const clickTab = async (label: string) => {
      const tab = wrapper.findAll('button').find((button) => button.text().includes(label))
      expect(tab).toBeDefined()
      await tab!.trigger('click')
      await nextTick()
    }

    await clickTab('Windows CMD')
    expect(wrapper.text()).toContain('%userprofile%\\.grok/config.toml')

    await clickTab('PowerShell')
    expect(wrapper.text()).toContain('%userprofile%\\.grok/config.toml')
  })

  it('defaults Grok to Grok CLI and exposes Codex / Claude Code / OpenCode tabs', async () => {
    const wrapper = mount(UseKeyModal, {
      props: {
        show: true,
        apiKey: 'sk-grok',
        baseUrl: 'https://example.com/v1',
        platform: 'grok'
      },
      global: {
        stubs: {
          BaseDialog: {
            template: '<div><slot /><slot name="footer" /></div>'
          },
          Icon: {
            template: '<span />'
          }
        }
      }
    })

    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.grokCli')
    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.codexCli')
    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.claudeCode')
    expect(wrapper.text()).toContain('keys.useKeyModal.cliTabs.opencode')

    const grokCode = wrapper.findAll('pre code').map((code) => code.text()).join('\n')
    expect(grokCode).toContain('GROK_MODELS_BASE_URL')
    expect(grokCode).toContain('XAI_API_KEY')
    expect(grokCode).toContain('[model.grok-4.5]')
    expect(grokCode).toContain('default = "grok-4.5"')

    const clickTab = async (label: string) => {
      const tab = wrapper.findAll('button').find((button) => button.text().includes(label))
      expect(tab).toBeDefined()
      await tab!.trigger('click')
      await nextTick()
    }

    await clickTab('keys.useKeyModal.cliTabs.claudeCode')
    const claudeCode = wrapper.findAll('pre code').map((code) => code.text()).join('\n')
    expect(claudeCode).toContain('ANTHROPIC_BASE_URL')
    expect(claudeCode).toContain('ANTHROPIC_AUTH_TOKEN')
    expect(claudeCode).toContain('ANTHROPIC_MODEL="grok-4.5"')
    expect(claudeCode).not.toContain('GROK_MODELS_BASE_URL')

    await clickTab('keys.useKeyModal.cliTabs.codexCli')
    const codexCode = wrapper.findAll('pre code').map((code) => code.text()).join('\n')
    expect(codexCode).toContain('model_provider = "sub2api"')
    expect(codexCode).toContain('model = "grok-4.5"')
    expect(codexCode).toContain('wire_api = "responses"')
    expect(codexCode).toContain('supports_websockets = false')
    expect(codexCode).toContain('experimental_bearer_token = "sk-grok"')

    await clickTab('keys.useKeyModal.cliTabs.opencode')
    const openCode = wrapper.findAll('pre code').map((code) => code.text()).join('\n')
    expect(openCode).toContain('"baseURL": "https://example.com/v1"')
    expect(openCode).toContain('"apiKey": "sk-grok"')
    expect(openCode).toContain('"grok"')
    expect(openCode).toContain('"grok-4.5"')
    expect(openCode).toContain('"grok-4.3"')
    expect(openCode).toContain('"grok-build-0.1"')
    expect(openCode).toContain('"grok-latest"')
    // Must not reuse OpenAI GPT catalog for Grok OpenCode
    expect(openCode).not.toContain('"gpt-5.4"')
    expect(openCode).not.toContain('"gpt-5.2"')
  })
})
