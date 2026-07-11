import { describe, expect, it, vi } from 'vitest'

vi.mock('@/api/admin/accounts', () => ({
  getAntigravityDefaultModelMapping: vi.fn()
}))

import { buildModelMappingObject, getModelsByPlatform, getPresetMappingsByPlatform } from '../useModelWhitelist'

describe('useModelWhitelist', () => {
  it('openai 模型列表包含 GPT-5.6 和 GPT-5.4 官方快照', () => {
    const models = getModelsByPlatform('openai')

    expect(models).toContain('gpt-4o')
    expect(models).toContain('gpt-4o-mini')
    expect(models).toContain('gpt-5.6')
    expect(models).toContain('gpt-5.6-sol')
    expect(models).toContain('gpt-5.6-terra')
    expect(models).toContain('gpt-5.6-luna')
    expect(models).toContain('gpt-5.4')
    expect(models).toContain('gpt-5.4-mini')
    expect(models).toContain('gpt-5.4-2026-03-05')
    expect(models).toContain('codex-auto-review')
    expect(models).toContain('gpt-5.6')
  })

  it('claude 模型列表保留常用旧别名，避免默认白名单覆盖回退', () => {
    const models = getModelsByPlatform('anthropic')

    expect(models).toContain('claude-3-5-sonnet')
    expect(models).toContain('claude-3-7-sonnet')
    expect(models).toContain('claude-sonnet-4')
    expect(models).toContain('claude-opus-4')
  })

  it('openai 模型列表不再暴露已下线的 ChatGPT 登录 Codex 模型', () => {
    const models = getModelsByPlatform('openai')

    expect(models).not.toContain('gpt-5')
    expect(models).not.toContain('gpt-5.1')
    expect(models).not.toContain('gpt-5.1-codex')
    expect(models).not.toContain('gpt-5.1-codex-max')
    expect(models).not.toContain('gpt-5.1-codex-mini')
    expect(models).not.toContain('gpt-5.2-codex')
  })

  it('antigravity 模型列表包含图片模型兼容项', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models).toContain('gemini-3-pro-image')
  })

  it('Claude 模型列表包含新发布的 Claude 模型', () => {
    expect(getModelsByPlatform('claude')).toContain('claude-fable-5')
    expect(getModelsByPlatform('antigravity')).toContain('claude-fable-5')
    expect(getModelsByPlatform('claude')).toContain('claude-opus-4-8')
    expect(getModelsByPlatform('antigravity')).toContain('claude-opus-4-8')
  })

  it('xAI 模型列表包含 Grok 4.5 官方模型和别名', () => {
    const models = getModelsByPlatform('grok')

    expect(models).toContain('grok-4.5')
    expect(models).toContain('grok-4.5-latest')
    expect(models).toContain('grok-3-mini')
    expect(models).toContain('grok-3-mini-fast')
    expect(models).toContain('grok-build-latest')
  })

  it('xAI 模型列表和预设包含 CPA 的 Imagine Video 1.5 preview ID 并保留旧别名', () => {
    const models = getModelsByPlatform('grok')
    const presets = getPresetMappingsByPlatform('grok')

    expect(models).toContain('grok-imagine-video-1.5-preview')
    expect(models).toContain('grok-imagine-video-1.5')
    expect(presets).toEqual(expect.arrayContaining([
      expect.objectContaining({ from: 'grok-imagine-video-1.5', to: 'grok-imagine-video-1.5-preview' }),
      expect.objectContaining({ from: 'grok-imagine-video-1.5-preview', to: 'grok-imagine-video-1.5-preview' })
    ]))
  })

  it('combined 模式支持 Grok 4.5 官方别名映射', () => {
    const mapping = buildModelMappingObject(
      'combined',
      ['grok-4.5'],
      [
        { from: 'grok-latest', to: 'grok-4.5' },
        { from: 'grok-4.5-latest', to: 'grok-4.5' },
        { from: 'grok-build-latest', to: 'grok-4.5' }
      ]
    )

    expect(mapping).toEqual({
      'grok-4.5': 'grok-4.5',
      'grok-latest': 'grok-4.5',
      'grok-4.5-latest': 'grok-4.5',
      'grok-build-latest': 'grok-4.5'
    })
  })

  it('grok 模型列表包含 Composer 默认项和兼容别名', () => {
    const models = getModelsByPlatform('grok')

    expect(models).toContain('grok-composer-2.5-fast')
    expect(models).toContain('grok-composer')
    expect(models).toContain('composer-2.5')
  })

  it('gemini 模型列表包含原生生图模型', () => {
    const models = getModelsByPlatform('gemini')

    expect(models).toContain('gemini-2.5-flash-image')
    expect(models).toContain('gemini-3.1-flash-image')
    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.0-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
  })

  it('antigravity 模型列表会把新的 Gemini 图片模型排在前面', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models.indexOf('gemini-3.1-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash'))
    expect(models.indexOf('gemini-2.5-flash-image')).toBeLessThan(models.indexOf('gemini-2.5-flash-lite'))
  })

  it('antigravity 模型列表包含 Gemini 3.1 Pro 通用别名', () => {
    const models = getModelsByPlatform('antigravity')

    expect(models).toContain('gemini-3.1-pro')
  })

  it('whitelist 模式会忽略通配符条目', () => {
    const mapping = buildModelMappingObject('whitelist', ['claude-*', 'gemini-3.1-flash-image'], [])
    expect(mapping).toEqual({
      'gemini-3.1-flash-image': 'gemini-3.1-flash-image'
    })
  })

  it('whitelist 模式会保留 GPT-5.4 官方快照的精确映射', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4-2026-03-05'], [])

    expect(mapping).toEqual({
      'gpt-5.4-2026-03-05': 'gpt-5.4-2026-03-05'
    })
  })

  it('openai 快速预设包含 GPT-5.6 canonical 和三个官方变体', () => {
    const presets = getPresetMappingsByPlatform('openai')

    expect(presets).toEqual(expect.arrayContaining([
      expect.objectContaining({ from: 'gpt-5.6', to: 'gpt-5.6' }),
      expect.objectContaining({ from: 'gpt-5.6-sol', to: 'gpt-5.6-sol' }),
      expect.objectContaining({ from: 'gpt-5.6-terra', to: 'gpt-5.6-terra' }),
      expect.objectContaining({ from: 'gpt-5.6-luna', to: 'gpt-5.6-luna' })
    ]))
  })

  it('whitelist keeps GPT-5.4 mini exact mappings', () => {
    const mapping = buildModelMappingObject('whitelist', ['gpt-5.4-mini'], [])

    expect(mapping).toEqual({
      'gpt-5.4-mini': 'gpt-5.4-mini'
    })
  })

  it('Kiro mapping mode skips cross-family mappings before backend submit', () => {
    const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const mapping = buildModelMappingObject('mapping', [], [
      { from: 'claude-opus-4-6', to: 'claude-sonnet-4.6' },
      { from: 'claude-sonnet-*', to: 'claude-sonnet-4.6' },
      { from: 'claude-haiku-4.5-thinking-1m', to: 'claude-haiku-4.6' }
    ], 'kiro')

    expect(mapping).toEqual({
      'claude-sonnet-*': 'claude-sonnet-4.6',
      'claude-haiku-4.5-thinking-1m': 'claude-haiku-4.6'
    })
    expect(warnSpy).toHaveBeenCalledWith(expect.stringContaining('Kiro 模型映射不能跨模型族'))
    warnSpy.mockRestore()
  })

  it('Kiro 快速映射按模型族通用映射到 4.5', () => {
    const presets = getPresetMappingsByPlatform('kiro')

    expect(presets).toEqual(expect.arrayContaining([
      expect.objectContaining({ from: 'claude-haiku-*', to: 'claude-haiku-4.5' }),
      expect.objectContaining({ from: 'claude-sonnet-*', to: 'claude-sonnet-4.5' }),
      expect.objectContaining({ from: 'claude-opus-*', to: 'claude-opus-4.5' })
    ]))
  })
})
