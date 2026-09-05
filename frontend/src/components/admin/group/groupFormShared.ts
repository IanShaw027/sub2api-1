// GroupsView 拆分：创建/编辑分组表单共享的纯函数与类型。
// 仅包含不依赖 useI18n()/组件上下文的纯逻辑，供 GroupCreateModal.vue、
// GroupEditModal.vue 及其子字段组件复用。零功能变更，均从 GroupsView.vue 原样搬迁。
import { adminAPI } from "@/api/admin";
import type { AdminGroup, GroupPlatform } from "@/types";
import type { PricingFormEntry } from "@/components/admin/channel/types";
import {
  apiIntervalsToForm,
  createDefaultTimePricingForm,
  formIntervalsToAPI,
  mTokToPerToken as mTokToPerTokenApi,
  perTokenToMTok,
  toNullableNumber,
} from "@/components/admin/channel/types";
import type { ChannelModelPricing } from "@/api/admin/channels";
import type { LiveCapability } from "@/api/admin/groups";
import { createModelsListState as createInitialModelsListState } from "@/views/admin/groupsModelsList";
import type { VideoModelPricesForm } from "@/views/admin/groupsVideoModelPricing";

export const supportsLivePlatform = (platform: string): boolean =>
  platform === "openai" || platform === "composite";

export const emptyGroupPricing = (): PricingFormEntry => ({
  models: [],
  billing_mode: "token",
  input_price: null,
  output_price: null,
  cache_write_price: null,
  cache_write_1h_price: null,
  cache_read_price: null,
  image_input_price: null,
  image_output_price: null,
  per_request_price: null,
  intervals: [],
  time_pricing: createDefaultTimePricingForm(),
});

export const addGroupPricing = (entries: PricingFormEntry[]) =>
  entries.push(emptyGroupPricing());

export const groupPricingFromAPI = (
  pricing: ChannelModelPricing[] | undefined,
): PricingFormEntry[] =>
  (pricing || []).map((entry) => ({
    models: entry.models || [],
    billing_mode: entry.billing_mode || "token",
    input_price: perTokenToMTok(entry.input_price),
    output_price: perTokenToMTok(entry.output_price),
    cache_write_price: perTokenToMTok(entry.cache_write_price),
    cache_write_1h_price: perTokenToMTok(entry.cache_write_1h_price),
    cache_read_price: perTokenToMTok(entry.cache_read_price),
    image_input_price: perTokenToMTok(entry.image_input_price),
    image_output_price: perTokenToMTok(entry.image_output_price),
    per_request_price: entry.per_request_price,
    intervals: apiIntervalsToForm(entry.intervals || []),
    time_pricing: createDefaultTimePricingForm(),
  }));

export const groupPricingToAPI = (
  pricing: PricingFormEntry[],
  platform: string,
): ChannelModelPricing[] =>
  pricing
    .filter((entry) => entry.models.length > 0)
    .map((entry) => ({
      platform,
      models: entry.models,
      billing_mode: entry.billing_mode,
      input_price: mTokToPerTokenApi(entry.input_price),
      output_price: mTokToPerTokenApi(entry.output_price),
      cache_write_price: mTokToPerTokenApi(entry.cache_write_price),
      cache_write_1h_price: mTokToPerTokenApi(entry.cache_write_1h_price),
      cache_read_price: mTokToPerTokenApi(entry.cache_read_price),
      image_input_price: mTokToPerTokenApi(entry.image_input_price),
      image_output_price: mTokToPerTokenApi(entry.image_output_price),
      per_request_price: toNullableNumber(entry.per_request_price),
      intervals:
        entry.billing_mode === "token"
          ? []
          : formIntervalsToAPI(entry.intervals || []),
      time_pricing: null,
    }));

// 简单账号类型（用于模型路由选择）
export interface SimpleAccount {
  id: number;
  name: string;
}

// 模型路由规则类型
export interface ModelRoutingRule {
  pattern: string;
  accounts: SimpleAccount[]; // 选中的账号对象数组
}

export const canCopyAccountsFromGroup = (
  targetPlatform: GroupPlatform,
  sourcePlatform: GroupPlatform,
) => targetPlatform === "composite" || sourcePlatform === targetPlatform;

export const copyAccountsGroupLabel = (
  t: (key: string, params?: Record<string, unknown>) => string,
  g: AdminGroup,
) => {
  const count = g.account_count || 0;
  const platform = t("admin.groups.platforms." + g.platform);
  return `${g.name} - ${platform} (${t("admin.groups.accountsCount", { count })})`;
};

export type ImagePricingFormState = {
  platform: GroupPlatform;
  allow_image_generation: boolean;
  allow_batch_image_generation: boolean;
  rate_multiplier: number;
  image_rate_independent: boolean;
  image_rate_multiplier: number;
  batch_image_discount_multiplier: number;
  batch_image_hold_multiplier: number;
  image_price_1k: number | string | null;
  image_price_2k: number | string | null;
  image_price_4k: number | string | null;
  peak_rate_enabled: boolean;
  peak_start: string;
  peak_end: string;
  peak_rate_multiplier: number;
};

export type VideoPricingFormState = {
  platform: GroupPlatform;
  rate_multiplier: number;
  video_rate_independent: boolean;
  video_rate_multiplier: number;
  video_price_480p: number | string | null;
  video_price_720p: number | string | null;
  video_price_1080p: number | string | null;
  video_model_prices: VideoModelPricesForm;
};

export const imagePricingTiers = [
  { key: "image_price_1k", label: "1K" },
  { key: "image_price_2k", label: "2K" },
  { key: "image_price_4k", label: "4K" },
] as const;

export const videoPricingTiers = [
  { key: "video_price_480p", label: "480p" },
  { key: "video_price_720p", label: "720p" },
  { key: "video_price_1080p", label: "1080p" },
] as const;

export const normalizePreviewNumber = (
  value: number | string | null | undefined,
  fallback = 0,
) => {
  if (value === null || value === undefined || value === "") {
    return fallback;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
};

export const parsePreviewPrice = (value: number | string | null | undefined) => {
  if (value === null || value === undefined || value === "") {
    return null;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
};

// Codex 网页搜索单次默认价（与后端 defaultWebSearchPricePerCall 一致，官方 $10/1000 次）
export const DEFAULT_WEB_SEARCH_PRICE_PER_CALL = 0.01;

export const resetDisabledBatchImagePricing = (
  form: Pick<
    ImagePricingFormState,
    | "platform"
    | "allow_image_generation"
    | "allow_batch_image_generation"
    | "batch_image_discount_multiplier"
    | "batch_image_hold_multiplier"
  >,
) => {
  if (form.platform !== "gemini" || !form.allow_image_generation) {
    form.allow_batch_image_generation = false;
  }
  if (!form.allow_batch_image_generation) {
    form.batch_image_discount_multiplier = 0.5;
    form.batch_image_hold_multiplier = 0.6;
  }
};

// 将 UI 格式的路由规则转换为 API 格式
export const convertRoutingRulesToApiFormat = (
  rules: ModelRoutingRule[],
): Record<string, number[]> | null => {
  const result: Record<string, number[]> = {};
  let hasValidRules = false;

  for (const rule of rules) {
    const pattern = rule.pattern.trim();
    if (!pattern) continue;

    const accountIds = rule.accounts.map((a) => a.id).filter((id) => id > 0);

    if (accountIds.length > 0) {
      result[pattern] = accountIds;
      hasValidRules = true;
    }
  }

  return hasValidRules ? result : null;
};

// 将 API 格式的路由规则转换为 UI 格式（需要加载账号名称）
export const convertApiFormatToRoutingRules = async (
  apiFormat: Record<string, number[]> | null,
): Promise<ModelRoutingRule[]> => {
  if (!apiFormat) return [];

  const rules: ModelRoutingRule[] = [];
  for (const [pattern, accountIds] of Object.entries(apiFormat)) {
    // 加载账号信息
    const accounts: SimpleAccount[] = [];
    for (const id of accountIds) {
      try {
        const account = await adminAPI.accounts.getById(id);
        accounts.push({ id: account.id, name: account.name });
      } catch {
        // 如果账号不存在，仍然显示 ID
        accounts.push({ id, name: `#${id}` });
      }
    }
    rules.push({ pattern, accounts });
  }
  return rules;
};

export const normalizeOptionalLimit = (
  value: number | string | null | undefined,
): number | null => {
  if (value === null || value === undefined) {
    return null;
  }

  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed) {
      return null;
    }
    const parsed = Number(trimmed);
    return Number.isFinite(parsed) && parsed > 0 ? parsed : null;
  }

  return Number.isFinite(value) && value > 0 ? value : null;
};

export const normalizeRateMultiplier = (
  value: number | string | null | undefined,
): number => {
  if (value === null || value === undefined || value === "") {
    return 1;
  }
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed >= 0 ? parsed : 1;
};

export const resetModelsListState = (
  state: ReturnType<typeof createInitialModelsListState>,
  config?: Parameters<typeof createInitialModelsListState>[0],
) => {
  const fresh = createInitialModelsListState(config);
  state.enabled = fresh.enabled;
  state.savedModels = fresh.savedModels;
  state.items = fresh.items;
};

export type ReasoningEffortPolicyFieldsExpose = {
  validate: () => boolean;
  resetValidation: () => void;
};

// Codex/OpenAI Live 能力探测：模块级单例缓存 + 请求去重，
// 与原 GroupsView.vue 中的模块级 `liveCapabilityRequest` 语义保持一致
// （无论多少个表单/组件实例调用，全局只请求一次）。
let liveCapabilityCache: LiveCapability | null = null;
let liveCapabilityRequest: Promise<LiveCapability> | null = null;

export const loadLiveCapability = async (): Promise<LiveCapability> => {
  if (liveCapabilityCache) return liveCapabilityCache;
  if (!liveCapabilityRequest) {
    liveCapabilityRequest = adminAPI.groups
      .getLiveCapability()
      .catch(() => ({ supported: false }))
      .finally(() => {
        liveCapabilityRequest = null;
      });
  }
  liveCapabilityCache = await liveCapabilityRequest;
  return liveCapabilityCache ?? { supported: false };
};
