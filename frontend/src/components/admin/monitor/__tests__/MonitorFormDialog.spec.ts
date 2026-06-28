import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import type { ChannelMonitorTemplate } from '@/api/admin/channelMonitorTemplate'
import MonitorFormDialog from '../MonitorFormDialog.vue'

const {
  showErrorMock,
  showSuccessMock,
  createMonitorMock,
  updateMonitorMock,
  listTemplatesMock,
} = vi.hoisted(() => ({
  showErrorMock: vi.fn(),
  showSuccessMock: vi.fn(),
  createMonitorMock: vi.fn(),
  updateMonitorMock: vi.fn(),
  listTemplatesMock: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    cachedPublicSettings: {
      channel_monitor_default_interval_seconds: 60,
    },
    showError: showErrorMock,
    showSuccess: showSuccessMock,
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: {
      create: createMonitorMock,
      update: updateMonitorMock,
    },
    channelMonitorTemplate: {
      list: listTemplatesMock,
    },
  },
}))

vi.mock('@/api/keys', () => ({
  keysAPI: {
    list: vi.fn(),
  },
}))

vi.mock('@/api/groups', () => ({
  userGroupsAPI: {
    getUserGroupRates: vi.fn(),
  },
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const messages: Record<string, string> = {
    'admin.channelMonitor.createTitle': 'Create Monitor',
    'admin.channelMonitor.editTitle': 'Edit Monitor',
    'admin.channelMonitor.form.name': 'Name',
    'admin.channelMonitor.form.endpoint': 'Endpoint',
    'admin.channelMonitor.form.apiKey': 'API Key',
    'admin.channelMonitor.form.primaryModel': 'Primary Model',
    'admin.channelMonitor.form.groupName': 'Group Name',
    'admin.channelMonitor.form.intervalSeconds': 'Interval',
    'admin.channelMonitor.form.enabled': 'Enabled',
    'admin.channelMonitor.form.apiMode': 'API Mode',
    'admin.channelMonitor.form.apiModeChatCompletions': 'Chat Completions',
    'admin.channelMonitor.form.apiModeChatCompletionsHint': 'Use the chat completions endpoint',
    'admin.channelMonitor.form.apiModeResponses': 'Responses',
    'admin.channelMonitor.form.apiModeResponsesHint': 'Use the responses endpoint',
    'monitorCommon.providers.openai': 'OpenAI',
    'monitorCommon.providers.anthropic': 'Anthropic',
    'monitorCommon.providers.gemini': 'Gemini',
    'monitorCommon.providers.kiro': 'Kiro',
    'monitorCommon.providers.grok': 'Grok',
    'common.update': 'Update',
    'common.create': 'Create',
    'common.cancel': 'Cancel',
    'common.submitting': 'Submitting',
    'common.error': 'Error',
    'admin.channelMonitor.createSuccess': 'Created',
    'admin.channelMonitor.updateSuccess': 'Updated',
    'admin.channelMonitor.templateField.none': 'No template',
  }

  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => messages[key] ?? key,
    }),
  }
})

const BaseDialogStub = defineComponent({
  name: 'BaseDialog',
  props: {
    show: {
      type: Boolean,
      default: false,
    },
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const ToggleStub = defineComponent({
  name: 'Toggle',
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ['update:modelValue'],
  template: '<div class="toggle-stub" :data-model-value="String(modelValue)" />',
})

const SelectStub = defineComponent({
  name: 'SelectStub',
  props: {
    modelValue: {
      type: String,
      default: '',
    },
    options: {
      type: Array,
      default: () => [],
    },
  },
  emits: ['update:modelValue'],
  template: `
    <select
      data-testid="template-select"
      :value="modelValue"
      @change="$emit('update:modelValue', $event.target.value)"
    >
      <option
        v-for="option in options"
        :key="String(option.value)"
        :value="String(option.value)"
      >
        {{ option.label }}
      </option>
    </select>
  `,
})

const ModelTagInputStub = defineComponent({
  name: 'ModelTagInput',
  props: {
    models: {
      type: Array,
      default: () => [],
    },
  },
  template: '<div class="model-tag-input-stub" :data-models="JSON.stringify(models)" />',
})

const MonitorKeyPickerDialogStub = defineComponent({
  name: 'MonitorKeyPickerDialog',
  template: '<div class="key-picker-stub" />',
})

const MonitorAdvancedRequestConfigStub = defineComponent({
  name: 'MonitorAdvancedRequestConfig',
  props: {
    provider: {
      type: String,
      default: '',
    },
    apiMode: {
      type: String,
      default: '',
    },
  },
  template: '<div class="advanced-config-stub" :data-provider="provider" :data-api-mode="apiMode" />',
})

const ProviderIconStub = defineComponent({
  name: 'ProviderIcon',
  template: '<span class="provider-icon-stub" />',
})

function buildMonitor(overrides: Partial<ChannelMonitor> = {}): ChannelMonitor {
  return {
    id: 42,
    name: 'OpenAI Monitor',
    provider: 'openai',
    api_mode: 'chat_completions',
    endpoint: 'https://api.openai.com',
    api_key_masked: 'sk-t***',
    primary_model: 'gpt-4o-mini',
    extra_models: [],
    group_name: '',
    enabled: true,
    interval_seconds: 60,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-05-20T00:00:00Z',
    updated_at: '2026-05-20T00:00:00Z',
    primary_status: '',
    primary_latency_ms: null,
    availability_7d: 100,
    extra_models_status: [],
    template_id: 1,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    ...overrides,
  }
}

function mountDialog(monitor: ChannelMonitor | null = null) {
  return mount(MonitorFormDialog, {
    props: {
      show: true,
      monitor,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        Toggle: ToggleStub,
        Select: SelectStub,
        ModelTagInput: ModelTagInputStub,
        MonitorKeyPickerDialog: MonitorKeyPickerDialogStub,
        MonitorAdvancedRequestConfig: MonitorAdvancedRequestConfigStub,
        ProviderIcon: ProviderIconStub,
      },
    },
  })
}

async function clickButtonByText(wrapper: ReturnType<typeof mountDialog>, text: string) {
  const button = wrapper
    .findAll('button')
    .find((candidate) => candidate.text().includes(text))

  expect(button, `button containing "${text}"`).toBeTruthy()
  await button!.trigger('click')
}

function createDeferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((res) => {
    resolve = res
  })
  return { promise, resolve }
}

describe('MonitorFormDialog', () => {
  beforeEach(() => {
    showErrorMock.mockReset()
    showSuccessMock.mockReset()
    createMonitorMock.mockReset()
    updateMonitorMock.mockReset()
    listTemplatesMock.mockReset()
    createMonitorMock.mockResolvedValue(buildMonitor({ template_id: null }))
    updateMonitorMock.mockResolvedValue(buildMonitor({ template_id: null }))
    listTemplatesMock.mockResolvedValue({ items: [] })
  })

  it('wires the selected OpenAI api_mode through advanced config props and the create payload', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await clickButtonByText(wrapper, 'OpenAI')
    await clickButtonByText(wrapper, 'Responses')

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('Responses Monitor')
    await inputs[1].setValue('https://api.openai.com')
    await inputs[2].setValue('sk-test')
    await inputs[3].setValue('gpt-4o-mini')

    const advanced = wrapper.find('.advanced-config-stub')
    expect(advanced.attributes('data-provider')).toBe('openai')
    expect(advanced.attributes('data-api-mode')).toBe('responses')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMonitorMock).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'openai',
      api_mode: 'responses',
      endpoint: 'https://api.openai.com',
      primary_model: 'gpt-4o-mini',
    }))
  })

  it('creates Grok monitors with chat completions mode', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await clickButtonByText(wrapper, 'Grok')

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('Grok Monitor')
    await inputs[1].setValue('https://api.x.ai')
    await inputs[2].setValue('xai-key')
    await inputs[3].setValue('grok-4.3')

    const advanced = wrapper.find('.advanced-config-stub')
    expect(advanced.attributes('data-provider')).toBe('grok')
    expect(advanced.attributes('data-api-mode')).toBe('chat_completions')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMonitorMock).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'grok',
      api_mode: 'chat_completions',
      endpoint: 'https://api.x.ai',
      primary_model: 'grok-4.3',
    }))
  })

  it('keeps OpenAI template selection aligned with the current api_mode during edit', async () => {
    listTemplatesMock.mockResolvedValue({
      items: [
        {
          id: 1,
          name: 'Chat Template',
          provider: 'openai',
          api_mode: 'chat_completions',
          description: '',
          extra_headers: {},
          body_override_mode: 'off',
          body_override: null,
          created_at: '2026-05-20T00:00:00Z',
          updated_at: '2026-05-20T00:00:00Z',
          associated_monitors: 1,
        },
        {
          id: 2,
          name: 'Responses Template',
          provider: 'openai',
          api_mode: 'responses',
          description: '',
          extra_headers: {},
          body_override_mode: 'off',
          body_override: null,
          created_at: '2026-05-20T00:00:00Z',
          updated_at: '2026-05-20T00:00:00Z',
          associated_monitors: 0,
        },
        {
          id: 3,
          name: 'Anthropic Template',
          provider: 'anthropic',
          api_mode: 'chat_completions',
          description: '',
          extra_headers: {},
          body_override_mode: 'off',
          body_override: null,
          created_at: '2026-05-20T00:00:00Z',
          updated_at: '2026-05-20T00:00:00Z',
          associated_monitors: 0,
        },
      ],
    })

    const wrapper = mountDialog(buildMonitor())
    await flushPromises()

    let selectControl = wrapper.getComponent(SelectStub)
    let select = wrapper.get('[data-testid="template-select"]')
    let optionLabels = select.findAll('option').map((option) => option.text())
    expect(optionLabels).toEqual(['No template', 'Chat Template'])
    expect(selectControl.props('modelValue')).toBe('1')

    await clickButtonByText(wrapper, 'Responses')
    await flushPromises()

    selectControl = wrapper.getComponent(SelectStub)
    select = wrapper.get('[data-testid="template-select"]')
    optionLabels = select.findAll('option').map((option) => option.text())
    expect(optionLabels).toEqual(['No template', 'Responses Template'])
    expect(selectControl.props('modelValue')).toBe('')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateMonitorMock).toHaveBeenNthCalledWith(1, 42, expect.objectContaining({
      provider: 'openai',
      api_mode: 'responses',
      clear_template: true,
    }))
    expect(updateMonitorMock.mock.calls[0]?.[1]).not.toHaveProperty('template_id')

    updateMonitorMock.mockClear()

    await select.setValue('2')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(updateMonitorMock).toHaveBeenCalledWith(42, expect.objectContaining({
      provider: 'openai',
      api_mode: 'responses',
      template_id: 2,
    }))
    expect(updateMonitorMock.mock.calls[0]?.[1]).not.toHaveProperty('clear_template')
  })

  it('waits for template loading before submitting an edited OpenAI monitor', async () => {
    const deferred = createDeferred<{ items: ChannelMonitorTemplate[] }>()
    listTemplatesMock.mockReturnValueOnce(deferred.promise)

    const wrapper = mountDialog(buildMonitor({ template_id: 1 }))
    await flushPromises()

    await clickButtonByText(wrapper, 'Responses')
    await wrapper.find('form').trigger('submit.prevent')

    expect(updateMonitorMock).not.toHaveBeenCalled()

    deferred.resolve({
      items: [
        {
          id: 1,
          name: 'Chat Template',
          provider: 'openai',
          api_mode: 'chat_completions',
          description: '',
          extra_headers: {},
          body_override_mode: 'off',
          body_override: null,
          created_at: '2026-05-20T00:00:00Z',
          updated_at: '2026-05-20T00:00:00Z',
          associated_monitors: 1,
        },
      ],
    })
    await flushPromises()

    expect(updateMonitorMock).toHaveBeenCalledWith(42, expect.objectContaining({
      provider: 'openai',
      api_mode: 'responses',
      clear_template: true,
    }))
    expect(updateMonitorMock.mock.calls[0]?.[1]).not.toHaveProperty('template_id')
  })

  it('clears copied advanced request config when switching OpenAI api_mode after applying a template', async () => {
    listTemplatesMock.mockResolvedValue({
      items: [
        {
          id: 2,
          name: 'Responses Template',
          provider: 'openai',
          api_mode: 'responses',
          description: '',
          extra_headers: { 'x-template-mode': 'responses' },
          body_override_mode: 'replace',
          body_override: {
            model: 'gpt-5.5',
            instructions: 'Return JSON.',
            input: 'hello',
          },
          created_at: '2026-05-20T00:00:00Z',
          updated_at: '2026-05-20T00:00:00Z',
          associated_monitors: 0,
        },
      ],
    })

    const wrapper = mountDialog()
    await flushPromises()

    await clickButtonByText(wrapper, 'OpenAI')
    await clickButtonByText(wrapper, 'Responses')

    const select = wrapper.get('[data-testid="template-select"]')
    await select.setValue('2')
    await flushPromises()

    await clickButtonByText(wrapper, 'Chat Completions')

    const inputs = wrapper.findAll('input')
    await inputs[0].setValue('Chat Monitor')
    await inputs[1].setValue('https://api.openai.com')
    await inputs[2].setValue('sk-test')
    await inputs[3].setValue('gpt-4o-mini')

    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(createMonitorMock).toHaveBeenCalledWith(expect.objectContaining({
      provider: 'openai',
      api_mode: 'chat_completions',
      template_id: null,
      extra_headers: {},
      body_override_mode: 'off',
      body_override: null,
    }))
  })
})
