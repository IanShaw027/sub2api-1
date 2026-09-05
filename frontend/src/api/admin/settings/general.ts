/**
 * Admin Settings API — core SystemSettings / UpdateSettingsRequest
 * aggregate types and the getSettings/updateSettings CRUD endpoints.
 */

import { apiClient } from "../../client";
import type {
  CustomEndpoint,
  CustomMenuItem,
  LoginAgreementDocument,
  NotifyEmailEntry,
  SupportQRCodeEntry,
} from "@/types";
import type {
  DefaultSubscriptionSetting,
  DefaultPlatformQuotasMap,
  AccountSchedulingThresholdsMap,
} from "./security";
import type { OpenAIFastPolicySettings } from "./gateway";

/**
 * System settings interface
 */
export interface SystemSettings {
  // Registration settings
  registration_enabled: boolean;
  email_verify_enabled: boolean;
  registration_email_suffix_whitelist: string[];
  registration_email_domain_quota_enabled: boolean;
  promo_code_enabled: boolean;
  password_reset_enabled: boolean;
  frontend_url: string;
  invitation_code_enabled: boolean;
  totp_enabled: boolean; // TOTP 双因素认证
  totp_encryption_key_configured: boolean; // TOTP 加密密钥是否已配置
  passkey_enabled: boolean;
  passkey_configured: boolean;
  passkey_rp_id: string;
  passkey_rp_origins: string[];
  session_binding_enabled: boolean; // 会话 IP/UA 绑定
  step_up_enabled: boolean; // 敏感操作 step-up 2FA
  audit_log_retention_days: number; // 审计日志保留天数
  login_agreement_enabled: boolean;
  login_agreement_mode: "modal" | "checkbox" | string;
  login_agreement_updated_at: string;
  login_agreement_documents: LoginAgreementDocument[];
  // Default settings
  default_balance: number;
  affiliate_rebate_rate: number;
  affiliate_rebate_freeze_hours: number;
  affiliate_rebate_duration_days: number;
  affiliate_rebate_per_invitee_cap: number;
  affiliate_rebate_cap: number;
  affiliate_rebate_invitee_limit: number;
  affiliate_signup_bonus: number;
  affiliate_admin_recharge_enabled: boolean;
  default_concurrency: number;
  default_user_rpm_limit: number;
  default_subscriptions: DefaultSubscriptionSetting[];
  auth_source_default_email_balance?: number;
  auth_source_default_email_concurrency?: number;
  auth_source_default_email_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_email_grant_on_signup?: boolean;
  auth_source_default_email_grant_on_first_bind?: boolean;
  auth_source_default_linuxdo_balance?: number;
  auth_source_default_linuxdo_concurrency?: number;
  auth_source_default_linuxdo_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_linuxdo_grant_on_signup?: boolean;
  auth_source_default_linuxdo_grant_on_first_bind?: boolean;
  auth_source_default_oidc_balance?: number;
  auth_source_default_oidc_concurrency?: number;
  auth_source_default_oidc_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_oidc_grant_on_signup?: boolean;
  auth_source_default_oidc_grant_on_first_bind?: boolean;
  auth_source_default_wechat_balance?: number;
  auth_source_default_wechat_concurrency?: number;
  auth_source_default_wechat_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_wechat_grant_on_signup?: boolean;
  auth_source_default_wechat_grant_on_first_bind?: boolean;
  auth_source_default_dingtalk_balance?: number;
  auth_source_default_dingtalk_concurrency?: number;
  auth_source_default_dingtalk_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_dingtalk_grant_on_signup?: boolean;
  auth_source_default_dingtalk_grant_on_first_bind?: boolean;
  auth_source_default_github_balance?: number;
  auth_source_default_github_concurrency?: number;
  auth_source_default_github_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_github_grant_on_signup?: boolean;
  auth_source_default_github_grant_on_first_bind?: boolean;
  auth_source_default_google_balance?: number;
  auth_source_default_google_concurrency?: number;
  auth_source_default_google_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_google_grant_on_signup?: boolean;
  auth_source_default_google_grant_on_first_bind?: boolean;
  force_email_on_third_party_signup?: boolean;
  // ── 平台限额（嵌套 JSON，系统层 + 7 auth-source 层）────────────────────────────────
  default_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_email_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_linuxdo_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_oidc_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_wechat_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_github_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_google_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_dingtalk_platform_quotas?: DefaultPlatformQuotasMap;
  // OEM settings
  site_name: string;
  site_logo: string;
  site_subtitle: string;
  api_base_url: string;
  contact_info: string;
  support_qr_codes: SupportQRCodeEntry[];
  doc_url: string;
  download_tools_url: string;
  home_content: string;
  compact_home_enabled: boolean;
  hide_ccs_import_button: boolean;
  table_default_page_size: number;
  table_page_size_options: number[];
  backend_mode_enabled: boolean;
  custom_menu_items: CustomMenuItem[];
  custom_endpoints: CustomEndpoint[];
  // SMTP settings
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password_configured: boolean;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
  // Cloudflare Turnstile settings
  turnstile_enabled: boolean;
  turnstile_site_key: string;
  turnstile_secret_key_configured: boolean;
  tencent_captcha_enabled: boolean;
  tencent_captcha_app_id: string;
  tencent_captcha_app_secret_key_configured: boolean;
  tencent_captcha_cloud_secret_id_configured: boolean;
  tencent_captcha_cloud_secret_key_configured: boolean;
  tencent_captcha_region: string;
  aliyun_captcha_enabled: boolean;
  aliyun_captcha_access_key_id: string;
  aliyun_captcha_access_key_secret_configured: boolean;
  aliyun_captcha_scene_id: string;
  aliyun_captcha_prefix: string;
  aliyun_captcha_region: string;
  api_key_acl_trust_forwarded_ip: boolean;
  forwarded_client_ip_headers: string[];

  // LinuxDo Connect OAuth settings
  linuxdo_connect_enabled: boolean;
  linuxdo_connect_client_id: string;
  linuxdo_connect_client_secret_configured: boolean;
  linuxdo_connect_redirect_url: string;

  // DingTalk Connect OAuth settings
  dingtalk_connect_enabled: boolean;
  dingtalk_connect_client_id: string;
  dingtalk_connect_client_secret_configured: boolean;
  dingtalk_connect_redirect_url: string;
  dingtalk_connect_corp_restriction_policy: string;
  dingtalk_connect_internal_corp_id: string;
  dingtalk_connect_bypass_registration: boolean;
  dingtalk_connect_sync_corp_email: boolean;
  dingtalk_connect_sync_display_name: boolean;
  dingtalk_connect_sync_dept: boolean;
  dingtalk_connect_sync_corp_email_attr_key: string;
  dingtalk_connect_sync_display_name_attr_key: string;
  dingtalk_connect_sync_dept_attr_key: string;
  dingtalk_connect_sync_corp_email_attr_name: string;
  dingtalk_connect_sync_display_name_attr_name: string;
  dingtalk_connect_sync_dept_attr_name: string;

  // WeChat Connect OAuth settings
  wechat_connect_enabled: boolean;
  wechat_connect_app_id: string;
  wechat_connect_app_secret_configured: boolean;
  wechat_connect_open_app_id?: string;
  wechat_connect_open_app_secret_configured?: boolean;
  wechat_connect_mp_app_id?: string;
  wechat_connect_mp_app_secret_configured?: boolean;
  wechat_connect_mobile_app_id?: string;
  wechat_connect_mobile_app_secret_configured?: boolean;
  wechat_connect_open_enabled?: boolean;
  wechat_connect_mp_enabled?: boolean;
  wechat_connect_mobile_enabled?: boolean;
  wechat_connect_mode: string;
  wechat_connect_scopes: string;
  wechat_connect_redirect_url: string;
  wechat_connect_frontend_redirect_url: string;

  // Generic OIDC OAuth settings
  oidc_connect_enabled: boolean;
  oidc_connect_provider_name: string;
  oidc_connect_client_id: string;
  oidc_connect_client_secret_configured: boolean;
  oidc_connect_issuer_url: string;
  oidc_connect_discovery_url: string;
  oidc_connect_authorize_url: string;
  oidc_connect_token_url: string;
  oidc_connect_userinfo_url: string;
  oidc_connect_jwks_url: string;
  oidc_connect_scopes: string;
  oidc_connect_redirect_url: string;
  oidc_connect_frontend_redirect_url: string;
  oidc_connect_token_auth_method: string;
  oidc_connect_use_pkce: boolean;
  oidc_connect_validate_id_token: boolean;
  oidc_connect_allowed_signing_algs: string;
  oidc_connect_clock_skew_seconds: number;
  oidc_connect_require_email_verified: boolean;
  oidc_connect_userinfo_email_path: string;
  oidc_connect_userinfo_id_path: string;
  oidc_connect_userinfo_username_path: string;
  github_oauth_enabled: boolean;
  github_oauth_client_id: string;
  github_oauth_client_secret_configured: boolean;
  github_oauth_redirect_url: string;
  github_oauth_frontend_redirect_url: string;
  google_oauth_enabled: boolean;
  google_oauth_client_id: string;
  google_oauth_client_secret_configured: boolean;
  google_oauth_redirect_url: string;
  google_oauth_frontend_redirect_url: string;

  // Model fallback configuration
  enable_model_fallback: boolean;
  fallback_model_anthropic: string;
  fallback_model_openai: string;
  fallback_model_gemini: string;
  fallback_model_antigravity: string;
  grok_default_text_model: string;
  grok_cross_client_model_map_enabled: boolean;
  grok_default_base_url_mode: string;

  // Per-platform account auto-pause thresholds (100 = disabled)
  account_scheduling_thresholds: AccountSchedulingThresholdsMap;

  // Identity patch configuration (Claude -> Gemini)
  enable_identity_patch: boolean;
  identity_patch_prompt: string;

  // Ops Monitoring (vNext)
  ops_monitoring_enabled: boolean;
  ops_realtime_monitoring_enabled: boolean;
  ops_query_mode_default: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds: number;

  // Claude Code version check
  min_claude_code_version: string;
  max_claude_code_version: string;

  // Kiro runtime defaults (admin-only)
  kiro_version: string;
  kiro_commit: string;
  system_version: string;
  node_version: string;
  kiro_code_execution_sandbox_command: string;
  cache_hit_rate_scale: number | null;
  cache_min_block_tokens: number | null;
  cache_independent_ttl_seconds: number | null;
  cache_prefix_ttl_seconds: number | null;

  // 分组隔离
  allow_ungrouped_key_scheduling: boolean;

  // Gateway forwarding behavior
  openai_ttft_mode: string;
  enable_fingerprint_unification: boolean;
  enable_metadata_passthrough: boolean;
  enable_cch_signing: boolean;
  enable_claude_oauth_system_prompt_injection: boolean;
  claude_oauth_system_prompt: string;
  claude_oauth_system_prompt_blocks: string;
  enable_anthropic_cache_ttl_1h_injection: boolean;
  rewrite_message_cache_control: boolean;
  enable_client_dateline_normalization: boolean;
  antigravity_user_agent_version: string;
  openai_codex_user_agent: string;
  openai_codex_client_version: string;
  openai_codex_client_version_synced: string;
  openai_codex_version_auto_sync_enabled: boolean;
  // codex_cli_only 加固
  min_codex_version: string;
  max_codex_version: string;
  codex_cli_only_blacklist: string;
  codex_cli_only_whitelist: string;
  codex_cli_only_allow_app_server_clients: boolean;
  codex_cli_only_engine_fingerprint_signals: string;
  web_search_emulation_enabled?: boolean;

  // Payment configuration
  payment_enabled: boolean;
  risk_control_enabled: boolean;

  // Cyber session block
  cyber_session_block_enabled: boolean;
  cyber_session_block_ttl_seconds: number;

  payment_min_amount: number;
  payment_max_amount: number;
  payment_daily_limit: number;
  payment_order_timeout_minutes: number;
  payment_max_pending_orders: number;
  payment_enabled_types: string[];
  payment_balance_disabled: boolean;
  payment_balance_recharge_multiplier: number;
  payment_subscription_usd_to_cny_rate: number;
  payment_recharge_fee_rate: number;
  payment_load_balance_strategy: string;
  payment_product_name_prefix: string;
  payment_product_name_suffix: string;
  payment_help_image_url: string;
  payment_help_text: string;
  payment_cancel_rate_limit_enabled: boolean;
  payment_cancel_rate_limit_max: number;
  payment_cancel_rate_limit_window: number;
  payment_cancel_rate_limit_unit: string;
  payment_cancel_rate_limit_window_mode: string;
  payment_alipay_force_qrcode?: boolean;
  payment_alipay_mobile_precreate_deep_link?: boolean;
  payment_visible_method_alipay_source?: string;
  payment_visible_method_wxpay_source?: string;
  payment_visible_method_alipay_enabled?: boolean;
  payment_visible_method_wxpay_enabled?: boolean;
  openai_low_upstream_rate_priority_enabled?: boolean;
  openai_oauth_scheduling_rate_multiplier?: number;
  openai_advanced_scheduler_enabled?: boolean;
  openai_advanced_scheduler_sticky_weighted_enabled?: boolean;
  openai_advanced_scheduler_subscription_priority_enabled?: boolean;
  openai_advanced_scheduler_lb_top_k?: string;
  openai_advanced_scheduler_weight_priority?: string;
  openai_advanced_scheduler_weight_load?: string;
  openai_advanced_scheduler_weight_queue?: string;
  openai_advanced_scheduler_weight_error_rate?: string;
  openai_advanced_scheduler_weight_ttft?: string;
  openai_advanced_scheduler_weight_reset?: string;
  openai_advanced_scheduler_weight_quota_headroom?: string;
  openai_advanced_scheduler_weight_upstream_cost?: string;
  openai_advanced_scheduler_weight_previous_response?: string;
  openai_advanced_scheduler_weight_session_sticky?: string;
  openai_advanced_scheduler_effective_lb_top_k?: string;
  openai_advanced_scheduler_effective_weight_priority?: string;
  openai_advanced_scheduler_effective_weight_load?: string;
  openai_advanced_scheduler_effective_weight_queue?: string;
  openai_advanced_scheduler_effective_weight_error_rate?: string;
  openai_advanced_scheduler_effective_weight_ttft?: string;
  openai_advanced_scheduler_effective_weight_reset?: string;
  openai_advanced_scheduler_effective_weight_quota_headroom?: string;
  openai_advanced_scheduler_effective_weight_upstream_cost?: string;
  openai_advanced_scheduler_effective_weight_previous_response?: string;
  openai_advanced_scheduler_effective_weight_session_sticky?: string;

  // 余额、订阅到期与账号限额通知
  balance_low_notify_enabled: boolean;
  balance_low_notify_threshold: number;
  balance_low_notify_recharge_url: string;
  subscription_expiry_notify_enabled: boolean;
  account_quota_notify_enabled: boolean;
  account_quota_notify_emails: NotifyEmailEntry[];

  // Channel Monitor feature switch
  channel_monitor_enabled: boolean;
  channel_monitor_mode?: 'v1' | 'v2';
  channel_monitor_default_interval_seconds: number;
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_show_quota?: boolean;

  // Available Channels feature switch
  available_channels_enabled: boolean;

  // Model Plaza feature switches + description
  model_plaza_enabled: boolean;
  model_plaza_require_auth: boolean;
  model_plaza_description: string;
  plugin_management_enabled: boolean;

  // Affiliate (邀请返利) feature switch
  affiliate_enabled: boolean;
  ticket_enabled: boolean;
  creation_center_enabled: boolean;

  ip_multi_account_ban_enabled: boolean;
  ip_multi_account_ban_window_minutes: number;
  ip_multi_account_ban_threshold: number;
  ip_multi_account_ban_window2_minutes: number;
  ip_multi_account_ban_threshold2: number;
  ip_multi_account_ban_learning_until: string;
  registration_block_datacenter_ip: boolean;

  // OpenAI fast/flex policy
  openai_fast_policy_settings?: OpenAIFastPolicySettings;

  // Allow user view error requests
  allow_user_view_error_requests: boolean;
}

export interface UpdateSettingsRequest {
  registration_enabled?: boolean;
  email_verify_enabled?: boolean;
  registration_email_suffix_whitelist?: string[];
  registration_email_domain_quota_enabled?: boolean;
  promo_code_enabled?: boolean;
  password_reset_enabled?: boolean;
  frontend_url?: string;
  invitation_code_enabled?: boolean;
  totp_enabled?: boolean; // TOTP 双因素认证
  passkey_enabled?: boolean;
  session_binding_enabled?: boolean; // 会话 IP/UA 绑定
  step_up_enabled?: boolean; // 敏感操作 step-up 2FA
  audit_log_retention_days?: number; // 审计日志保留天数
  login_agreement_enabled?: boolean;
  login_agreement_mode?: "modal" | "checkbox" | string;
  login_agreement_updated_at?: string;
  login_agreement_documents?: LoginAgreementDocument[];
  default_balance?: number;
  affiliate_rebate_rate?: number;
  affiliate_rebate_freeze_hours?: number;
  affiliate_rebate_duration_days?: number;
  affiliate_rebate_per_invitee_cap?: number;
  affiliate_rebate_cap?: number;
  affiliate_rebate_invitee_limit?: number;
  affiliate_signup_bonus?: number;
  affiliate_admin_recharge_enabled?: boolean;
  default_concurrency?: number;
  default_user_rpm_limit?: number;
  default_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_email_balance?: number;
  auth_source_default_email_concurrency?: number;
  auth_source_default_email_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_email_grant_on_signup?: boolean;
  auth_source_default_email_grant_on_first_bind?: boolean;
  auth_source_default_linuxdo_balance?: number;
  auth_source_default_linuxdo_concurrency?: number;
  auth_source_default_linuxdo_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_linuxdo_grant_on_signup?: boolean;
  auth_source_default_linuxdo_grant_on_first_bind?: boolean;
  auth_source_default_oidc_balance?: number;
  auth_source_default_oidc_concurrency?: number;
  auth_source_default_oidc_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_oidc_grant_on_signup?: boolean;
  auth_source_default_oidc_grant_on_first_bind?: boolean;
  auth_source_default_wechat_balance?: number;
  auth_source_default_wechat_concurrency?: number;
  auth_source_default_wechat_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_wechat_grant_on_signup?: boolean;
  auth_source_default_wechat_grant_on_first_bind?: boolean;
  auth_source_default_dingtalk_balance?: number;
  auth_source_default_dingtalk_concurrency?: number;
  auth_source_default_dingtalk_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_dingtalk_grant_on_signup?: boolean;
  auth_source_default_dingtalk_grant_on_first_bind?: boolean;
  auth_source_default_github_balance?: number;
  auth_source_default_github_concurrency?: number;
  auth_source_default_github_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_github_grant_on_signup?: boolean;
  auth_source_default_github_grant_on_first_bind?: boolean;
  auth_source_default_google_balance?: number;
  auth_source_default_google_concurrency?: number;
  auth_source_default_google_subscriptions?: DefaultSubscriptionSetting[];
  auth_source_default_google_grant_on_signup?: boolean;
  auth_source_default_google_grant_on_first_bind?: boolean;
  force_email_on_third_party_signup?: boolean;
  // ── 平台限额（嵌套 JSON，系统层 + 7 auth-source 层）────────────────────────────────
  default_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_email_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_linuxdo_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_oidc_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_wechat_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_github_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_google_platform_quotas?: DefaultPlatformQuotasMap;
  auth_source_default_dingtalk_platform_quotas?: DefaultPlatformQuotasMap;
  site_name?: string;
  site_logo?: string;
  site_subtitle?: string;
  api_base_url?: string;
  contact_info?: string;
  support_qr_codes?: SupportQRCodeEntry[];
  doc_url?: string;
  download_tools_url?: string;
  home_content?: string;
  compact_home_enabled?: boolean;
  hide_ccs_import_button?: boolean;
  table_default_page_size?: number;
  table_page_size_options?: number[];
  backend_mode_enabled?: boolean;
  custom_menu_items?: CustomMenuItem[];
  custom_endpoints?: CustomEndpoint[];
  smtp_host?: string;
  smtp_port?: number;
  smtp_username?: string;
  smtp_password?: string;
  smtp_from_email?: string;
  smtp_from_name?: string;
  smtp_use_tls?: boolean;
  turnstile_enabled?: boolean;
  turnstile_site_key?: string;
  turnstile_secret_key?: string;
  tencent_captcha_enabled?: boolean;
  tencent_captcha_app_id?: string;
  tencent_captcha_app_secret_key?: string;
  tencent_captcha_cloud_secret_id?: string;
  tencent_captcha_cloud_secret_key?: string;
  tencent_captcha_region?: string;
  aliyun_captcha_enabled?: boolean;
  aliyun_captcha_access_key_id?: string;
  aliyun_captcha_access_key_secret?: string;
  aliyun_captcha_scene_id?: string;
  aliyun_captcha_prefix?: string;
  aliyun_captcha_region?: string;
  api_key_acl_trust_forwarded_ip?: boolean;
  forwarded_client_ip_headers?: string[];
  linuxdo_connect_enabled?: boolean;
  linuxdo_connect_client_id?: string;
  linuxdo_connect_client_secret?: string;
  linuxdo_connect_redirect_url?: string;
  dingtalk_connect_enabled?: boolean;
  dingtalk_connect_client_id?: string;
  dingtalk_connect_client_secret?: string;
  dingtalk_connect_redirect_url?: string;
  dingtalk_connect_corp_restriction_policy?: string;
  dingtalk_connect_internal_corp_id?: string;
  dingtalk_connect_bypass_registration?: boolean;
  dingtalk_connect_sync_corp_email?: boolean;
  dingtalk_connect_sync_display_name?: boolean;
  dingtalk_connect_sync_dept?: boolean;
  dingtalk_connect_sync_corp_email_attr_key?: string;
  dingtalk_connect_sync_display_name_attr_key?: string;
  dingtalk_connect_sync_dept_attr_key?: string;
  dingtalk_connect_sync_corp_email_attr_name?: string;
  dingtalk_connect_sync_display_name_attr_name?: string;
  dingtalk_connect_sync_dept_attr_name?: string;
  wechat_connect_enabled?: boolean;
  wechat_connect_app_id?: string;
  wechat_connect_app_secret?: string;
  wechat_connect_open_app_id?: string;
  wechat_connect_open_app_secret?: string;
  wechat_connect_mp_app_id?: string;
  wechat_connect_mp_app_secret?: string;
  wechat_connect_mobile_app_id?: string;
  wechat_connect_mobile_app_secret?: string;
  wechat_connect_open_enabled?: boolean;
  wechat_connect_mp_enabled?: boolean;
  wechat_connect_mobile_enabled?: boolean;
  wechat_connect_mode?: string;
  wechat_connect_scopes?: string;
  wechat_connect_redirect_url?: string;
  wechat_connect_frontend_redirect_url?: string;
  oidc_connect_enabled?: boolean;
  oidc_connect_provider_name?: string;
  oidc_connect_client_id?: string;
  oidc_connect_client_secret?: string;
  oidc_connect_issuer_url?: string;
  oidc_connect_discovery_url?: string;
  oidc_connect_authorize_url?: string;
  oidc_connect_token_url?: string;
  oidc_connect_userinfo_url?: string;
  oidc_connect_jwks_url?: string;
  oidc_connect_scopes?: string;
  oidc_connect_redirect_url?: string;
  oidc_connect_frontend_redirect_url?: string;
  oidc_connect_token_auth_method?: string;
  oidc_connect_use_pkce?: boolean;
  oidc_connect_validate_id_token?: boolean;
  oidc_connect_allowed_signing_algs?: string;
  oidc_connect_clock_skew_seconds?: number;
  oidc_connect_require_email_verified?: boolean;
  oidc_connect_userinfo_email_path?: string;
  oidc_connect_userinfo_id_path?: string;
  oidc_connect_userinfo_username_path?: string;
  github_oauth_enabled?: boolean;
  github_oauth_client_id?: string;
  github_oauth_client_secret?: string;
  github_oauth_redirect_url?: string;
  github_oauth_frontend_redirect_url?: string;
  google_oauth_enabled?: boolean;
  google_oauth_client_id?: string;
  google_oauth_client_secret?: string;
  google_oauth_redirect_url?: string;
  google_oauth_frontend_redirect_url?: string;
  enable_model_fallback?: boolean;
  fallback_model_anthropic?: string;
  fallback_model_openai?: string;
  fallback_model_gemini?: string;
  fallback_model_antigravity?: string;
  grok_default_text_model?: string;
  grok_cross_client_model_map_enabled?: boolean;
  grok_default_base_url_mode?: string;
  account_scheduling_thresholds?: AccountSchedulingThresholdsMap;
  enable_identity_patch?: boolean;
  identity_patch_prompt?: string;
  ops_monitoring_enabled?: boolean;
  ops_realtime_monitoring_enabled?: boolean;
  ops_query_mode_default?: "auto" | "raw" | "preagg" | string;
  ops_metrics_interval_seconds?: number;
  min_claude_code_version?: string;
  max_claude_code_version?: string;
  kiro_version?: string;
  kiro_commit?: string;
  system_version?: string;
  node_version?: string;
  kiro_code_execution_sandbox_command?: string;
  cache_hit_rate_scale?: number;
  cache_min_block_tokens?: number;
  cache_independent_ttl_seconds?: number;
  cache_prefix_ttl_seconds?: number;
  allow_ungrouped_key_scheduling?: boolean;
  openai_ttft_mode?: string;
  enable_fingerprint_unification?: boolean;
  enable_metadata_passthrough?: boolean;
  enable_cch_signing?: boolean;
  enable_claude_oauth_system_prompt_injection?: boolean;
  claude_oauth_system_prompt?: string;
  claude_oauth_system_prompt_blocks?: string;
  enable_anthropic_cache_ttl_1h_injection?: boolean;
  rewrite_message_cache_control?: boolean;
  enable_client_dateline_normalization?: boolean;
  antigravity_user_agent_version?: string;
  openai_codex_user_agent?: string;
  openai_codex_client_version?: string;
  openai_codex_version_auto_sync_enabled?: boolean;
  // codex_cli_only 加固
  min_codex_version?: string;
  max_codex_version?: string;
  codex_cli_only_blacklist?: string;
  codex_cli_only_whitelist?: string;
  codex_cli_only_allow_app_server_clients?: boolean;
  codex_cli_only_engine_fingerprint_signals?: string;
  // Payment configuration
  payment_enabled?: boolean;
  risk_control_enabled?: boolean;

  // Cyber session block
  cyber_session_block_enabled?: boolean;
  cyber_session_block_ttl_seconds?: number;

  payment_min_amount?: number;
  payment_max_amount?: number;
  payment_daily_limit?: number;
  payment_order_timeout_minutes?: number;
  payment_max_pending_orders?: number;
  payment_enabled_types?: string[];
  payment_balance_disabled?: boolean;
  payment_balance_recharge_multiplier?: number;
  payment_subscription_usd_to_cny_rate?: number;
  payment_recharge_fee_rate?: number;
  payment_load_balance_strategy?: string;
  payment_product_name_prefix?: string;
  payment_product_name_suffix?: string;
  payment_help_image_url?: string;
  payment_help_text?: string;
  payment_cancel_rate_limit_enabled?: boolean;
  payment_cancel_rate_limit_max?: number;
  payment_cancel_rate_limit_window?: number;
  payment_cancel_rate_limit_unit?: string;
  payment_cancel_rate_limit_window_mode?: string;
  payment_alipay_force_qrcode?: boolean;
  payment_alipay_mobile_precreate_deep_link?: boolean;
  payment_visible_method_alipay_source?: string;
  payment_visible_method_wxpay_source?: string;
  payment_visible_method_alipay_enabled?: boolean;
  payment_visible_method_wxpay_enabled?: boolean;
  openai_low_upstream_rate_priority_enabled?: boolean;
  openai_oauth_scheduling_rate_multiplier?: number;
  openai_advanced_scheduler_enabled?: boolean;
  openai_advanced_scheduler_sticky_weighted_enabled?: boolean;
  openai_advanced_scheduler_subscription_priority_enabled?: boolean;
  openai_advanced_scheduler_lb_top_k?: string;
  openai_advanced_scheduler_weight_priority?: string;
  openai_advanced_scheduler_weight_load?: string;
  openai_advanced_scheduler_weight_queue?: string;
  openai_advanced_scheduler_weight_error_rate?: string;
  openai_advanced_scheduler_weight_ttft?: string;
  openai_advanced_scheduler_weight_reset?: string;
  openai_advanced_scheduler_weight_quota_headroom?: string;
  openai_advanced_scheduler_weight_upstream_cost?: string;
  openai_advanced_scheduler_weight_previous_response?: string;
  openai_advanced_scheduler_weight_session_sticky?: string;
  // 余额、订阅到期与账号限额通知
  balance_low_notify_enabled?: boolean;
  balance_low_notify_threshold?: number;
  balance_low_notify_recharge_url?: string;
  subscription_expiry_notify_enabled?: boolean;
  account_quota_notify_enabled?: boolean;
  account_quota_notify_emails?: NotifyEmailEntry[];

  // Channel Monitor feature switch
  channel_monitor_enabled?: boolean;
  channel_monitor_mode?: 'v1' | 'v2';
  channel_monitor_default_interval_seconds?: number;
  channel_monitor_hide_throughput?: boolean;
  channel_monitor_show_quota?: boolean;

  // Available Channels feature switch
  available_channels_enabled?: boolean;

  // Model Plaza feature switches + description
  model_plaza_enabled?: boolean;
  model_plaza_require_auth?: boolean;
  model_plaza_description?: string;
  plugin_management_enabled?: boolean;

  // Affiliate (邀请返利) feature switch
  affiliate_enabled?: boolean;
  ticket_enabled?: boolean;
  creation_center_enabled?: boolean;

  ip_multi_account_ban_enabled?: boolean;
  ip_multi_account_ban_window_minutes?: number;
  ip_multi_account_ban_threshold?: number;
  ip_multi_account_ban_window2_minutes?: number;
  ip_multi_account_ban_threshold2?: number;
  ip_multi_account_ban_learning_until?: string;
  registration_block_datacenter_ip?: boolean;

  // OpenAI fast/flex policy
  openai_fast_policy_settings?: OpenAIFastPolicySettings;

  allow_user_view_error_requests?: boolean;
}

/**
 * Get all system settings
 * @returns System settings
 */
export async function getSettings(): Promise<SystemSettings> {
  const { data } = await apiClient.get<SystemSettings>("/admin/settings");
  return data;
}

/**
 * Update system settings
 * @param settings - Partial settings to update
 * @returns Updated settings
 */
export async function updateSettings(
  settings: UpdateSettingsRequest,
): Promise<SystemSettings> {
  const { data } = await apiClient.put<SystemSettings>(
    "/admin/settings",
    settings,
  );
  return data;
}
