// GroupsView 拆分：编辑分组弹窗的表单状态与业务逻辑（零功能变更，从
// GroupsView.vue 原样搬迁）。GroupEditModal.vue 仅负责模板装配与 UI 事件转发。
import { computed, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useAppStore } from "@/stores/app";
import { adminAPI } from "@/api/admin";
import type { AdminGroup, GroupPlatform, SubscriptionType } from "@/types";
import { GROUP_PLATFORM_OPTIONS } from "@/constants/platforms";
import type { PricingFormEntry } from "@/components/admin/channel/types";
import type { ChannelModelPricing } from "@/api/admin/channels";
import { extractApiErrorMessage } from "@/utils/apiError";
import {
  supportsLivePlatform,
  groupPricingFromAPI,
  groupPricingToAPI,
  canCopyAccountsFromGroup,
  copyAccountsGroupLabel,
  convertApiFormatToRoutingRules,
  convertRoutingRulesToApiFormat,
  normalizeOptionalLimit,
  normalizeRateMultiplier,
  resetDisabledBatchImagePricing,
  resetModelsListState,
  loadLiveCapability,
  type ModelRoutingRule,
  type ReasoningEffortPolicyFieldsExpose,
} from "./groupFormShared";
import {
  createDefaultMessagesDispatchFormState,
  messagesDispatchConfigToFormState,
  messagesDispatchFormStateToConfig,
  resetMessagesDispatchFormState,
  supportsMessagesDispatchPlatform,
  type MessagesDispatchMappingRow,
} from "@/views/admin/groupsMessagesDispatch";
import { normalizeGroupOpenAIFast } from "@/views/admin/groupsOpenAIFast";
import {
  buildModelsListConfig,
  createModelsListState as createInitialModelsListState,
  moveModelsListItem,
  setModelsListCandidates,
} from "@/views/admin/groupsModelsList";
import { createModelsListCandidatesTracker } from "@/views/admin/groupsModelsListCandidates";
import { normalizeSupportedModelScopesForPlatform } from "@/views/admin/groupsSupportedModelScopes";
import {
  isProfitControlPlatform,
  profitDecimalToPercent,
  profitPercentToDecimal,
  validateProfitControlFormState,
  type ProfitControlFormState,
} from "@/views/admin/groupsProfitControl";
import {
  normalizeReasoningEffortForPlatform,
  normalizeReasoningEffortOverLimit,
  reasoningEffortMappingsToAPI,
  reasoningEffortMappingsToRows,
  reasoningEffortOverLimitDowngrade,
  supportsReasoningEffortPolicyPlatform,
  type ReasoningEffortMappingRow,
} from "@/views/admin/groupsReasoningEffort";
import {
  createVideoModelPricesForm,
  serializeVideoModelPrices,
} from "@/views/admin/groupsVideoModelPricing";

export interface UseEditGroupFormProps {
  groups: AdminGroup[];
}

export interface UseEditGroupFormEmits {
  (e: "close"): void;
  (e: "updated"): void;
  (e: "unsupportedLive"): void;
  (e: "open"): void;
}

export const useEditGroupForm = (
  props: UseEditGroupFormProps,
  emit: UseEditGroupFormEmits,
) => {
  const { t } = useI18n();
  const appStore = useAppStore();

  const submitting = ref(false);

  const editMessagesDispatchDefaults = createDefaultMessagesDispatchFormState();
  const editModelsListState = reactive(createInitialModelsListState());
  const editModelsListLoading = ref(false);
  const modelsListCandidatesTracker = createModelsListCandidatesTracker();
  const editModelsListSelectedCount = computed(
    () => editModelsListState.items.filter((item) => item.selected).length,
  );

  const editReasoningEffortPolicyRef =
    ref<ReasoningEffortPolicyFieldsExpose | null>(null);
  const modelRoutingRef = ref<{ reset: () => void } | null>(null);

  const editingGroup = ref<AdminGroup | null>(null);

  const editForm = reactive({
    name: "",
    description: "",
    platform: "anthropic" as GroupPlatform,
    rate_multiplier: 1.0,
    is_exclusive: false,
    status: "active" as "active" | "inactive",
    subscription_type: "standard" as SubscriptionType,
    daily_limit_usd: null as number | null,
    weekly_limit_usd: null as number | null,
    monthly_limit_usd: null as number | null,
    long_context_pricing_enabled: true,
    force_openai_fast: false,
    free_openai_fast: false,
    model_pricing: [] as PricingFormEntry[],
    // 图片生成计费配置
    allow_image_generation: false,
    allow_batch_image_generation: false,
    image_rate_independent: false,
    image_rate_multiplier: 1,
    batch_image_discount_multiplier: 0.5,
    batch_image_hold_multiplier: 0.6,
    image_price_1k: null as number | null,
    image_price_2k: null as number | null,
    image_price_4k: null as number | null,
    // 视频生成计费配置（仅 Grok 平台）
    video_rate_independent: false,
    video_rate_multiplier: 1,
    video_price_480p: null as number | null,
    video_price_720p: null as number | null,
    video_price_1080p: null as number | null,
    video_model_prices: createVideoModelPricesForm(),
    // Codex 网页搜索按次计费（仅 openai 平台使用）；null = 使用默认价 0.01
    web_search_price_per_call: null as number | null,
    search_price_per_1k: null as number | null,
    audio_realtime_price_per_min: null as number | null,
    audio_tts_price_per_million_chars: null as number | null,
    audio_stt_price_per_hour: null as number | null,
    // 高峰时段倍率配置
    peak_rate_enabled: false,
    peak_start: "",
    peak_end: "",
    peak_rate_multiplier: 1.0,
    // 分组利润控制（五个 token 平台）；界面按百分比输入，提交时转小数
    profit_control_enabled: false,
    profit_min_margin_percent: 0,
    profit_safety_buffer_percent: 0,
    // Claude Code 客户端限制（仅 anthropic 平台使用）
    claude_code_only: false,
    fallback_group_id: null as number | null,
    fallback_group_id_on_invalid_request: null as number | null,
    // OpenAI Messages 调度配置（仅 openai 平台使用）
    allow_messages_dispatch: false,
    allow_live: false,
    default_mapped_model: "",
    opus_mapped_model: editMessagesDispatchDefaults.opus_mapped_model,
    sonnet_mapped_model: editMessagesDispatchDefaults.sonnet_mapped_model,
    haiku_mapped_model: editMessagesDispatchDefaults.haiku_mapped_model,
    exact_model_mappings: [] as MessagesDispatchMappingRow[],
    // 账号过滤控制（OpenAI/Antigravity 平台）
    require_oauth_only: false,
    require_privacy_set: false,
    // 模型路由开关
    model_routing_enabled: false,
    // 支持的模型系列（仅 antigravity 平台）
    supported_model_scopes: ["claude", "gemini_text", "gemini_image"] as string[],
    // MCP XML 协议注入开关（仅 antigravity 平台）
    mcp_xml_inject: true,
    // 从分组复制账号
    copy_accounts_from_group_ids: [] as number[],
    // 分组级 RPM 限制（每用户每分钟最大请求数；0 = 不限制）
    rpm_limit: 0 as number,
    max_reasoning_effort: "",
    max_reasoning_effort_over_limit: reasoningEffortOverLimitDowngrade,
    reasoning_effort_mappings: [] as ReasoningEffortMappingRow[],
  });

  const editModelRoutingRules = ref<ModelRoutingRule[]>([]);

  const platformOptions = computed(() => [...GROUP_PLATFORM_OPTIONS]);

  const editStatusOptions = computed(() => [
    { value: "active", label: t("admin.accounts.status.active") },
    { value: "inactive", label: t("admin.accounts.status.inactive") },
  ]);

  const subscriptionTypeOptions = computed(() => [
    { value: "standard", label: t("admin.groups.subscription.standard") },
    { value: "subscription", label: t("admin.groups.subscription.subscription") },
  ]);

  // 降级分组选项（编辑时）- 排除自身
  const fallbackGroupOptionsForEdit = computed(() => {
    const options: { value: number | null; label: string }[] = [
      { value: null, label: t("admin.groups.claudeCode.noFallback") },
    ];
    const currentId = editingGroup.value?.id;
    const eligibleGroups = props.groups.filter(
      (g) =>
        g.platform === "anthropic" &&
        !g.claude_code_only &&
        g.status === "active" &&
        g.id !== currentId,
    );
    eligibleGroups.forEach((g) => {
      options.push({ value: g.id, label: g.name });
    });
    return options;
  });

  // 无效请求兜底分组选项（编辑时）- 排除自身
  const invalidRequestFallbackOptionsForEdit = computed(() => {
    const options: { value: number | null; label: string }[] = [
      { value: null, label: t("admin.groups.invalidRequestFallback.noFallback") },
    ];
    const currentId = editingGroup.value?.id;
    const eligibleGroups = props.groups.filter(
      (g) =>
        g.platform === "anthropic" &&
        g.status === "active" &&
        g.subscription_type !== "subscription" &&
        g.fallback_group_id_on_invalid_request === null &&
        g.id !== currentId,
    );
    eligibleGroups.forEach((g) => {
      options.push({ value: g.id, label: g.name });
    });
    return options;
  });

  // 复制账号的源分组选项（编辑时）- 相同平台；composite 分组可汇总各平台账号，排除自身
  const copyAccountsGroupOptionsForEdit = computed(() => {
    const currentId = editingGroup.value?.id;
    const eligibleGroups = props.groups.filter(
      (g) =>
        canCopyAccountsFromGroup(editForm.platform, g.platform) &&
        (g.account_count || 0) > 0 &&
        g.id !== currentId,
    );
    return eligibleGroups.map((g) => ({
      value: g.id,
      label: copyAccountsGroupLabel(t, g),
    }));
  });

  const toggleEditScope = (scope: string) => {
    const idx = editForm.supported_model_scopes.indexOf(scope);
    if (idx === -1) {
      editForm.supported_model_scopes.push(scope);
    } else {
      editForm.supported_model_scopes.splice(idx, 1);
    }
  };

  const loadModelsListCandidates = async (
    groupID: number,
    platform: GroupPlatform,
  ) => {
    const request = { mode: "edit" as const, groupID, platform };
    const requestID = modelsListCandidatesTracker.next(request);
    editModelsListLoading.value = true;
    try {
      const models = await adminAPI.groups.getModelsListCandidates(groupID, platform);
      if (!modelsListCandidatesTracker.isCurrent(requestID, request)) {
        return;
      }
      setModelsListCandidates(editModelsListState, models);
    } catch (error) {
      if (!modelsListCandidatesTracker.isCurrent(requestID, request)) {
        return;
      }
      console.error("Error loading group models list candidates:", error);
    } finally {
      if (modelsListCandidatesTracker.isCurrent(requestID, request)) {
        editModelsListLoading.value = false;
      }
    }
  };

  const moveEditModelsListItem = (fromIndex: number, toIndex: number) => {
    moveModelsListItem(editModelsListState, fromIndex, toIndex);
  };

  // Codex 网页搜索单次默认价（与后端 defaultWebSearchPricePerCall 一致，官方 $10/1000 次）
  const DEFAULT_WEB_SEARCH_PRICE_PER_CALL = 0.01;

  const formatImagePricePreview = (value: number | string | null | undefined) => {
    if (value === null || value === undefined || value === "") {
      return t("admin.groups.imagePricing.notConfigured");
    }
    const price = Number(value);
    if (!Number.isFinite(price) || price < 0) {
      return t("admin.groups.imagePricing.notConfigured");
    }
    return `$${price.toFixed(6).replace(/0+$/, "").replace(/\.$/, "")}`;
  };

  const normalizePreviewNumber = (
    value: number | string | null | undefined,
    fallback = 0,
  ) => {
    if (value === null || value === undefined || value === "") {
      return fallback;
    }
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  };

  const parsePreviewPrice = (value: number | string | null | undefined) => {
    if (value === null || value === undefined || value === "") {
      return null;
    }
    const parsed = Number(value);
    return Number.isFinite(parsed) && parsed >= 0 ? parsed : null;
  };

  const buildWebSearchFinalPricePreview = (form: {
    web_search_price_per_call: number | string | null;
    rate_multiplier: number | string | null;
  }) => {
    const basePrice =
      parsePreviewPrice(form.web_search_price_per_call) ??
      DEFAULT_WEB_SEARCH_PRICE_PER_CALL;
    const multiplier = normalizePreviewNumber(form.rate_multiplier, 1);
    return formatImagePricePreview(basePrice * multiplier);
  };

  const editWebSearchFinalPricePreview = computed(() =>
    buildWebSearchFinalPricePreview(editForm),
  );

  // 利润控制表单辅助（换算与校验逻辑见 groupsProfitControl.ts，便于单测）。
  const percentToDecimal = profitPercentToDecimal;
  const decimalToPercent = profitDecimalToPercent;

  const validateProfitControlForm = (form: ProfitControlFormState): boolean => {
    const errorKey = validateProfitControlFormState(form);
    if (errorKey) {
      appStore.showError(t(`admin.groups.profitControl.${errorKey}`));
      return false;
    }
    return true;
  };

  const toggleLive = async () => {
    if (editForm.allow_live) {
      editForm.allow_live = false;
      return;
    }
    const capability = await loadLiveCapability();
    if (capability.supported) {
      editForm.allow_live = true;
      return;
    }
    emit("unsupportedLive");
  };

  const confirmLive = () => {
    editForm.allow_live = true;
  };

  const open = async (group: AdminGroup) => {
    editingGroup.value = group;
    editForm.name = group.name;
    editForm.description = group.description || "";
    editForm.platform = group.platform;
    editForm.rate_multiplier = group.rate_multiplier;
    editForm.is_exclusive = group.is_exclusive;
    editForm.status = group.status;
    editForm.subscription_type = group.subscription_type || "standard";
    editForm.daily_limit_usd = group.daily_limit_usd;
    editForm.weekly_limit_usd = group.weekly_limit_usd;
    editForm.monthly_limit_usd = group.monthly_limit_usd;
    editForm.long_context_pricing_enabled =
      group.long_context_pricing_enabled ?? true;
    editForm.force_openai_fast = group.force_openai_fast ?? false;
    editForm.free_openai_fast = group.free_openai_fast ?? false;
    editForm.model_pricing = groupPricingFromAPI(group.model_pricing);
    editForm.allow_image_generation = group.allow_image_generation ?? false;
    editForm.allow_batch_image_generation =
      group.allow_batch_image_generation ?? false;
    editForm.image_rate_independent = group.image_rate_independent ?? false;
    editForm.image_rate_multiplier = group.image_rate_multiplier ?? 1;
    editForm.batch_image_discount_multiplier =
      group.batch_image_discount_multiplier ?? 0.5;
    editForm.batch_image_hold_multiplier = group.batch_image_hold_multiplier ?? 0.6;
    editForm.image_price_1k = group.image_price_1k;
    editForm.image_price_2k = group.image_price_2k;
    editForm.image_price_4k = group.image_price_4k;
    editForm.video_rate_independent = group.video_rate_independent ?? false;
    editForm.video_rate_multiplier = group.video_rate_multiplier ?? 1;
    editForm.video_price_480p = group.video_price_480p;
    editForm.video_price_720p = group.video_price_720p;
    editForm.video_price_1080p = group.video_price_1080p;
    editForm.video_model_prices = createVideoModelPricesForm(
      group.video_model_prices,
    );
    editForm.web_search_price_per_call = group.web_search_price_per_call ?? null;
    editForm.search_price_per_1k = group.search_price_per_1k ?? null;
    editForm.audio_realtime_price_per_min = group.audio_realtime_price_per_min ?? null;
    editForm.audio_tts_price_per_million_chars = group.audio_tts_price_per_million_chars ?? null;
    editForm.audio_stt_price_per_hour = group.audio_stt_price_per_hour ?? null;
    editForm.peak_rate_enabled = group.peak_rate_enabled ?? false;
    editForm.peak_start = group.peak_start ?? "";
    editForm.peak_end = group.peak_end ?? "";
    editForm.peak_rate_multiplier = group.peak_rate_multiplier ?? 1.0;
    editForm.profit_control_enabled = group.profit_control_enabled ?? false;
    editForm.profit_min_margin_percent = decimalToPercent(
      group.profit_min_margin ?? 0,
    );
    editForm.profit_safety_buffer_percent = decimalToPercent(
      group.profit_safety_buffer ?? 0,
    );
    editForm.claude_code_only = group.claude_code_only || false;
    editForm.fallback_group_id = group.fallback_group_id;
    editForm.fallback_group_id_on_invalid_request =
      group.fallback_group_id_on_invalid_request;
    const messagesDispatchFormState = messagesDispatchConfigToFormState(
      group.messages_dispatch_model_config,
    );
    editForm.allow_messages_dispatch =
      group.allow_messages_dispatch ||
      messagesDispatchFormState.allow_messages_dispatch;
    editForm.allow_live = group.allow_live ?? false;
    editForm.opus_mapped_model = messagesDispatchFormState.opus_mapped_model;
    editForm.sonnet_mapped_model = messagesDispatchFormState.sonnet_mapped_model;
    editForm.haiku_mapped_model = messagesDispatchFormState.haiku_mapped_model;
    editForm.exact_model_mappings =
      messagesDispatchFormState.exact_model_mappings;
    editForm.require_oauth_only = group.require_oauth_only ?? false;
    editForm.require_privacy_set = group.require_privacy_set ?? false;
    editForm.model_routing_enabled = group.model_routing_enabled || false;
    editForm.supported_model_scopes = group.supported_model_scopes || [
      "claude",
      "gemini_text",
      "gemini_image",
    ];
    editForm.mcp_xml_inject = group.mcp_xml_inject ?? true;
    editForm.copy_accounts_from_group_ids = []; // 复制账号字段每次编辑时重置为空
    editForm.rpm_limit = group.rpm_limit ?? 0;
    editForm.max_reasoning_effort = normalizeReasoningEffortForPlatform(
      group.platform,
      group.max_reasoning_effort,
    );
    editForm.max_reasoning_effort_over_limit = normalizeReasoningEffortOverLimit(
      group.max_reasoning_effort_over_limit,
    );
    editForm.reasoning_effort_mappings = reasoningEffortMappingsToRows(
      group.reasoning_effort_mappings,
      group.platform,
    );
    resetModelsListState(editModelsListState, group.models_list_config);
    // 加载模型路由规则（异步加载账号名称）
    editModelRoutingRules.value = await convertApiFormatToRoutingRules(
      group.model_routing,
    );
    loadModelsListCandidates(group.id, group.platform);
    emit("open");
  };

  const closeEditModal = () => {
    modelRoutingRef.value?.reset();
    editingGroup.value = null;
    editForm.max_reasoning_effort = "";
    editForm.max_reasoning_effort_over_limit = reasoningEffortOverLimitDowngrade;
    editForm.reasoning_effort_mappings = [];
    editReasoningEffortPolicyRef.value?.resetValidation();
    editModelRoutingRules.value = [];
    editForm.copy_accounts_from_group_ids = [];
    editForm.peak_rate_enabled = false;
    editForm.peak_start = "";
    editForm.peak_end = "";
    editForm.peak_rate_multiplier = 1.0;
    editForm.profit_control_enabled = false;
    editForm.profit_min_margin_percent = 0;
    editForm.profit_safety_buffer_percent = 0;
    editForm.video_rate_independent = false;
    editForm.video_rate_multiplier = 1;
    editForm.video_price_480p = null;
    editForm.video_price_720p = null;
    editForm.video_price_1080p = null;
    editForm.video_model_prices = createVideoModelPricesForm();
    editForm.long_context_pricing_enabled = true;
    editForm.force_openai_fast = false;
    editForm.free_openai_fast = false;
    editForm.model_pricing = [];
    editForm.web_search_price_per_call = null;
    editForm.search_price_per_1k = null;
    editForm.audio_realtime_price_per_min = null;
    editForm.audio_tts_price_per_million_chars = null;
    editForm.audio_stt_price_per_hour = null;
    resetMessagesDispatchFormState(editForm);
    editForm.allow_live = false;
    resetModelsListState(editModelsListState);
    emit("close");
  };

  const handleUpdateGroup = async () => {
    if (!editingGroup.value) return;
    if (!editForm.name.trim()) {
      appStore.showError(t("admin.groups.nameRequired"));
      return;
    }
    if (
      supportsReasoningEffortPolicyPlatform(editForm.platform) &&
      editReasoningEffortPolicyRef.value &&
      !editReasoningEffortPolicyRef.value.validate()
    ) {
      return;
    }
    if (!validateProfitControlForm(editForm)) {
      return;
    }

    submitting.value = true;
    try {
      // 转换 fallback_group_id: null -> 0 (后端使用 0 表示清除)
      const payload: Record<string, unknown> = {
        ...editForm,
        force_openai_fast: normalizeGroupOpenAIFast(
          editForm.platform,
          editForm.force_openai_fast,
        ),
        free_openai_fast: normalizeGroupOpenAIFast(
          editForm.platform,
          editForm.free_openai_fast,
        ),
        model_pricing: groupPricingToAPI(
          editForm.model_pricing,
          editForm.platform,
        ) as ChannelModelPricing[],
        daily_limit_usd: normalizeOptionalLimit(
          editForm.daily_limit_usd as number | string | null,
        ),
        weekly_limit_usd: normalizeOptionalLimit(
          editForm.weekly_limit_usd as number | string | null,
        ),
        monthly_limit_usd: normalizeOptionalLimit(
          editForm.monthly_limit_usd as number | string | null,
        ),
        video_model_prices: serializeVideoModelPrices(
          editForm.video_model_prices,
        ),
        fallback_group_id:
          editForm.fallback_group_id === null ? 0 : editForm.fallback_group_id,
        fallback_group_id_on_invalid_request:
          editForm.fallback_group_id_on_invalid_request === null
            ? 0
            : editForm.fallback_group_id_on_invalid_request,
        model_routing: convertRoutingRulesToApiFormat(
          editModelRoutingRules.value,
        ),
        models_list_config: buildModelsListConfig(editModelsListState),
        supported_model_scopes: normalizeSupportedModelScopesForPlatform(
          editForm.platform,
          editForm.supported_model_scopes,
        ),
        messages_dispatch_model_config:
          editForm.platform === "openai"
            ? messagesDispatchFormStateToConfig({
                allow_messages_dispatch: editForm.allow_messages_dispatch,
                opus_mapped_model: editForm.opus_mapped_model,
                sonnet_mapped_model: editForm.sonnet_mapped_model,
                haiku_mapped_model: editForm.haiku_mapped_model,
                exact_model_mappings: editForm.exact_model_mappings,
              })
            : undefined,
        reasoning_effort_mappings: reasoningEffortMappingsToAPI(
          editForm.reasoning_effort_mappings,
        ),
        // 利润控制：界面百分比转小数提交；仅五个 token 平台可启用
        profit_control_enabled:
          isProfitControlPlatform(editForm.platform) &&
          editForm.profit_control_enabled,
        profit_min_margin: percentToDecimal(editForm.profit_min_margin_percent),
        profit_safety_buffer: percentToDecimal(
          editForm.profit_safety_buffer_percent,
        ),
      };
      delete payload.profit_min_margin_percent;
      delete payload.profit_safety_buffer_percent;
      // v-model.number 清空输入框时产生 ""，转为 null 让后端设为无限制
      const emptyToNull = (v: any) => (v === "" ? null : v);
      payload.daily_limit_usd = emptyToNull(payload.daily_limit_usd);
      payload.weekly_limit_usd = emptyToNull(payload.weekly_limit_usd);
      payload.monthly_limit_usd = emptyToNull(payload.monthly_limit_usd);
      payload.image_rate_multiplier = normalizeRateMultiplier(
        payload.image_rate_multiplier as number | string | null,
      );
      resetDisabledBatchImagePricing(payload as any);
      payload.batch_image_discount_multiplier = normalizeRateMultiplier(
        payload.batch_image_discount_multiplier as number | string | null,
      );
      payload.batch_image_hold_multiplier = normalizeRateMultiplier(
        payload.batch_image_hold_multiplier as number | string | null,
      );
      payload.video_rate_multiplier = normalizeRateMultiplier(
        payload.video_rate_multiplier as number | string | null,
      );
      // 媒体价格输入清空时 v-model.number 产生 ""，直接提交会被后端 *float64 反序列化拒绝（400）。
      // 更新语义中 null 表示"不修改"，因此清空后的字段发送 -1：后端 normalizePrice 将负价归一为
      // NULL，从而真正清除已配置的价格。
      const emptyPriceToClear = (v: any) => (v === "" || v === null ? -1 : v);
      payload.image_price_1k = emptyPriceToClear(payload.image_price_1k);
      payload.image_price_2k = emptyPriceToClear(payload.image_price_2k);
      payload.image_price_4k = emptyPriceToClear(payload.image_price_4k);
      payload.video_price_480p = emptyPriceToClear(payload.video_price_480p);
      payload.video_price_720p = emptyPriceToClear(payload.video_price_720p);
      payload.video_price_1080p = emptyPriceToClear(payload.video_price_1080p);
      payload.search_price_per_1k = emptyPriceToClear(
        payload.search_price_per_1k,
      );
      payload.audio_realtime_price_per_min = emptyPriceToClear(
        payload.audio_realtime_price_per_min,
      );
      payload.audio_tts_price_per_million_chars = emptyPriceToClear(
        payload.audio_tts_price_per_million_chars,
      );
      payload.audio_stt_price_per_hour = emptyPriceToClear(
        payload.audio_stt_price_per_hour,
      );
      payload.web_search_price_per_call = emptyPriceToClear(
        payload.web_search_price_per_call,
      );
      payload.peak_rate_enabled = editForm.peak_rate_enabled;
      payload.peak_start = editForm.peak_start;
      payload.peak_end = editForm.peak_end;
      payload.peak_rate_multiplier = normalizeRateMultiplier(
        editForm.peak_rate_multiplier,
      );
      await adminAPI.groups.update(editingGroup.value.id, payload as any);
      appStore.showSuccess(t("admin.groups.groupUpdated"));
      closeEditModal();
      emit("updated");
    } catch (error: any) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.groups.failedToUpdate")),
      );
      console.error("Error updating group:", error);
    } finally {
      submitting.value = false;
    }
  };

  // 编辑表单：切回标准模式时清空高峰配置，避免残留随更新请求提交被后端拒绝
  watch(
    () => editForm.subscription_type,
    (newVal) => {
      if (newVal !== "subscription") {
        editForm.peak_rate_enabled = false;
        editForm.peak_start = "";
        editForm.peak_end = "";
        editForm.peak_rate_multiplier = 1.0;
      }
    },
  );

  watch(
    () => editForm.platform,
    (newVal) => {
      if (!["anthropic", "antigravity"].includes(newVal)) {
        editForm.fallback_group_id_on_invalid_request = null;
      }
      if (!supportsMessagesDispatchPlatform(newVal)) {
        resetMessagesDispatchFormState(editForm);
      }
      if (!supportsLivePlatform(newVal)) {
        editForm.allow_live = false;
      }
      if (!isProfitControlPlatform(newVal)) {
        editForm.profit_control_enabled = false;
        editForm.profit_min_margin_percent = 0;
        editForm.profit_safety_buffer_percent = 0;
      }
      editForm.max_reasoning_effort = normalizeReasoningEffortForPlatform(
        newVal,
        editForm.max_reasoning_effort,
      );
      editForm.max_reasoning_effort_over_limit = supportsReasoningEffortPolicyPlatform(
        newVal,
      )
        ? normalizeReasoningEffortOverLimit(
            editForm.max_reasoning_effort_over_limit,
          )
        : reasoningEffortOverLimitDowngrade;
      editForm.reasoning_effort_mappings = reasoningEffortMappingsToRows(
        reasoningEffortMappingsToAPI(editForm.reasoning_effort_mappings),
        newVal,
      );
      editReasoningEffortPolicyRef.value?.resetValidation();
      if (!["openai", "antigravity", "anthropic", "gemini"].includes(newVal)) {
        editForm.require_oauth_only = false;
        editForm.require_privacy_set = false;
      }
      resetDisabledBatchImagePricing(editForm);
      if (editingGroup.value) {
        resetModelsListState(
          editModelsListState,
          editForm.platform === editingGroup.value.platform
            ? editingGroup.value.models_list_config
            : undefined,
        );
        loadModelsListCandidates(editingGroup.value.id, newVal);
      }
    },
  );

  watch(
    () => editForm.allow_image_generation,
    () => {
      resetDisabledBatchImagePricing(editForm);
    },
  );

  watch(
    () => editForm.allow_batch_image_generation,
    () => {
      resetDisabledBatchImagePricing(editForm);
    },
  );

  // 注：以下为原 GroupsView.vue 中重复定义的第二个 editForm.platform 侦听器（历史遗留、
  // 与上一个侦听器存在功能重叠），按“零功能变更”要求原样保留，不做合并或删除。
  watch(
    () => editForm.platform,
    (newVal) => {
      if (!["anthropic", "antigravity"].includes(newVal)) {
        editForm.fallback_group_id_on_invalid_request = null;
      }
      if (!supportsMessagesDispatchPlatform(newVal)) {
        editForm.allow_messages_dispatch = false;
        editForm.default_mapped_model = "";
      }
      if (!supportsLivePlatform(newVal)) {
        editForm.allow_live = false;
      }
    },
  );

  return {
    submitting,
    editingGroup,
    editForm,
    editModelRoutingRules,
    editModelsListState,
    editModelsListLoading,
    editModelsListSelectedCount,
    editReasoningEffortPolicyRef,
    modelRoutingRef,
    platformOptions,
    editStatusOptions,
    subscriptionTypeOptions,
    fallbackGroupOptionsForEdit,
    invalidRequestFallbackOptionsForEdit,
    copyAccountsGroupOptionsForEdit,
    toggleEditScope,
    moveEditModelsListItem,
    editWebSearchFinalPricePreview,
    toggleLive,
    confirmLive,
    open,
    closeEditModal,
    handleUpdateGroup,
  };
};
