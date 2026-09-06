/**
 * Account usage / quota progress domain types.
 */

import type { ModelStat, EndpointStat } from './ops'

// Account Usage types
export interface WindowStats {
  requests: number
  tokens: number
  cost: number // Account cost (account multiplier)
  standard_cost?: number
  user_cost?: number
}

export interface UsageProgress {
  utilization: number // Percentage (0-100+, 100 = 100%)
  resets_at: string | null
  remaining_seconds: number
  window_stats?: WindowStats | null // 窗口期统计（从窗口开始到当前的使用量）
  predicted_total_cost?: number | null // 按当前金额使用率外推的7d总金额
  used_requests?: number
  limit_requests?: number
}

// Antigravity 单个模型的配额信息
export interface AntigravityModelQuota {
  utilization: number // 使用率 0-100
  reset_time: string  // 重置时间 ISO8601
}

export interface KiroQuotaBreakdown {
  current_usage: number
  usage_limit: number
  remaining: number
  utilization: number
  resets_at?: string | null
}

export interface GrokQuotaWindow {
  limit?: number | null
  remaining?: number | null
  reset_unix?: number | null
  reset_at?: string | null
}

export interface GrokBillingProductUsage {
  product: string
  usage_percent?: number | null
}

export interface GrokBillingSummary {
  period_type?: string
  usage_percent?: number | null
  period_start?: string
  period_end?: string
  product_usage?: GrokBillingProductUsage[]
  monthly_limit_cents?: number | null
  used_cents?: number | null
  included_used_cents?: number | null
  billing_period_start?: string
  billing_period_end?: string
  used_percent?: number | null
  /** Absolute USD money from billing probes */
  prepaid_balance?: number | null
  monthly_limit?: number | null
  monthly_used?: number | null
  on_demand_cap?: number | null
  on_demand_used?: number | null
  top_up_method?: string
  is_unified_billing_user?: boolean
  plan?: string
  status_code?: number
  source?: string
  fetched_at?: string
  updated_at?: string
  weekly_updated_at?: string
  monthly_updated_at?: string
  partial?: boolean
  failed_windows?: string[]
}

export interface AccountUsageInfo {
  source?: 'passive' | 'active'
  updated_at: string | null
  five_hour: UsageProgress | null
  seven_day: UsageProgress | null
  seven_day_sonnet: UsageProgress | null
  seven_day_fable?: UsageProgress | null
  thirty_day?: UsageProgress | null
  kiro_quota?: UsageProgress | null
  kiro_subscription_title?: string
  kiro_current_usage?: number
  kiro_usage_limit?: number
  kiro_remaining?: number
  kiro_email?: string
  kiro_overage_capability?: string
  kiro_overage_enabled?: boolean | null
  kiro_profile_id?: string
  kiro_login_provider?: string
  kiro_status_reason?: string
  kiro_monthly_quota?: KiroQuotaBreakdown | null
  kiro_bonus_quota?: KiroQuotaBreakdown | null
  kiro_free_trial_quota?: KiroQuotaBreakdown | null
  kiro_total_quota?: KiroQuotaBreakdown | null
  gemini_shared_daily?: UsageProgress | null
  gemini_pro_daily?: UsageProgress | null
  gemini_flash_daily?: UsageProgress | null
  gemini_shared_minute?: UsageProgress | null
  gemini_pro_minute?: UsageProgress | null
  gemini_flash_minute?: UsageProgress | null
  antigravity_quota?: Record<string, AntigravityModelQuota> | null
  grok_request_quota?: GrokQuotaWindow | null
  grok_token_quota?: GrokQuotaWindow | null
  grok_retry_after_seconds?: number | null
  grok_entitlement_status?: string
  grok_quota_snapshot_state?: string
  grok_last_quota_probe_at?: string
  grok_last_headers_seen_at?: string
  grok_last_status_code?: number
  grok_free_token_limit?: number
  grok_local_usage?: WindowStats | null
  grok_local_usage_24h?: WindowStats | null
  grok_local_usage_7d?: WindowStats | null
  grok_local_usage_monthly?: WindowStats | null
  grok_billing?: GrokBillingSummary | null
  subscription_tier?: string
  subscription_tier_raw?: string
  ai_credits?: Array<{
    credit_type?: string
    amount?: number
    minimum_balance?: number
  }> | null
  // Antigravity 403 forbidden 状态
  is_forbidden?: boolean
  forbidden_reason?: string
  forbidden_type?: string   // "validation" | "violation" | "forbidden"
  validation_url?: string   // 验证/申诉链接

  // 状态标记（后端自动推导）
  needs_verify?: boolean    // 需要人工验证（forbidden_type=validation）
  is_banned?: boolean       // 账号被封（forbidden_type=violation）
  needs_reauth?: boolean    // token 失效需重新授权（401）

  // 机器可读错误码：forbidden / unauthenticated / rate_limited / network_error
  error_code?: string

  error?: string            // usage 获取失败时的错误信息
}

// OpenAI Codex usage snapshot (from response headers)
export interface CodexUsageSnapshot {
  // Legacy fields (kept for backwards compatibility)
  // NOTE: The naming is ambiguous - actual window type is determined by window_minutes value
  codex_primary_used_percent?: number // Usage percentage (check window_minutes for actual window type)
  codex_primary_reset_after_seconds?: number // Seconds until reset
  codex_primary_window_minutes?: number // Window in minutes
  codex_secondary_used_percent?: number // Usage percentage (check window_minutes for actual window type)
  codex_secondary_reset_after_seconds?: number // Seconds until reset
  codex_secondary_window_minutes?: number // Window in minutes
  codex_primary_over_secondary_percent?: number // Overflow ratio

  // Canonical fields (normalized by backend, use these preferentially)
  codex_5h_used_percent?: number // 5-hour window usage percentage
  codex_5h_reset_after_seconds?: number // Seconds until 5h window reset
  codex_5h_reset_at?: string // 5-hour window absolute reset time (RFC3339)
  codex_5h_window_minutes?: number // 5h window in minutes (should be ~300)
  codex_7d_used_percent?: number // 7-day window usage percentage
  codex_7d_reset_after_seconds?: number // Seconds until 7d window reset
  codex_7d_reset_at?: string // 7-day window absolute reset time (RFC3339)
  codex_7d_window_minutes?: number // 7d window in minutes (should be ~10080)

  codex_usage_updated_at?: string // Last update timestamp
}

export type OpenAICompactMode = 'auto' | 'force_on' | 'force_off'
export type OpenAIResponsesMode = 'auto' | 'force_responses' | 'force_chat_completions'
export type OpenAIEndpointCapability = 'chat_completions' | 'embeddings'

export interface OpenAICompactState {
  openai_compact_mode?: OpenAICompactMode
  openai_compact_supported?: boolean
  openai_compact_checked_at?: string
  openai_compact_last_status?: number
  openai_compact_last_error?: string
}

export interface OpenAIResponsesState {
  openai_responses_mode?: OpenAIResponsesMode
  openai_responses_supported?: boolean
}

// ==================== Account Usage Statistics ====================

export interface AccountUsageHistory {
  date: string
  label: string
  requests: number
  tokens: number
  cost: number
  actual_cost: number // Account cost (account multiplier)
  user_cost: number // User/API key billed cost (group multiplier)
}

export interface AccountUsageSummary {
  days: number
  actual_days_used: number
  total_cost: number // Account cost (account multiplier)
  total_user_cost: number
  total_standard_cost: number
  total_requests: number
  total_tokens: number
  avg_daily_cost: number // Account cost
  avg_daily_user_cost: number
  avg_daily_requests: number
  avg_daily_tokens: number
  avg_duration_ms: number
  today: {
    date: string
    cost: number
    user_cost: number
    requests: number
    tokens: number
  } | null
  highest_cost_day: {
    date: string
    label: string
    cost: number
    user_cost: number
    requests: number
  } | null
  highest_request_day: {
    date: string
    label: string
    requests: number
    cost: number
    user_cost: number
  } | null
}

export interface AccountUsageStatsResponse {
  history: AccountUsageHistory[]
  summary: AccountUsageSummary
  models: ModelStat[]
  endpoints: EndpointStat[]
  upstream_endpoints: EndpointStat[]
  seven_day_forecasts?: Array<{
    bucket: number
    window_start: string
    observed_at: string
    used_cost: number
    predicted_total_cost: number
    rate_limit_429: number | null
    sessions: number | null
  }>
}
