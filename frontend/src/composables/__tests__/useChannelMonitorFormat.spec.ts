import { describe, expect, it } from 'vitest'
import { getOverallMonitorStatus } from '@/composables/useChannelMonitorFormat'

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
})
