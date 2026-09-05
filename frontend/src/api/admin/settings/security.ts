/**
 * Admin Settings API — security / auth-defaults / platform-quota /
 * WeChat-connect / Kiro-runtime-validation / admin-api-key / panel-rate-limit
 * helpers.
 *
 * NOTE: the WeChat `wechat_connect_mode` helpers below are relocated
 * verbatim (unchanged logic) from the original monolithic settings.ts.
 */

import { apiClient } from "../../client";
import type { SystemSettings, UpdateSettingsRequest } from "./general";

export interface DefaultSubscriptionSetting {
  group_id: number;
  validity_days: number;
}

// ── 平台限额类型 ──────────────────────────────────────────────────
export type PlatformType = "anthropic" | "openai" | "gemini" | "antigravity" | "grok" | "kiro"
export type QuotaWindowType = "daily" | "weekly" | "monthly"

/** 单平台三档限额；null = 不限制，undefined = 未填（等价 null） */
export interface PlatformQuotaLimits {
  daily:   number | null
  weekly:  number | null
  monthly: number | null
}

/** 全平台默认限额 map（key = PlatformType） */
export type DefaultPlatformQuotasMap = Partial<Record<PlatformType, PlatformQuotaLimits>>

const PLATFORMS: PlatformType[] = ["anthropic", "openai", "gemini", "antigravity", "grok", "kiro"]

export type SchedulingThresholdPlatformType =
  | "openai"
  | "anthropic"
  | "grok"
  | "kimi"
  | "zhipu"

export type AccountSchedulingThresholdsMap = Record<SchedulingThresholdPlatformType, number>

// 与后端 AllowedSchedulingThresholdPlatforms 保持一致（deepseek 为余额型，
// 走余额检测而非用量阈值）。
export const SCHEDULING_THRESHOLD_PLATFORMS: SchedulingThresholdPlatformType[] = [
  "openai",
  "anthropic",
  "grok",
  "kimi",
  "zhipu",
]

export function normalizeAccountSchedulingThresholdsMap(
  input?: Partial<Record<SchedulingThresholdPlatformType, number>> | null,
): AccountSchedulingThresholdsMap {
  const result = {} as AccountSchedulingThresholdsMap
  for (const platform of SCHEDULING_THRESHOLD_PLATFORMS) {
    const value = input?.[platform]
    result[platform] = typeof value === "number" && Number.isFinite(value)
      ? Math.min(100, Math.max(1, Math.trunc(value)))
      : 100
  }
  return result
}

export function sanitizeAccountSchedulingThresholdsMap(
  input?: Partial<Record<SchedulingThresholdPlatformType, number>> | null,
): AccountSchedulingThresholdsMap {
  return normalizeAccountSchedulingThresholdsMap(input)
}

/** 归一化为全 4 平台 × 3 窗口（缺失填 null），供模板非空绑定 */
export function normalizePlatformQuotasMap(input?: DefaultPlatformQuotasMap | null): DefaultPlatformQuotasMap {
  const result: DefaultPlatformQuotasMap = {}
  for (const p of PLATFORMS) {
    const src = input?.[p]
    result[p] = {
      daily:   typeof src?.daily === "number" ? src.daily : null,
      weekly:  typeof src?.weekly === "number" ? src.weekly : null,
      monthly: typeof src?.monthly === "number" ? src.monthly : null,
    }
  }
  return result
}

/** 提交前清洗：非有限数/负数/空字符串 → null（保留 0 = 显式禁用），返回全 4 平台嵌套 map */
export function sanitizePlatformQuotasMap(input?: DefaultPlatformQuotasMap | null): DefaultPlatformQuotasMap {
  const clean = (v: unknown): number | null => (typeof v === "number" && Number.isFinite(v) && v >= 0 ? v : null)
  const result: DefaultPlatformQuotasMap = {}
  for (const p of PLATFORMS) {
    const src = input?.[p]
    result[p] = { daily: clean(src?.daily), weekly: clean(src?.weekly), monthly: clean(src?.monthly) }
  }
  return result
}

export type AuthSourceType =
  | "email"
  | "linuxdo"
  | "oidc"
  | "wechat"
  | "github"
  | "google"
  | "dingtalk";

export interface AuthSourceDefaultsValue {
  balance: number;
  concurrency: number;
  subscriptions: DefaultSubscriptionSetting[];
  grant_on_signup: boolean;
  grant_on_first_bind: boolean;
  // ★ 新增：平台限额覆盖（key = PlatformType）
  platform_quotas: DefaultPlatformQuotasMap;
}

export type AuthSourceDefaultsState = Record<
  AuthSourceType,
  AuthSourceDefaultsValue
>;
export type PaymentVisibleMethod = "alipay" | "wxpay";
export type PaymentVisibleMethodSource =
  | ""
  | "official_alipay"
  | "easypay_alipay"
  | "official_wxpay"
  | "easypay_wxpay";
export type WeChatConnectMode = "open" | "mp" | "mobile";

export interface PaymentVisibleMethodSourceOption {
  value: PaymentVisibleMethodSource;
  labelZh: string;
  labelEn: string;
}

export interface WeChatConnectModeOption {
  value: WeChatConnectMode;
  labelZh: string;
  labelEn: string;
}

const AUTH_SOURCE_TYPES: AuthSourceType[] = [
  "email",
  "linuxdo",
  "oidc",
  "wechat",
  "github",
  "google",
  "dingtalk",
];
const AUTH_SOURCE_DEFAULT_BALANCE = 0;
const AUTH_SOURCE_DEFAULT_CONCURRENCY = 5;
const PAYMENT_VISIBLE_METHOD_SOURCE_OPTIONS: Record<
  PaymentVisibleMethod,
  PaymentVisibleMethodSourceOption[]
> = {
  alipay: [
    { value: "", labelZh: "未配置", labelEn: "Not configured" },
    {
      value: "official_alipay",
      labelZh: "支付宝官方",
      labelEn: "Official Alipay",
    },
    {
      value: "easypay_alipay",
      labelZh: "易支付支付宝",
      labelEn: "EasyPay Alipay",
    },
  ],
  wxpay: [
    { value: "", labelZh: "未配置", labelEn: "Not configured" },
    {
      value: "official_wxpay",
      labelZh: "微信官方",
      labelEn: "Official WeChat Pay",
    },
    {
      value: "easypay_wxpay",
      labelZh: "易支付微信",
      labelEn: "EasyPay WeChat Pay",
    },
  ],
};
const PAYMENT_VISIBLE_METHOD_SOURCE_ALIASES: Record<
  PaymentVisibleMethod,
  Record<string, PaymentVisibleMethodSource>
> = {
  alipay: {
    official_alipay: "official_alipay",
    alipay: "official_alipay",
    alipay_direct: "official_alipay",
    official: "official_alipay",
    easypay_alipay: "easypay_alipay",
    easypay: "easypay_alipay",
  },
  wxpay: {
    official_wxpay: "official_wxpay",
    wxpay: "official_wxpay",
    wxpay_direct: "official_wxpay",
    wechat: "official_wxpay",
    official: "official_wxpay",
    easypay_wxpay: "easypay_wxpay",
    easypay: "easypay_wxpay",
  },
};
const WECHAT_CONNECT_MODE_OPTIONS: WeChatConnectModeOption[] = [
  { value: "open", labelZh: "PC 应用", labelEn: "PC App" },
  {
    value: "mp",
    labelZh: "公众号",
    labelEn: "Official Account",
  },
  {
    value: "mobile",
    labelZh: "移动应用",
    labelEn: "Mobile App",
  },
];
export type KiroRuntimeValidationError =
  | "cache_hit_rate_scale_range"
  | "cache_min_block_tokens_range"
  | "cache_independent_ttl_seconds_range"
  | "cache_prefix_ttl_seconds_range"
  | "cache_prefix_ttl_seconds_exceeds_independent";

export interface KiroRuntimeSettingsInput {
  kiro_version?: string | null;
  kiro_commit?: string | null;
  system_version?: string | null;
  node_version?: string | null;
  kiro_code_execution_sandbox_command?: string | null;
  cache_hit_rate_scale?: number | null;
  cache_min_block_tokens?: number | null;
  cache_independent_ttl_seconds?: number | null;
  cache_prefix_ttl_seconds?: number | null;
}

export const KIRO_CACHE_HIT_RATE_SCALE_DEFAULT = 85;
export const KIRO_CACHE_MIN_BLOCK_TOKENS_DEFAULT = 1024;
export const KIRO_CACHE_MIN_BLOCK_TOKENS_MAX = 1 << 20;
export const KIRO_CACHE_INDEPENDENT_TTL_SECONDS_DEFAULT = 3600;
export const KIRO_CACHE_PREFIX_TTL_SECONDS_DEFAULT = 3600;

const WECHAT_CONNECT_MODE_ALIASES: Record<string, WeChatConnectMode> = {
  open: "open",
  open_platform: "open",
  official: "open",
  wx_open: "open",
  mp: "mp",
  official_account: "mp",
  wechat_mp: "mp",
  mini_program: "mp",
  mobile: "mobile",
  mobile_app: "mobile",
  native_app: "mobile",
};

export function normalizeDefaultSubscriptionSettings(
  subscriptions: DefaultSubscriptionSetting[] | null | undefined,
): DefaultSubscriptionSetting[] {
  if (!Array.isArray(subscriptions)) return [];

  return subscriptions
    .filter((item) => item.group_id > 0 && item.validity_days > 0)
    .map((item) => ({
      group_id: Math.floor(item.group_id),
      validity_days: Math.min(
        36500,
        Math.max(1, Math.floor(item.validity_days)),
      ),
    }));
}

export function buildAuthSourceDefaultsState(
  settings: Partial<SystemSettings>,
): AuthSourceDefaultsState {
  const raw = settings as Record<string, unknown>;

  return AUTH_SOURCE_TYPES.reduce((acc, source) => {
    const subscriptions = raw[`auth_source_default_${source}_subscriptions`];
    acc[source] = {
      balance: Number(
        raw[`auth_source_default_${source}_balance`] ??
          AUTH_SOURCE_DEFAULT_BALANCE,
      ),
      concurrency: Math.max(
        1,
        Number(
          raw[`auth_source_default_${source}_concurrency`] ??
            AUTH_SOURCE_DEFAULT_CONCURRENCY,
        ),
      ),
      subscriptions: normalizeDefaultSubscriptionSettings(
        Array.isArray(subscriptions)
          ? (subscriptions as DefaultSubscriptionSetting[])
          : [],
      ),
      grant_on_signup:
        raw[`auth_source_default_${source}_grant_on_signup`] === true,
      grant_on_first_bind:
        raw[`auth_source_default_${source}_grant_on_first_bind`] === true,
      platform_quotas: normalizePlatformQuotasMap(raw[`auth_source_default_${source}_platform_quotas`] as DefaultPlatformQuotasMap | undefined),
    };
    return acc;
  }, {} as AuthSourceDefaultsState);
}

export function appendAuthSourceDefaultsToUpdateRequest(
  payload: UpdateSettingsRequest,
  authSourceDefaults: AuthSourceDefaultsState,
): UpdateSettingsRequest {
  const target = payload as Record<string, unknown>;

  for (const source of AUTH_SOURCE_TYPES) {
    const current = authSourceDefaults[source];
    target[`auth_source_default_${source}_balance`] =
      Number(current.balance) || 0;
    target[`auth_source_default_${source}_concurrency`] = Math.max(
      1,
      Math.floor(
        Number(current.concurrency) || AUTH_SOURCE_DEFAULT_CONCURRENCY,
      ),
    );
    target[`auth_source_default_${source}_subscriptions`] =
      normalizeDefaultSubscriptionSettings(current.subscriptions);
    target[`auth_source_default_${source}_grant_on_signup`] =
      current.grant_on_signup;
    target[`auth_source_default_${source}_grant_on_first_bind`] =
      current.grant_on_first_bind;
    target[`auth_source_default_${source}_platform_quotas`] = sanitizePlatformQuotasMap(current.platform_quotas)
  }

  return payload;
}

export function getPaymentVisibleMethodSourceOptions(
  method: PaymentVisibleMethod,
): PaymentVisibleMethodSourceOption[] {
  return PAYMENT_VISIBLE_METHOD_SOURCE_OPTIONS[method];
}

export function normalizePaymentVisibleMethodSource(
  method: PaymentVisibleMethod,
  source: unknown,
): PaymentVisibleMethodSource {
  if (typeof source !== "string") return "";

  const normalized = source.trim().toLowerCase();
  if (!normalized) return "";

  return PAYMENT_VISIBLE_METHOD_SOURCE_ALIASES[method][normalized] ?? "";
}

export function getWeChatConnectModeOptions(): WeChatConnectModeOption[] {
  return WECHAT_CONNECT_MODE_OPTIONS;
}

export function normalizeWeChatConnectMode(source: unknown): WeChatConnectMode {
  if (typeof source !== "string") return "open";

  const normalized = source.trim().toLowerCase();
  if (!normalized) return "open";

  return WECHAT_CONNECT_MODE_ALIASES[normalized] ?? "open";
}

export function defaultWeChatConnectScopesForMode(mode: unknown): string {
  switch (normalizeWeChatConnectMode(mode)) {
    case "mp":
      return "snsapi_userinfo";
    case "mobile":
      return "";
    default:
      return "snsapi_login";
  }
}

export function resolveWeChatConnectModeCapabilities(
  openEnabled: unknown,
  mpEnabled: unknown,
  mobileEnabled: unknown,
  legacyMode: unknown,
): { openEnabled: boolean; mpEnabled: boolean; mobileEnabled: boolean } {
  if (
    typeof openEnabled === "boolean" ||
    typeof mpEnabled === "boolean" ||
    typeof mobileEnabled === "boolean"
  ) {
    return {
      openEnabled: openEnabled === true,
      mpEnabled: mpEnabled === true,
      mobileEnabled: mobileEnabled === true,
    };
  }

  switch (normalizeWeChatConnectMode(legacyMode)) {
    case "mp":
      return { openEnabled: false, mpEnabled: true, mobileEnabled: false };
    case "mobile":
      return { openEnabled: false, mpEnabled: false, mobileEnabled: true };
    default:
      return { openEnabled: true, mpEnabled: false, mobileEnabled: false };
  }
}

export function deriveWeChatConnectStoredMode(
  openEnabled: boolean,
  mpEnabled: boolean,
  mobileEnabled: boolean,
  legacyMode: unknown,
): WeChatConnectMode {
  if (mpEnabled) return "mp";
  if (mobileEnabled) return "mobile";
  if (openEnabled) return "open";
  return normalizeWeChatConnectMode(legacyMode);
}

function normalizeOptionalInteger(value: unknown): number | undefined {
  if (value == null || value === "") return undefined;

  const normalized = Math.floor(Number(value));
  return Number.isFinite(normalized) ? normalized : undefined;
}

function normalizeOptionalIntegerForValidation(
  value: unknown,
  resetValue: number,
): number | undefined {
  if (value === undefined) return undefined;
  if (value === null || value === "") return resetValue;

  return normalizeOptionalInteger(value);
}

function normalizeOptionalIntegerForUpdate(
  value: unknown,
  resetValue: number,
): number | undefined {
  if (value === undefined) return undefined;
  if (value === null || value === "") return resetValue;

  return normalizeOptionalInteger(value);
}

export function validateKiroRuntimeSettings(
  settings: KiroRuntimeSettingsInput,
): KiroRuntimeValidationError | null {
  const cacheHitRateScale = normalizeOptionalIntegerForValidation(
    settings.cache_hit_rate_scale,
    KIRO_CACHE_HIT_RATE_SCALE_DEFAULT,
  );
  if (
    cacheHitRateScale != null &&
    (cacheHitRateScale < 0 || cacheHitRateScale > 100)
  ) {
    return "cache_hit_rate_scale_range";
  }

  const cacheMinBlockTokens = normalizeOptionalIntegerForValidation(
    settings.cache_min_block_tokens,
    KIRO_CACHE_MIN_BLOCK_TOKENS_DEFAULT,
  );
  if (
    cacheMinBlockTokens != null &&
    (cacheMinBlockTokens < 0 ||
      cacheMinBlockTokens > KIRO_CACHE_MIN_BLOCK_TOKENS_MAX)
  ) {
    return "cache_min_block_tokens_range";
  }

  const cacheIndependentTtlSeconds = normalizeOptionalIntegerForValidation(
    settings.cache_independent_ttl_seconds,
    KIRO_CACHE_INDEPENDENT_TTL_SECONDS_DEFAULT,
  );
  if (
    cacheIndependentTtlSeconds != null &&
    (cacheIndependentTtlSeconds < 60 || cacheIndependentTtlSeconds > 86400)
  ) {
    return "cache_independent_ttl_seconds_range";
  }

  const cachePrefixTtlSeconds = normalizeOptionalIntegerForValidation(
    settings.cache_prefix_ttl_seconds,
    KIRO_CACHE_PREFIX_TTL_SECONDS_DEFAULT,
  );
  if (
    cachePrefixTtlSeconds != null &&
    (cachePrefixTtlSeconds < 60 || cachePrefixTtlSeconds > 3600)
  ) {
    return "cache_prefix_ttl_seconds_range";
  }

  if (
    cacheIndependentTtlSeconds != null &&
    cachePrefixTtlSeconds != null &&
    cachePrefixTtlSeconds > cacheIndependentTtlSeconds
  ) {
    return "cache_prefix_ttl_seconds_exceeds_independent";
  }

  return null;
}

export function normalizeKiroRuntimeSettingsForUpdate(
  settings: KiroRuntimeSettingsInput,
): Pick<
  UpdateSettingsRequest,
  | "kiro_version"
  | "kiro_commit"
  | "system_version"
  | "node_version"
  | "kiro_code_execution_sandbox_command"
  | "cache_hit_rate_scale"
  | "cache_min_block_tokens"
  | "cache_independent_ttl_seconds"
  | "cache_prefix_ttl_seconds"
> {
  const payload: Pick<
    UpdateSettingsRequest,
    | "kiro_version"
    | "kiro_commit"
    | "system_version"
    | "node_version"
    | "kiro_code_execution_sandbox_command"
    | "cache_hit_rate_scale"
    | "cache_min_block_tokens"
    | "cache_independent_ttl_seconds"
    | "cache_prefix_ttl_seconds"
  > = {};

  if (settings.kiro_version !== undefined) {
    payload.kiro_version = String(settings.kiro_version ?? "").trim();
  }
  if (settings.kiro_commit !== undefined) {
    payload.kiro_commit = String(settings.kiro_commit ?? "").trim();
  }
  if (settings.system_version !== undefined) {
    payload.system_version = String(settings.system_version ?? "").trim();
  }
  if (settings.node_version !== undefined) {
    payload.node_version = String(settings.node_version ?? "").trim();
  }
  if (settings.kiro_code_execution_sandbox_command !== undefined) {
    payload.kiro_code_execution_sandbox_command = String(
      settings.kiro_code_execution_sandbox_command ?? "",
    ).trim();
  }

  const cacheHitRateScale = normalizeOptionalIntegerForUpdate(
    settings.cache_hit_rate_scale,
    KIRO_CACHE_HIT_RATE_SCALE_DEFAULT,
  );
  if (cacheHitRateScale !== undefined) {
    payload.cache_hit_rate_scale = cacheHitRateScale;
  }

  const cacheMinBlockTokens = normalizeOptionalIntegerForUpdate(
    settings.cache_min_block_tokens,
    KIRO_CACHE_MIN_BLOCK_TOKENS_DEFAULT,
  );
  if (cacheMinBlockTokens !== undefined) {
    payload.cache_min_block_tokens = cacheMinBlockTokens;
  }

  const cacheIndependentTtlSeconds = normalizeOptionalIntegerForUpdate(
    settings.cache_independent_ttl_seconds,
    KIRO_CACHE_INDEPENDENT_TTL_SECONDS_DEFAULT,
  );
  if (cacheIndependentTtlSeconds !== undefined) {
    payload.cache_independent_ttl_seconds = cacheIndependentTtlSeconds;
  }

  const cachePrefixTtlSeconds = normalizeOptionalIntegerForUpdate(
    settings.cache_prefix_ttl_seconds,
    KIRO_CACHE_PREFIX_TTL_SECONDS_DEFAULT,
  );
  if (cachePrefixTtlSeconds !== undefined) {
    payload.cache_prefix_ttl_seconds = cachePrefixTtlSeconds;
  }

  return payload;
}

/**
 * Admin API Key status response
 */
export interface AdminApiKeyStatus {
  exists: boolean;
  masked_key: string;
}

/**
 * Get admin API key status
 * @returns Status indicating if key exists and masked version
 */
export async function getAdminApiKey(): Promise<AdminApiKeyStatus> {
  const { data } = await apiClient.get<AdminApiKeyStatus>(
    "/admin/settings/admin-api-key",
  );
  return data;
}

/**
 * Regenerate admin API key
 * @returns The new full API key (only shown once)
 */
export async function regenerateAdminApiKey(): Promise<{ key: string }> {
  const { data } = await apiClient.post<{ key: string }>(
    "/admin/settings/admin-api-key/regenerate",
  );
  return data;
}

/**
 * Delete admin API key
 * @returns Success message
 */
export async function deleteAdminApiKey(): Promise<{ message: string }> {
  const { data } = await apiClient.delete<{ message: string }>(
    "/admin/settings/admin-api-key",
  );
  return data;
}

// ==================== Panel Rate Limit Settings ====================

/**
 * Panel API rate limit settings.
 * Authenticated panel endpoints are limited per user account (reverse-proxy
 * safe); public endpoints are limited per publicly routable client IP.
 */
export interface PanelRateLimitSettings {
  enabled: boolean;
  user_rpm: number;
  heavy_rpm: number;
  exempt_admin: boolean;
  public_ip_rpm: number;
}

export async function getPanelRateLimitSettings(): Promise<PanelRateLimitSettings> {
  const { data } = await apiClient.get<PanelRateLimitSettings>(
    "/admin/settings/panel-rate-limit",
  );
  return data;
}

export async function updatePanelRateLimitSettings(
  settings: PanelRateLimitSettings,
): Promise<PanelRateLimitSettings> {
  const { data } = await apiClient.put<PanelRateLimitSettings>(
    "/admin/settings/panel-rate-limit",
    settings,
  );
  return data;
}
