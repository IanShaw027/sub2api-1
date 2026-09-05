// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：网关/面板运维类子设置（上游计费探测 /
// Ollama Cloud 用量 / 过载冷却 529 / 429 冷却 / 面板限流 / 流式超时 / 签名整流 / Beta 策略）的状态
// 与独立加载函数。逐字保留原实现，各表单本身仍在各自 Section 组件的保存逻辑中读写。
import { ref, reactive } from "vue";
import { adminAPI } from "@/api";

export function useGatewayOpsForms() {
  const upstreamBillingProbeLoading = ref(true);

  const upstreamBillingProbeForm = reactive({
    enabled: true,
    interval_minutes: 30,
  });

  const ollamaCloudUsageLoading = ref(true);

  const ollamaCloudUsageForm = reactive({
    enabled: false,
    interval_minutes: 60,
    debounce_minutes: 1,
  });

  const overloadCooldownLoading = ref(true);

  const overloadCooldownForm = reactive({
    enabled: true,
    cooldown_minutes: 10,
  });

  const rateLimit429CooldownLoading = ref(true);

  const rateLimit429CooldownForm = reactive({
    enabled: true,
    cooldown_seconds: 5,
    strategy: "cooldown" as "cooldown" | "same_account_retry",
    retry_interval_ms: 500,
    retry_max_duration_seconds: 120,
    max_account_switches: 2,
  });

  const panelRateLimitLoading = ref(true);

  const panelRateLimitForm = reactive({
    enabled: true,
    user_rpm: 240,
    heavy_rpm: 60,
    exempt_admin: true,
    public_ip_rpm: 300,
  });

  const streamTimeoutLoading = ref(true);

  const streamTimeoutForm = reactive({
    enabled: true,
    action: "temp_unsched" as "temp_unsched" | "error" | "none",
    temp_unsched_minutes: 5,
    threshold_count: 3,
    threshold_window_minutes: 10,
  });

  const rectifierLoading = ref(true);

  const rectifierForm = reactive({
    enabled: true,
    thinking_signature_enabled: true,
    thinking_budget_enabled: true,
    apikey_signature_enabled: false,
    apikey_signature_patterns: [] as string[],
  });

  const betaPolicyLoading = ref(true);

  const betaPolicyForm = reactive({
    rules: [] as Array<{
      beta_token: string;
      action: "pass" | "filter" | "block";
      scope: "all" | "oauth" | "apikey" | "bedrock";
      error_message?: string;
      model_whitelist?: string[];
      fallback_action?: "pass" | "filter" | "block";
      fallback_error_message?: string;
    }>,
  });

  async function loadUpstreamBillingProbeSettings() {
    upstreamBillingProbeLoading.value = true;
    try {
      Object.assign(
        upstreamBillingProbeForm,
        await adminAPI.accounts.getUpstreamBillingProbeSettings(),
      );
    } catch (_error: unknown) {
      // Keep defaults when this optional setting cannot be loaded.
    } finally {
      upstreamBillingProbeLoading.value = false;
    }
  }

  async function loadOllamaCloudUsageSettings() {
    ollamaCloudUsageLoading.value = true;
    try {
      Object.assign(
        ollamaCloudUsageForm,
        await adminAPI.accounts.getOllamaCloudUsageSettings(),
      );
    } catch (_error: unknown) {
      // Keep the fail-safe disabled defaults when this optional setting cannot be loaded.
    } finally {
      ollamaCloudUsageLoading.value = false;
    }
  }

  async function loadOverloadCooldownSettings() {
    overloadCooldownLoading.value = true;
    try {
      const settings = await adminAPI.settings.getOverloadCooldownSettings();
      Object.assign(overloadCooldownForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      overloadCooldownLoading.value = false;
    }
  }

  async function loadPanelRateLimitSettings() {
    panelRateLimitLoading.value = true;
    try {
      const settings = await adminAPI.settings.getPanelRateLimitSettings();
      Object.assign(panelRateLimitForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      panelRateLimitLoading.value = false;
    }
  }

  async function loadRateLimit429CooldownSettings() {
    rateLimit429CooldownLoading.value = true;
    try {
      const settings = await adminAPI.settings.getRateLimit429CooldownSettings();
      Object.assign(rateLimit429CooldownForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      rateLimit429CooldownLoading.value = false;
    }
  }

  async function loadStreamTimeoutSettings() {
    streamTimeoutLoading.value = true;
    try {
      const settings = await adminAPI.settings.getStreamTimeoutSettings();
      Object.assign(streamTimeoutForm, settings);
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      streamTimeoutLoading.value = false;
    }
  }

  async function loadRectifierSettings() {
    rectifierLoading.value = true;
    try {
      const settings = await adminAPI.settings.getRectifierSettings();
      Object.assign(rectifierForm, settings);
      // 确保 patterns 是数组（旧数据可能为 null）
      if (!Array.isArray(rectifierForm.apikey_signature_patterns)) {
        rectifierForm.apikey_signature_patterns = [];
      }
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      rectifierLoading.value = false;
    }
  }

  async function loadBetaPolicySettings() {
    betaPolicyLoading.value = true;
    try {
      const settings = await adminAPI.settings.getBetaPolicySettings();
      betaPolicyForm.rules = settings.rules;
    } catch (_error: unknown) {
      // Silent fail - settings will use defaults
    } finally {
      betaPolicyLoading.value = false;
    }
  }


  return {
    upstreamBillingProbeLoading,
    upstreamBillingProbeForm,
    ollamaCloudUsageLoading,
    ollamaCloudUsageForm,
    overloadCooldownLoading,
    overloadCooldownForm,
    rateLimit429CooldownLoading,
    rateLimit429CooldownForm,
    panelRateLimitLoading,
    panelRateLimitForm,
    streamTimeoutLoading,
    streamTimeoutForm,
    rectifierLoading,
    rectifierForm,
    betaPolicyLoading,
    betaPolicyForm,
    loadUpstreamBillingProbeSettings,
    loadOllamaCloudUsageSettings,
    loadOverloadCooldownSettings,
    loadPanelRateLimitSettings,
    loadRateLimit429CooldownSettings,
    loadStreamTimeoutSettings,
    loadRectifierSettings,
    loadBetaPolicySettings,
  };
}
