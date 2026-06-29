import { describe, expect, it } from 'vitest'
import { readdirSync, readFileSync } from 'node:fs'
import { join, relative } from 'node:path'

import { mergeLocaleMessages } from '../index'
import enJson from '../locales/en.json'
import zhJson from '../locales/zh.json'
import enTs from '../locales/en.ts'
import zhTs from '../locales/zh.ts'

const requiredRuntimeKeys = [
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
  'admin.settings.gatewayForwarding.claudeTelemetryMode',
  'admin.settings.gatewayForwarding.claudeTelemetryModeDrop',
  'admin.settings.gatewayForwarding.claudeTelemetryModeForward',
  'admin.settings.gatewayForwarding.claudeTelemetryModeHint',
  'admin.settings.emailTemplates.title',
  'admin.settings.emailTemplates.description',
  'admin.settings.emailTemplates.preview',
  'admin.settings.emailTemplates.restoreOfficial',
  'admin.settings.emailTemplates.save',
  'admin.settings.emailTemplates.event',
  'admin.settings.emailTemplates.locale',
  'admin.settings.emailTemplates.localeEn',
  'admin.settings.emailTemplates.localeZh',
  'admin.settings.emailTemplates.subject',
  'admin.settings.emailTemplates.subjectPlaceholder',
  'admin.settings.emailTemplates.html',
  'admin.settings.emailTemplates.htmlPlaceholder',
  'admin.settings.emailTemplates.placeholders',
  'admin.settings.emailTemplates.placeholdersHelp',
  'admin.settings.emailTemplates.livePreview',
  'admin.settings.emailTemplates.previewSecurityHint',
  'admin.backup.columns.id',
  'admin.riskControl.action.keywordBlock',
  'usage.imageBillingSize',
  'usage.imageSizeSource',
  'usage.imageInputSize',
  'usage.imageOutputSize',
  'usage.imageSizeSourceDefault',
  'usage.imageSizeUnknown',
  'payment.admin.allowUserRefund',
]

const requiredRedeemBatchUpdateKeys = [
  'admin.redeem.batchUpdate',
  'admin.redeem.batchUpdateTitle',
  'admin.redeem.selectedCount',
  'admin.redeem.clearSelection',
  'admin.redeem.codeExpiry',
  'admin.redeem.neverExpires',
  'admin.redeem.customExpiry',
  'admin.redeem.customExpiryDays',
  'admin.redeem.expiryPresetDays',
  'admin.redeem.expiryDaysRequired',
  'admin.redeem.columns.expiresAt',
  'admin.redeem.batchFields.status',
  'admin.redeem.batchFields.expiresAt',
  'admin.redeem.batchFields.notes',
  'admin.redeem.batchFields.group',
  'admin.redeem.batchNotesPlaceholder',
  'admin.redeem.clearGroup',
  'admin.redeem.selectCodesFirst',
  'admin.redeem.noBatchFieldsSelected',
  'admin.redeem.batchUpdateSuccess',
  'admin.redeem.failedToBatchUpdate',
]

function lookup(obj: unknown, path: string): unknown {
  return path.split('.').reduce<unknown>((current, key) => {
    if (!current || typeof current !== 'object') return undefined
    return (current as Record<string, unknown>)[key]
  }, obj)
}

function collectSourceFiles(dir: string, files: string[] = []): string[] {
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const fullPath = join(dir, entry.name)
    if (entry.isDirectory()) {
      if (entry.name === '__tests__' || entry.name === 'locales') continue
      collectSourceFiles(fullPath, files)
      continue
    }
    if (/\.(vue|ts|js)$/.test(entry.name)) {
      files.push(fullPath)
    }
  }
  return files
}

function collectStaticTranslationKeys() {
  const srcRoot = join(process.cwd(), 'src')
  const files = collectSourceFiles(srcRoot)
  const keys = new Map<string, Set<string>>()
  const translationCall = /\b(?:t|\$t)\(\s*(['"])([A-Za-z0-9_.-]+)\1/g

  for (const file of files) {
    const text = readFileSync(file, 'utf8')
    let match: RegExpExecArray | null
    while ((match = translationCall.exec(text))) {
      const key = match[2]
      if (!key.includes('.') || key.endsWith('.')) continue
      const relativeFile = relative(srcRoot, file)
      if (!keys.has(key)) keys.set(key, new Set())
      keys.get(key)?.add(relativeFile)
    }
  }

  return keys
}

describe('admin locale parity', () => {
  it('keeps newly referenced admin keys present in runtime locales', () => {
    const localeSources = [
      { name: 'en runtime', messages: mergeLocaleMessages(enJson, enTs) },
      { name: 'zh runtime', messages: mergeLocaleMessages(zhJson, zhTs) },
    ]

    for (const key of requiredRuntimeKeys) {
      for (const { name, messages } of localeSources) {
        expect(lookup(messages, key), `missing ${name} locale key: ${key}`).toBeTypeOf('string')
      }
    }
  })

  it('keeps static translation references present in runtime locales', () => {
    const localeSources = [
      { name: 'en runtime', messages: mergeLocaleMessages(enJson, enTs) },
      { name: 'zh runtime', messages: mergeLocaleMessages(zhJson, zhTs) },
    ]
    const missing: string[] = []

    for (const [key, files] of collectStaticTranslationKeys()) {
      for (const { name, messages } of localeSources) {
        if (typeof lookup(messages, key) === 'undefined') {
          missing.push(`${name}: ${key} (${Array.from(files).slice(0, 3).join(', ')})`)
        }
      }
    }

    expect(missing).toEqual([])
  })

  it('keeps redeem batch-update keys present in JSON, TS, and runtime locales', () => {
    const localeSources = [
      { name: 'en JSON', messages: enJson },
      { name: 'zh JSON', messages: zhJson },
      { name: 'en TS', messages: enTs },
      { name: 'zh TS', messages: zhTs },
      { name: 'en runtime', messages: mergeLocaleMessages(enJson, enTs) },
      { name: 'zh runtime', messages: mergeLocaleMessages(zhJson, zhTs) },
    ]

    for (const key of requiredRedeemBatchUpdateKeys) {
      for (const { name, messages } of localeSources) {
        expect(lookup(messages, key), `missing ${name} locale key: ${key}`).toBeTypeOf('string')
      }
    }
  })
})
