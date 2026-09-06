import type { ChatSettings } from './localChat'

type Effort = NonNullable<ChatSettings['reasoningEffort']>
const standard: Effort[] = ['low', 'medium', 'high']
const anthropicPlatforms = new Set(['anthropic', 'antigravity', 'kiro', 'gemini'])

function modelId(model: string): string {
  const parts = model.trim().toLowerCase().split('/')
  return parts[parts.length - 1]!.replace(/^(?:[a-z]+\.)?anthropic\./, '')
}

export function supportsChatTemperature(model: string): boolean {
  return !/^(o[134](?:[.-]|$)|gpt-[5-9])/i.test(modelId(model))
}

export function getChatReasoningEfforts(model: string, platform: string, metadata?: unknown): Effort[] {
  const id = modelId(model)
  // These are the actual camelCase capability fields emitted by gateway.Models.
  if (metadata && typeof metadata === 'object') {
    const detail = metadata as Record<string, unknown>
    if (detail.supportsReasoningEffort === false) return []
    if (detail.supportsReasoningEffort === true && Array.isArray(detail.reasoningEfforts)) {
      const allowed = new Set<Effort>(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])
      return [...new Set(detail.reasoningEfforts.flatMap(option => {
        if (!option || typeof option !== 'object') return []
        const value = (option as Record<string, unknown>).value
        return typeof value === 'string' && allowed.has(value as Effort) ? [value as Effort] : []
      }))]
    }
  }
  if (platform === 'openai') {
    if (/^gpt-5-pro(?:-|$)/.test(id)) return ['high']
    return /^(o[134](?:[.-]|$)|gpt-[5-9])/.test(id) ? [...standard] : []
  }
  if (!anthropicPlatforms.has(platform)) return []
  if (/^gemini-3(?:\.\d+)?-flash(?:-|$)/.test(id)) return ['minimal', ...standard]
  if (/^gemini-3\.1-pro(?:-|$)/.test(id)) return [...standard]
  if (/^gemini-3-pro(?:-|$)/.test(id)) return ['low', 'high']
  if (/^gemini-2\.5-flash(?:-|$)/.test(id)) return ['none', ...standard]
  if (/^gemini-2\.5-pro(?:-|$)/.test(id)) return [...standard]
  const claude = id.replace(/\./g, '-')
  if (platform === 'antigravity') return /^claude-(?:opus|sonnet)-4(?:-|$)/.test(claude) ? [...standard] : []
  // Mirror backend/internal/pkg/claude/effort_catalog.go, not a guessed API field.
  if (/^claude-(?:mythos-5|fable-5|sonnet-5|opus-(?:4-[78]|5))(?:-|$)/.test(claude)) return [...standard, 'xhigh', 'max']
  if (/^claude-(?:mythos-preview|sonnet-4-6|opus-4-6)(?:-|$)/.test(claude)) return [...standard, 'max']
  if (/^claude-opus-4-5(?:-|$)/.test(claude)) return [...standard]
  return []
}
