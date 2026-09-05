// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：支付渠道（Provider）管理——
// 列表加载/新建/编辑/删除、启用互斥校验（支付宝/微信支付同一方式不能被多个渠道同时声明）。
// 逐字保留原实现；t/appStore/form/saveSettings 由调用方传入以复用同一实例。
import { ref, computed } from "vue";
import type { useI18n } from "vue-i18n";
import type { useAppStore } from "@/stores";
import { adminAPI } from "@/api";
import { extractI18nErrorMessage } from "@/utils/apiError";
import { normalizeVisibleMethod } from "@/components/payment/paymentFlow";
import PaymentProviderDialog from "@/components/payment/PaymentProviderDialog.vue";
import type { ProviderInstance } from "@/types/payment";
import type { SettingsForm, ProviderEnablementCandidate } from "../useSettingsForm";

export function usePaymentProvidersForm(
  t: ReturnType<typeof useI18n>["t"],
  appStore: ReturnType<typeof useAppStore>,
  form: SettingsForm,
  saveSettings: () => Promise<void>,
) {
  const allPaymentTypes = computed(() => [
    { value: "easypay", label: t("payment.methods.easypay") },
    { value: "alipay", label: t("payment.methods.alipay") },
    { value: "wxpay", label: t("payment.methods.wxpay") },
    { value: "stripe", label: t("payment.methods.stripe") },
    { value: "airwallex", label: t("payment.methods.airwallex") },
  ]);

  const providersLoading = ref(false);

  const providerSaving = ref(false);

  const providers = ref<ProviderInstance[]>([]);

  const showProviderDialog = ref(false);

  const showDeleteProviderDialog = ref(false);

  const editingProvider = ref<ProviderInstance | null>(null);

  const deletingProviderId = ref<number | null>(null);

  const providerDialogRef = ref<InstanceType<
    typeof PaymentProviderDialog
  > | null>(null);

  const providerKeyOptions = computed(() => [
    { value: "easypay", label: t("admin.settings.payment.providerEasypay") },
    { value: "alipay", label: t("admin.settings.payment.providerAlipay") },
    { value: "wxpay", label: t("admin.settings.payment.providerWxpay") },
    { value: "stripe", label: t("admin.settings.payment.providerStripe") },
    { value: "airwallex", label: t("admin.settings.payment.providerAirwallex") },
  ]);

  const enabledProviderKeyOptions = computed(() => {
    const enabled = form.payment_enabled_types;
    return providerKeyOptions.value.filter((opt) => enabled.includes(opt.value));
  });

  function getProviderVisibleMethods(
    provider: ProviderEnablementCandidate,
  ): Array<"alipay" | "wxpay"> {
    if (!provider.enabled) {
      return [];
    }

    const supportedTypes = Array.isArray(provider.supported_types)
      ? provider.supported_types
      : [];
    const methods = new Set<"alipay" | "wxpay">();
    const addMethod = (type: string) => {
      const method = normalizeVisibleMethod(type);
      if (method === "alipay" || method === "wxpay") {
        methods.add(method);
      }
    };

    if (provider.provider_key === "alipay") {
      if (supportedTypes.length === 0) {
        methods.add("alipay");
      } else {
        supportedTypes.forEach((type) => {
          if (normalizeVisibleMethod(type) === "alipay") {
            methods.add("alipay");
          }
        });
      }
    } else if (provider.provider_key === "wxpay") {
      if (supportedTypes.length === 0) {
        methods.add("wxpay");
      } else {
        supportedTypes.forEach((type) => {
          if (normalizeVisibleMethod(type) === "wxpay") {
            methods.add("wxpay");
          }
        });
      }
    } else if (provider.provider_key === "easypay") {
      supportedTypes.forEach(addMethod);
    }

    return Array.from(methods);
  }

  function findProviderEnablementConflict(
    candidate: ProviderEnablementCandidate,
  ): { method: "alipay" | "wxpay"; conflicting: ProviderInstance } | null {
    const claimedMethods = getProviderVisibleMethods(candidate);
    if (claimedMethods.length === 0) {
      return null;
    }

    for (const other of providers.value) {
      if (other.id === candidate.id || !other.enabled) {
        continue;
      }

      const otherMethods = getProviderVisibleMethods(other);
      const matchedMethod = claimedMethods.find((method) =>
        otherMethods.includes(method),
      );
      if (matchedMethod) {
        return {
          method: matchedMethod,
          conflicting: other,
        };
      }
    }

    return null;
  }

  function showProviderEnablementConflict(
    conflict: { method: "alipay" | "wxpay"; conflicting: ProviderInstance },
  ) {
    appStore.showError(
      t("admin.settings.payment.enableConflict", {
        method: t(`payment.methods.${conflict.method}`),
        provider: conflict.conflicting.name,
      }),
    );
  }

  async function loadProviders() {
    providersLoading.value = true;
    try {
      const res = await adminAPI.payment.getProviders();
      // Normalize supported_types: backend returns null when the list is empty
      // (Go nil slice → JSON null). Without this, ProviderCard's isSelected()
      // throws TypeError on null.includes(), causing the card to vanish.
      providers.value = (res.data || []).map((p) => ({
        ...p,
        supported_types: Array.isArray(p.supported_types)
          ? p.supported_types
          : [],
      }));
    } catch (err: unknown) {
      appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
    } finally {
      providersLoading.value = false;
    }
  }

  async function handleSaveProvider(payload: Partial<ProviderInstance>) {
    providerSaving.value = true;
    try {
      const candidate: ProviderEnablementCandidate = {
        id: editingProvider.value?.id ?? 0,
        provider_key:
          payload.provider_key ?? editingProvider.value?.provider_key ?? "",
        supported_types:
          payload.supported_types ?? editingProvider.value?.supported_types ?? [],
        enabled: payload.enabled ?? editingProvider.value?.enabled ?? false,
        name: payload.name ?? editingProvider.value?.name ?? "",
      };
      const conflict = findProviderEnablementConflict(candidate);
      if (conflict) {
        showProviderEnablementConflict(conflict);
        return;
      }

      if (editingProvider.value) {
        await adminAPI.payment.updateProvider(editingProvider.value.id, payload);
      } else {
        await adminAPI.payment.createProvider(payload);
      }
      showProviderDialog.value = false;
      // Reload full list (API returns decrypted/formatted data with correct sort order)
      await loadProviders();
      // Auto-save settings so provider changes take effect immediately
      await saveSettings();
    } catch (err: unknown) {
      appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
    } finally {
      providerSaving.value = false;
    }
  }

  async function handleDeleteProvider() {
    if (!deletingProviderId.value) return;
    try {
      await adminAPI.payment.deleteProvider(deletingProviderId.value);
      appStore.showSuccess(t("common.deleted"));
      showDeleteProviderDialog.value = false;
      loadProviders();
    } catch (err: unknown) {
      appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
    }
  }


  return {
    allPaymentTypes,
    providersLoading,
    providerSaving,
    providers,
    showProviderDialog,
    showDeleteProviderDialog,
    editingProvider,
    deletingProviderId,
    providerDialogRef,
    providerKeyOptions,
    enabledProviderKeyOptions,
    getProviderVisibleMethods,
    findProviderEnablementConflict,
    showProviderEnablementConflict,
    loadProviders,
    handleSaveProvider,
    handleDeleteProvider,
  };
}
