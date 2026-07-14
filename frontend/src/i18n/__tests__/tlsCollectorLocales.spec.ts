import { describe, expect, it } from 'vitest'
import en from '../locales/en'
import zh from '../locales/zh'

const requiredMessagePaths = [
  'common.copy',
  'common.expand',
  'common.collapse',
  'tlsCollector.eyebrow',
  'tlsCollector.title',
  'tlsCollector.description',
  'tlsCollector.backHome',
  'tlsCollector.captureURL',
  'tlsCollector.platformBaseURL',
  'tlsCollector.probeURL',
  'tlsCollector.platform',
  'tlsCollector.token',
  'tlsCollector.copied',
  'tlsCollector.copyAll',
  'tlsCollector.howItWorksTitle',
  'tlsCollector.howItWorksBody',
  'tlsCollector.requiredTitle',
  'tlsCollector.required.token',
  'tlsCollector.required.url',
  'tlsCollector.required.request',
  'tlsCollector.required.headers',
  'tlsCollector.required.cert',
  'tlsCollector.capturesTitle',
  'tlsCollector.captures.rawClientHello',
  'tlsCollector.captures.userAgent',
  'tlsCollector.captures.originator',
  'tlsCollector.captures.platform',
  'tlsCollector.clientGuidesTitle',
  'tlsCollector.clientGuidesHint',
  'tlsCollector.guides.codexCli.title',
  'tlsCollector.guides.codexCli.body',
  'tlsCollector.guides.codexExec.title',
  'tlsCollector.guides.codexExec.body',
  'tlsCollector.guides.codexDesktop.title',
  'tlsCollector.guides.codexDesktop.body',
  'tlsCollector.guides.claudeCode.title',
  'tlsCollector.guides.claudeCode.body',
  'tlsCollector.guides.claudePrint.title',
  'tlsCollector.guides.claudePrint.body',
  'tlsCollector.guides.grokCurl.title',
  'tlsCollector.guides.grokCurl.body',
  'tlsCollector.guides.grokBase.title',
  'tlsCollector.guides.grokBase.body',
  'tlsCollector.guides.kiroCurl.title',
  'tlsCollector.guides.kiroCurl.body',
  'tlsCollector.guides.kiroBase.title',
  'tlsCollector.guides.kiroBase.body',
  'tlsCollector.guides.node.title',
  'tlsCollector.guides.node.body',
  'tlsCollector.guides.python.title',
  'tlsCollector.guides.python.body',
  'tlsCollector.guides.curl.title',
  'tlsCollector.guides.curl.body',
  'admin.tlsFingerprintProfiles.form.http2Fingerprint',
  'admin.tlsFingerprintProfiles.form.http2FingerprintPlaceholder',
  'admin.tlsFingerprintProfiles.form.http2FingerprintHint',
  'admin.tlsFingerprintProfiles.capture.storeBody',
  'admin.tlsFingerprintProfiles.capture.storeBodyHint'
]

const resolvePath = (messages: unknown, path: string): unknown => {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object' && segment in current) {
      return (current as Record<string, unknown>)[segment]
    }
    return undefined
  }, messages)
}

describe('tlsCollector locales', () => {
  it('defines all messages used by the native capture guide page', () => {
    for (const path of requiredMessagePaths) {
      expect(resolvePath(en, path), `missing English message: ${path}`).toBeTruthy()
      expect(resolvePath(zh, path), `missing Chinese message: ${path}`).toBeTruthy()
    }
  })
})
