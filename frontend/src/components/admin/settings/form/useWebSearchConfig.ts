// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：网关搜索模拟（Web Search
// Emulation）配置的加载与保存。逐字保留原实现，t/appStore 由调用方传入以复用同一实例。
import { ref, reactive } from "vue";
import { adminAPI } from "@/api";
import { extractApiErrorMessage } from "@/utils/apiError";
import type { Proxy } from "@/types";
import type {
  WebSearchEmulationConfig,
  WebSearchProviderConfig,
} from "@/api/admin/settings";
import type { useI18n } from "vue-i18n";
import type { useAppStore } from "@/stores";

export function useWebSearchConfig(
  t: ReturnType<typeof useI18n>["t"],
  appStore: ReturnType<typeof useAppStore>,
) {
  const webSearchProxies = ref<Proxy[]>([]);

  const webSearchConfig = reactive<WebSearchEmulationConfig>({
    enabled: false,
    providers: [],
  });

  async function loadWebSearchConfig() {
    try {
      const [resp, proxiesResp] = await Promise.all([
        adminAPI.settings.getWebSearchEmulationConfig(),
        adminAPI.proxies.list().catch(() => ({ items: [] as Proxy[] })),
      ]);
      if (resp) {
        webSearchConfig.enabled = resp.enabled || false;
        webSearchConfig.providers = resp.providers || [];
      }
      webSearchProxies.value = proxiesResp.items || [];
    } catch (err: unknown) {
      // 404 is expected when config hasn't been created yet; show error for other failures
      const status = (err as { status?: number })?.status;
      if (status !== 404 && status !== undefined) {
        appStore.showError(extractApiErrorMessage(err, t("common.error")));
      }
    }
  }

  async function saveWebSearchConfig(): Promise<boolean> {
    try {
      for (const p of webSearchConfig.providers) {
        const raw = p.quota_limit;
        if (raw != null && Number(raw) !== 0 && Number(raw) < 1) {
          appStore.showError(
            t("admin.settings.webSearchEmulation.quotaLimitMustBePositive"),
          );
          return false;
        }
      }
      const providers = webSearchConfig.providers.map(
        (p: WebSearchProviderConfig) => ({
          ...p,
          quota_limit: Number(p.quota_limit) > 0 ? Number(p.quota_limit) : null,
        }),
      );
      await adminAPI.settings.updateWebSearchEmulationConfig({
        enabled: webSearchConfig.enabled,
        providers,
      });
      return true;
    } catch (err: unknown) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
      return false;
    }
  }

  return {
    webSearchProxies,
    webSearchConfig,
    loadWebSearchConfig,
    saveWebSearchConfig,
  };
}
