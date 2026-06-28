import { describe, expect, it, vi } from 'vitest'
import {
  getOverallMonitorStatus,
  providerGradient,
  useChannelMonitorFormat,
} from '@/composables/useChannelMonitorFormat'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => ({
      'monitorCommon.providers.grok': 'Grok',
    })[key] ?? key,
  }),
}))

describe('getOverallMonitorStatus', () => {
  it('ignores fresh monitors with no history when computing the overall status', () => {
    expect(getOverallMonitorStatus([
      { primary_status: '', availability_7d: null },
      { primary_status: 'operational', availability_7d: 100 },
    ])).toBe('operational')
  })

  it('still marks the view degraded when any monitor has a real degraded status', () => {
    expect(getOverallMonitorStatus([
      { primary_status: '', availability_7d: null },
      { primary_status: 'failed', availability_7d: 0 },
    ])).toBe('degraded')
  })

  it('formats Grok as a first-class monitor provider', () => {
    const { providerLabel, providerBadgeClass, providerPickerClass } = useChannelMonitorFormat()

    expect(providerLabel('grok')).toBe('Grok')
    expect(providerBadgeClass('grok')).toContain('slate')
    expect(providerPickerClass('grok', true)).toContain('slate')
    expect(providerGradient('grok')).toContain('slate')
  })
})
