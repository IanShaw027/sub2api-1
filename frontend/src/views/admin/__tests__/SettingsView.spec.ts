import { beforeEach, describe, expect, it, vi } from "vitest";
import { defineComponent, h } from "vue";
import { flushPromises, mount } from "@vue/test-utils";

import { KIRO_CACHE_MIN_BLOCK_TOKENS_MAX } from "@/api/admin/settings";
import SettingsView from "../SettingsView.vue";

const {
  getSettings,
  updateSettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  getAdminApiKey,
  getOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getStreamTimeoutSettings,
  getTempUnschedThresholdSettings,
  updateTempUnschedThresholdSettings,
  getRectifierSettings,
  getBetaPolicySettings,
  getGroups,
  listProxies,
  getProviders,
  updateProvider,
  createProvider,
  deleteProvider,
  fetchPublicSettings,
  adminSettingsFetch,
  showError,
  showSuccess,
} = vi.hoisted(() => ({
  getSettings: vi.fn(),
  updateSettings: vi.fn(),
  getWebSearchEmulationConfig: vi.fn(),
  updateWebSearchEmulationConfig: vi.fn(),
  getAdminApiKey: vi.fn(),
  getOverloadCooldownSettings: vi.fn(),
  getRateLimit429CooldownSettings: vi.fn(),
  updateRateLimit429CooldownSettings: vi.fn(),
  getStreamTimeoutSettings: vi.fn(),
  getTempUnschedThresholdSettings: vi.fn(),
  updateTempUnschedThresholdSettings: vi.fn(),
  getRectifierSettings: vi.fn(),
  getBetaPolicySettings: vi.fn(),
  getGroups: vi.fn(),
  listProxies: vi.fn(),
  getProviders: vi.fn(),
  updateProvider: vi.fn(),
  createProvider: vi.fn(),
  deleteProvider: vi.fn(),
  fetchPublicSettings: vi.fn(),
  adminSettingsFetch: vi.fn(),
  showError: vi.fn(),
  showSuccess: vi.fn(),
}));

const localeRef = vi.hoisted(() => ({ value: "zh-CN" }));

vi.mock("@/api", () => ({
  adminAPI: {
    settings: {
      getSettings,
      updateSettings,
      getWebSearchEmulationConfig,
      updateWebSearchEmulationConfig,
      getAdminApiKey,
      getOverloadCooldownSettings,
      getRateLimit429CooldownSettings,
      updateRateLimit429CooldownSettings,
      getStreamTimeoutSettings,
      getTempUnschedThresholdSettings,
      updateTempUnschedThresholdSettings,
      getRectifierSettings,
      getBetaPolicySettings,
    },
    groups: {
      getAll: getGroups,
    },
    proxies: {
      list: listProxies,
    },
    payment: {
      getProviders,
      updateProvider,
      createProvider,
      deleteProvider,
    },
  },
}));

vi.mock("@/stores", () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning: vi.fn(),
    showInfo: vi.fn(),
    fetchPublicSettings,
  }),
}));

vi.mock("@/stores/app", () => ({
  useAppStore: () => ({
    showError,
    showSuccess,
    showWarning: vi.fn(),
    showInfo: vi.fn(),
    fetchPublicSettings,
  }),
}));

vi.mock("@/stores/adminSettings", () => ({
  useAdminSettingsStore: () => ({
    fetch: adminSettingsFetch,
  }),
}));

vi.mock("@/composables/useClipboard", () => ({
  useClipboard: () => ({
    copyToClipboard: vi.fn(),
  }),
}));

vi.mock("@/utils/apiError", () => ({
  extractApiErrorMessage: () => "error",
}));

vi.mock("vue-i18n", async () => {
  const actual = await vi.importActual<typeof import("vue-i18n")>("vue-i18n");
  const translations: Record<string, string> = {
    "admin.settings.wechatConnect.title": "微信登录",
    "admin.settings.wechatConnect.description": "用于微信开放平台或公众号/小程序的第三方登录配置。",
    "admin.settings.wechatConnect.enabledLabel": "启用微信登录",
    "admin.settings.wechatConnect.enabledHint": "开启后可使用微信第三方登录回调与授权配置。",
    "admin.settings.wechatConnect.appIdLabel": "AppID",
    "admin.settings.wechatConnect.appIdPlaceholder": "微信开放平台 AppID",
    "admin.settings.wechatConnect.appSecretLabel": "AppSecret",
    "admin.settings.wechatConnect.appSecretConfiguredPlaceholder": "密钥已配置，留空以保留当前值。",
    "admin.settings.wechatConnect.appSecretPlaceholder": "微信开放平台 AppSecret",
    "admin.settings.wechatConnect.appSecretConfiguredHint": "密钥已配置，留空以保留当前值。",
    "admin.settings.wechatConnect.appSecretHint": "填写后会覆盖当前微信密钥。",
    "admin.settings.wechatConnect.modeLabel": "模式",
    "admin.settings.wechatConnect.openModeLabel": "非微信环境使用开放平台",
    "admin.settings.wechatConnect.openModeHint": "浏览器不在微信内时，自动走开放平台扫码授权。",
    "admin.settings.wechatConnect.mpModeLabel": "微信环境使用公众号",
    "admin.settings.wechatConnect.mpModeHint": "浏览器在微信内时，自动走公众号授权。",
    "admin.settings.wechatConnect.redirectUrlLabel": "回调地址",
    "admin.settings.wechatConnect.redirectUrlPlaceholder": "https://your-site.com/api/v1/auth/oauth/wechat/callback",
    "admin.settings.wechatConnect.generateAndCopy": "使用当前站点生成并复制",
    "admin.settings.wechatConnect.redirectUrlSetAndCopied": "已使用当前站点生成回调地址并复制到剪贴板",
    "admin.settings.wechatConnect.frontendRedirectUrlLabel": "前端回调地址",
    "admin.settings.wechatConnect.frontendRedirectUrlPlaceholder": "/auth/wechat/callback",
    "admin.settings.wechatConnect.frontendRedirectUrlHint": "通常用于前端路由回调地址，需与后端配置保持一致。",
    "admin.settings.authSourceDefaults.title": "来源附加授权",
    "admin.settings.authSourceDefaults.description": "按第三方认证来源配置附加余额、并发和订阅。注册时会在用户默认值基础上叠加，首次绑定开启后也会叠加；不同来源可累计，同一来源只发一次。",
    "admin.settings.authSourceDefaults.requireEmailLabel": "第三方注册强制补充邮箱",
    "admin.settings.authSourceDefaults.requireEmailHint": "启用后，Linux DO、OIDC、微信注册缺少邮箱时必须先补充邮箱地址。",
    "admin.settings.authSourceDefaults.sources.email.title": "邮箱注册",
    "admin.settings.authSourceDefaults.sources.email.description": "邮箱注册或首次绑定邮箱时可追加的附加权益。",
    "admin.settings.authSourceDefaults.sources.linuxdo.title": "Linux DO 登录",
    "admin.settings.authSourceDefaults.sources.linuxdo.description": "Linux DO 注册或首次绑定时可追加的附加权益。",
    "admin.settings.authSourceDefaults.sources.oidc.title": "OIDC 登录",
    "admin.settings.authSourceDefaults.sources.oidc.description": "OIDC 注册或首次绑定时可追加的附加权益。",
    "admin.settings.authSourceDefaults.sources.wechat.title": "微信登录",
    "admin.settings.authSourceDefaults.sources.wechat.description": "微信注册或首次绑定时可追加的附加权益。",
    "admin.settings.authSourceDefaults.grantOnSignupLabel": "注册时叠加授权",
    "admin.settings.authSourceDefaults.grantOnSignupHint": "新用户通过该来源注册时，在用户默认值基础上追加发放。",
    "admin.settings.authSourceDefaults.grantOnFirstBindLabel": "首次绑定时叠加授权",
    "admin.settings.authSourceDefaults.grantOnFirstBindHint": "已有账号首次绑定该来源时追加发放；以后重复绑定同一来源不会再获得。",
    "admin.settings.authSourceDefaults.bonusBalanceLabel": "附加余额",
    "admin.settings.authSourceDefaults.bonusConcurrencyLabel": "附加并发数",
    "admin.settings.authSourceDefaults.defaultSubscriptionsLabel": "附加订阅",
    "admin.settings.authSourceDefaults.defaultSubscriptionsHint": "仅对当前认证来源生效，会和用户默认订阅一起叠加发放。",
    "admin.settings.authSourceDefaults.addBonusSubscription": "添加附加订阅",
    "admin.settings.authSourceDefaults.subscriptionGroupLabel": "订阅分组",
    "admin.settings.authSourceDefaults.noSourceSubscriptions": "当前来源未配置附加订阅。",
    "admin.settings.kiroRuntime.title": "Kiro 运行默认值",
    "admin.settings.kiroRuntime.description": "配置 Kiro 全局运行默认值与缓存参数，供新请求复用。",
    "admin.settings.kiroRuntime.kiroVersion": "Kiro 版本",
    "admin.settings.kiroRuntime.kiroVersionPlaceholder": "例如 0.10.0",
    "admin.settings.kiroRuntime.kiroCommit": "Kiro Commit",
    "admin.settings.kiroRuntime.kiroCommitPlaceholder": "例如 a1b2c3d4",
    "admin.settings.kiroRuntime.systemVersion": "系统版本",
    "admin.settings.kiroRuntime.systemVersionPlaceholder": "例如 darwin#24.6.0",
    "admin.settings.kiroRuntime.nodeVersion": "Node.js 版本",
    "admin.settings.kiroRuntime.nodeVersionPlaceholder": "例如 22.21.1",
    "admin.settings.kiroRuntime.codeExecutionSandboxCommand": "Code execution sandbox command",
    "admin.settings.kiroRuntime.codeExecutionSandboxCommandPlaceholder": "sandbox runner command",
    "admin.settings.kiroRuntime.codeExecutionSandboxCommandHint": "Empty disables code_execution; command receives code on stdin.",
    "admin.settings.kiroRuntime.cacheHitRateScale": "缓存命中率缩放",
    "admin.settings.kiroRuntime.cacheHitRateScalePlaceholder": "0 - 100",
    "admin.settings.kiroRuntime.cacheHitRateScaleHint": "范围 0-100，按百分比填写。",
    "admin.settings.kiroRuntime.cacheMinBlockTokens": "缓存最小块 Token 数",
    "admin.settings.kiroRuntime.cacheMinBlockTokensPlaceholder": ">= 0",
    "admin.settings.kiroRuntime.cacheMinBlockTokensHint": "大于等于 0。",
    "admin.settings.kiroRuntime.cacheIndependentTtlSeconds": "独立缓存 TTL（秒）",
    "admin.settings.kiroRuntime.cacheIndependentTtlSecondsPlaceholder": "60 - 86400",
    "admin.settings.kiroRuntime.cacheIndependentTtlSecondsHint": "范围 60-86400 秒。",
    "admin.settings.kiroRuntime.cachePrefixTtlSeconds": "前缀缓存 TTL（秒）",
    "admin.settings.kiroRuntime.cachePrefixTtlSecondsPlaceholder": "60 - 3600",
    "admin.settings.kiroRuntime.cachePrefixTtlSecondsHint": "范围 60-3600 秒，且不能大于独立缓存 TTL。",
    "admin.settings.scheduling.accountSchedulingThresholdsTitle": "平台账号自动停调阈值",
    "admin.settings.scheduling.accountSchedulingThresholdsDescription": "按平台设置账号自动停调阈值。",
    "admin.settings.scheduling.accountSchedulingThresholdsGlobalHint": "系统级全局设置，对该平台全部账号生效。",
    "admin.settings.scheduling.accountSchedulingThresholdsDisabledHint": "100 表示禁用该平台的自动停调阈值。",
    "admin.settings.scheduling.accountSchedulingThresholdsRangeHint": "范围 1-100，按百分比填写。",
    "admin.settings.kiroRuntime.cache_hit_rate_scale_range": "缓存命中率缩放必须在 0-100 之间。",
    "admin.settings.kiroRuntime.cache_min_block_tokens_range": "缓存最小块 Token 数必须大于等于 0。",
    "admin.settings.kiroRuntime.cache_independent_ttl_seconds_range": "独立缓存 TTL 必须在 60-86400 秒之间。",
    "admin.settings.kiroRuntime.cache_prefix_ttl_seconds_range": "前缀缓存 TTL 必须在 60-3600 秒之间。",
    "admin.settings.kiroRuntime.cache_prefix_ttl_seconds_exceeds_independent": "前缀缓存 TTL 不能大于独立缓存 TTL。",
    "admin.settings.paymentVisibleMethods.methodLabel": "{title} 可见方式",
    "admin.settings.paymentVisibleMethods.methodHint": "控制前台结算页是否展示该方式，以及展示时使用的来源键。",
    "admin.settings.paymentVisibleMethods.sourceLabel": "支付来源",
    "admin.settings.paymentVisibleMethods.sourceHint": "启用后必须明确选择一个来源；未配置状态不会对外展示该支付方式。",
    "admin.settings.paymentVisibleMethods.sourceRequiredError": "{title} 已启用，请先选择支付来源。",
    "admin.settings.payment.configGuide": "查看支付配置说明",
    "admin.settings.payment.findProvider": "查看支持的支付方式",
    "admin.settings.openaiExperimentalScheduler.title": "OpenAI 实验调度策略",
    "admin.settings.openaiExperimentalScheduler.description": "默认关闭。开启后仅影响本网关在 OpenAI 账号间的实验性调度选择逻辑，不代表上游 OpenAI 官方能力。",
    "admin.settings.site.uploadImage": "上传图片",
    "admin.settings.site.remove": "移除",
    "admin.settings.site.contactInfo": "客服联系方式",
    "admin.settings.site.contactInfoPlaceholder": "例如：QQ: 123456789",
    "admin.settings.site.contactInfoHint": "填写客服联系方式，将展示在兑换页面、个人资料等位置",
    "admin.settings.platformQuota.platform": "平台",
    "admin.settings.platformQuota.daily": "日限额 (USD)",
    "admin.settings.platformQuota.weekly": "周限额 (USD)",
    "admin.settings.platformQuota.monthly": "月限额 (USD, 30天滚动)",
    "admin.settings.platformQuota.placeholder": "不限",
    "admin.settings.defaults.defaultPlatformQuotas": "默认平台限额（注册时分配）",
    "admin.settings.defaults.defaultPlatformQuotasHint": "新用户注册时自动写入平台限额记录；已有用户不受影响。留空 = 该平台该窗口不限制。",
    "admin.settings.defaults.platformQuotaNotice": "月限额为 30 天滚动窗口，非自然月",
    "admin.settings.authSourceDefaults.platformQuotasOverride": "平台限额覆盖",
    "admin.settings.authSourceDefaults.platformQuotasOverrideHint": "留空的字段继承「系统默认平台限额」；填 0 表示禁止该窗口使用。",
  };
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string, params?: Record<string, string>) =>
        (translations[key] ?? key).replace(/\{(\w+)\}/g, (_, token) => params?.[token] ?? `{${token}}`),
      locale: localeRef,
    }),
  };
});

const AppLayoutStub = { template: "<div><slot /></div>" };
const ToggleStub = defineComponent({
  props: {
    modelValue: {
      type: Boolean,
      default: false,
    },
  },
  emits: ["update:modelValue"],
  inheritAttrs: false,
  setup(props, { attrs, emit }) {
    return () =>
      h("input", {
        ...attrs,
        class: "toggle-stub",
        type: "checkbox",
        checked: props.modelValue,
        onChange: (event: Event) => {
          emit("update:modelValue", (event.target as HTMLInputElement).checked);
        },
      });
  },
});

const SelectStub = defineComponent({
  props: {
    modelValue: {
      type: [String, Number, Boolean, null],
      default: "",
    },
    options: {
      type: Array,
      default: () => [],
    },
    placeholder: {
      type: String,
      default: "",
    },
  },
  emits: ["update:modelValue", "change"],
  inheritAttrs: false,
  setup(props, { attrs, emit }) {
    const onChange = (event: Event) => {
      const target = event.target as HTMLSelectElement;
      emit("update:modelValue", target.value);
      const option =
        (props.options as Array<Record<string, unknown>>).find(
          (item) => String(item.value ?? "") === target.value,
        ) ?? null;
      emit("change", target.value, option);
    };

    return () =>
      h(
        "select",
        {
          ...attrs,
          class: "select-stub",
          value: props.modelValue ?? "",
          "data-placeholder": props.placeholder,
          onChange,
        },
        (props.options as Array<Record<string, unknown>>).map((option) =>
          h(
            "option",
            {
              key: `${String(option.value ?? "")}:${String(option.label ?? "")}`,
              value: option.value as string,
            },
            String(option.label ?? ""),
          ),
        ),
      );
  },
});

const ImageUploadStub = defineComponent({
  props: {
    modelValue: {
      type: String,
      default: "",
    },
    uploadLabel: {
      type: String,
      default: "",
    },
    removeLabel: {
      type: String,
      default: "",
    },
    placeholder: {
      type: String,
      default: "",
    },
  },
  setup(props) {
    return () =>
      h("div", {
        class: "image-upload-stub",
        "data-model-value": props.modelValue,
        "data-upload-label": props.uploadLabel,
        "data-remove-label": props.removeLabel,
        "data-placeholder": props.placeholder,
      });
  },
});

const ModelWhitelistSelectorStub = defineComponent({
  props: {
    modelValue: {
      type: Array,
      default: () => [],
    },
  },
  emits: ["update:modelValue"],
  setup(props, { emit }) {
    return () =>
      h("button", {
        type: "button",
        "data-testid": "model-whitelist-selector-stub",
        "data-value": JSON.stringify(props.modelValue),
        onClick: () => emit("update:modelValue", props.modelValue),
      });
  },
});

const baseSettingsResponse = {
  registration_enabled: true,
  email_verify_enabled: false,
  registration_email_suffix_whitelist: [],
  promo_code_enabled: true,
  invitation_code_enabled: false,
  password_reset_enabled: false,
  totp_enabled: false,
  totp_encryption_key_configured: false,
  default_balance: 0,
  default_concurrency: 1,
  default_subscriptions: [],
  site_name: "Sub2API",
  site_logo: "",
  site_subtitle: "",
  api_base_url: "",
  contact_info: "",
  support_qr_codes: [],
  doc_url: "",
  home_content: "",
  hide_ccs_import_button: false,
  table_default_page_size: 20,
  table_page_size_options: [10, 20, 50, 100],
  backend_mode_enabled: false,
  custom_menu_items: [],
  custom_endpoints: [],
  frontend_url: "",
  smtp_host: "",
  smtp_port: 587,
  smtp_username: "",
  smtp_password_configured: false,
  smtp_from_email: "",
  smtp_from_name: "",
  smtp_use_tls: true,
  turnstile_enabled: false,
  turnstile_site_key: "",
  turnstile_secret_key_configured: false,
  linuxdo_connect_enabled: false,
  linuxdo_connect_client_id: "",
  linuxdo_connect_client_secret_configured: false,
  linuxdo_connect_redirect_url: "",
  dingtalk_connect_enabled: false,
  dingtalk_connect_client_id: "",
  dingtalk_connect_client_secret_configured: false,
  dingtalk_connect_redirect_url: "",
  dingtalk_connect_corp_restriction_policy: "none",
  dingtalk_connect_internal_corp_id: "",
  dingtalk_connect_bypass_registration: false,
  dingtalk_connect_sync_corp_email: false,
  dingtalk_connect_sync_display_name: false,
  dingtalk_connect_sync_dept: false,
  dingtalk_connect_sync_corp_email_attr_key: "",
  dingtalk_connect_sync_display_name_attr_key: "",
  dingtalk_connect_sync_dept_attr_key: "",
  dingtalk_connect_sync_corp_email_attr_name: "",
  dingtalk_connect_sync_display_name_attr_name: "",
  dingtalk_connect_sync_dept_attr_name: "",
  wechat_connect_enabled: true,
  wechat_connect_app_id: "wx-app-id-123",
  wechat_connect_app_secret_configured: true,
  wechat_connect_open_enabled: false,
  wechat_connect_mp_enabled: true,
  wechat_connect_mode: "mp",
  wechat_connect_scopes: "",
  wechat_connect_redirect_url:
    "https://admin.example.com/api/v1/auth/oauth/wechat/callback",
  wechat_connect_frontend_redirect_url: "/auth/wechat/callback",
  oidc_connect_enabled: false,
  oidc_connect_provider_name: "OIDC",
  oidc_connect_client_id: "",
  oidc_connect_client_secret_configured: false,
  oidc_connect_issuer_url: "",
  oidc_connect_discovery_url: "",
  oidc_connect_authorize_url: "",
  oidc_connect_token_url: "",
  oidc_connect_userinfo_url: "",
  oidc_connect_jwks_url: "",
  oidc_connect_scopes: "openid email profile",
  oidc_connect_redirect_url: "",
  oidc_connect_frontend_redirect_url: "/auth/oidc/callback",
  oidc_connect_token_auth_method: "client_secret_post",
  oidc_connect_use_pkce: true,
  oidc_connect_validate_id_token: true,
  oidc_connect_allowed_signing_algs: "RS256,ES256,PS256",
  oidc_connect_clock_skew_seconds: 120,
  oidc_connect_require_email_verified: false,
  oidc_connect_userinfo_email_path: "",
  oidc_connect_userinfo_id_path: "",
  oidc_connect_userinfo_username_path: "",
  enable_model_fallback: false,
  fallback_model_anthropic: "",
  fallback_model_openai: "",
  fallback_model_gemini: "",
  fallback_model_antigravity: "",
  enable_identity_patch: false,
  identity_patch_prompt: "",
  ops_monitoring_enabled: false,
  ops_realtime_monitoring_enabled: false,
  ops_query_mode_default: "auto",
  ops_metrics_interval_seconds: 60,
  min_claude_code_version: "",
  max_claude_code_version: "",
  allow_ungrouped_key_scheduling: false,
  account_scheduling_thresholds: {
    openai: 100,
    anthropic: 100,
  },
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  claude_telemetry_mode: "drop",
  enable_claude_oauth_system_prompt_injection: true,
  claude_oauth_system_prompt: "",
  claude_oauth_system_prompt_blocks: "",
  enable_anthropic_cache_ttl_1h_injection: false,
  rewrite_message_cache_control: false,
  enable_client_dateline_normalization: true,
  antigravity_user_agent_version: "",
  openai_codex_user_agent: "",
  payment_enabled: true,
  payment_min_amount: 1,
  payment_max_amount: 10000,
  payment_daily_limit: 50000,
  payment_order_timeout_minutes: 30,
  payment_max_pending_orders: 3,
  payment_enabled_types: [],
  payment_balance_disabled: false,
  payment_balance_recharge_multiplier: 1,
  payment_recharge_fee_rate: 0,
  payment_load_balance_strategy: "round-robin",
  payment_product_name_prefix: "",
  payment_product_name_suffix: "",
  payment_help_image_url: "",
  payment_help_text: "",
  payment_cancel_rate_limit_enabled: false,
  payment_cancel_rate_limit_max: 10,
  payment_cancel_rate_limit_window: 1,
  payment_cancel_rate_limit_unit: "day",
  payment_cancel_rate_limit_window_mode: "rolling",
  payment_visible_method_alipay_source: "alipay_direct",
  payment_visible_method_wxpay_source: "invalid-source",
  payment_visible_method_alipay_enabled: true,
  payment_visible_method_wxpay_enabled: true,
  openai_advanced_scheduler_enabled: false,
  openai_ws_neutral_prewarm_percent: 20,
  openai_ws_session_idle_ttl_seconds: 120,
  openai_oauth_image_bridge_disable_keepalives: false,
  openai_oauth_image_bridge_fresh_upstream_client: false,
  balance_low_notify_enabled: false,
  balance_low_notify_threshold: 0,
  balance_low_notify_recharge_url: "",
  subscription_expiry_notify_enabled: true,
  account_quota_notify_enabled: false,
  account_quota_notify_emails: [],
  kiro_version: "0.10.0",
  kiro_commit: "",
  system_version: "darwin#24.6.0",
  node_version: "22.21.1",
  kiro_code_execution_sandbox_command: "sandbox-current",
  cache_hit_rate_scale: 85,
  cache_min_block_tokens: 1024,
  cache_independent_ttl_seconds: 3600,
  cache_prefix_ttl_seconds: 3600,
  default_platform_quotas: {
    anthropic:   { daily: null, weekly: null, monthly: null },
    openai:      { daily: null, weekly: 12.5, monthly: null },
    gemini:      { daily: null, weekly: null, monthly: 200 },
    antigravity: { daily: null, weekly: null, monthly: null },
  },
};

function mountView() {
  return mount(SettingsView, {
    global: {
      stubs: {
        AppLayout: AppLayoutStub,
        Select: SelectStub,
        Toggle: ToggleStub,
        Icon: true,
        ConfirmDialog: true,
        PaymentProviderList: true,
        PaymentProviderDialog: true,
        GroupBadge: true,
        GroupOptionItem: true,
        ProxySelector: true,
        ImageUpload: ImageUploadStub,
        ModelWhitelistSelector: ModelWhitelistSelectorStub,
        BackupSettings: true,
        EmailTemplateEditor: {
          template: '<div data-testid="email-template-editor-stub" />',
        },
      },
    },
  });
}

async function openPaymentTab(wrapper: ReturnType<typeof mountView>) {
  const paymentTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.tabs.payment"));

  expect(paymentTabButton).toBeDefined();
  await paymentTabButton?.trigger("click");
  await flushPromises();
}

async function openSecurityTab(wrapper: ReturnType<typeof mountView>) {
  const securityTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.tabs.security"));

  expect(securityTabButton).toBeDefined();
  await securityTabButton?.trigger("click");
  await flushPromises();
}

async function openUsersTab(wrapper: ReturnType<typeof mountView>) {
  const usersTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.tabs.users"));

  expect(usersTabButton).toBeDefined();
  await usersTabButton?.trigger("click");
  await flushPromises();
}

async function openGatewayTab(wrapper: ReturnType<typeof mountView>) {
  const gatewayTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.tabs.gateway"));

  expect(gatewayTabButton).toBeDefined();
  await gatewayTabButton?.trigger("click");
  await flushPromises();
}

async function openEmailTab(wrapper: ReturnType<typeof mountView>) {
  const emailTabButton = wrapper
    .findAll("button")
    .find((node) => node.text().includes("admin.settings.tabs.email"));

  expect(emailTabButton).toBeDefined();
  await emailTabButton?.trigger("click");
  await flushPromises();
}

describe("admin SettingsView payment visible method controls", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getWebSearchEmulationConfig.mockReset();
    updateWebSearchEmulationConfig.mockReset();
    getAdminApiKey.mockReset();
    getOverloadCooldownSettings.mockReset();
    getRateLimit429CooldownSettings.mockReset();
    updateRateLimit429CooldownSettings.mockReset();
    getStreamTimeoutSettings.mockReset();
    getTempUnschedThresholdSettings.mockReset();
    updateTempUnschedThresholdSettings.mockReset();
    getRectifierSettings.mockReset();
    getBetaPolicySettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    getProviders.mockReset();
    updateProvider.mockReset();
    createProvider.mockReset();
    deleteProvider.mockReset();
    fetchPublicSettings.mockReset();
    adminSettingsFetch.mockReset();
    showError.mockReset();
    showSuccess.mockReset();
    localeRef.value = "zh-CN";

    getSettings.mockResolvedValue({ ...baseSettingsResponse });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
    }));
    getWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    updateWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    getAdminApiKey.mockResolvedValue({
      exists: false,
      masked_key: "",
    });
    getOverloadCooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_minutes: 10,
    });
    getRateLimit429CooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_seconds: 5,
    });
    updateRateLimit429CooldownSettings.mockImplementation(async (payload) => payload);
    getStreamTimeoutSettings.mockResolvedValue({
      enabled: true,
      action: "temp_unsched",
      temp_unsched_minutes: 5,
      threshold_count: 3,
      threshold_window_minutes: 10,
    });
    getTempUnschedThresholdSettings.mockResolvedValue({
      enabled: true,
      threshold_count: 3,
      threshold_window_minutes: 1,
    });
    updateTempUnschedThresholdSettings.mockImplementation(async (payload) => payload);
    getRectifierSettings.mockResolvedValue({
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: false,
      apikey_signature_patterns: [],
    });
    getBetaPolicySettings.mockResolvedValue({
      rules: [],
    });
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({
      items: [],
    });
    getProviders.mockResolvedValue({
      data: [],
    });
    fetchPublicSettings.mockResolvedValue(undefined);
    adminSettingsFetch.mockResolvedValue(undefined);
  });

  it("does not eagerly load security and gateway secondary settings before the tabs are opened", async () => {
    const wrapper = mountView();

    await flushPromises();

    expect(getSettings).toHaveBeenCalledTimes(1);
    expect(getGroups).toHaveBeenCalledTimes(1);
    expect(getAdminApiKey).not.toHaveBeenCalled();
    expect(getOverloadCooldownSettings).not.toHaveBeenCalled();
    expect(getRateLimit429CooldownSettings).not.toHaveBeenCalled();
    expect(getStreamTimeoutSettings).not.toHaveBeenCalled();
    expect(getTempUnschedThresholdSettings).not.toHaveBeenCalled();
    expect(getRectifierSettings).not.toHaveBeenCalled();
    expect(getBetaPolicySettings).not.toHaveBeenCalled();
    expect(getWebSearchEmulationConfig).not.toHaveBeenCalled();

    await openSecurityTab(wrapper);
    expect(getAdminApiKey).toHaveBeenCalledTimes(1);

    await openGatewayTab(wrapper);
    expect(getOverloadCooldownSettings).toHaveBeenCalledTimes(1);
    expect(getRateLimit429CooldownSettings).toHaveBeenCalledTimes(1);
    expect(getStreamTimeoutSettings).toHaveBeenCalledTimes(1);
    expect(getTempUnschedThresholdSettings).toHaveBeenCalledTimes(1);
    expect(getRectifierSettings).toHaveBeenCalledTimes(1);
    expect(getBetaPolicySettings).toHaveBeenCalledTimes(1);
    expect(getWebSearchEmulationConfig).toHaveBeenCalledTimes(1);
  });

  it("does not render legacy visible payment method controls", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openPaymentTab(wrapper);

    expect(wrapper.text()).not.toContain("可见方式");
    expect(wrapper.text()).not.toContain("支付来源");
  });

  it("links payment guidance to README sections instead of removed payment docs", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openPaymentTab(wrapper);

    const paymentLinks = wrapper
      .findAll("a")
      .filter((node) =>
        ["查看支付配置说明", "查看支持的支付方式"].includes(node.text()),
      );

    expect(paymentLinks).toHaveLength(2);
    expect(paymentLinks[0]?.attributes("href")).toBe(
      "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md",
    );
    expect(paymentLinks[1]?.attributes("href")).toBe(
      "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md#支持的支付方式",
    );
    for (const link of paymentLinks) {
      expect(link.attributes("href")).toContain("docs/PAYMENT");
    }
  });

  it("does not submit legacy visible payment method settings", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openPaymentTab(wrapper);
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    const payload = updateSettings.mock.calls[0]?.[0];
    expect(payload).not.toHaveProperty("payment_visible_method_alipay_source");
    expect(payload).not.toHaveProperty("payment_visible_method_wxpay_source");
    expect(payload).not.toHaveProperty("payment_visible_method_alipay_enabled");
    expect(payload).not.toHaveProperty("payment_visible_method_wxpay_enabled");
  });

  it("submits Anthropic cache TTL injection gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      enable_anthropic_cache_ttl_1h_injection: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enable_anthropic_cache_ttl_1h_injection: true,
      }),
    );
  });

  it("submits message cache_control rewrite gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      rewrite_message_cache_control: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        rewrite_message_cache_control: true,
      }),
    );
  });

  it("submits Claude telemetry mode gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      claude_telemetry_mode: "forward",
    });

    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);
    expect(wrapper.text()).toContain("admin.settings.gatewayForwarding.claudeTelemetryMode");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        claude_telemetry_mode: "forward",
      }),
    );
  });

  it("clamps temp-unschedulable threshold settings before saving", async () => {
    const wrapper = mountView();

    await flushPromises();
    (wrapper.vm as any).tempUnschedThresholdForm.enabled = true;
    (wrapper.vm as any).tempUnschedThresholdForm.threshold_count = 0;
    (wrapper.vm as any).tempUnschedThresholdForm.threshold_window_minutes = 99;

    await (wrapper.vm as any).saveTempUnschedThresholdSettings();
    await flushPromises();

    expect(updateTempUnschedThresholdSettings).toHaveBeenCalledTimes(1);
    expect(updateTempUnschedThresholdSettings).toHaveBeenCalledWith({
      enabled: true,
      threshold_count: 1,
      threshold_window_minutes: 60,
    });
  });

  it("allows temp-unschedulable threshold count up to backend contract max", async () => {
    const wrapper = mountView();

    await flushPromises();
    (wrapper.vm as any).tempUnschedThresholdForm.enabled = true;
    (wrapper.vm as any).tempUnschedThresholdForm.threshold_count = 999;
    (wrapper.vm as any).tempUnschedThresholdForm.threshold_window_minutes = 1;

    await (wrapper.vm as any).saveTempUnschedThresholdSettings();
    await flushPromises();

    expect(updateTempUnschedThresholdSettings).toHaveBeenCalledWith({
      enabled: true,
      threshold_count: 999,
      threshold_window_minutes: 1,
    });
  });

  it("preserves loaded anti-ban platform toggles when saving untouched settings", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      anti_ban_platforms: {
        anthropic: true,
        openai: false,
        grok: true,
      },
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        anti_ban_platforms: expect.objectContaining({
          anthropic: true,
          openai: false,
          grok: true,
          gemini: false,
          kiro: false,
          antigravity: false,
        }),
      }),
    );
  });

  it("keeps sticky wait timeout from updateSettings response in the form", async () => {
    updateSettings.mockImplementationOnce(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
      openai_sticky_wait_timeout_seconds: 73,
    }));
    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect((wrapper.vm as any).form.openai_sticky_wait_timeout_seconds).toBe(73);
  });

  it("submits Claude OAuth system prompt injection gateway settings", async () => {
    const blocks = `[{"type":"text","text":"custom block","cache_control":true}]`;
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      enable_claude_oauth_system_prompt_injection: false,
      claude_oauth_system_prompt_blocks: blocks,
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        enable_claude_oauth_system_prompt_injection: false,
      }),
    );
    const payload = updateSettings.mock.calls[0][0] as {
      claude_oauth_system_prompt_blocks: string;
    };
    expect(JSON.parse(payload.claude_oauth_system_prompt_blocks)).toEqual([
      {
        enabled: true,
        type: "text",
        text: "custom block",
        cache_control: {
          type: "ephemeral",
          ttl: "5m",
        },
      },
    ]);
  });

  it("preserves inherited Claude OAuth system prompt blocks when unchanged", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      claude_oauth_system_prompt_blocks: "",
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    const payload = updateSettings.mock.calls[0][0] as {
      claude_oauth_system_prompt_blocks: string;
    };
    expect(payload.claude_oauth_system_prompt_blocks).toBe("");
  });

  it("submits Antigravity user agent version gateway setting", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      antigravity_user_agent_version: "1.23.2",
    });

    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        antigravity_user_agent_version: "1.23.2",
      }),
    );
  });

  it("submits OpenAI WS neutral prewarm percent and session idle TTL settings", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.form.openai_ws_neutral_prewarm_percent = 120;
    setupState.form.openai_ws_session_idle_ttl_seconds = 0;
    setupState.form.openai_ws_min_idle_per_account = 5;
    setupState.form.openai_ws_max_idle_per_account = 2;

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).not.toHaveBeenCalled();
    expect(updateSettings).toHaveBeenCalledTimes(1);
    const payload = updateSettings.mock.calls[0][0];
    expect(payload).toEqual(
      expect.objectContaining({
        openai_ws_neutral_prewarm_percent: 100,
        openai_ws_session_idle_ttl_seconds: 1,
      }),
    );
    expect(payload).not.toHaveProperty("openai_ws_min_idle_per_account");
    expect(payload).not.toHaveProperty("openai_ws_max_idle_per_account");
  });

  it("submits dynamic OpenAI WS delta and diagnostic log switches", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.form.openai_ws_delta_shadow_enabled = true;
    setupState.form.openai_ws_active_delta_enabled = false;
    setupState.form.openai_ws_temp_diag_logs_enabled = true;

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        openai_ws_delta_shadow_enabled: true,
        openai_ws_active_delta_enabled: false,
        openai_ws_temp_diag_logs_enabled: true,
      }),
    );
  });

  it("updates provider enablement immediately and reloads providers", async () => {
    const provider = {
      id: 7,
      provider_key: "alipay",
      name: "Official Alipay",
      config: {},
      supported_types: ["alipay"],
      enabled: false,
      payment_mode: "",
      refund_enabled: false,
      allow_user_refund: false,
      limits: "",
      sort_order: 0,
    };
    getProviders.mockReset();
    getProviders
      .mockResolvedValueOnce({ data: [provider] })
      .mockResolvedValueOnce({ data: [{ ...provider, enabled: true }] });
    updateProvider.mockResolvedValue({ data: { ...provider, enabled: true } });

    const PaymentProviderListStub = defineComponent({
      emits: ["toggleField"],
      setup(_, { emit }) {
        return () =>
          h(
            "button",
            {
              class: "provider-toggle-stub",
              onClick: () => emit("toggleField", provider, "enabled"),
            },
            "toggle provider",
          );
      },
    });

    const wrapper = mount(SettingsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Select: SelectStub,
          Toggle: ToggleStub,
          Icon: true,
          ConfirmDialog: true,
          PaymentProviderList: PaymentProviderListStub,
          PaymentProviderDialog: true,
          GroupBadge: true,
          GroupOptionItem: true,
          ProxySelector: true,
          ImageUpload: ImageUploadStub,
          ModelWhitelistSelector: ModelWhitelistSelectorStub,
          BackupSettings: true,
        },
      },
    });

    await flushPromises();
    await openPaymentTab(wrapper);
    await wrapper.get(".provider-toggle-stub").trigger("click");
    await flushPromises();

    expect(updateProvider).toHaveBeenCalledWith(7, { enabled: true });
    expect(getProviders).toHaveBeenCalledTimes(2);
  });

  it("renders advanced scheduler copy as local experimental gateway policy", async () => {
    const wrapper = mountView();

    await flushPromises();

    expect(wrapper.text()).toContain("OpenAI 实验调度策略");
    expect(wrapper.text()).toContain(
      "默认关闭。开启后仅影响本网关在 OpenAI 账号间的实验性调度选择逻辑",
    );
    expect(wrapper.text()).not.toContain("OpenAI 高级调度器");
  });

  it("passes translated upload and remove labels to the payment help image uploader", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openPaymentTab(wrapper);

    const imageUploads = wrapper.findAll(".image-upload-stub");
    expect(imageUploads.length).toBeGreaterThan(0);

    const paymentHelpImageUpload = imageUploads.find(
      (node) => node.attributes("data-placeholder") === "admin.settings.payment.helpImagePlaceholder",
    );

    expect(paymentHelpImageUpload).toBeDefined();
    expect(paymentHelpImageUpload?.attributes("data-upload-label")).toBe("上传图片");
    expect(paymentHelpImageUpload?.attributes("data-remove-label")).toBe("移除");
  });

  it("does not render removed Kiro thinking compatibility controls", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    expect(wrapper.get('[data-testid="security-settings-panel"]').text()).not.toContain("Thinking 兼容");

    await openGatewayTab(wrapper);

    expect(wrapper.text()).not.toContain("Thinking 兼容");
    expect(wrapper.findAll('[data-testid="kiro-runtime-thinking-mode"]')).toHaveLength(0);
    expect(wrapper.findAll('[data-testid="kiro-runtime-thinking-template"]')).toHaveLength(0);
  });

  it("does not render or submit removed OpenAI Web2API image model settings", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    expect(wrapper.text()).not.toContain("admin.settings.openaiImageWebModels.title");
    expect(wrapper.findAll('[data-testid="openai-image-web-free-model"]')).toHaveLength(0);
    expect(wrapper.findAll('[data-testid="openai-image-web-paid-model"]')).toHaveLength(0);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    const payload = updateSettings.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(payload).not.toHaveProperty("openai_image_web_free_model");
    expect(payload).not.toHaveProperty("openai_image_web_paid_model");
  });

  it("keeps security and gateway content inside a single tab panel each", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    const securityPanel = wrapper.get('[data-testid="security-settings-panel"]');
    expect(securityPanel.text()).toContain("admin.settings.registration.title");

    await openGatewayTab(wrapper);

	const gatewayPanel = wrapper.get('[data-testid="gateway-settings-panel"]');
	expect(gatewayPanel.text()).toContain("admin.settings.claudeCode.title");
	expect(gatewayPanel.text()).toContain("admin.settings.scheduling.title");
	expect(gatewayPanel.text()).toContain("Kiro 运行默认值");
	expect(gatewayPanel.text()).not.toContain("Thinking 兼容");
  });

  it("renders platform account auto-pause thresholds inside the gateway scheduling card", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      account_scheduling_thresholds: {
        openai: 82,
        anthropic: 67,
        gemini: 100,
        kiro: 58,
        antigravity: 91,
      },
    });

    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    expect(wrapper.text()).toContain("平台账号自动停调阈值");
    expect(wrapper.text()).toContain("100 表示禁用该平台的自动停调阈值。");
    expect(wrapper.findAll('[data-testid^="account-scheduling-threshold-"]')).toHaveLength(3);
    expect(
      (
        wrapper.get('[data-testid="account-scheduling-threshold-openai"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("82");
    expect(
      (
        wrapper.get('[data-testid="account-scheduling-threshold-anthropic"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("67");
    expect(
      (
        wrapper.get('[data-testid="account-scheduling-threshold-grok"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("100");
    expect(wrapper.find('[data-testid="account-scheduling-threshold-kiro"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="account-scheduling-threshold-gemini"]').exists()).toBe(false);
  });

  it("normalizes and submits account scheduling thresholds for supported platforms only", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      account_scheduling_thresholds: {
        openai: 0,
        anthropic: 45,
        grok: 67,
        gemini: 999,
      },
    });

    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="account-scheduling-threshold-openai"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("1");
    expect(
      (
        wrapper.get('[data-testid="account-scheduling-threshold-grok"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("67");
    expect(wrapper.find('[data-testid="account-scheduling-threshold-gemini"]').exists()).toBe(false);
    expect(wrapper.find('[data-testid="account-scheduling-threshold-kiro"]').exists()).toBe(false);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalled();
    const payload = updateSettings.mock.calls.at(-1)?.[0] as Record<string, unknown>;
    expect(payload.account_scheduling_thresholds).toEqual({
      openai: 1,
      anthropic: 45,
      grok: 67,
    });
  });

  it("loads support contact info in the general tab", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      contact_info: "QQ: 123456789",
      support_qr_codes: [{ image_url: "https://example.com/qr.png", note: "工作日" }],
    });

    const wrapper = mountView();

    await flushPromises();

    expect(
      (wrapper.get('[data-testid="site-contact-info"]').element as HTMLInputElement).value,
    ).toBe("QQ: 123456789");
    expect(wrapper.get('[data-testid="support-qr-grid"]').classes()).not.toContain("xl:grid-cols-2");
  });

  it("auto-centers the active settings tab when switching tabs", async () => {
    const scrollTo = vi.fn();
    Object.defineProperty(HTMLElement.prototype, "scrollTo", {
      value: scrollTo,
      configurable: true,
      writable: true,
    });

    const wrapper = mountView();

    await flushPromises();
    scrollTo.mockClear();

    await openGatewayTab(wrapper);

    expect(scrollTo).toHaveBeenCalled();
    expect(scrollTo).toHaveBeenLastCalledWith(
      expect.objectContaining({
        behavior: "smooth",
      }),
    );
  });

  it("normalizes null supported_types from API so provider card stays visible", async () => {
    // Backend returns null for supported_types when the list is empty
    // (Go nil slice → JSON null). Without normalization, ProviderCard's
    // isSelected() throws TypeError on null.includes(), causing the card
    // to vanish from the list.
    const providerWithNullTypes = {
      id: 42,
      provider_key: "easypay",
      name: "EasyPay",
      config: {},
      supported_types: null as unknown as string[],
      enabled: true,
      payment_mode: "",
      refund_enabled: false,
      allow_user_refund: false,
      limits: "",
      sort_order: 0,
    };
    getProviders.mockReset();
    getProviders.mockResolvedValue({ data: [providerWithNullTypes] });

    let receivedProviders: Array<Record<string, unknown>> = [];
    const PaymentProviderListCapture = defineComponent({
      props: {
        providers: {
          type: Array,
          default: () => [],
        },
      },
      setup(props) {
        return () => {
          receivedProviders = props.providers as Array<Record<string, unknown>>;
          return h("div", { class: "provider-list-capture" });
        };
      },
    });

    const wrapper = mount(SettingsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          Select: SelectStub,
          Toggle: ToggleStub,
          Icon: true,
          ConfirmDialog: true,
          PaymentProviderList: PaymentProviderListCapture,
          PaymentProviderDialog: true,
          GroupBadge: true,
          GroupOptionItem: true,
          ProxySelector: true,
          ImageUpload: ImageUploadStub,
          BackupSettings: true,
        },
      },
    });

    await flushPromises();
    await openPaymentTab(wrapper);

    // The provider should still be in the list
    expect(receivedProviders.length).toBe(1);
    // supported_types should be normalized to an empty array, not null
    expect(Array.isArray(receivedProviders[0].supported_types)).toBe(true);
    expect(receivedProviders[0].supported_types).toEqual([]);
  });
});

describe("admin SettingsView wechat connect controls", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getWebSearchEmulationConfig.mockReset();
    updateWebSearchEmulationConfig.mockReset();
    getAdminApiKey.mockReset();
    getOverloadCooldownSettings.mockReset();
    getRateLimit429CooldownSettings.mockReset();
    updateRateLimit429CooldownSettings.mockReset();
    getStreamTimeoutSettings.mockReset();
    getTempUnschedThresholdSettings.mockReset();
    updateTempUnschedThresholdSettings.mockReset();
    getRectifierSettings.mockReset();
    getBetaPolicySettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    getProviders.mockReset();
    updateProvider.mockReset();
    createProvider.mockReset();
    deleteProvider.mockReset();
    fetchPublicSettings.mockReset();
    adminSettingsFetch.mockReset();
    showError.mockReset();
    showSuccess.mockReset();

    getSettings.mockResolvedValue({
      ...baseSettingsResponse,
      payment_visible_method_wxpay_source: "official_wxpay",
    });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      payment_visible_method_wxpay_source: "official_wxpay",
      ...payload,
    }));
    getWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    updateWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    getAdminApiKey.mockResolvedValue({
      exists: false,
      masked_key: "",
    });
    getOverloadCooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_minutes: 10,
    });
    getRateLimit429CooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_seconds: 5,
    });
    updateRateLimit429CooldownSettings.mockImplementation(async (payload) => payload);
    getStreamTimeoutSettings.mockResolvedValue({
      enabled: true,
      action: "temp_unsched",
      temp_unsched_minutes: 5,
      threshold_count: 3,
      threshold_window_minutes: 10,
    });
    getTempUnschedThresholdSettings.mockResolvedValue({
      enabled: true,
      threshold_count: 3,
      threshold_window_minutes: 1,
    });
    updateTempUnschedThresholdSettings.mockImplementation(async (payload) => payload);
    getRectifierSettings.mockResolvedValue({
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: false,
      apikey_signature_patterns: [],
    });
    getBetaPolicySettings.mockResolvedValue({
      rules: [],
    });
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({
      items: [],
    });
    getProviders.mockResolvedValue({
      data: [],
    });
    fetchPublicSettings.mockResolvedValue(undefined);
    adminSettingsFetch.mockResolvedValue(undefined);
  });

  it("loads and echoes WeChat Connect fields from the backend payload", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="wechat-connect-mp-app-id"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("wx-app-id-123");
    expect(
      (
        wrapper.get('[data-testid="wechat-connect-open-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(false);
    expect(
      (
        wrapper.get('[data-testid="wechat-connect-mp-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(true);
    expect(wrapper.find('[data-testid="wechat-connect-scopes"]').exists()).toBe(
      false,
    );
    expect(
      wrapper
        .get('[data-testid="wechat-connect-mp-app-secret"]')
        .attributes("placeholder"),
    ).toContain("密钥已配置");
    expect(
      (
        wrapper.get('[data-testid="wechat-connect-frontend-redirect-url"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("/auth/wechat/callback");
  });

  it("links GitHub OAuth Apps guide to GitHub developer settings", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      github_oauth_enabled: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    const link = wrapper.get('[data-testid="github-oauth-apps-guide-link"]');
    expect(link.text()).toContain("OAuth Apps");
    expect(link.attributes("href")).toBe("https://github.com/settings/developers");
    expect(link.attributes("target")).toBe("_blank");
    expect(link.attributes("rel")).toContain("noopener");
  });

  it("builds OAuth backend callback suggestions from api_base_url when present", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      api_base_url: "https://api.example.com/api/v1",
      dingtalk_connect_enabled: true,
      github_oauth_enabled: true,
      google_oauth_enabled: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    expect(wrapper.text()).toContain(
      "https://api.example.com/api/v1/auth/oauth/github/callback",
    );
    expect(wrapper.text()).toContain(
      "https://api.example.com/api/v1/auth/oauth/google/callback",
    );
    expect(wrapper.text()).toContain(
      "https://api.example.com/api/v1/auth/oauth/dingtalk/callback",
    );
  });

  it("saves WeChat Connect fields using the backend contract and clears the secret after save", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    await wrapper
      .get('[data-testid="wechat-connect-mp-app-id"]')
      .setValue("wx-app-id-updated");
    await wrapper
      .get('[data-testid="wechat-connect-mp-app-secret"]')
      .setValue("new-secret");
    await wrapper
      .get('[data-testid="wechat-connect-open-enabled"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="wechat-connect-mp-enabled"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="wechat-connect-redirect-url"]')
      .setValue("https://admin.example.com/api/v1/auth/oauth/wechat/callback");
    await wrapper
      .get('[data-testid="wechat-connect-frontend-redirect-url"]')
      .setValue("/auth/wechat/callback");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        wechat_connect_enabled: true,
        wechat_connect_app_id: "wx-app-id-updated",
        wechat_connect_open_enabled: true,
        wechat_connect_mp_enabled: true,
        wechat_connect_mp_app_id: "wx-app-id-updated",
        wechat_connect_mp_app_secret: "new-secret",
        wechat_connect_redirect_url:
          "https://admin.example.com/api/v1/auth/oauth/wechat/callback",
        wechat_connect_frontend_redirect_url: "/auth/wechat/callback",
      }),
    );
    expect(
      (
        wrapper.get('[data-testid="wechat-connect-mp-app-secret"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("");
    expect(
      wrapper
        .get('[data-testid="wechat-connect-mp-app-secret"]')
        .attributes("placeholder"),
    ).toContain("密钥已配置");
  });

  it("shows source bonus grants settings only after the source grant is enabled", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openUsersTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="auth-source-email-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(false);
    expect(
      wrapper.find('[data-testid="auth-source-email-panel"]').exists(),
    ).toBe(false);
    expect(
      wrapper
        .find('[data-testid="auth-source-email-first-bind-enabled"]')
        .exists(),
    ).toBe(false);

    await wrapper
      .get('[data-testid="auth-source-email-enabled"]')
      .setValue(true);

    const panel = wrapper.get('[data-testid="auth-source-email-panel"]');
    expect(panel.exists()).toBe(true);
    expect(
      wrapper
        .find('[data-testid="auth-source-email-first-bind-enabled"]')
        .exists(),
    ).toBe(true);
  });

  it("serializes auth-source grant toggles while preserving omitted and explicit zero concurrency", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      auth_source_default_linuxdo_balance: 2,
      auth_source_default_linuxdo_concurrency: 0,
      auth_source_default_linuxdo_grant_on_signup: true,
      auth_source_default_linuxdo_grant_on_first_bind: false,
    });

    const wrapper = mountView();

    await flushPromises();
    await openUsersTab(wrapper);

    await wrapper
      .get('[data-testid="auth-source-email-enabled"]')
      .setValue(true);
    await flushPromises();
    await wrapper
      .get('[data-testid="auth-source-email-first-bind-enabled"]')
      .setValue(true);
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        auth_source_default_email_grant_on_signup: true,
        auth_source_default_email_grant_on_first_bind: true,
        auth_source_default_linuxdo_concurrency: 0,
      }),
    );
    expect(updateSettings.mock.calls[0]?.[0]).not.toHaveProperty(
      "auth_source_default_email_concurrency",
    );
  });

  it("preserves first-bind auth-source grants without forcing signup grants", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      auth_source_default_email_balance: 5,
      auth_source_default_email_grant_on_signup: false,
      auth_source_default_email_grant_on_first_bind: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await openUsersTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="auth-source-email-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(false);
    expect(
      wrapper.find('[data-testid="auth-source-email-panel"]').exists(),
    ).toBe(true);
    expect(
      (
        wrapper.get('[data-testid="auth-source-email-first-bind-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(true);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        auth_source_default_email_grant_on_signup: false,
        auth_source_default_email_grant_on_first_bind: true,
      }),
    );
  });

  it("preserves notification settings values when saving untouched data", async () => {
    const wrapper = mountView();

    await flushPromises();
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        balance_low_notify_recharge_url: "",
        subscription_expiry_notify_enabled: true,
      }),
    );
  });

  it("does not block save for duplicate subscriptions on disabled source bonus sections", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      auth_source_default_email_balance: 5,
      auth_source_default_email_subscriptions: [
        { group_id: 1, validity_days: 30 },
        { group_id: 1, validity_days: 90 },
      ],
      auth_source_default_email_grant_on_signup: false,
      auth_source_default_email_grant_on_first_bind: false,
    });

    const wrapper = mountView();

    await flushPromises();
    await openUsersTab(wrapper);
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).not.toHaveBeenCalledWith(
      expect.stringContaining("邮箱注册"),
    );
    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        auth_source_default_email_subscriptions: [
          { group_id: 1, validity_days: 30 },
        ],
        auth_source_default_email_grant_on_signup: false,
        auth_source_default_email_grant_on_first_bind: false,
      }),
    );
  });

  it("preserves optional OIDC compatibility flags instead of forcing them on save", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      oidc_connect_enabled: true,
      oidc_connect_use_pkce: false,
      oidc_connect_validate_id_token: false,
    });

    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        oidc_connect_use_pkce: false,
        oidc_connect_validate_id_token: false,
      }),
    );
  });

  it("renders kiro runtime defaults from settings and submits normalized values", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="kiro-runtime-version"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("0.10.0");
    expect(
      (
        wrapper.get('[data-testid="kiro-runtime-cache-hit-rate-scale"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("85");
    expect(
      (
        wrapper.get('[data-testid="kiro-runtime-code-execution-sandbox-command"]')
          .element as HTMLTextAreaElement
      ).value,
    ).toBe("sandbox-current");

    await wrapper.get('[data-testid="kiro-runtime-version"]').setValue(" 0.11.0 ");
    await wrapper
      .get('[data-testid="kiro-runtime-commit"]')
      .setValue(" abc123 ");
    await wrapper
      .get('[data-testid="kiro-runtime-system-version"]')
      .setValue(" linux#6.8.0 ");
    await wrapper
      .get('[data-testid="kiro-runtime-node-version"]')
      .setValue(" 24.1.0 ");
    await wrapper
      .get('[data-testid="kiro-runtime-code-execution-sandbox-command"]')
      .setValue("  sandbox-next --language-env  ");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-hit-rate-scale"]')
      .setValue("88.6");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-min-block-tokens"]')
      .setValue("2048.9");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-independent-ttl-seconds"]')
      .setValue("7200.2");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-prefix-ttl-seconds"]')
      .setValue("600.7");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        kiro_version: "0.11.0",
        kiro_commit: "abc123",
        system_version: "linux#6.8.0",
        node_version: "24.1.0",
        kiro_code_execution_sandbox_command: "sandbox-next --language-env",
        cache_hit_rate_scale: 88,
        cache_min_block_tokens: 2048,
        cache_independent_ttl_seconds: 7200,
        cache_prefix_ttl_seconds: 600,
      }),
    );
  });

  it("submits default resets when kiro runtime cache fields are cleared", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    await wrapper
      .get('[data-testid="kiro-runtime-cache-hit-rate-scale"]')
      .setValue("");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-min-block-tokens"]')
      .setValue("");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-independent-ttl-seconds"]')
      .setValue("");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-prefix-ttl-seconds"]')
      .setValue("");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        cache_hit_rate_scale: 85,
        cache_min_block_tokens: 1024,
        cache_independent_ttl_seconds: 3600,
        cache_prefix_ttl_seconds: 3600,
      }),
    );
  });

  it("submits platform default account model config JSON", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      kiro: {
        model_whitelist: ["claude-sonnet-4-6"],
        model_mapping: {
          "claude-sonnet-4-6": "claude-sonnet-4.6",
        },
        compact_model_mapping: {
          "claude-sonnet-4-6": "claude-haiku-4.5",
        },
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        platform_default_account_model_config: {
          kiro: {
            model_whitelist: ["claude-sonnet-4-6"],
            model_mapping: {
              "claude-sonnet-4-6": "claude-sonnet-4.6",
            },
            compact_model_mapping: {
              "claude-sonnet-4-6": "claude-haiku-4.5",
            },
          },
        },
      }),
    );
  });

  it("submits kiro subscription type platform defaults edited through the form", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.platformDefaultAccountModelConfig = {
      kiro: {
        kiro_subscription_type_model_config: {
          pro: {
            model_mapping: {
              "claude-sonnet-*": "claude-sonnet-4.6",
            },
          },
        },
      },
    };
    await flushPromises();

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger("click");
    await flushPromises();

    expect(wrapper.find('[data-testid="kiro-subscription-type-config"]').exists()).toBe(true);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        platform_default_account_model_config: {
          kiro: {
            kiro_subscription_type_model_config: {
              pro: {
                model_mapping: {
                  "claude-sonnet-*": "claude-sonnet-4.6",
                },
              },
            },
          },
        },
      }),
    );
  });

  it("blocks malformed kiro subscription JSON edited through the platform defaults form", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.platformDefaultAccountModelConfig = {
      kiro: {
        kiro_subscription_type_model_config: {
          pro: {
            model_mapping: {
              "claude-sonnet-*": "claude-sonnet-4.6",
            },
          },
        },
      },
    };
    await flushPromises();

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger("click");
    await flushPromises();

    await wrapper.get('[data-testid="kiro-subscription-type-config"]').setValue("{");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "kiro.kiro_subscription_type_model_config 必须是对象。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("keeps the previous kiro subscription defaults when JSON editing is malformed", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    const previousConfig = {
      kiro: {
        kiro_subscription_type_model_config: {
          pro: {
            model_mapping: {
              "claude-sonnet-*": "claude-sonnet-4.6",
            },
          },
        },
      },
    };
    setupState.platformDefaultAccountModelConfig = previousConfig;
    await flushPromises();

    await wrapper.get('[data-testid="platform-default-tab-kiro"]').trigger("click");
    await flushPromises();

    await wrapper.get('[data-testid="kiro-subscription-type-config"]').setValue("{");
    await flushPromises();

    expect(setupState.platformDefaultAccountModelConfig).toEqual(previousConfig);
    expect(wrapper.find('[data-testid="kiro-subscription-type-config"]').exists()).toBe(true);
  });

  it("blocks malformed platform default account model config shape", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      kiro: {
        model_mapping: {
          "claude-sonnet-4-6": 42,
        },
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "kiro.model_mapping 只能包含非空字符串到非空字符串的映射。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks invalid platform default account model mapping wildcards", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      kiro: {
        model_mapping: {
          "claude-*sonnet": "claude-*",
        },
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "kiro.model_mapping 的请求模型通配符 * 只能位于末尾。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks invalid platform default temp-unsched and custom error codes", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      openai: {
        temp_unschedulable_rules: [
          {
            error_code: 99,
            duration_minutes: 10,
          },
        ],
        custom_error_codes: [600],
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "openai.temp_unschedulable_rules 的 error_code 必须在 100-599 之间。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks fractional platform default temp-unsched error code before save", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      openai: {
        temp_unschedulable_rules: [
          {
            error_code: 524.5,
            duration_minutes: 10,
            keywords: [],
          },
        ],
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "openai.temp_unschedulable_rules 的 error_code 必须在 100-599 之间。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks fractional platform default temp-unsched duration before save", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      openai: {
        temp_unschedulable_rules: [
          {
            error_code: 524,
            duration_minutes: 10.5,
            keywords: [],
          },
        ],
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "openai.temp_unschedulable_rules 的 duration_minutes 必须是大于 0 的整数。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks unsupported kiro subscription type non-model defaults before save", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    (wrapper.vm as any).platformDefaultAccountModelConfig = {
      kiro: {
        kiro_subscription_type_model_config: {
          pro: {
            temp_unschedulable_enabled: true,
            temp_unschedulable_rules: [
              {
                error_code: 502,
                keywords: ["Upstream request failed"],
                duration_minutes: 10,
                description: "upstream failed",
              },
            ],
          },
        },
      },
    };
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "kiro.kiro_subscription_type_model_config.pro 仅支持模型白名单和模型映射配置。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("refreshes platform model config editors from update response", async () => {
    updateSettings.mockImplementationOnce(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
      platform_default_account_model_config: {
        kiro: {
          model_mapping: {
            "claude-old": "claude-normalized",
          },
        },
      },
    }));
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.platformDefaultAccountModelConfig = {
      kiro: {
        model_mapping: {
          "claude-old": "claude-raw",
        },
      },
    };

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(setupState.platformDefaultAccountModelConfig).toEqual({
      kiro: {
        model_mapping: {
          "claude-old": "claude-normalized",
        },
      },
    });
  });

  it("does not refresh platform default model config editor when updateSettings reports business failure", async () => {
    updateSettings.mockImplementationOnce(async () => ({
      success: false,
      message: "业务保存失败",
    }));
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.platformDefaultAccountModelConfig = {
      kiro: {
        model_mapping: {
          "claude-old": "claude-raw",
        },
      },
    };

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith("业务保存失败");
    expect(showSuccess).not.toHaveBeenCalledWith("admin.settings.settingsSaved");
    expect(setupState.platformDefaultAccountModelConfig).toEqual({
      kiro: {
        model_mapping: {
          "claude-old": "claude-raw",
        },
      },
    });
  });

  it("blocks save when kiro prefix ttl exceeds independent ttl", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    await wrapper
      .get('[data-testid="kiro-runtime-cache-independent-ttl-seconds"]')
      .setValue("300");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-prefix-ttl-seconds"]')
      .setValue("301");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "前缀缓存 TTL 不能大于独立缓存 TTL。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("validates cleared kiro prefix ttl against its default reset value", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    await wrapper
      .get('[data-testid="kiro-runtime-cache-independent-ttl-seconds"]')
      .setValue("120");
    await wrapper
      .get('[data-testid="kiro-runtime-cache-prefix-ttl-seconds"]')
      .setValue("");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "前缀缓存 TTL 不能大于独立缓存 TTL。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("blocks save when kiro cache min block tokens exceeds the contract max", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    await wrapper
      .get('[data-testid="kiro-runtime-cache-min-block-tokens"]')
      .setValue(String(KIRO_CACHE_MIN_BLOCK_TOKENS_MAX + 1));
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      `缓存最小块 Token 数必须在 0-${KIRO_CACHE_MIN_BLOCK_TOKENS_MAX} 之间。`,
    );
    expect(updateSettings).not.toHaveBeenCalled();
  });

  it("renders DingTalk auth-source bonus controls from the settings contract", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      auth_source_default_dingtalk_balance: 8,
      auth_source_default_dingtalk_grant_on_signup: true,
      auth_source_default_dingtalk_grant_on_first_bind: true,
    });

    const wrapper = mountView();

    await flushPromises();
    await openUsersTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="auth-source-dingtalk-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(true);
    expect(
      (
        wrapper.get('[data-testid="auth-source-dingtalk-first-bind-enabled"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(true);
    expect(
      wrapper.find('[data-testid="auth-source-dingtalk-panel"]').exists(),
    ).toBe(true);
  });
});

describe("admin SettingsView DingTalk and email template surfaces", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getWebSearchEmulationConfig.mockReset();
    updateWebSearchEmulationConfig.mockReset();
    getAdminApiKey.mockReset();
    getOverloadCooldownSettings.mockReset();
    getRateLimit429CooldownSettings.mockReset();
    updateRateLimit429CooldownSettings.mockReset();
    getStreamTimeoutSettings.mockReset();
    getTempUnschedThresholdSettings.mockReset();
    updateTempUnschedThresholdSettings.mockReset();
    getRectifierSettings.mockReset();
    getBetaPolicySettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    getProviders.mockReset();
    updateProvider.mockReset();
    createProvider.mockReset();
    deleteProvider.mockReset();
    fetchPublicSettings.mockReset();
    adminSettingsFetch.mockReset();
    showError.mockReset();
    showSuccess.mockReset();
    localeRef.value = "zh-CN";

    getSettings.mockResolvedValue({ ...baseSettingsResponse });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
    }));
    getWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    updateWebSearchEmulationConfig.mockResolvedValue({
      enabled: false,
      providers: [],
    });
    getAdminApiKey.mockResolvedValue({
      exists: false,
      masked_key: "",
    });
    getOverloadCooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_minutes: 10,
    });
    getRateLimit429CooldownSettings.mockResolvedValue({
      enabled: true,
      cooldown_seconds: 5,
    });
    updateRateLimit429CooldownSettings.mockImplementation(async (payload) => payload);
    getStreamTimeoutSettings.mockResolvedValue({
      enabled: true,
      action: "temp_unsched",
      temp_unsched_minutes: 5,
      threshold_count: 3,
      threshold_window_minutes: 10,
    });
    getTempUnschedThresholdSettings.mockResolvedValue({
      enabled: true,
      threshold_count: 3,
      threshold_window_minutes: 1,
    });
    updateTempUnschedThresholdSettings.mockImplementation(async (payload) => payload);
    getRectifierSettings.mockResolvedValue({
      enabled: true,
      thinking_signature_enabled: true,
      thinking_budget_enabled: true,
      apikey_signature_enabled: false,
      apikey_signature_patterns: [],
    });
    getBetaPolicySettings.mockResolvedValue({
      rules: [],
    });
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({
      items: [],
    });
    getProviders.mockResolvedValue({
      data: [],
    });
    fetchPublicSettings.mockResolvedValue(undefined);
    adminSettingsFetch.mockResolvedValue(undefined);
  });

  it("keeps the email template editor reachable from the email tab", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openEmailTab(wrapper);

    expect(wrapper.find('[data-testid="email-template-editor-stub"]').exists()).toBe(
      true,
    );
  });

  it("loads DingTalk connect fields from the backend payload", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      dingtalk_connect_enabled: true,
      dingtalk_connect_client_id: "ding-client-id-123",
      dingtalk_connect_client_secret_configured: true,
      dingtalk_connect_redirect_url:
        "https://admin.example.com/api/v1/auth/oauth/dingtalk/callback",
      dingtalk_connect_corp_restriction_policy: "internal_only",
      dingtalk_connect_internal_corp_id: "dingcorp123456",
      dingtalk_connect_bypass_registration: true,
      dingtalk_connect_sync_corp_email: true,
      dingtalk_connect_sync_display_name: true,
      dingtalk_connect_sync_dept: true,
      dingtalk_connect_sync_corp_email_attr_key: "dingtalk_email",
      dingtalk_connect_sync_display_name_attr_key: "dingtalk_name",
      dingtalk_connect_sync_dept_attr_key: "dingtalk_department",
      dingtalk_connect_sync_corp_email_attr_name: "企业邮箱",
      dingtalk_connect_sync_display_name_attr_name: "钉钉昵称",
      dingtalk_connect_sync_dept_attr_name: "所在部门",
    });

    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    expect(
      (
        wrapper.get('[data-testid="dingtalk-connect-client-id"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("ding-client-id-123");
    expect(
      wrapper
        .get('[data-testid="dingtalk-connect-client-secret"]')
        .attributes("placeholder"),
    ).toContain("密钥已配置");
    expect(
      (
        wrapper.get('[data-testid="dingtalk-connect-corp-restriction-policy"]')
          .element as HTMLSelectElement
      ).value,
    ).toBe("internal_only");
    expect(
      (
        wrapper.get('[data-testid="dingtalk-connect-sync-display-name"]')
          .element as HTMLInputElement
      ).checked,
    ).toBe(true);
    expect(
      (
        wrapper.get('[data-testid="dingtalk-connect-sync-dept-attr-key"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("dingtalk_department");
  });

  it("saves DingTalk connect fields and clears the secret after save", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    await wrapper
      .get('[data-testid="dingtalk-connect-enabled"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="dingtalk-connect-client-id"]')
      .setValue("ding-client-id-updated");
    await wrapper
      .get('[data-testid="dingtalk-connect-client-secret"]')
      .setValue("ding-secret-updated");
    await wrapper
      .get('[data-testid="dingtalk-connect-redirect-url"]')
      .setValue("https://admin.example.com/api/v1/auth/oauth/dingtalk/callback");
    await wrapper
      .get('[data-testid="dingtalk-connect-corp-restriction-policy"]')
      .setValue("internal_only");
    await wrapper
      .get('[data-testid="dingtalk-connect-internal-corp-id"]')
      .setValue("dingcorp-updated");
    await wrapper
      .get('[data-testid="dingtalk-connect-bypass-registration"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-corp-email"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-display-name"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-dept"]')
      .setValue(true);
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-corp-email-attr-key"]')
      .setValue("corp_email");
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-display-name-attr-key"]')
      .setValue("display_name");
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-dept-attr-key"]')
      .setValue("department");
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-corp-email-attr-name"]')
      .setValue("企业邮箱");
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-display-name-attr-name"]')
      .setValue("展示名称");
    await wrapper
      .get('[data-testid="dingtalk-connect-sync-dept-attr-name"]')
      .setValue("部门名称");
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalledTimes(1);
    expect(updateSettings).toHaveBeenCalledWith(
      expect.objectContaining({
        dingtalk_connect_enabled: true,
        dingtalk_connect_client_id: "ding-client-id-updated",
        dingtalk_connect_client_secret: "ding-secret-updated",
        dingtalk_connect_redirect_url:
          "https://admin.example.com/api/v1/auth/oauth/dingtalk/callback",
        dingtalk_connect_corp_restriction_policy: "internal_only",
        dingtalk_connect_internal_corp_id: "dingcorp-updated",
        dingtalk_connect_bypass_registration: true,
        dingtalk_connect_sync_corp_email: true,
        dingtalk_connect_sync_display_name: true,
        dingtalk_connect_sync_dept: true,
        dingtalk_connect_sync_corp_email_attr_key: "corp_email",
        dingtalk_connect_sync_display_name_attr_key: "display_name",
        dingtalk_connect_sync_dept_attr_key: "department",
        dingtalk_connect_sync_corp_email_attr_name: "企业邮箱",
        dingtalk_connect_sync_display_name_attr_name: "展示名称",
        dingtalk_connect_sync_dept_attr_name: "部门名称",
      }),
    );
    expect(
      (
        wrapper.get('[data-testid="dingtalk-connect-client-secret"]')
          .element as HTMLInputElement
      ).value,
    ).toBe("");
  });
});

describe("admin SettingsView platform quota matrix", () => {
  beforeEach(() => {
    getSettings.mockReset();
    updateSettings.mockReset();
    getWebSearchEmulationConfig.mockReset();
    updateWebSearchEmulationConfig.mockReset();
    getAdminApiKey.mockReset();
    getOverloadCooldownSettings.mockReset();
    getRateLimit429CooldownSettings.mockReset();
    updateRateLimit429CooldownSettings.mockReset();
    getStreamTimeoutSettings.mockReset();
    getTempUnschedThresholdSettings.mockReset();
    updateTempUnschedThresholdSettings.mockReset();
    getRectifierSettings.mockReset();
    getBetaPolicySettings.mockReset();
    getGroups.mockReset();
    listProxies.mockReset();
    getProviders.mockReset();
    updateProvider.mockReset();
    createProvider.mockReset();
    deleteProvider.mockReset();
    fetchPublicSettings.mockReset();
    adminSettingsFetch.mockReset();
    showError.mockReset();
    showSuccess.mockReset();
    localeRef.value = "zh-CN";

    getSettings.mockResolvedValue({ ...baseSettingsResponse });
    updateSettings.mockImplementation(async (payload) => ({
      ...baseSettingsResponse,
      ...payload,
    }));
    getWebSearchEmulationConfig.mockResolvedValue({ enabled: false, providers: [] });
    updateWebSearchEmulationConfig.mockResolvedValue({ enabled: false, providers: [] });
    getAdminApiKey.mockResolvedValue({ exists: false, masked_key: "" });
    getOverloadCooldownSettings.mockResolvedValue({});
    getRateLimit429CooldownSettings.mockResolvedValue({});
    updateRateLimit429CooldownSettings.mockResolvedValue({});
    getStreamTimeoutSettings.mockResolvedValue({});
    getTempUnschedThresholdSettings.mockResolvedValue({});
    updateTempUnschedThresholdSettings.mockResolvedValue({});
    getRectifierSettings.mockResolvedValue({});
    getBetaPolicySettings.mockResolvedValue({});
    getGroups.mockResolvedValue([]);
    listProxies.mockResolvedValue({ items: [] });
    getProviders.mockResolvedValue({ data: [] });
  });

  it("从 baseSettings 加载默认平台配额数据并在 Users tab 渲染 6 平台行", async () => {
    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    expect(getSettings).toHaveBeenCalled();

    const html = wrapper.html();
    // 表格行的平台字段：font-mono 渲染纯英文 platform key
    expect(html).toContain("anthropic");
    expect(html).toContain("openai");
    expect(html).toContain("gemini");
    expect(html).toContain("antigravity");
    expect(html).toContain("kiro");
    expect(html).toContain("grok");
  });

  it("保存时 updateSettings payload 应包含嵌套 default_platform_quotas 对象（含全 6 平台）", async () => {
    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalled();
    const lastCallArgs = updateSettings.mock.calls.at(-1);
    expect(lastCallArgs).toBeDefined();
    const payload = lastCallArgs![0] as Record<string, unknown>;

    // 应携带嵌套对象，而非扁平字段
    expect(payload).toHaveProperty("default_platform_quotas");
    const quotas = payload["default_platform_quotas"] as Record<string, unknown>;
    const platforms = ["anthropic", "openai", "gemini", "antigravity", "kiro", "grok"];
    for (const p of platforms) {
      expect(quotas).toHaveProperty(p);
      const pq = quotas[p] as Record<string, unknown>;
      expect(pq).toHaveProperty("daily");
      expect(pq).toHaveProperty("weekly");
      expect(pq).toHaveProperty("monthly");
    }

    // 不应存在旧扁平字段
    expect(payload).not.toHaveProperty("default_platform_quota_anthropic_daily");
    expect(payload).not.toHaveProperty("default_platform_quota_openai_weekly");
  });

  it("加载后 form.default_platform_quotas 含全 6 平台，从嵌套 JSON 正确读取数值", async () => {
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      default_platform_quotas: {
        anthropic: { daily: 5, weekly: null, monthly: null },
        openai:    { daily: null, weekly: 12.5, monthly: null },
        // gemini / antigravity 缺失 → 应被归一化为全 null
      },
    });

    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    const payload = updateSettings.mock.calls.at(-1)![0] as Record<string, unknown>;
    const quotas = payload["default_platform_quotas"] as Record<string, Record<string, unknown>>;

    expect(quotas["anthropic"]?.["daily"]).toBe(5);
    expect(quotas["openai"]?.["weekly"]).toBe(12.5);
    // 缺失平台应补全为 null
    expect(quotas["gemini"]).toEqual({ daily: null, weekly: null, monthly: null });
    expect(quotas["antigravity"]).toEqual({ daily: null, weekly: null, monthly: null });
    expect(quotas["kiro"]).toEqual({ daily: null, weekly: null, monthly: null });
    expect(quotas["grok"]).toEqual({ daily: null, weekly: null, monthly: null });
  });

  it("空输入（v-model.number 产出 \"\"）在提交时清洗为 null 而非空字符串", async () => {
    // 模拟后端返回带有 anthropic daily 值的配额
    getSettings.mockResolvedValueOnce({
      ...baseSettingsResponse,
      default_platform_quotas: {
        anthropic: { daily: 10, weekly: null, monthly: null },
        openai:    { daily: null, weekly: null, monthly: null },
        gemini:    { daily: null, weekly: null, monthly: null },
        antigravity: { daily: null, weekly: null, monthly: null },
      },
    });

    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    // 找到 anthropic daily 输入框并清空（模拟用户删除值）
    const inputs = wrapper.findAll('input[type="number"]');
    const anthropicDailyInput = inputs.find((i) => {
      const parent = i.element.closest("tr");
      return parent?.textContent?.includes("anthropic");
    });

    if (anthropicDailyInput) {
      // 设置为空字符串，模拟 v-model.number 在清空时产出 ""
      await anthropicDailyInput.setValue("");
    }

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    const payload = updateSettings.mock.calls.at(-1)![0] as Record<string, unknown>;
    const quotas = payload["default_platform_quotas"] as Record<string, Record<string, unknown>>;
    // 不管输入是什么，提交值应为 null（而非 "" 或 NaN）
    expect(quotas["anthropic"]?.["daily"]).toBe(null);
  });

  it("默认平台限额存在非法中间态时阻止保存，而不是静默写成 null", async () => {
    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.form.default_platform_quotas.anthropic.daily = Number.NaN;

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalled();
  });

  it("来源附加授权的平台限额存在非法中间态时阻止保存，而不是静默写成 null", async () => {
    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.authSourceDefaults.email.platform_quotas.openai.weekly = "-" as unknown as number;

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).not.toHaveBeenCalled();
    expect(showError).toHaveBeenCalled();
  });

  it("来源附加授权平台限额留空时按继承处理，不写 override；显式 null 保留为 null", async () => {
    const wrapper = mountView();
    await flushPromises();
    await openUsersTab(wrapper);

    const setupState = (wrapper.vm as any).$?.setupState ?? wrapper.vm;
    setupState.authSourceDefaults.email.platform_quotas.anthropic.daily = "";
    setupState.authSourceDefaults.email.platform_quotas.openai.weekly = null;
    setupState.authSourceDefaults.email.platform_quotas.gemini.monthly = 25;

    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(updateSettings).toHaveBeenCalled();
    const payload = updateSettings.mock.calls.at(-1)![0] as Record<string, any>;
    expect(payload.auth_source_default_email_platform_quotas).toEqual({
      openai: { weekly: null },
      gemini: { monthly: 25 },
    });
    expect(payload.auth_source_default_email_platform_quotas).not.toHaveProperty("anthropic");
  });
});
