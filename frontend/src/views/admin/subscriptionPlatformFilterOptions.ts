export interface SubscriptionPlatformFilterOption {
  value: string
  label: string
  [key: string]: unknown
}

export function buildSubscriptionPlatformFilterOptions(allPlatformsLabel: string): SubscriptionPlatformFilterOption[] {
  return [
    { value: '', label: allPlatformsLabel },
    { value: 'anthropic', label: 'Anthropic' },
    { value: 'openai', label: 'OpenAI' },
    { value: 'gemini', label: 'Gemini' },
    { value: 'antigravity', label: 'Antigravity' },
    { value: 'kiro', label: 'Kiro' },
    { value: 'grok', label: 'Grok' },
  ]
}
