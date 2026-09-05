// GroupsView 拆分：创建分组弹窗的表单状态与业务逻辑（零功能变更，从
// GroupsView.vue 原样搬迁）。GroupCreateModal.vue 仅负责模板装配与 UI 事件转发。
import { computed, onMounted, reactive, ref, watch } from "vue";
import { useI18n } from "vue-i18n";
import { useAppStore } from "@/stores/app";
import { useOnboardingStore } from "@/stores/onboarding";
import { adminAPI } from "@/api/admin";
import type { AdminGroup, GroupPlatform, SubscriptionType } from "@/types";
import { GROUP_PLATFORM_OPTIONS } from "@/constants/platforms";
import type { PricingFormEntry } from "@/components/admin/channel/types";
import type { ChannelModelPricing } from "@/api/admin/channels";
import { extractApiErrorMessage } from "@/utils/apiError";
import {
  supportsLivePlatform,
  groupPricingToAPI,
  canCopyAccountsFromGroup,
  copyAccountsGroupLabel,
  normalizeOptionalLimit,
  normalizeRateMultiplier,
  resetDisabledBatchImagePricing,
  convertRoutingRulesToApiFormat,
  resetModelsListState,
  loadLiveCapability,
  type ModelRoutingRule,
  type ReasoningEffortPolicyFieldsExpose,
} from "./groupFormShared";
import {
  createDefaultMessagesDispatchFormState,
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

export interface UseCreateGroupFormProps {
  groups: AdminGroup[];
}

export interface UseCreateGroupFormEmits {
  (e: "close"): void;
  (e: "created"): void;
  (e: "unsupportedLive"): void;
}

export const useCreateGroupForm = (
  props: UseCreateGroupFormProps,
  emit: UseCreateGroupFormEmits,
) => {
  const { t } = useI18n();
  const appStore = useAppStore();
  const onboardingStore = useOnboardingStore();

  const submitting = ref(false);

  const createMessagesDispatchDefaults = createDefaultMessagesDispatchFormState();
  const createModelsListState = reactive(createInitialModelsListState());
  const createModelsListLoading = ref(false);
  const modelsListCandidatesTracker = createModelsListCandidatesTracker();
  const createModelsListSelectedCount = computed(
    () => createModelsListState.items.filter((item) => item.selected).length,
  );

  const createReasoningEffortPolicyRef =
    ref<ReasoningEffortPolicyFieldsExpose | null>(null);
  const modelRoutingRef = ref<{ reset: () => void } | null>(null);

  const createForm = reactive({
    name: "",
    description: "",
    platform: "anthropic" as GroupPlatform,
    rate_multiplier: 1.0,
    is_exclusive: false,
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
    opus_mapped_model: createMessagesDispatchDefaults.opus_mapped_model,
    sonnet_mapped_model: createMessagesDispatchDefaults.sonnet_mapped_model,
    haiku_mapped_model: createMessagesDispatchDefaults.haiku_mapped_model,
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

  const createModelRoutingRules = ref<ModelRoutingRule[]>([]);

  const platformOptions = computed(() => [...GROUP_PLATFORM_OPTIONS]);

  const subscriptionTypeOptions = computed(() => [
    { value: "standard", label: t("admin.groups.subscription.standard") },
    { value: "subscription", label: t("admin.groups.subscription.subscription") },
  ]);

  // 降级分组选项（创建时）- 仅包含 anthropic 平台且未启用 claude_code_only 的分组
  const fallbackGroupOptions = computed(() => {
    const options: { value: number | null; label: string }[] = [
      { value: null, label: t("admin.groups.claudeCode.noFallback") },
    ];
    const eligibleGroups = props.groups.filter(
      (g) =>
        g.platform === "anthropic" &&
        !g.claude_code_only &&
        g.status === "active",
    );
    eligibleGroups.forEach((g) => {
      options.push({ value: g.id, label: g.name });
    });
    return options;
  });

  // 无效请求兜底分组选项（创建时）- 仅包含 anthropic 平台、非订阅且未配置兜底的分组
  const invalidRequestFallbackOptions = computed(() => {
    const options: { value: number | null; label: string }[] = [
      { value: null, label: t("admin.groups.invalidRequestFallback.noFallback") },
    ];
    const eligibleGroups = props.groups.filter(
      (g) =>
        g.platform === "anthropic" &&
        g.status === "active" &&
        g.subscription_type !== "subscription" &&
        g.fallback_group_id_on_invalid_request === null,
    );
    eligibleGroups.forEach((g) => {
      options.push({ value: g.id, label: g.name });
    });
    return options;
  });

  // 复制账号的源分组选项（创建时）- 相同平台；composite 分组可汇总各平台账号
  const copyAccountsGroupOptions = computed(() => {
    const eligibleGroups = props.groups.filter(
      (g) =>
        canCopyAccountsFromGroup(createForm.platform, g.platform) &&
        (g.account_count || 0) > 0,
    );
    return eligibleGroups.map((g) => ({
      value: g.id,
      label: copyAccountsGroupLabel(t, g),
    }));
  });

  const toggleCreateScope = (scope: string) => {
    const idx = createForm.supported_model_scopes.indexOf(scope);
    if (idx === -1) {
      createForm.supported_model_scopes.push(scope);
    } else {
      createForm.supported_model_scopes.splice(idx, 1);
    }
  };

  const loadModelsListCandidates = async (
    groupID: number,
    platform: GroupPlatform,
  ) => {
    const request = { mode: "create" as const, groupID, platform };
    const requestID = modelsListCandidatesTracker.next(request);
    createModelsListLoading.value = true;
    try {
      const models = await adminAPI.groups.getModelsListCandidates(groupID, platform);
      if (!modelsListCandidatesTracker.isCurrent(requestID, request)) {
        return;
      }
      setModelsListCandidates(createModelsListState, models);
    } catch (error) {
      if (!modelsListCandidatesTracker.isCurrent(requestID, request)) {
        return;
      }
      console.error("Error loading group models list candidates:", error);
    } finally {
      if (modelsListCandidatesTracker.isCurrent(requestID, request)) {
        createModelsListLoading.value = false;
      }
    }
  };

  const moveCreateModelsListItem = (fromIndex: number, toIndex: number) => {
    moveModelsListItem(createModelsListState, fromIndex, toIndex);
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
    return `${price.toFixed(6).replace(/0+$/, "").replace(/\.$/, "")}`;
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

  const createWebSearchFinalPricePreview = computed(() =>
    buildWebSearchFinalPricePreview(createForm),
  );

  // 利润控制表单辅助（换算与校验逻辑见 groupsProfitControl.ts，便于单测）。
  const percentToDecimal = profitPercentToDecimal;

  const validateProfitControlForm = (form: ProfitControlFormState): boolean => {
    const errorKey = validateProfitControlFormState(form);
    if (errorKey) {
      appStore.showError(t(`admin.groups.profitControl.${errorKey}`));
      return false;
    }
    return true;
  };

  const toggleLive = async () => {
    if (createForm.allow_live) {
      createForm.allow_live = false;
      return;
    }
    const capability = await loadLiveCapability();
    if (capability.supported) {
      createForm.allow_live = true;
      return;
    }
    emit("unsupportedLive");
  };

  const confirmLive = () => {
    createForm.allow_live = true;
  };

  const refreshModelsList = () => {
    loadModelsListCandidates(0, createForm.platform);
  };

  const closeCreateModal = () => {
    modelRoutingRef.value?.reset();
    createForm.name = "";
    createForm.description = "";
    createForm.platform = "anthropic";
    createForm.rate_multiplier = 1.0;
    createForm.is_exclusive = false;
    createForm.subscription_type = "standard";
    createForm.daily_limit_usd = null;
    createForm.weekly_limit_usd = null;
    createForm.monthly_limit_usd = null;
    createForm.allow_image_generation = false;
    createForm.allow_batch_image_generation = false;
    createForm.image_rate_independent = false;
    createForm.image_rate_multiplier = 1;
    createForm.batch_image_discount_multiplier = 0.5;
    createForm.batch_image_hold_multiplier = 0.6;
    createForm.image_price_1k = null;
    createForm.image_price_2k = null;
    createForm.image_price_4k = null;
    createForm.video_rate_independent = false;
    createForm.video_rate_multiplier = 1;
    createForm.video_price_480p = null;
    createForm.video_price_720p = null;
    createForm.video_price_1080p = null;
    createForm.video_model_prices = createVideoModelPricesForm();
    createForm.long_context_pricing_enabled = true;
    createForm.force_openai_fast = false;
    createForm.free_openai_fast = false;
    createForm.model_pricing = [];
    createForm.web_search_price_per_call = null;
    createForm.search_price_per_1k = null;
    createForm.audio_realtime_price_per_min = null;
    createForm.audio_tts_price_per_million_chars = null;
    createForm.audio_stt_price_per_hour = null;
    createForm.peak_rate_enabled = false;
    createForm.peak_start = "";
    createForm.peak_end = "";
    createForm.peak_rate_multiplier = 1.0;
    createForm.profit_control_enabled = false;
    createForm.profit_min_margin_percent = 0;
    createForm.profit_safety_buffer_percent = 0;
    createForm.claude_code_only = false;
    createForm.fallback_group_id = null;
    createForm.fallback_group_id_on_invalid_request = null;
    resetMessagesDispatchFormState(createForm);
    createForm.allow_live = false;
    createForm.require_oauth_only = false;
    createForm.require_privacy_set = false;
    createForm.supported_model_scopes = ["claude", "gemini_text", "gemini_image"];
    createForm.mcp_xml_inject = true;
    createForm.copy_accounts_from_group_ids = [];
    createForm.rpm_limit = 0;
    createForm.max_reasoning_effort = "";
    createForm.max_reasoning_effort_over_limit = reasoningEffortOverLimitDowngrade;
    createForm.reasoning_effort_mappings = [];
    createReasoningEffortPolicyRef.value?.resetValidation();
    resetModelsListState(createModelsListState);
    createModelRoutingRules.value = [];
    emit("close");
  };

  const handleCreateGroup = async () => {
    if (!createForm.name.trim()) {
      appStore.showError(t("admin.groups.nameRequired"));
      return;
    }
    if (
      supportsReasoningEffortPolicyPlatform(createForm.platform) &&
      createReasoningEffortPolicyRef.value &&
      !createReasoningEffortPolicyRef.value.validate()
    ) {
      return;
    }
    if (!validateProfitControlForm(createForm)) {
      return;
    }
    submitting.value = true;
    try {
      const {
        video_model_prices: _createFormVideoModelPrices,
        ...createGroupForm
      } = createForm;
      const videoModelPrices = serializeVideoModelPrices(
        createForm.video_model_prices,
      );
      // 构建请求数据，包含模型路由配置
      const requestData: Record<string, unknown> = {
        ...createGroupForm,
        force_openai_fast: normalizeGroupOpenAIFast(
          createForm.platform,
          createForm.force_openai_fast,
        ),
        free_openai_fast: normalizeGroupOpenAIFast(
          createForm.platform,
          createForm.free_openai_fast,
        ),
        model_pricing: groupPricingToAPI(
          createForm.model_pricing,
          createForm.platform,
        ) as ChannelModelPricing[],
        daily_limit_usd: normalizeOptionalLimit(
          createForm.daily_limit_usd as number | string | null,
        ),
        weekly_limit_usd: normalizeOptionalLimit(
          createForm.weekly_limit_usd as number | string | null,
        ),
        monthly_limit_usd: normalizeOptionalLimit(
          createForm.monthly_limit_usd as number | string | null,
        ),
        ...(Object.keys(videoModelPrices).length > 0
          ? { video_model_prices: videoModelPrices }
          : {}),
        model_routing: convertRoutingRulesToApiFormat(
          createModelRoutingRules.value,
        ),
        models_list_config: buildModelsListConfig(createModelsListState),
        supported_model_scopes: normalizeSupportedModelScopesForPlatform(
          createForm.platform,
          createForm.supported_model_scopes,
        ),
        messages_dispatch_model_config:
          createForm.platform === "openai"
            ? messagesDispatchFormStateToConfig({
                allow_messages_dispatch: createForm.allow_messages_dispatch,
                opus_mapped_model: createForm.opus_mapped_model,
                sonnet_mapped_model: createForm.sonnet_mapped_model,
                haiku_mapped_model: createForm.haiku_mapped_model,
                exact_model_mappings: createForm.exact_model_mappings,
              })
            : undefined,
        reasoning_effort_mappings: reasoningEffortMappingsToAPI(
          createForm.reasoning_effort_mappings,
        ),
        // 利润控制：界面百分比转小数提交；仅五个 token 平台可启用
        profit_control_enabled:
          isProfitControlPlatform(createForm.platform) &&
          createForm.profit_control_enabled,
        profit_min_margin: percentToDecimal(createForm.profit_min_margin_percent),
        profit_safety_buffer: percentToDecimal(
          createForm.profit_safety_buffer_percent,
        ),
      };
      delete requestData.profit_min_margin_percent;
      delete requestData.profit_safety_buffer_percent;
      // v-model.number 清空输入框时产生 ""，转为 null 让后端设为无限制
      const emptyToNull = (v: any) => (v === "" ? null : v);
      requestData.daily_limit_usd = emptyToNull(requestData.daily_limit_usd);
      requestData.weekly_limit_usd = emptyToNull(requestData.weekly_limit_usd);
      requestData.monthly_limit_usd = emptyToNull(requestData.monthly_limit_usd);
      requestData.image_rate_multiplier = normalizeRateMultiplier(
        requestData.image_rate_multiplier as number | string | null,
      );
      resetDisabledBatchImagePricing(requestData as any);
      requestData.batch_image_discount_multiplier = normalizeRateMultiplier(
        requestData.batch_image_discount_multiplier as number | string | null,
      );
      requestData.batch_image_hold_multiplier = normalizeRateMultiplier(
        requestData.batch_image_hold_multiplier as number | string | null,
      );
      requestData.video_rate_multiplier = normalizeRateMultiplier(
        requestData.video_rate_multiplier as number | string | null,
      );
      // 媒体价格输入清空时 v-model.number 产生 ""，直接提交会被后端 *float64 反序列化拒绝（400），
      // 创建时按"未配置"（null）处理。
      requestData.image_price_1k = emptyToNull(requestData.image_price_1k);
      requestData.image_price_2k = emptyToNull(requestData.image_price_2k);
      requestData.image_price_4k = emptyToNull(requestData.image_price_4k);
      requestData.video_price_480p = emptyToNull(requestData.video_price_480p);
      requestData.video_price_720p = emptyToNull(requestData.video_price_720p);
      requestData.video_price_1080p = emptyToNull(requestData.video_price_1080p);
      requestData.search_price_per_1k = emptyToNull(
        requestData.search_price_per_1k,
      );
      requestData.audio_realtime_price_per_min = emptyToNull(
        requestData.audio_realtime_price_per_min,
      );
      requestData.audio_tts_price_per_million_chars = emptyToNull(
        requestData.audio_tts_price_per_million_chars,
      );
      requestData.audio_stt_price_per_hour = emptyToNull(
        requestData.audio_stt_price_per_hour,
      );
      requestData.web_search_price_per_call = emptyToNull(
        requestData.web_search_price_per_call,
      );
      requestData.peak_rate_enabled = createForm.peak_rate_enabled;
      requestData.peak_start = createForm.peak_start;
      requestData.peak_end = createForm.peak_end;
      requestData.peak_rate_multiplier = normalizeRateMultiplier(
        createForm.peak_rate_multiplier,
      );
      await adminAPI.groups.create(requestData as any);
      appStore.showSuccess(t("admin.groups.groupCreated"));
      closeCreateModal();
      emit("created");
      // Only advance tour if active, on submit step, and creation succeeded
      if (onboardingStore.isCurrentStep('[data-tour="group-form-submit"]')) {
        onboardingStore.nextStep(500);
      }
    } catch (error: any) {
      appStore.showError(
        extractApiErrorMessage(error, t("admin.groups.failedToCreate")),
      );
      console.error("Error creating group:", error);
      // Don't advance tour on error
    } finally {
      submitting.value = false;
    }
  };

  watch(
    () => createForm.subscription_type,
    (newVal) => {
      if (newVal === "subscription") {
        createForm.is_exclusive = true;
        createForm.fallback_group_id_on_invalid_request = null;
      } else {
        createForm.peak_rate_enabled = false;
        createForm.peak_start = "";
        createForm.peak_end = "";
        createForm.peak_rate_multiplier = 1.0;
      }
    },
  );

  watch(
    () => createForm.platform,
    (newVal) => {
      if (!["anthropic", "antigravity"].includes(newVal)) {
        createForm.fallback_group_id_on_invalid_request = null;
      }
      if (!supportsMessagesDispatchPlatform(newVal)) {
        resetMessagesDispatchFormState(createForm);
      }
      if (!supportsLivePlatform(newVal)) {
        createForm.allow_live = false;
      }
      if (!isProfitControlPlatform(newVal)) {
        createForm.profit_control_enabled = false;
        createForm.profit_min_margin_percent = 0;
        createForm.profit_safety_buffer_percent = 0;
      }
      createForm.max_reasoning_effort = normalizeReasoningEffortForPlatform(
        newVal,
        createForm.max_reasoning_effort,
      );
      createForm.max_reasoning_effort_over_limit = supportsReasoningEffortPolicyPlatform(
        newVal,
      )
        ? normalizeReasoningEffortOverLimit(
            createForm.max_reasoning_effort_over_limit,
          )
        : reasoningEffortOverLimitDowngrade;
      createForm.reasoning_effort_mappings = reasoningEffortMappingsToRows(
        reasoningEffortMappingsToAPI(createForm.reasoning_effort_mappings),
        newVal,
      );
      createReasoningEffortPolicyRef.value?.resetValidation();
      if (!["openai", "antigravity", "anthropic", "gemini"].includes(newVal)) {
        createForm.require_oauth_only = false;
        createForm.require_privacy_set = false;
      }
      resetDisabledBatchImagePricing(createForm);
      resetModelsListState(createModelsListState);
      loadModelsListCandidates(0, newVal);
    },
  );

  watch(
    () => createForm.allow_image_generation,
    () => {
      resetDisabledBatchImagePricing(createForm);
    },
  );

  watch(
    () => createForm.allow_batch_image_generation,
    () => {
      resetDisabledBatchImagePricing(createForm);
    },
  );

  onMounted(() => {
    loadModelsListCandidates(0, createForm.platform);
  });

  return {
    submitting,
    createForm,
    createModelRoutingRules,
    createModelsListState,
    createModelsListLoading,
    createModelsListSelectedCount,
    createReasoningEffortPolicyRef,
    modelRoutingRef,
    platformOptions,
    subscriptionTypeOptions,
    fallbackGroupOptions,
    invalidRequestFallbackOptions,
    copyAccountsGroupOptions,
    toggleCreateScope,
    moveCreateModelsListItem,
    createWebSearchFinalPricePreview,
    toggleLive,
    confirmLive,
    refreshModelsList,
    closeCreateModal,
    handleCreateGroup,
  };
};
