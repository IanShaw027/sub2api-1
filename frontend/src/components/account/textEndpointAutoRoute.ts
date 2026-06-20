import type { AccountPlatform, AccountType } from '@/types'

const supportedTypesByPlatform: Record<AccountPlatform, ReadonlySet<AccountType>> = {
  openai: new Set(['apikey']),
  anthropic: new Set(['apikey', 'upstream']),
  gemini: new Set(),
  kiro: new Set(),
  antigravity: new Set(),
  sora: new Set()
}

export function supportsTextEndpointAutoRoute(
  platform: AccountPlatform | null | undefined,
  type: AccountType | null | undefined
): boolean {
  if (!platform || !type) return false
  return supportedTypesByPlatform[platform]?.has(type) === true
}
