import { beforeEach, describe, expect, it } from 'vitest'
import { useAccountColumnVisibility } from '../useAccountColumnVisibility'

describe('account column preference preservation', () => {
  beforeEach(() => localStorage.clear())

  it('preserves the pre-Glass default readable fields', () => {
    const preferences = useAccountColumnVisibility()
    expect([...preferences.hiddenColumns]).toEqual(['today_stats', 'proxy', 'notes', 'scheduler_score', 'rate_multiplier'])
    for (const key of ['id', 'groups', 'upstream_billing_rate', 'created_at', 'expires_at']) {
      expect(preferences.isColumnVisible(key), key).toBe(true)
    }
  })

  it.each(['glass-04-default-columns', 'scheduler-score-hidden-by-default', 'preserve-readable-columns-v2'])(
    'does not rewrite explicit visibility preferences from %s', (version) => {
      localStorage.setItem('account-hidden-columns', JSON.stringify(['priority', 'created_at']))
      localStorage.setItem('account-hidden-columns-version', version)
      const preferences = useAccountColumnVisibility()
      expect([...preferences.hiddenColumns]).toEqual(['priority', 'created_at'])
      expect(preferences.isColumnVisible('upstream_billing_rate')).toBe(true)
      preferences.toggleColumnVisibility('created_at')
      const restored = useAccountColumnVisibility()
      expect([...restored.hiddenColumns]).toEqual(['priority'])
    },
  )

  it('retains the legacy opt-in for expensive scoring without hiding readable fields', () => {
    localStorage.setItem('account-hidden-columns', JSON.stringify(['today_stats']))
    const preferences = useAccountColumnVisibility()
    expect([...preferences.hiddenColumns]).toEqual(['today_stats', 'scheduler_score'])
    expect(preferences.isColumnVisible('created_at')).toBe(true)
    expect(preferences.isColumnVisible('upstream_billing_rate')).toBe(true)
  })
})
