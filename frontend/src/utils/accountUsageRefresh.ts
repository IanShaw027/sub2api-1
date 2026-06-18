import type { Account } from '@/types'

const normalizeUsageRefreshValue = (value: unknown): string => {
  if (value == null) return ''
  return String(value)
}

export const buildOpenAIUsageRefreshKey = (account: Pick<Account, 'id' | 'platform' | 'type' | 'updated_at' | 'last_used_at' | 'rate_limit_reset_at' | 'extra'>): string => {
  if (account.platform !== 'openai' || account.type !== 'oauth') {
    return ''
  }

  const extra = account.extra ?? {}
  return [
    account.id,
    account.updated_at,
    account.last_used_at,
    account.rate_limit_reset_at,
    extra.codex_usage_updated_at,
    extra.codex_5h_used_percent,
    extra.codex_5h_reset_at,
    extra.codex_5h_reset_after_seconds,
    extra.codex_5h_window_minutes,
    extra.codex_7d_used_percent,
    extra.codex_7d_reset_at,
    extra.codex_7d_reset_after_seconds,
    extra.codex_7d_window_minutes,
    extra.codex_invite_reset_available_count,
    extra.codex_invite_reset_updated_at,
    JSON.stringify(extra.codex_invite_reset_credit_ids ?? []),
    JSON.stringify(extra.codex_invite_reset_credits ?? [])
  ].map(normalizeUsageRefreshValue).join('|')
}

export const buildGeminiUsageRefreshKey = (account: Pick<Account, 'id' | 'platform' | 'type' | 'updated_at' | 'last_used_at' | 'rate_limit_reset_at' | 'credentials' | 'extra'>): string => {
  if (account.platform !== 'gemini') {
    return ''
  }

  const credentials = account.credentials ?? {}
  const extra = account.extra ?? {}

  return [
    account.id,
    account.updated_at,
    account.last_used_at,
    account.rate_limit_reset_at,
    credentials.usage_updated_at ?? extra.usage_updated_at,
    credentials.quota_query_last_error ?? extra.quota_query_last_error,
    credentials.quota_query_last_error_at ?? extra.quota_query_last_error_at,
    credentials.gemini_status ?? extra.gemini_status,
    credentials.gemini_status_reason ?? extra.gemini_status_reason,
    credentials.project_id,
    credentials.oauth_type,
    credentials.tier_id
  ].map(normalizeUsageRefreshValue).join('|')
}
