import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import MonitorTemplateManagerDialog from '../MonitorTemplateManagerDialog.vue'

const { showError, showSuccess, listTemplatesMock } = vi.hoisted(() => ({
  showError: vi.fn(),
  showSuccess: vi.fn(),
  listTemplatesMock: vi.fn(),
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
  }),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitorTemplate: {
      list: listTemplatesMock,
    },
  },
}))

vi.mock('@/composables/useChannelMonitorFormat', () => ({
  useChannelMonitorFormat: () => ({
    providerPickerClass: () => '',
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => {
        const translations: Record<string, string> = {
          'monitorCommon.providers.anthropic': 'Anthropic',
          'monitorCommon.providers.openai': 'OpenAI',
          'monitorCommon.providers.gemini': 'Gemini',
          'monitorCommon.providers.kiro': 'Kiro',
          'monitorCommon.providers.grok': 'Grok',
          'admin.channelMonitor.template.managerTitle': 'Template Manager',
          'admin.channelMonitor.template.createButton': 'Create',
          'admin.channelMonitor.template.emptyState': 'No templates',
          'admin.channelMonitor.template.form.name': 'Name',
          'admin.channelMonitor.template.form.description': 'Description',
          'admin.channelMonitor.template.form.namePlaceholder': 'Name placeholder',
          'admin.channelMonitor.template.form.descriptionPlaceholder': 'Description placeholder',
          'admin.channelMonitor.form.provider': 'Provider',
          'admin.channelMonitor.form.apiMode': 'API Mode',
          'admin.channelMonitor.form.apiModeChatCompletions': 'Chat Completions',
          'admin.channelMonitor.form.apiModeChatCompletionsHint': 'Use the Chat Completions request shape.',
          'admin.channelMonitor.form.apiModeResponses': 'Responses',
          'admin.channelMonitor.form.apiModeResponsesHint': 'Use the Responses request shape.',
          'admin.channelMonitor.template.headersSummary': 'Headers: {n}',
          'admin.channelMonitor.template.applyButton': 'Apply',
          'admin.channelMonitor.template.applyTooltip': 'Apply',
          'admin.channelMonitor.template.associatedCount': 'Associated: {n}',
          'common.loading': 'Loading',
          'common.edit': 'Edit',
          'common.delete': 'Delete',
          'common.close': 'Close',
          'common.back': 'Back',
          'common.create': 'Create',
          'common.update': 'Update',
          'common.submitting': 'Submitting',
          'common.cancel': 'Cancel',
        }
        return translations[key] ?? key
      },
    }),
  }
})

const BaseDialogStub = defineComponent({
  props: {
    show: {
      type: Boolean,
      default: false,
    },
    title: {
      type: String,
      default: '',
    },
  },
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

const ConfirmDialogStub = defineComponent({
  template: '<div />',
})

const IconStub = defineComponent({
  template: '<span />',
})

const MonitorAdvancedRequestConfigStub = defineComponent({
  template: '<div class="advanced-config-stub" />',
})

const MonitorTemplateApplyPickerDialogStub = defineComponent({
  template: '<div />',
})

async function clickButtonByText(wrapper: ReturnType<typeof mountDialog>, text: string) {
  const button = wrapper
    .findAll('button')
    .find((candidate) => candidate.text().includes(text))

  expect(button, `button containing "${text}"`).toBeTruthy()
  await button!.trigger('click')
}

function mountDialog() {
  return mount(MonitorTemplateManagerDialog, {
    props: {
      show: true,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
        ConfirmDialog: ConfirmDialogStub,
        Icon: IconStub,
        MonitorAdvancedRequestConfig: MonitorAdvancedRequestConfigStub,
        MonitorTemplateApplyPickerDialog: MonitorTemplateApplyPickerDialogStub,
      },
    },
  })
}

describe('MonitorTemplateManagerDialog', () => {
  beforeEach(() => {
    showError.mockReset()
    showSuccess.mockReset()
    listTemplatesMock.mockReset()
    listTemplatesMock.mockResolvedValue({ items: [] })
  })

  it('falls back to readable API mode labels when translations are missing', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    await clickButtonByText(wrapper, 'Create')
    const openAiButtons = wrapper.findAll('button').filter((candidate) => candidate.text().includes('OpenAI'))
    expect(openAiButtons.length).toBeGreaterThan(1)
    await openAiButtons[1].trigger('click')

    expect(wrapper.text()).toContain('API Mode')
    expect(wrapper.text()).toContain('Chat Completions')
    expect(wrapper.text()).toContain('Use the Chat Completions request shape.')
    expect(wrapper.text()).toContain('Responses')
    expect(wrapper.text()).toContain('Use the Responses request shape.')
  })

  it('shows Kiro as a template provider option', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.text()).toContain('Kiro')

    await clickButtonByText(wrapper, 'Create')

    const kiroButtons = wrapper.findAll('button').filter((candidate) => candidate.text().includes('Kiro'))
    expect(kiroButtons.length).toBeGreaterThan(1)
  })

  it('shows Grok as a template provider option', async () => {
    const wrapper = mountDialog()
    await flushPromises()

    expect(wrapper.text()).toContain('Grok')

    await clickButtonByText(wrapper, 'Create')

    const grokButtons = wrapper.findAll('button').filter((candidate) => candidate.text().includes('Grok'))
    expect(grokButtons.length).toBeGreaterThan(1)
  })
})
