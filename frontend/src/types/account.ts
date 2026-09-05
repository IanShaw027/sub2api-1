/**
 * Account (upstream channel account) & proxy domain types.
 */

import type { Group } from './group'
import type { CodexUsageSnapshot, OpenAICompactState } from './accountUsage'

// ==================== Account & Proxy Types ====================

export type AccountPlatform = 'anthropic' | 'openai' | 'gemini' | 'antigravity' | 'grok' | 'kiro' | 'kimi' | 'zhipu' | 'deepseek'
export type AccountType = 'oauth' | 'setup-token' | 'apikey' | 'upstream' | 'bedrock' | 'service_account'
export type OAuthAddMethod = 'oauth' | 'setup-token'
export type ProxyProtocol = 'http' | 'https' | 'socks5' | 'socks5h'

// Claude Model type (returned by /v1/models and account models API)
export interface ClaudeModel {
  id: string
  type: string
  display_name: string
  created_at: string
}

export interface Proxy {
  id: number
  name: string
  protocol: ProxyProtocol
  host: string
  port: number
  username: string | null
  password?: string | null
  has_password?: boolean
  status: 'active' | 'inactive' | 'expired'
  account_count?: number // Number of accounts using this proxy
  latency_ms?: number
  latency_status?: 'success' | 'failed'
  latency_message?: string
  ip_address?: string
  country?: string
  country_code?: string
  region?: string
  city?: string
  quality_status?: 'healthy' | 'warn' | 'challenge' | 'failed'
  quality_score?: number
  quality_grade?: string
  quality_summary?: string
  quality_checked?: number
  expires_at: string | null
  fallback_mode: 'none' | 'proxy' | 'direct'
  backup_proxy_id?: number | null
  expiry_warn_days: number
  created_at: string
  updated_at: string
}

export interface ProxyAccountSummary {
  id: number
  name: string
  platform: AccountPlatform
  type: AccountType
  notes?: string | null
}

export interface ProxyQualityCheckItem {
  target: string
  status: 'pass' | 'warn' | 'fail' | 'challenge'
  http_status?: number
  latency_ms?: number
  message?: string
  cf_ray?: string
}

export interface ProxyQualityCheckResult {
  proxy_id: number
  score: number
  grade: string
  summary: string
  exit_ip?: string
  country?: string
  country_code?: string
  base_latency_ms?: number
  passed_count: number
  warn_count: number
  failed_count: number
  challenge_count: number
  checked_at: number
  items: ProxyQualityCheckItem[]
}

// Gemini credentials structure for OAuth and API Key authentication
export interface GeminiCredentials {
  // API Key authentication
  api_key?: string

  // OAuth authentication
  access_token?: string
  refresh_token?: string
  oauth_type?: 'code_assist' | 'google_one' | 'ai_studio' | string
  tier_id?:
    | 'google_one_free'
    | 'google_ai_pro'
    | 'google_ai_ultra'
    | 'gcp_standard'
    | 'gcp_enterprise'
    | 'aistudio_free'
    | 'aistudio_paid'
    | 'LEGACY'
    | 'PRO'
    | 'ULTRA'
    | string
  project_id?: string
  token_type?: string
  scope?: string
  expires_at?: string
  model_mapping?: Record<string, string>
}

export type KiroAuthMethod = 'social' | 'idc' | 'external_idp'

export interface KiroCredentials {
  access_token?: string
  refresh_token?: string
  expires_at?: string
  auth_method?: KiroAuthMethod | string
  client_id?: string
  client_secret?: string
  token_endpoint?: string
  issuer_url?: string
  scopes?: string
  login_hint?: string
  region?: string
  auth_region?: string
  api_region?: string
  profile_arn?: string
  profile_id?: string
  machine_id?: string
  model_mapping?: Record<string, string>
  subscription_type?: string
  plan_name?: string
  plan_tier?: string
}

export interface KiroAccountExtra {
  kiro_version?: string
  system_version?: string
  node_version?: string
}

export interface TempUnschedulableRule {
  error_code: number
  keywords: string[]
  duration_minutes: number
  description: string
}

export interface TempUnschedulableState {
  until_unix: number
  triggered_at_unix: number
  status_code: number
  matched_keyword: string
  rule_index: number
  error_message: string
  trigger_count?: number
  trigger_threshold?: number
  trigger_window_minutes?: number
}

export interface TempUnschedulableStatus {
  active: boolean
  state?: TempUnschedulableState
}

export interface UpstreamBillingData {
  object: 'sub2api.key_billing'
  schema_version: 1
  billing_scope: 'token'
  group_rate_multiplier: number
  user_rate_multiplier?: number
  resolved_rate_multiplier: number
  peak_rate_enabled: boolean
  peak_start?: string
  peak_end?: string
  peak_rate_multiplier?: number
  applied_peak_multiplier?: number
  effective_rate_multiplier: number
  timezone?: string
  observed_at: string
}

export type UpstreamBillingProbeStatus = 'ok' | 'unsupported' | 'failed'

export interface UpstreamBillingProbeSnapshot {
  status: UpstreamBillingProbeStatus
  data?: UpstreamBillingData
  received_at?: string
  fresh_until?: string
  last_attempt_at: string
  next_probe_at: string
  failure_count?: number
  http_status?: number
  last_error?: string
  // Value this probe wrote into the account rate multiplier; absent when the
  // probe did not sync a rate.
  synced_rate_multiplier?: number
}

export interface UpstreamBillingProbeSettings {
  enabled: boolean
  interval_minutes: number
}

export interface UpstreamBillingProbeResult {
  account_id: number
  snapshot?: UpstreamBillingProbeSnapshot
  error?: string
}

export interface UpstreamBillingRateSnapshotItem {
  account_id: number
  snapshot?: UpstreamBillingProbeSnapshot | null
}

export interface UpstreamBillingRatesResponse {
  items: UpstreamBillingRateSnapshotItem[]
  total: number
  page: number
  page_size: number
}

export type OllamaCloudUsageStatus = 'ok' | 'unauthorized' | 'failed'

export interface OllamaCloudUsageWindow {
  used_percent: number
  reset_at?: string
  reset_text?: string
}

export interface OllamaCloudUsageModel {
  model: string
  window: 'five_hour' | 'seven_day'
  requests: number
}

export interface OllamaCloudUsageData {
  plan?: string
  five_hour?: OllamaCloudUsageWindow
  seven_day?: OllamaCloudUsageWindow
  balance?: string
  models?: OllamaCloudUsageModel[]
}

export interface OllamaCloudUsageSnapshot {
  status: OllamaCloudUsageStatus
  data?: OllamaCloudUsageData
  fetched_at?: string
  last_attempt_at: string
  next_refresh_at: string
  failure_count?: number
  http_status?: number
  last_error?: string
}

export interface OllamaCloudUsageState {
  account_id: number
  eligible: boolean
  configured: boolean
  auto_refresh_enabled: boolean
  encryption_key_configured: boolean
  snapshot?: OllamaCloudUsageSnapshot
}

export interface OllamaCloudUsageSettings {
  enabled: boolean
  /** Max wait while model requests keep arriving (minutes). */
  interval_minutes: number
  /** Trailing quiet period after the latest model request (minutes). */
  debounce_minutes: number
}

export interface Account {
  id: number
  name: string
  notes?: string | null
  platform: AccountPlatform
  type: AccountType
  // 后端响应里 credentials 已脱敏：access_token / refresh_token / id_token /
  // api_key / session_key / cookie / aws_secret_access_key / aws_session_token /
  // service_account_json / service_account / private_key 不会出现，
  // 改为通过 credentials_status.has_<key> 暴露存在性。
  credentials?: Record<string, unknown>
  credentials_status?: Record<string, boolean>
  ollama_cloud_usage?: OllamaCloudUsageState
  // Extra fields including Codex usage, OpenAI compact capability, and model-level rate limits.
  extra?: (CodexUsageSnapshot & OpenAICompactState & {
    model_rate_limits?: Record<string, { rate_limited_at: string; rate_limit_reset_at: string }>
    antigravity_credits_overages?: Record<string, { activated_at: string; active_until: string }>
    upstream_billing_probe_enabled?: boolean
    upstream_billing_rate_sync_enabled?: boolean
    upstream_billing_probe?: UpstreamBillingProbeSnapshot
    codex_reset_credit_snapshot?: {
      available_count?: number
      credits?: { expires_at?: string }[]
    }
    auto_reset_credit_enabled?: boolean
    auto_reset_credit_5h_threshold?: number
    auto_reset_credit_7d_threshold?: number
    codex_auto_reset_credit_state?: {
      status?: 'checking' | 'available' | 'resetting' | 'success' | 'no_credit' | 'failed'
      trigger_window?: string
      available_count?: number
      checked_at?: string
      last_result_at?: string
      error_code?: string
    }
  } & Record<string, unknown>)
  proxy_id: number | null
  proxy_fallback_origin_id?: number | null
  proxy_fallback_origin_name?: string | null
  concurrency: number
  load_factor?: number | null
  current_concurrency?: number // Real-time concurrency count from Redis
  scheduler_score?: {
    base_score: number
    sticky_score?: number
    sticky_score_infinity?: boolean
    sticky_weighted_enabled: boolean
  } | null
  scheduler_scores?: AccountSchedulerGroupScore[] | null
  cyber_count?: number | null
  cyber_latest_at?: string | null
  priority: number
  rate_multiplier?: number // Account billing multiplier (>=0, 0 means free)
  status: 'active' | 'inactive' | 'error'
  error_message: string | null
  last_used_at: string | null
  expires_at: number | null
  auto_pause_on_expired: boolean
  created_at: string
  updated_at: string
  proxy?: Proxy
  group_ids?: number[] // Groups this account belongs to
  groups?: Group[] // Preloaded group objects

  // Rate limit & scheduling fields
  schedulable: boolean
  rate_limited_at: string | null
  rate_limit_reset_at: string | null
  overload_until: string | null
  temp_unschedulable_until: string | null
  temp_unschedulable_reason: string | null

  // Session window fields (5-hour window)
  session_window_start: string | null
  session_window_end: string | null
  session_window_status: 'allowed' | 'allowed_warning' | 'rejected' | null

  // 5h窗口费用控制（仅 Anthropic OAuth/SetupToken 账号有效）
  window_cost_limit?: number | null
  window_cost_sticky_reserve?: number | null

  // 会话数量控制（仅 Anthropic OAuth/SetupToken 账号有效）
  max_sessions?: number | null
  session_idle_timeout_minutes?: number | null

  // RPM 限制（仅 Anthropic OAuth/SetupToken 账号有效）
  base_rpm?: number | null
  rpm_strategy?: string | null
  rpm_sticky_buffer?: number | null
  user_msg_queue_mode?: string | null  // "serialize" | "throttle" | null

  // TLS指纹伪装（全平台）
  enable_tls_fingerprint?: boolean | null
  tls_fingerprint_profile_id?: number | null
  tls_fingerprint_router_id?: number | null
  tls_fingerprint_bindings?: Record<string, number> | null
  tls_fingerprint_default_os?: string | null
  device_learning_enabled?: boolean | null
  device_tls_profile_id?: number | null

  // 会话ID伪装（仅 Anthropic OAuth/SetupToken 账号有效）
  // 启用后将在15分钟内固定 metadata.user_id 中的 session ID
  session_id_masking_enabled?: boolean | null

  // 缓存 TTL 强制替换（仅 Anthropic OAuth/SetupToken 账号有效）
  cache_ttl_override_enabled?: boolean | null
  cache_ttl_override_target?: string | null

  // 自定义 Base URL 中继转发（仅 Anthropic OAuth/SetupToken 账号有效）
  custom_base_url_enabled?: boolean | null
  custom_base_url?: string | null

  // API Key 账号配额限制
  quota_limit?: number | null
  quota_used?: number | null
  quota_daily_limit?: number | null
  quota_daily_used?: number | null
  quota_weekly_limit?: number | null
  quota_weekly_used?: number | null

  // 配额固定时间重置配置
  quota_daily_reset_mode?: 'rolling' | 'fixed' | null
  quota_daily_reset_hour?: number | null
  quota_weekly_reset_mode?: 'rolling' | 'fixed' | null
  quota_weekly_reset_day?: number | null
  quota_weekly_reset_hour?: number | null
  quota_reset_timezone?: string | null
  quota_daily_reset_at?: string | null
  quota_weekly_reset_at?: string | null

  // 运行时状态（仅当启用对应限制时返回）
  current_window_cost?: number | null // 当前窗口费用
  active_sessions?: number | null // 当前活跃会话数
  current_rpm?: number | null // 当前分钟 RPM 计数

  // 影子账号关系（spark 维度影子）
  parent_account_id?: number | null
  quota_dimension?: string
  // 影子账号回填的母账号信息（仅影子非空）
  parent_email?: string
  parent_plan_type?: string
  parent_privacy_mode?: string
  parent_subscription_expires_at?: string
  parent_chatgpt_account_id?: string
}

export interface AccountDeviceProfile {
  account_id: number
  platform: string
  client_family: string
  client_version: string
  user_agent: string
  os_family: string
  arch: string
  revision: number
  learned_from: string
  learning_enabled: boolean
  transport_family: string
  updated_at: string
}

export interface AccountSchedulerGroupScore {
  group_id?: number | null
  group_name?: string
  group_priority?: number | null
  base_score: number
  sticky_score?: number
  sticky_score_infinity?: boolean
  sticky_weighted_enabled: boolean
}

export interface AccountCyberEvent {
  error_id?: number | null
  created_at: string
  request_id: string
  model: string
  status_code?: number | null
  message: string
}

export interface CreateAccountRequest {
  name: string
  notes?: string | null
  platform: AccountPlatform
  type: AccountType
  credentials: Record<string, unknown>
  extra?: Record<string, unknown>
  proxy_id?: number | null
  concurrency?: number
  load_factor?: number | null
  priority?: number
  rate_multiplier?: number // Account billing multiplier (>=0, 0 means free)
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  upstream_billing_probe_enabled?: boolean
  confirm_mixed_channel_risk?: boolean
}

export interface UpdateAccountRequest {
  name?: string
  notes?: string | null
  type?: AccountType
  credentials?: Record<string, unknown>
  extra?: Record<string, unknown>
  proxy_id?: number | null
  concurrency?: number
  load_factor?: number | null
  priority?: number
  rate_multiplier?: number // Account billing multiplier (>=0, 0 means free)
  schedulable?: boolean
  status?: 'active' | 'inactive' | 'error'
  group_ids?: number[]
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  upstream_billing_probe_enabled?: boolean
  upstream_billing_rate_sync_enabled?: boolean
  confirm_mixed_channel_risk?: boolean
}

export interface CheckMixedChannelRequest {
  platform: AccountPlatform
  group_ids: number[]
  account_id?: number
}

export interface MixedChannelWarningDetails {
  group_id: number
  group_name: string
  current_platform: string
  other_platform: string
}

export interface CheckMixedChannelResponse {
  has_risk: boolean
  error?: string
  message?: string
  details?: MixedChannelWarningDetails
}

export interface CreateProxyRequest {
  name: string
  protocol: ProxyProtocol
  host: string
  port: number
  username?: string | null
  password?: string | null
  expires_at?: number | null   // unix 秒；null/0 = 永不过期
  fallback_mode?: 'none' | 'proxy' | 'direct'
  backup_proxy_id?: number | null
  expiry_warn_days?: number
}

export interface UpdateProxyRequest {
  name?: string
  protocol?: ProxyProtocol
  host?: string
  port?: number
  username?: string | null
  password?: string | null
  status?: 'active' | 'inactive'
  expires_at?: number | null   // unix 秒；null/0 = 永不过期
  fallback_mode?: 'none' | 'proxy' | 'direct'
  backup_proxy_id?: number | null
  expiry_warn_days?: number
}

export interface AdminDataPayload {
  type?: string
  version?: number
  exported_at: string
  proxies: AdminDataProxy[]
  accounts: AdminDataAccount[]
  // 导出时被排除的 spark 影子账号数量(影子不持凭据、其调度配置不在备份范围)。
  skipped_shadows?: number
}

export interface AdminDataProxy {
  proxy_key: string
  name: string
  protocol: ProxyProtocol
  host: string
  port: number
  username?: string | null
  password?: string | null
  status: 'active' | 'inactive'
}

export interface AdminDataAccount {
  name: string
  notes?: string | null
  platform: AccountPlatform
  type: AccountType
  credentials: Record<string, unknown>
  extra?: Record<string, unknown>
  proxy_key?: string | null
  concurrency: number
  priority: number
  rate_multiplier?: number | null
  expires_at?: number | null
  auto_pause_on_expired?: boolean
}

export interface AdminDataImportError {
  kind: 'proxy' | 'account'
  name?: string
  proxy_key?: string
  message: string
}

export interface AdminDataImportResult {
  proxy_created: number
  proxy_reused: number
  proxy_failed: number
  account_created: number
  account_failed: number
  errors?: AdminDataImportError[]
}

export interface CodexSessionImportRequest {
  content?: string
  contents?: string[]
  name?: string
  notes?: string | null
  group_ids?: number[]
  proxy_id?: number | null
  concurrency?: number
  priority?: number
  rate_multiplier?: number
  load_factor?: number | null
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  credential_extras?: Record<string, unknown>
  extra?: Record<string, unknown>
  update_existing?: boolean
  skip_default_group_bind?: boolean
  confirm_mixed_channel_risk?: boolean
}

export interface OpenAICodexPATCreateRequest {
  access_token: string
  name?: string
  notes?: string | null
  group_ids?: number[]
  proxy_id?: number | null
  concurrency?: number
  priority?: number
  rate_multiplier?: number
  load_factor?: number | null
  expires_at?: number | null
  auto_pause_on_expired?: boolean
  credential_extras?: Record<string, unknown>
  extra?: Record<string, unknown>
  skip_default_group_bind?: boolean
  confirm_mixed_channel_risk?: boolean
}

export interface CodexSessionImportMessage {
  index: number
  name?: string
  message: string
}

export interface CodexSessionImportItem {
  index: number
  name?: string
  action: 'created' | 'updated' | 'skipped' | 'failed'
  account_id?: number
  message?: string
}

export interface CodexSessionImportResult {
  total: number
  created: number
  updated: number
  skipped: number
  failed: number
  items?: CodexSessionImportItem[]
  warnings?: CodexSessionImportMessage[]
  errors?: CodexSessionImportMessage[]
}

export type {
  PlatformQuotaItem,
  PlatformQuotaUpdateItem,
  PlatformQuotaPlatform,
  PlatformQuotaWindow,
  PlatformQuotasResponse,
} from '@/api/admin/users'
