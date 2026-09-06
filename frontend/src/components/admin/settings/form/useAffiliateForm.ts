// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：邀请返利（Affiliate）后台用户
// 列表状态、二次确认弹窗与加载函数。逐字保留原实现；t/appStore 由调用方传入以复用同一实例。
import { reactive } from "vue";
import type { useI18n } from "vue-i18n";
import type { useAppStore } from "@/stores";
import { affiliatesAPI } from "@/api/admin/affiliates";
import { extractApiErrorMessage } from "@/utils/apiError";
import type { AffiliateState } from "../useSettingsForm";

export function useAffiliateForm(
  t: ReturnType<typeof useI18n>["t"],
  appStore: ReturnType<typeof useAppStore>,
) {
  const affiliateState = reactive<AffiliateState>({
    loading: false,
    entries: [],
    total: 0,
    page: 1,
    pageSize: 20,
    search: "",
    selected: [],
    searchTimer: null,
  });

  const affiliateConfirmDialog = reactive<{
    show: boolean;
    title: string;
    message: string;
    confirmText: string;
    pending: (() => Promise<unknown>) | null;
  }>({
    show: false,
    title: "",
    message: "",
    confirmText: "",
    pending: null,
  });

  async function handleAffiliateConfirm() {
    const fn = affiliateConfirmDialog.pending;
    affiliateConfirmDialog.show = false;
    affiliateConfirmDialog.pending = null;
    if (!fn) return;
    try {
      await fn();
      appStore.showSuccess(t("common.saved"));
      await loadAffiliateUsers();
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    }
  }

  function cancelAffiliateConfirm() {
    affiliateConfirmDialog.show = false;
    affiliateConfirmDialog.pending = null;
  }

  async function loadAffiliateUsers() {
    affiliateState.loading = true;
    try {
      const res = await affiliatesAPI.listUsers({
        page: affiliateState.page,
        page_size: affiliateState.pageSize,
        search: affiliateState.search,
      });
      affiliateState.entries = res.items ?? [];
      affiliateState.total = res.total ?? 0;
      // Drop selections that are no longer visible.
      const visibleIds = new Set(affiliateState.entries.map((e) => e.user_id));
      affiliateState.selected = affiliateState.selected.filter((id) => visibleIds.has(id));
    } catch (err) {
      appStore.showError(extractApiErrorMessage(err, t("common.error")));
    } finally {
      affiliateState.loading = false;
    }
  }

  return {
    affiliateState,
    affiliateConfirmDialog,
    handleAffiliateConfirm,
    cancelAffiliateConfirm,
    loadAffiliateUsers,
  };
}
