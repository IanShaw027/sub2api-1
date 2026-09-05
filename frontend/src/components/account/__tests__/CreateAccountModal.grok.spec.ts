import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const source = readFileSync(
  resolve(process.cwd(), 'src/components/account/CreateAccountModal.vue'),
  'utf8'
)

const grokPanelSource = readFileSync(
  resolve(process.cwd(), 'src/components/account/platform/GrokPanel.vue'),
  'utf8'
)

const grokCustomBaseUrlSectionSource = readFileSync(
  resolve(process.cwd(), 'src/components/account/shared/GrokCustomBaseUrlSection.vue'),
  'utf8'
)

describe('CreateAccountModal Grok account types', () => {
  it('offers API-key setup alongside OAuth with the official xAI default', () => {
    // Account-type selector markup was extracted into GrokPanel.vue during the
    // Glass UI platform-panel split; the rest of the Grok-specific logic stays
    // in CreateAccountModal.vue.
    expect(grokPanelSource).toContain('data-testid="grok-account-type-api-key"')
    expect(grokPanelSource).toContain("@click=\"accountCategory = 'apikey'\"")
    expect(source).toContain("newPlatform === 'grok'")
    expect(source).toContain("? 'https://api.x.ai/v1'")
    expect(source).toContain("form.platform === 'grok'")
    expect(source).toContain(':placeholder="apiKeyValuePlaceholder"')
    expect(source).toContain("return 'xai-...'")
  })

  it('exposes custom upstream URL and header override for the OAuth create flow', () => {
    // The custom-base-url toggle/input markup was extracted into the shared
    // GrokCustomBaseUrlSection.vue component (same pattern as GrokPanel.vue
    // above); the outer v-if gating condition stays in this file.
    expect(grokCustomBaseUrlSectionSource).toContain('data-testid="grok-custom-base-url-toggle"')
    expect(grokCustomBaseUrlSectionSource).toContain('data-testid="grok-custom-base-url-input"')
    expect(source).toContain('form.platform === \'grok\' && isOAuthFlow')
  })

  it('validates and applies upstream config on Grok OAuth create paths', () => {
    // 授权码兑换 / RT 批量 / SSO 批量（密码授权已隐藏）
    expect(source.match(/validateGrokOAuthUpstreamConfig\(\)/g)?.length).toBeGreaterThanOrEqual(3)
    expect(source.match(/applyGrokOAuthUpstreamConfig\(credentials\)/g)?.length).toBeGreaterThanOrEqual(3)
  })

  it('hides Grok password authorize option in the create flow', () => {
    expect(source).toContain(':show-email-password-option="false"')
  })
})
