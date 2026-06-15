import { flushPromises, mount } from '@vue/test-utils'
import { defineComponent } from 'vue'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { ChannelMonitor } from '@/api/admin/channelMonitor'
import MonitorAvailabilityAdjustDialog from '../MonitorAvailabilityAdjustDialog.vue'

const {
  adjustAvailabilityMock,
  showErrorMock,
} = vi.hoisted(() => ({
  adjustAvailabilityMock: vi.fn(),
  showErrorMock: vi.fn(),
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    channelMonitor: {
      adjustAvailability7d: adjustAvailabilityMock,
    },
  },
}))

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: showErrorMock,
  }),
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  const messages: Record<string, string> = {
    'admin.channelMonitor.adjustAvailability.title': 'Adjust availability',
    'admin.channelMonitor.adjustAvailability.current': 'Current {value}',
    'admin.channelMonitor.adjustAvailability.target': 'Target',
    'admin.channelMonitor.adjustAvailability.placeholder': 'Target percent',
    'admin.channelMonitor.adjustAvailability.hint': 'Approximate',
    'admin.channelMonitor.adjustAvailability.submit': 'Apply',
    'admin.channelMonitor.adjustAvailability.invalid': 'Invalid',
    'admin.channelMonitor.adjustAvailability.failed': 'Failed',
    'common.cancel': 'Cancel',
    'common.submitting': 'Submitting',
  }
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) => {
        const value = messages[key] ?? key
        if (!params) return value
        return Object.entries(params).reduce(
          (acc, [name, replacement]) => acc.replace(`{${name}}`, replacement),
          value
        )
      },
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
  emits: ['close'],
  template: '<div v-if="show"><slot /><slot name="footer" /></div>',
})

function buildMonitor(overrides: Partial<ChannelMonitor> = {}): ChannelMonitor {
  return {
    id: 42,
    name: 'Claude Monitor',
    provider: 'anthropic',
    api_mode: 'chat_completions',
    endpoint: 'https://api.anthropic.com',
    api_key_masked: 'sk-a***',
    primary_model: 'claude-sonnet-4',
    extra_models: [],
    group_name: '',
    enabled: true,
    interval_seconds: 60,
    last_checked_at: null,
    created_by: 1,
    created_at: '2026-06-01T00:00:00Z',
    updated_at: '2026-06-01T00:00:00Z',
    primary_status: 'operational',
    primary_latency_ms: 500,
    availability_7d: 88.8,
    extra_models_status: [],
    template_id: null,
    extra_headers: {},
    body_override_mode: 'off',
    body_override: null,
    ...overrides,
  }
}

function mountDialog(monitor: ChannelMonitor = buildMonitor()) {
  return mount(MonitorAvailabilityAdjustDialog, {
    props: {
      show: true,
      monitor,
    },
    global: {
      stubs: {
        BaseDialog: BaseDialogStub,
      },
    },
  })
}

describe('MonitorAvailabilityAdjustDialog', () => {
  beforeEach(() => {
    adjustAvailabilityMock.mockReset()
    showErrorMock.mockReset()
  })

  it('submits zero percent as availability_pct', async () => {
    adjustAvailabilityMock.mockResolvedValue({
      monitor_id: 42,
      model: 'claude-sonnet-4',
      total_checks: 10,
      previous_operational_checks: 8,
      target_operational_checks: 0,
      actual_operational_checks: 0,
      changed_rows: 8,
      previous_availability_pct: 80,
      requested_availability_pct: 0,
      actual_availability_pct: 0,
    })
    const wrapper = mountDialog()

    await wrapper.find('input[type="number"]').setValue('0')
    await wrapper.find('form').trigger('submit.prevent')
    await flushPromises()

    expect(adjustAvailabilityMock).toHaveBeenCalledWith(42, {
      availability_pct: 0,
    })
    expect(wrapper.emitted('adjusted')?.[0]?.[0]).toMatchObject({
      actual_availability_pct: 0,
      changed_rows: 8,
    })
  })
})
