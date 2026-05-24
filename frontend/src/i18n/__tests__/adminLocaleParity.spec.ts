import { describe, expect, it } from 'vitest'

import en from '../locales/en.json'
import zh from '../locales/zh.json'

const requiredKeys = [
  'admin.dashboard.newUsersToday',
  'admin.dashboard.active',
  'admin.dashboard.ok',
  'admin.dashboard.err',
  'admin.dashboard.create',
  'admin.dashboard.userUsageTrend',
  'admin.groups.claudeMaxSimulation.title',
  'admin.groups.claudeMaxSimulation.tooltip',
  'admin.groups.claudeMaxSimulation.enabled',
  'admin.groups.claudeMaxSimulation.disabled',
  'admin.groups.claudeMaxSimulation.hint',
  'admin.settings.gatewayForwarding.apiKeyAclTrustForwardedIP',
  'admin.settings.gatewayForwarding.apiKeyAclTrustForwardedIPHint',
  'admin.settings.gatewayForwarding.openaiCodexUserAgent',
  'admin.settings.gatewayForwarding.openaiCodexUserAgentPlaceholder',
  'admin.settings.gatewayForwarding.openaiCodexUserAgentHint',
  'payment.admin.allowUserRefund',
]

function lookup(obj: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((current, key) => {
    if (!current || typeof current !== 'object') return undefined
    return (current as Record<string, unknown>)[key]
  }, obj)
}

describe('admin locale parity', () => {
  it('keeps newly referenced admin keys present in both English and Chinese locales', () => {
    for (const key of requiredKeys) {
      expect(lookup(en, key), `missing en locale key: ${key}`).toBeTypeOf('string')
      expect(lookup(zh, key), `missing zh locale key: ${key}`).toBeTypeOf('string')
    }
  })
})
