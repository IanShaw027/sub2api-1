/**
 * User & auth domain types (accounts, login/register, affiliate,
 * user subscriptions, user attributes, TOTP 2FA).
 */

import type { NotifyEmailEntry } from './common'
import type { Group } from './group'

// ==================== User & Auth Types ====================

export type UserAuthProvider = 'email' | 'linuxdo' | 'oidc' | 'wechat' | 'github' | 'google' | 'dingtalk'

export interface UserAuthBindingStatus {
  bound?: boolean
  bound_count?: number
  provider?: UserAuthProvider | string
  provider_key?: string | null
  provider_subject?: string | null
  issuer?: string | null
  label?: string | null
  provider_label?: string | null
  display_name?: string | null
  subject_hint?: string | null
  verified_at?: string | null
  bind_start_path?: string | null
  can_bind?: boolean
  can_unbind?: boolean
  note_key?: string | null
  note?: string | null
  metadata?: Record<string, unknown>
}

export interface UserProfileSourceContext {
  provider?: UserAuthProvider | string
  source?: string | null
  label?: string | null
  provider_label?: string | null
}

export interface User {
  id: number
  username: string
  email: string
  avatar_url?: string | null
  avatar_source?: string | UserProfileSourceContext | null
  username_source?: string | UserProfileSourceContext | null
  display_name_source?: string | UserProfileSourceContext | null
  nickname_source?: string | UserProfileSourceContext | null
  profile_sources?: {
    avatar?: string | UserProfileSourceContext | null
    username?: string | UserProfileSourceContext | null
    display_name?: string | UserProfileSourceContext | null
    nickname?: string | UserProfileSourceContext | null
  }
  auth_bindings?: Partial<Record<UserAuthProvider, boolean | UserAuthBindingStatus>>
  identity_bindings?: Partial<Record<UserAuthProvider, boolean | UserAuthBindingStatus>>
  email_bound?: boolean
  linuxdo_bound?: boolean
  oidc_bound?: boolean
  wechat_bound?: boolean
  role: 'admin' | 'user' // User role for authorization
  balance: number // User balance for API usage
  frozen_balance?: number // Balance currently held by async batch jobs
  concurrency: number // Allowed concurrent requests
  rpm_limit?: number // User-level RPM cap (0 = unlimited); effective as fallback when group has no rpm_limit
  status: 'active' | 'disabled' // Account status
  allowed_groups: number[] | null // Allowed group IDs (null = all non-exclusive groups)
  balance_notify_enabled: boolean
  balance_notify_threshold: number | null
  balance_notify_extra_emails: NotifyEmailEntry[]
  subscriptions?: UserSubscription[] // User's active subscriptions
  last_active_at?: string | null
  created_at: string
  updated_at: string
  deleted_at?: string | null
}

export interface AdminUser extends User {
  // 管理员备注（普通用户接口不返回）
  notes: string
  last_used_at?: string | null
  // 用户专属分组倍率配置 (group_id -> rate_multiplier)
  group_rates?: Record<number, number>
  // 为 true 时该用户仅可使用 allowed_groups 中列出的公开分组。
  // 管理侧权限开关，普通用户接口不返回。
  restrict_public_groups?: boolean
  // 当前并发数（仅管理员列表接口返回）
  current_concurrency?: number
  current_rpm?: number
  today_actual_cost?: number
  total_actual_cost?: number
  today_balance_actual_cost?: number
  today_subscription_actual_cost?: number
}

export interface LoginRequest {
  email: string
  password: string
  turnstile_token?: string
  tencent_captcha_ticket?: string
  tencent_captcha_randstr?: string
}

export interface TencentCaptchaRequestProof {
  tencent_captcha_ticket: string
  tencent_captcha_randstr: string
}

// 动作触发式验证码（OAuth 启动、passkey 等入口）的请求凭据：
// 腾讯填 tencent_captcha_*，阿里云的 captchaVerifyParam 复用 turnstile_token 字段
export interface ActionCaptchaRequestProof extends Partial<TencentCaptchaRequestProof> {
  turnstile_token?: string
}

export interface RegisterRequest {
  email: string
  password: string
  verify_code?: string
  turnstile_token?: string
  tencent_captcha_ticket?: string
  tencent_captcha_randstr?: string
  promo_code?: string
  invitation_code?: string
  aff_code?: string
}

export interface AffiliateInvitee {
  user_id: number
  email: string
  username: string
  created_at?: string
  total_rebate: number
}

export interface UserAffiliateDetail {
  user_id: number
  aff_code: string
  inviter_id?: number | null
  aff_count: number
  aff_quota: number
  aff_frozen_quota: number
  aff_history_quota: number
  /** 当前用户作为邀请人时实际生效的返利比例（专属覆盖全局）。0-100。 */
  effective_rebate_rate_percent: number
  invitees: AffiliateInvitee[]
}

export interface AffiliateTransferResponse {
  transferred_quota: number
  balance: number
}

export interface SendVerifyCodeRequest {
  email: string
  turnstile_token?: string
  tencent_captcha_ticket?: string
  tencent_captcha_randstr?: string
  pending_auth_token?: string
  pending_oauth_token?: string
}

export interface SendVerifyCodeResponse {
  message: string
  countdown: number
}

export interface AuthResponse {
  access_token: string
  refresh_token?: string  // New: Refresh Token for token renewal
  expires_in?: number     // New: Access Token expiry time in seconds
  token_type: string
  user: User & { run_mode?: 'standard' | 'simple' }
}

export interface CurrentUserResponse extends User {
  run_mode?: 'standard' | 'simple'
}

// ==================== Admin User Management ====================

export interface UpdateUserRequest {
  email?: string
  password?: string
  username?: string
  notes?: string
  role?: 'admin' | 'user'
  balance?: number
  concurrency?: number
  rpm_limit?: number
  status?: 'active' | 'disabled'
  allowed_groups?: number[] | null
  restrict_public_groups?: boolean
  // 用户专属分组倍率配置 (group_id -> rate_multiplier | null)
  // null 表示删除该分组的专属倍率
  group_rates?: Record<number, number | null>
}

export interface ChangePasswordRequest {
  old_password: string
  new_password: string
}

// ==================== User Subscription Types ====================

export interface UserSubscription {
  id: number
  user_id: number
  group_id: number
  status: 'active' | 'expired' | 'revoked' | 'suspended'
  starts_at: string
  daily_usage_usd: number
  weekly_usage_usd: number
  monthly_usage_usd: number
  daily_window_start: string | null
  weekly_window_start: string | null
  monthly_window_start: string | null
  created_at: string
  updated_at: string
  revoked_at?: string | null
  expires_at: string | null
  user?: User
  group?: Group
}

export interface SubscriptionProgress {
  subscription_id: number
  daily: {
    used: number
    limit: number | null
    percentage: number
    reset_in_seconds: number | null
  } | null
  weekly: {
    used: number
    limit: number | null
    percentage: number
    reset_in_seconds: number | null
  } | null
  monthly: {
    used: number
    limit: number | null
    percentage: number
    reset_in_seconds: number | null
  } | null
  expires_at: string | null
  days_remaining: number | null
}

export interface AssignSubscriptionRequest {
  user_id: number
  group_id: number
  validity_days?: number
}

export interface BulkAssignSubscriptionRequest {
  user_ids: number[]
  group_id: number
  validity_days?: number
}

export interface ExtendSubscriptionRequest {
  days: number
}

// ==================== User Attribute Types ====================

export type UserAttributeType = 'text' | 'textarea' | 'number' | 'email' | 'url' | 'date' | 'select' | 'multi_select'

export interface UserAttributeOption {
  value: string
  label: string
  [key: string]: unknown
}

export interface UserAttributeValidation {
  min_length?: number
  max_length?: number
  min?: number
  max?: number
  pattern?: string
  message?: string
}

export interface UserAttributeDefinition {
  id: number
  key: string
  name: string
  description: string
  type: UserAttributeType
  options: UserAttributeOption[]
  required: boolean
  validation: UserAttributeValidation
  placeholder: string
  display_order: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface UserAttributeValue {
  id: number
  user_id: number
  attribute_id: number
  value: string
  created_at: string
  updated_at: string
}

export interface CreateUserAttributeRequest {
  key: string
  name: string
  description?: string
  type: UserAttributeType
  options?: UserAttributeOption[]
  required?: boolean
  validation?: UserAttributeValidation
  placeholder?: string
  display_order?: number
  enabled?: boolean
}

export interface UpdateUserAttributeRequest {
  key?: string
  name?: string
  description?: string
  type?: UserAttributeType
  options?: UserAttributeOption[]
  required?: boolean
  validation?: UserAttributeValidation
  placeholder?: string
  display_order?: number
  enabled?: boolean
}

export interface UserAttributeValuesMap {
  [attributeId: number]: string
}

// ==================== TOTP (2FA) Types ====================

export interface TotpStatus {
  enabled: boolean
  enabled_at: number | null  // Unix timestamp in seconds
  feature_enabled: boolean
}

export interface TotpSetupRequest {
  email_code?: string
  password?: string
}

export interface TotpSetupResponse {
  secret: string
  qr_code_url: string
  setup_token: string
  countdown: number
}

export interface TotpEnableRequest {
  totp_code: string
  setup_token: string
}

export interface TotpEnableResponse {
  success: boolean
}

export interface TotpDisableRequest {
  email_code?: string
  password?: string
}

export interface TotpVerificationMethod {
  method: 'email' | 'password'
}

export interface TotpLoginResponse {
  requires_2fa: boolean
  temp_token?: string
  user_email_masked?: string
}

export interface TotpLogin2FARequest {
  temp_token: string
  totp_code: string
}

// Payment types
export type { SubscriptionPlan, PaymentOrder, CheckoutInfoResponse } from './payment'
