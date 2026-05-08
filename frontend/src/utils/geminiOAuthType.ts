import { collectGeminiTierMetadataSources } from './geminiExtra'

const normalizeString = (value: unknown): string => {
  return typeof value === 'string' ? value.trim().toLowerCase() : ''
}

export function inferGeminiOAuthType(
  credentials?: Record<string, unknown> | null,
  extra?: Record<string, unknown> | null
): 'google_one' | 'code_assist'
export function inferGeminiOAuthType(
  credentials: Record<string, unknown> | null | undefined,
  extra: Record<string, unknown> | null | undefined,
  fallback: ''
): 'google_one' | 'code_assist' | ''
export function inferGeminiOAuthType(
  credentials: Record<string, unknown> | null | undefined,
  extra: Record<string, unknown> | null | undefined,
  fallback: 'google_one' | 'code_assist'
): 'google_one' | 'code_assist'
export function inferGeminiOAuthType(
  credentials?: Record<string, unknown> | null,
  extra?: Record<string, unknown> | null,
  fallback: 'google_one' | 'code_assist' | '' = 'code_assist'
): 'google_one' | 'code_assist' | '' {
  const explicitType = normalizeString(credentials?.oauth_type) || normalizeString(extra?.oauth_type)
  if (explicitType === 'google_one' || explicitType === 'code_assist') {
    return explicitType
  }

  const tierSources = collectGeminiTierMetadataSources(credentials, extra)
    .map(normalizeString)
    .filter((value) => value.length > 0)

  for (const source of tierSources) {
    if (
      source.includes('google one') ||
      source.includes('google_one') ||
      source.includes('google ai') ||
      source.includes('google_ai') ||
      source.startsWith('g1-') ||
      source === 'free-tier'
    ) {
      return 'google_one'
    }
    if (
      source.includes('gcp_') ||
      source === 'standard' ||
      source === 'enterprise' ||
      source === 'standard-tier' ||
      source === 'pro-tier' ||
      source === 'ultra-tier'
    ) {
      return 'code_assist'
    }
  }

  return fallback
}
