// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：管理员 API Key 状态与加载函数。
// 逐字保留原实现。
import { ref } from "vue";
import { adminAPI } from "@/api";

export function useAdminApiKeyState() {
  const adminApiKeyLoading = ref(true);

  const adminApiKeyExists = ref(false);

  const adminApiKeyMasked = ref("");

  async function loadAdminApiKey() {
    adminApiKeyLoading.value = true;
    try {
      const status = await adminAPI.settings.getAdminApiKey();
      adminApiKeyExists.value = status.exists;
      adminApiKeyMasked.value = status.masked_key;
    } catch (_error: unknown) {
      // Silent fail - admin API key status is non-critical
    } finally {
      adminApiKeyLoading.value = false;
    }
  }

  return {
    adminApiKeyLoading,
    adminApiKeyExists,
    adminApiKeyMasked,
    loadAdminApiKey,
  };
}
