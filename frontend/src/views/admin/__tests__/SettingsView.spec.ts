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
    "admin.settings.kiroRuntime.thinkingTitle": "Thinking 兼容",
    "admin.settings.kiroRuntime.thinkingDescription": "配置 Thinking 兼容模式和阈值。",
    "admin.settings.kiroRuntime.thinkingMode": "Thinking 模式",
    "admin.settings.kiroRuntime.thinkingModeHint": "控制请求的 Thinking 兼容行为。",
    "admin.settings.kiroRuntime.thinkingEffortThreshold": "Thinking 阈值",
    "admin.settings.kiroRuntime.thinkingEffortThresholdHint": "达到阈值后启用兼容逻辑。",
    "admin.settings.kiroRuntime.thinkingSimulationTemplate": "Thinking 模板",
    "admin.settings.kiroRuntime.thinkingSimulationTemplatePlaceholder": "模拟模板",
    "admin.settings.kiroRuntime.thinkingSimulationTemplateHint": "用于模拟 Thinking 内容。",
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
  enable_fingerprint_unification: true,
  enable_metadata_passthrough: false,
  enable_cch_signing: false,
  enable_anthropic_cache_ttl_1h_injection: false,
  rewrite_message_cache_control: false,
  antigravity_user_agent_version: "",
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
  openai_oauth_image_bridge_disable_keepalives: false,
  openai_oauth_image_bridge_fresh_upstream_client: false,
  balance_low_notify_enabled: false,
  balance_low_notify_threshold: 0,
  balance_low_notify_recharge_url: "",
  account_quota_notify_enabled: false,
  account_quota_notify_emails: [],
  kiro_version: "0.10.0",
  kiro_commit: "",
  system_version: "darwin#24.6.0",
  node_version: "22.21.1",
  cache_hit_rate_scale: 95,
  cache_min_block_tokens: 1024,
  cache_independent_ttl_seconds: 3600,
  cache_prefix_ttl_seconds: 300,
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
        BackupSettings: true,
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

  it("keeps thinking compatibility controls only in the gateway tab", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openSecurityTab(wrapper);

    expect(wrapper.get('[data-testid="security-settings-panel"]').text()).not.toContain(
      "Thinking 兼容",
    );

    await openGatewayTab(wrapper);

    expect(wrapper.text()).toContain("Thinking 兼容");
    expect(wrapper.findAll('[data-testid="kiro-runtime-thinking-mode"]')).toHaveLength(1);
    expect(wrapper.findAll('[data-testid="kiro-runtime-thinking-template"]')).toHaveLength(1);
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
    expect(gatewayPanel.text()).toContain("Thinking 兼容");
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
    ).toBe("95");

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
        cache_hit_rate_scale: 95,
        cache_min_block_tokens: 1024,
        cache_independent_ttl_seconds: 3600,
        cache_prefix_ttl_seconds: 300,
      }),
    );
  });

  it("submits platform default account model config JSON", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const configTextarea = wrapper
      .findAll("textarea")
      .find((node) =>
        (node.element as HTMLTextAreaElement).placeholder.includes(
          "model_mapping",
        ),
      );
    expect(configTextarea).toBeDefined();
    await configTextarea?.setValue(
      JSON.stringify({
        kiro: {
          model_whitelist: ["claude-sonnet-4-6"],
          model_mapping: {
            "claude-sonnet-4-6": "claude-sonnet-4.6",
          },
          compact_model_mapping: {
            "claude-sonnet-4-6": "claude-haiku-4.5",
          },
        },
      }),
    );
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

  it("blocks malformed platform default account model config JSON shape", async () => {
    const wrapper = mountView();

    await flushPromises();
    await openGatewayTab(wrapper);

    const configTextarea = wrapper
      .findAll("textarea")
      .find((node) =>
        (node.element as HTMLTextAreaElement).placeholder.includes(
          "model_mapping",
        ),
      );
    expect(configTextarea).toBeDefined();
    await configTextarea?.setValue(
      JSON.stringify({
        kiro: {
          model_mapping: {
            "claude-sonnet-4-6": 42,
          },
        },
      }),
    );
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

    const configTextarea = wrapper
      .findAll("textarea")
      .find((node) =>
        (node.element as HTMLTextAreaElement).placeholder.includes(
          "model_mapping",
        ),
      );
    expect(configTextarea).toBeDefined();
    await configTextarea?.setValue(
      JSON.stringify({
        kiro: {
          model_mapping: {
            "claude-*sonnet": "claude-*",
          },
        },
      }),
    );
    await wrapper.find("form").trigger("submit.prevent");
    await flushPromises();

    expect(showError).toHaveBeenCalledWith(
      "kiro.model_mapping 的请求模型通配符 * 只能位于末尾。",
    );
    expect(updateSettings).not.toHaveBeenCalled();
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
});
