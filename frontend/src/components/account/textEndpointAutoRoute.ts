import type { AccountPlatform, AccountType } from '@/types'

export type OpenAIEndpointCapability = 'chat_completions' | 'embeddings'

const OPENAI_ENDPOINT_CAPABILITIES: OpenAIEndpointCapability[] = ['chat_completions', 'embeddings']

type TextEndpointAutoRouteOptions = {
  openAIEndpointCapabilities?: readonly string[]
}

const supportedTypesByPlatform: Record<AccountPlatform, ReadonlySet<AccountType>> = {
  openai: new Set(['apikey']),
  anthropic: new Set(['apikey']),
  gemini: new Set(),
  kiro: new Set(),
  antigravity: new Set(),
  sora: new Set()
}

export function supportsTextEndpointAutoRoute(
  platform: AccountPlatform | null | undefined,
  type: AccountType | null | undefined,
  options: TextEndpointAutoRouteOptions = {}
): boolean {
  if (!platform || !type) return false
  if (
    platform === 'openai' &&
    type === 'apikey' &&
    options.openAIEndpointCapabilities &&
    !options.openAIEndpointCapabilities.includes('chat_completions')
  ) {
    return false
  }
  return supportedTypesByPlatform[platform]?.has(type) === true
}

export function normalizeOpenAIEndpointCapabilities(raw: unknown): OpenAIEndpointCapability[] {
  if (!Array.isArray(raw)) return [...OPENAI_ENDPOINT_CAPABILITIES]
  const selected = new Set<OpenAIEndpointCapability>()
  for (const value of raw) {
    if (value === 'chat_completions' || value === 'embeddings') {
      selected.add(value)
    }
  }
  return selected.size > 0
    ? OPENAI_ENDPOINT_CAPABILITIES.filter((capability) => selected.has(capability))
    : [...OPENAI_ENDPOINT_CAPABILITIES]
}
