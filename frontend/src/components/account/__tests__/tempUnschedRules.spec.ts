import { defineComponent, h, nextTick } from 'vue'
import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import {
  buildTempUnschedRulesResult,
  buildTempUnschedRules,
  loadTempUnschedRules,
  splitTempUnschedKeywords,
  formatTempUnschedKeywords
} from '../tempUnschedRules'
import { buildCustomErrorCodesResult } from '../customErrorCodes'
import TempUnschedRulesForm from '../TempUnschedRulesForm.vue'
import CustomErrorCodesForm from '../CustomErrorCodesForm.vue'
import ModelWhitelistSelector from '@/components/account/ModelWhitelistSelector.vue'
import PlatformDefaultAccountModelConfigForm from '@/components/admin/PlatformDefaultAccountModelConfigForm.vue'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string, params?: Record<string, number | string>) =>
      key.replace(/\{(\w+)\}/g, (_, token) => String(params?.[token] ?? `{${token}}`))
  })
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showInfo: vi.fn()
  })
}))

vi.mock('@/api/admin/accounts', () => ({
  accountsAPI: {
    syncUpstreamModels: vi.fn()
  }
}))

const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false
    }
  },
  emits: ['update:modelValue'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        type: 'checkbox',
        class: 'toggle-stub',
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit('update:modelValue', (event.target as HTMLInputElement).checked)
        }
      })
  }
})

const componentStubs = {
  Icon: true,
  Toggle: ToggleStub,
  ModelIcon: true
}

describe('tempUnschedRules helpers', () => {
  it('builds rules in order and skips invalid entries', () => {
    const rules = buildTempUnschedRules([
      {
        error_code: 502,
        keywords: 'Upstream request failed, overloaded',
        duration_minutes: 10,
        description: 'svc'
      },
      // 错误码越界 → 跳过
      { error_code: 99, keywords: 'x', duration_minutes: 10, description: '' },
      // duration 非法 → 跳过
      { error_code: 503, keywords: 'y', duration_minutes: 0, description: '' },
      // keywords 留空 → 仍保留（纯错误码匹配）
      { error_code: 524, keywords: '', duration_minutes: 10, description: '' }
    ])

    expect(rules).toEqual([
      {
        error_code: 502,
        keywords: ['Upstream request failed', 'overloaded'],
        duration_minutes: 10,
        description: 'svc'
      },
      {
        error_code: 524,
        keywords: [],
        duration_minutes: 10,
        description: ''
      }
    ])
  })

  it('reports invalid rows so callers do not silently save a partial rule set', () => {
    const result = buildTempUnschedRulesResult([
      {
        error_code: 524,
        keywords: '',
        duration_minutes: 10,
        description: 'valid'
      },
      {
        error_code: 99,
        keywords: 'invalid',
        duration_minutes: 10,
        description: 'bad status'
      }
    ])

    expect(result.invalid).toBe(true)
    expect(result.rules).toEqual([
      {
        error_code: 524,
        keywords: [],
        duration_minutes: 10,
        description: 'valid'
      }
    ])
  })

  it('marks fractional, non-finite, empty, and out-of-range numeric fields invalid while preserving boundaries', () => {
    const result = buildTempUnschedRulesResult([
      { error_code: 100, keywords: '', duration_minutes: 1, description: 'lower bound' },
      { error_code: 599, keywords: '', duration_minutes: 30, description: 'upper bound' },
      { error_code: 502.5, keywords: '', duration_minutes: 10, description: 'fractional code' },
      { error_code: Number.NaN, keywords: '', duration_minutes: 10, description: 'nan code' },
      { error_code: Number.POSITIVE_INFINITY, keywords: '', duration_minutes: 10, description: 'infinite code' },
      { error_code: '' as any, keywords: '', duration_minutes: 10, description: 'empty code' },
      { error_code: 99, keywords: '', duration_minutes: 10, description: 'below bound' },
      { error_code: 600, keywords: '', duration_minutes: 10, description: 'above bound' },
      { error_code: 502, keywords: '', duration_minutes: 10.5, description: 'fractional duration' },
      { error_code: 502, keywords: '', duration_minutes: Number.NaN, description: 'nan duration' },
      { error_code: 502, keywords: '', duration_minutes: Number.POSITIVE_INFINITY, description: 'infinite duration' },
      { error_code: 502, keywords: '', duration_minutes: '' as any, description: 'empty duration' }
    ])

    expect(result.invalid).toBe(true)
    expect(result.rules).toEqual([
      {
        error_code: 100,
        keywords: [],
        duration_minutes: 1,
        description: 'lower bound'
      },
      {
        error_code: 599,
        keywords: [],
        duration_minutes: 30,
        description: 'upper bound'
      }
    ])
  })

  it('splits keywords on comma and semicolon, trimming blanks', () => {
    expect(splitTempUnschedKeywords('a, b; ,c ')).toEqual(['a', 'b', 'c'])
    expect(splitTempUnschedKeywords('')).toEqual([])
  })

  it('formats keyword arrays back into a comma string', () => {
    expect(formatTempUnschedKeywords(['a', ' b ', '', 'c'])).toBe('a, b, c')
    expect(formatTempUnschedKeywords('raw string')).toBe('raw string')
    expect(formatTempUnschedKeywords(123)).toBe('')
  })

  it('loads rules from credentials, normalizing keyword arrays to strings', () => {
    const forms = loadTempUnschedRules({
      temp_unschedulable_rules: [
        { error_code: 502, keywords: ['a', 'b'], duration_minutes: 10, description: 'd' },
        { error_code: 524, duration_minutes: 10 }
      ]
    })
    expect(forms).toEqual([
      { error_code: 502, keywords: 'a, b', duration_minutes: 10, description: 'd' },
      { error_code: 524, keywords: '', duration_minutes: 10, description: '' }
    ])
  })

  it('returns empty array when credentials lack rules', () => {
    expect(loadTempUnschedRules(undefined)).toEqual([])
    expect(loadTempUnschedRules({})).toEqual([])
    expect(loadTempUnschedRules({ temp_unschedulable_rules: 'nope' })).toEqual([])
  })
})

describe('TempUnschedRulesForm', () => {
  it('emits cloned rules when editing an existing rule without mutating props', async () => {
    const rules = [
      {
        error_code: 502,
        keywords: 'old keyword',
        duration_minutes: 10,
        description: 'old description'
      }
    ]
    const wrapper = mount(TempUnschedRulesForm, {
      props: {
        enabled: true,
        rules
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.find('input[type="number"]').setValue('503')

    expect(rules[0].error_code).toBe(502)
    expect(wrapper.emitted('update:rules')?.at(-1)?.[0]).toEqual([
      {
        error_code: 503,
        keywords: 'old keyword',
        duration_minutes: 10,
        description: 'old description'
      }
    ])
  })

  it('preserves fractional numeric input so save validation can reject it', async () => {
    const wrapper = mount(TempUnschedRulesForm, {
      props: {
        enabled: true,
        rules: [
          {
            error_code: 502,
            keywords: 'old keyword',
            duration_minutes: 10,
            description: 'old description'
          }
        ]
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.findAll('input[type="number"]')[1].setValue('10.5')

    expect(wrapper.emitted('update:rules')?.at(-1)?.[0]).toEqual([
      {
        error_code: 502,
        keywords: 'old keyword',
        duration_minutes: 10.5,
        description: 'old description'
      }
    ])
  })
})

describe('CustomErrorCodesForm', () => {
  it('marks fractional, non-finite, empty, and out-of-range codes invalid while preserving boundaries', () => {
    const result = buildCustomErrorCodesResult([
      100,
      599,
      99,
      600,
      502.5,
      Number.NaN,
      Number.POSITIVE_INFINITY,
      '',
      null
    ])

    expect(result.invalid).toBe(true)
    expect(result.codes).toEqual([100, 599])
  })

  it('does not emit fractional manual error codes', async () => {
    const wrapper = mount(CustomErrorCodesForm, {
      props: {
        enabled: true,
        codes: []
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.find('input[type="number"]').setValue('502.9')
    await wrapper.find('button.btn-secondary').trigger('click')

    expect(wrapper.emitted('update:codes')).toBeUndefined()
  })
})

describe('PlatformDefaultAccountModelConfigForm', () => {
  it('emits updated platform defaults when editing an existing temp-unsched rule field', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {
          openai: {
            temp_unschedulable_enabled: true,
            temp_unschedulable_rules: [
              {
                error_code: 502,
                keywords: ['old keyword'],
                duration_minutes: 10,
                description: 'old description'
              }
            ]
          }
        }
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.getComponent(TempUnschedRulesForm).find('input[type="number"]').setValue('503')

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      openai: {
        temp_unschedulable_enabled: true,
        temp_unschedulable_rules: [
          {
            error_code: 503,
            keywords: ['old keyword'],
            duration_minutes: 10,
            description: 'old description'
          }
        ]
      }
    })
  })

  it('emits invalid temp-unsched edits so parent validation can block save', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {
          openai: {
            temp_unschedulable_enabled: true,
            temp_unschedulable_rules: [
              {
                error_code: 502,
                keywords: ['old keyword'],
                duration_minutes: 10,
                description: 'old description'
              }
            ]
          }
        }
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.getComponent(TempUnschedRulesForm).find('input[type="number"]').setValue('99')

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      openai: {
        temp_unschedulable_enabled: true,
        temp_unschedulable_rules: [
          {
            error_code: 99,
            keywords: ['old keyword'],
            duration_minutes: 10,
            description: 'old description'
          }
        ]
      }
    })
  })

  it('edits kiro subscription type defaults through the advanced JSON editor', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {}
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger('click')
    await nextTick()

    const editor = wrapper.get('[data-testid="kiro-subscription-type-config"]')
    await editor.setValue(
      JSON.stringify({
        pro: {
          model_whitelist: ['claude-sonnet-4-6'],
          model_mapping: {
            'claude-sonnet-*': 'claude-sonnet-4.6'
          },
          compact_model_mapping: {
            'claude-sonnet-*': 'claude-haiku-4.5'
          }
        }
      })
    )

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      kiro: {
        kiro_subscription_type_model_config: {
          pro: {
            model_whitelist: ['claude-sonnet-4-6'],
            model_mapping: {
              'claude-sonnet-*': 'claude-sonnet-4.6'
            },
            compact_model_mapping: {
              'claude-sonnet-*': 'claude-haiku-4.5'
            }
          }
        }
      }
    })
  })

  it('emits Grok platform defaults from the Grok tab', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {}
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.get('[data-testid="platform-default-tab-grok"]').trigger('click')
    await nextTick()

    wrapper.getComponent(ModelWhitelistSelector).vm.$emit('update:modelValue', ['grok-4.3'])
    await nextTick()

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      grok: {
        model_whitelist: ['grok-4.3']
      }
    })
  })

  it('emits validation error without overwriting kiro subscription config when the JSON editor is malformed', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {
          kiro: {
            kiro_subscription_type_model_config: {
              pro: {
                model_mapping: {
                  'claude-sonnet-*': 'claude-sonnet-4.6'
                }
              }
            }
          }
        }
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger('click')
    await nextTick()

    await wrapper.get('[data-testid="kiro-subscription-type-config"]').setValue('{')

    expect(wrapper.emitted('validation-error')?.at(-1)?.[0]).toBe(true)
    expect(wrapper.emitted('update:modelValue')).toBeUndefined()
  })

  it('drops null kiro subscription config marker when editing other kiro defaults', async () => {
    const wrapper = mount(PlatformDefaultAccountModelConfigForm, {
      props: {
        modelValue: {
          kiro: {
            kiro_subscription_type_model_config: null as any
          }
        }
      },
      global: {
        stubs: componentStubs
      }
    })

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger('click')
    await nextTick()

    const customErrorToggle = wrapper.findAll('.toggle-stub').at(1)
    expect(customErrorToggle).toBeTruthy()
    await customErrorToggle!.setValue(true)

    expect(wrapper.emitted('update:modelValue')?.at(-1)?.[0]).toEqual({
      kiro: {
        custom_error_codes_enabled: true
      }
    })
  })
})
