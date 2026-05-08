const STALE_GEMINI_TIER_EXTRA_KEYS = [
  'oauth_type',
  'subscription_type',
  'plan_type',
  'plan_name',
  'gemini_current_tier_id',
  'gemini_current_tier_name',
  'gemini_current_tier_user_defined_project',
  'gemini_paid_tier_id',
  'gemini_paid_tier_name',
  'gemini_paid_tier_user_defined_project',
  'gemini_has_onboarded_previously',
  'gemini_available_credits',
  'gemini_code_assist_updated_at'
] as const

const normalizeGeminiExtraString = (value: unknown): string => {
  return typeof value === 'string' ? value.trim() : ''
}

export function collectGeminiTierMetadataSources(
  credentials?: Record<string, unknown> | null,
  extra?: Record<string, unknown> | null
): string[] {
  return [
    extra?.gemini_paid_tier_id,
    credentials?.gemini_paid_tier_id,
    credentials?.tier_id,
    credentials?.gemini_current_tier_id,
    extra?.gemini_current_tier_id,
    extra?.tier_id,
    credentials?.plan_type,
    credentials?.plan_name,
    credentials?.gemini_paid_tier_name,
    credentials?.gemini_current_tier_name,
    extra?.plan_type,
    extra?.plan_name,
    extra?.gemini_paid_tier_name,
    extra?.gemini_current_tier_name
  ]
    .map(normalizeGeminiExtraString)
    .filter((value) => value.length > 0)
}

export function stripStaleGeminiExtra(
  extra?: Record<string, unknown> | null
): Record<string, unknown> {
  const cleaned = { ...(extra || {}) }
  for (const key of STALE_GEMINI_TIER_EXTRA_KEYS) {
    delete cleaned[key]
  }
  return cleaned
}
