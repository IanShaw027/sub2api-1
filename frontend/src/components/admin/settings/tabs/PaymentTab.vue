<template>
  <div v-show="activeTab === 'payment'" class="settings-stack">
    <!-- Payment System Settings -->
    <SettingsSection>
      <template #header>
        <h2 class="settings-card-title">
          {{ t("admin.settings.payment.title") }}
        </h2>
        <p class="settings-card-desc">
          {{ t("admin.settings.payment.description") }}
          <a
            :href="paymentGuideHref"
            target="_blank"
            rel="noopener noreferrer"
            class="ml-2 inline-flex items-center text-accent hover:text-accent"
          >
            <svg
              class="mr-0.5 h-3.5 w-3.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
              />
            </svg>
            {{ t("admin.settings.payment.configGuide") }}
          </a>
          <router-link
            :to="{ name: 'AdminPaymentPlans' }"
            class="ml-3 inline-flex items-center text-accent hover:text-accent"
          >
            <svg
              class="mr-0.5 h-3.5 w-3.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                stroke-linecap="round"
                stroke-linejoin="round"
                stroke-width="2"
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2v-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"
              />
            </svg>
            {{ t("nav.paymentPlans") }}
          </router-link>
        </p>
      </template>
      <div class="settings-rows">
        <!-- Enable toggle --><SettingRow
          :label="t('admin.settings.payment.enabled')"
          :description="t('admin.settings.payment.enabledHint')"
        >
          <Toggle v-model="form.payment_enabled" /> </SettingRow
        ><template v-if="form.payment_enabled">
          <!-- Row 1: Product name -->
          <div class="settings-rows">
            <SettingRow :label="t('admin.settings.payment.productNamePrefix')">
              <input
                v-model="form.payment_product_name_prefix"
                type="text"
                class="input"
                placeholder="Sub2API"
              />
            </SettingRow>
            <SettingRow :label="t('admin.settings.payment.productNameSuffix')">
              <input
                v-model="form.payment_product_name_suffix"
                type="text"
                class="input"
                placeholder="CNY"
              />
            </SettingRow>
            <SettingRow :label="t('admin.settings.payment.preview')">
              <div
                class="rounded-lg border border-line bg-surface-2 px-3 py-2 text-sm text-muted"
              >
                {{
                  (form.payment_product_name_prefix || "Sub2API") +
                  " 100 " +
                  (form.payment_product_name_suffix || "CNY")
                }}
              </div>
            </SettingRow>
          </div>
          <!-- Row 2: Balance toggle + amounts -->
          <div class="grid grid-cols-2 gap-3 sm:grid-cols-5 settings-block">
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.minAmount")
              }}</label
              ><input
                :value="form.payment_min_amount || ''"
                @input="
                  form.payment_min_amount =
                    parseFloat(($event.target as HTMLInputElement).value) || 0
                "
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.maxAmount")
              }}</label
              ><input
                :value="form.payment_max_amount || ''"
                @input="
                  form.payment_max_amount =
                    parseFloat(($event.target as HTMLInputElement).value) || 0
                "
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.dailyLimit")
              }}</label
              ><input
                :value="form.payment_daily_limit || ''"
                @input="
                  form.payment_daily_limit =
                    parseFloat(($event.target as HTMLInputElement).value) || 0
                "
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="t('admin.settings.payment.noLimit')"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.balanceRechargeMultiplier")
              }}</label>
              <input
                :value="form.payment_balance_recharge_multiplier || ''"
                @input="
                  form.payment_balance_recharge_multiplier =
                    parseFloat(($event.target as HTMLInputElement).value) || 1
                "
                type="number"
                step="0.01"
                min="0.01"
                class="input"
              />
              <p class="mt-0.5 text-xs text-muted">
                {{ t("admin.settings.payment.balanceRechargeMultiplierHint") }}
              </p>
              <p class="mt-1 text-xs font-medium text-accent">
                {{
                  t("admin.settings.payment.balanceRechargePreview", {
                    usd: (
                      Number(form.payment_balance_recharge_multiplier) || 1
                    ).toFixed(2),
                  })
                }}
              </p>
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.subscriptionUsdToCnyRate")
              }}</label>
              <input
                :value="form.payment_subscription_usd_to_cny_rate || ''"
                @input="
                  form.payment_subscription_usd_to_cny_rate =
                    parseFloat(($event.target as HTMLInputElement).value) || 0
                "
                type="number"
                step="0.01"
                min="0"
                class="input"
                :placeholder="
                  t('admin.settings.payment.subscriptionUsdToCnyRateDisabled')
                "
              />
              <p class="mt-0.5 text-xs text-muted">
                {{ t("admin.settings.payment.subscriptionUsdToCnyRateHint") }}
              </p>
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.rechargeFeeRate")
              }}</label>
              <div class="relative">
                <input
                  :value="form.payment_recharge_fee_rate ?? ''"
                  @input="
                    form.payment_recharge_fee_rate = Math.min(
                      100,
                      Math.max(
                        0,
                        Math.round(
                          parseFloat(
                            ($event.target as HTMLInputElement).value || '0',
                          ) * 100,
                        ) / 100,
                      ),
                    )
                  "
                  type="number"
                  step="0.01"
                  min="0"
                  max="100"
                  class="input pr-8"
                />
                <span
                  class="settings-flex-row pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3 text-muted"
                  >%</span
                >
              </div>
              <p class="mt-0.5 text-xs text-muted">
                {{ t("admin.settings.payment.rechargeFeeRateHint") }}
              </p>
              <p
                v-if="(Number(form.payment_recharge_fee_rate) || 0) > 0"
                class="mt-1 text-xs font-medium text-accent"
              >
                {{
                  t("admin.settings.payment.rechargeFeePreview", {
                    fee: (Number(form.payment_recharge_fee_rate) || 0).toFixed(
                      2,
                    ),
                  })
                }}
              </p>
            </div>
            <div>
              <label class="input-label"
                >{{ t("admin.settings.payment.orderTimeout") }}
                <span class="text-danger-500">*</span></label
              ><input
                v-model.number="form.payment_order_timeout_minutes"
                type="number"
                min="1"
                class="input"
                required
              />
              <p class="mt-0.5 text-xs text-muted">
                {{ t("admin.settings.payment.orderTimeoutHint") }}
              </p>
            </div>
          </div>
          <!-- Row 3: Pending orders + load balance + cancel rate limit (all in one row) -->
          <div
            class="settings-flex-row flex flex-wrap items-end gap-4 settings-block"
          >
            <div class="w-28">
              <label class="input-label">{{
                t("admin.settings.payment.maxPendingOrders")
              }}</label
              ><input
                v-model.number="form.payment_max_pending_orders"
                type="number"
                min="1"
                class="input"
              />
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.loadBalanceStrategy")
              }}</label>
              <Select
                v-model="form.payment_load_balance_strategy"
                :options="loadBalanceOptions"
                class="w-40"
              />
            </div>
            <div class="max-w-full">
              <label class="input-label">{{
                t("admin.settings.payment.cancelRateLimit")
              }}</label>
              <div class="settings-flex-row flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
                    form.payment_cancel_rate_limit_enabled
                      ? 'bg-accent'
                      : 'bg-surface-3 ',
                  ]"
                  @click="
                    form.payment_cancel_rate_limit_enabled =
                      !form.payment_cancel_rate_limit_enabled
                  "
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out',
                      form.payment_cancel_rate_limit_enabled
                        ? 'translate-x-5'
                        : 'translate-x-0',
                    ]"
                  />
                </button>
                <Select
                  v-model="form.payment_cancel_rate_limit_window_mode"
                  :options="cancelRateLimitModeOptions"
                  class="w-24"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span
                  :class="[
                    'text-sm whitespace-nowrap',
                    form.payment_cancel_rate_limit_enabled
                      ? 'text-foreground '
                      : 'text-muted ',
                  ]"
                  >{{ t("admin.settings.payment.cancelRateLimitEvery") }}</span
                >
                <input
                  v-model.number="form.payment_cancel_rate_limit_window"
                  type="number"
                  min="1"
                  required
                  class="input w-14 text-center"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <Select
                  v-model="form.payment_cancel_rate_limit_unit"
                  :options="cancelRateLimitUnitOptions"
                  class="w-28"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span
                  :class="[
                    'text-sm whitespace-nowrap',
                    form.payment_cancel_rate_limit_enabled
                      ? 'text-foreground '
                      : 'text-muted ',
                  ]"
                  >{{
                    t("admin.settings.payment.cancelRateLimitAllowMax")
                  }}</span
                >
                <input
                  v-model.number="form.payment_cancel_rate_limit_max"
                  type="number"
                  min="1"
                  required
                  class="input w-14 text-center"
                  :disabled="!form.payment_cancel_rate_limit_enabled"
                />
                <span
                  :class="[
                    'text-sm whitespace-nowrap',
                    form.payment_cancel_rate_limit_enabled
                      ? 'text-foreground '
                      : 'text-muted ',
                  ]"
                  >{{ t("admin.settings.payment.cancelRateLimitTimes") }}</span
                >
              </div>
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.alipayForceQRCode")
              }}</label>
              <div class="settings-flex-row flex items-center gap-2">
                <button
                  type="button"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
                    form.payment_alipay_force_qrcode
                      ? 'bg-accent'
                      : 'bg-surface-3 ',
                  ]"
                  @click="
                    form.payment_alipay_force_qrcode =
                      !form.payment_alipay_force_qrcode
                  "
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out',
                      form.payment_alipay_force_qrcode
                        ? 'translate-x-5'
                        : 'translate-x-0',
                    ]"
                  />
                </button>
                <span class="text-sm text-muted">{{
                  t("admin.settings.payment.alipayForceQRCodeHint")
                }}</span>
              </div>
            </div>
            <div>
              <label class="input-label">{{
                t("admin.settings.payment.alipayMobilePrecreateDeepLink")
              }}</label>
              <div class="settings-flex-row flex items-center gap-2">
                <button
                  type="button"
                  :class="[
                    'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
                    form.payment_alipay_mobile_precreate_deep_link
                      ? 'bg-accent'
                      : 'bg-surface-3 ',
                  ]"
                  @click="
                    form.payment_alipay_mobile_precreate_deep_link =
                      !form.payment_alipay_mobile_precreate_deep_link
                  "
                >
                  <span
                    :class="[
                      'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-surface shadow ring-0 transition duration-200 ease-in-out',
                      form.payment_alipay_mobile_precreate_deep_link
                        ? 'translate-x-5'
                        : 'translate-x-0',
                    ]"
                  />
                </button>
                <span class="text-sm text-muted">{{
                  t("admin.settings.payment.alipayMobilePrecreateDeepLinkHint")
                }}</span>
              </div>
            </div>
          </div>
          <!-- Row 4: Enabled payment types (provider badges like sub2apipay) -->
          <SettingRow :label="t('admin.settings.payment.enabledPaymentTypes')">
            <div class="settings-flex-row mt-1.5 flex flex-wrap gap-2">
              <button
                v-for="pt in allPaymentTypes"
                :key="pt.value"
                type="button"
                @click="togglePaymentType(pt.value)"
                :class="[
                  'rounded-lg border px-3 py-1.5 text-sm font-medium transition-all',
                  isPaymentTypeEnabled(pt.value)
                    ? 'border-accent bg-accent text-white shadow-sm'
                    : 'border-line bg-surface text-muted hover:border-line hover:bg-surface-2 ',
                ]"
              >
                {{ pt.label }}
              </button>
            </div>
            <p class="mt-2 text-xs text-muted">
              {{ t("admin.settings.payment.enabledPaymentTypesHint") }}
              <a
                :href="paymentMethodsHref"
                target="_blank"
                rel="noopener noreferrer"
                class="ml-1 text-accent hover:text-accent"
              >
                {{ t("admin.settings.payment.findProvider") }}
                <svg
                  class="mb-0.5 ml-0.5 inline h-3 w-3"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    stroke-linecap="round"
                    stroke-linejoin="round"
                    stroke-width="2"
                    d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
                  />
                </svg>
              </a>
            </p>
          </SettingRow>
          <!-- Row 5: Help image + text -->
          <div class="settings-rows">
            <SettingRow :label="t('admin.settings.payment.helpImage')">
              <ImageUpload
                v-model="form.payment_help_image_url"
                :upload-label="t('admin.settings.site.uploadImage')"
                :remove-label="t('admin.settings.site.remove')"
                :placeholder="t('admin.settings.payment.helpImagePlaceholder')"
              />
            </SettingRow>
            <SettingRow :label="t('admin.settings.payment.helpText')">
              <textarea
                v-model="form.payment_help_text"
                rows="3"
                class="input"
                :placeholder="t('admin.settings.payment.helpTextPlaceholder')"
              ></textarea>
            </SettingRow>
          </div>
        </template>
      </div>
    </SettingsSection>

    <!-- Provider Management -->
    <PaymentProviderList
      v-if="form.payment_enabled"
      :providers="providers"
      :loading="providersLoading"
      :can-create="hasAnyPaymentTypeEnabled"
      :enabled-payment-types="form.payment_enabled_types"
      :all-payment-types="allPaymentTypes"
      :redirect-label="t('admin.settings.payment.easypayRedirect')"
      @refresh="loadProviders"
      @create="openCreateProvider"
      @edit="openEditProvider"
      @delete="confirmDeleteProvider"
      @toggle-field="handleToggleField"
      @toggle-type="handleToggleType"
      @reorder="handleReorderProviders"
    />
  </div>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import { computed } from "vue";
import { adminAPI } from "@/api";
import type { ProviderInstance } from "@/types/payment";
import Select from "@/components/common/Select.vue";
import PaymentProviderList from "@/components/payment/PaymentProviderList.vue";
import Toggle from "@/components/common/Toggle.vue";
import ImageUpload from "@/components/common/ImageUpload.vue";
import { extractI18nErrorMessage } from "@/utils/apiError";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  locale,
  appStore,
  activeTab,
  form,
  allPaymentTypes,
  providersLoading,
  providers,
  showProviderDialog,
  showDeleteProviderDialog,
  editingProvider,
  deletingProviderId,
  providerDialogRef,
  enabledProviderKeyOptions,
  findProviderEnablementConflict,
  showProviderEnablementConflict,
  loadProviders,
} = settingsForm;

const paymentGuideHref = computed(() =>
  locale.value.startsWith("zh")
    ? "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md"
    : "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT.md",
);

const paymentMethodsHref = computed(() =>
  locale.value.startsWith("zh")
    ? "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT_CN.md#支持的支付方式"
    : "https://github.com/Wei-Shaw/sub2api/blob/main/docs/PAYMENT.md#supported-payment-methods",
);

function isPaymentTypeEnabled(type: string): boolean {
  return form.payment_enabled_types.includes(type);
}

const hasAnyPaymentTypeEnabled = computed(
  () => form.payment_enabled_types.length > 0,
);

function togglePaymentType(type: string) {
  if (form.payment_enabled_types.includes(type)) {
    form.payment_enabled_types = form.payment_enabled_types.filter(
      (t) => t !== type,
    );
    // Disable all provider instances matching this type
    disableProvidersByType(type);
  } else {
    form.payment_enabled_types = [...form.payment_enabled_types, type];
  }
}

async function disableProvidersByType(type: string) {
  const matching = providers.value.filter(
    (p) => p.provider_key === type && p.enabled,
  );
  for (const p of matching) {
    try {
      await adminAPI.payment.updateProvider(p.id, { enabled: false });
      p.enabled = false;
    } catch (err: unknown) {
      slog("disable provider failed", p.id, err);
    }
  }
}

function slog(...args: unknown[]) {
  console.warn("[payment]", ...args);
}

const loadBalanceOptions = computed(() => [
  {
    value: "round-robin",
    label: t("admin.settings.payment.strategyRoundRobin"),
  },
  {
    value: "least-amount",
    label: t("admin.settings.payment.strategyLeastAmount"),
  },
]);

const cancelRateLimitUnitOptions = computed(() => [
  {
    value: "minute",
    label: t("admin.settings.payment.cancelRateLimitUnitMinute"),
  },
  { value: "hour", label: t("admin.settings.payment.cancelRateLimitUnitHour") },
  { value: "day", label: t("admin.settings.payment.cancelRateLimitUnitDay") },
]);

const cancelRateLimitModeOptions = computed(() => [
  {
    value: "rolling",
    label: t("admin.settings.payment.cancelRateLimitWindowModeRolling"),
  },
  {
    value: "fixed",
    label: t("admin.settings.payment.cancelRateLimitWindowModeFixed"),
  },
]);

function openCreateProvider() {
  editingProvider.value = null;
  providerDialogRef.value?.reset(
    enabledProviderKeyOptions.value[0]?.value || "easypay",
  );
  showProviderDialog.value = true;
}

function openEditProvider(provider: ProviderInstance) {
  editingProvider.value = provider;
  providerDialogRef.value?.loadProvider(provider);
  showProviderDialog.value = true;
}

async function handleToggleField(
  provider: ProviderInstance,
  field: "enabled" | "refund_enabled" | "allow_user_refund" | "invoice_enabled",
) {
  let newValue: boolean;
  if (field === "enabled") newValue = !provider.enabled;
  else if (field === "refund_enabled") newValue = !provider.refund_enabled;
  else if (field === "invoice_enabled") newValue = !provider.invoice_enabled;
  else newValue = !provider.allow_user_refund;

  if (field === "enabled" && newValue) {
    const conflict = findProviderEnablementConflict({
      id: provider.id,
      provider_key: provider.provider_key,
      supported_types: provider.supported_types,
      enabled: true,
      name: provider.name,
    });
    if (conflict) {
      showProviderEnablementConflict(conflict);
      return;
    }
  }

  const payload: Record<string, boolean> = { [field]: newValue };
  // Cascade: turning off refund_enabled also turns off allow_user_refund
  if (field === "refund_enabled" && !newValue) {
    payload.allow_user_refund = false;
  }
  try {
    await adminAPI.payment.updateProvider(provider.id, payload);
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  }
}

async function handleToggleType(provider: ProviderInstance, type: string) {
  const currentTypes = Array.isArray(provider.supported_types)
    ? provider.supported_types
    : [];
  const updated = currentTypes.includes(type)
    ? currentTypes.filter((t) => t !== type)
    : [...currentTypes, type];
  const conflict = findProviderEnablementConflict({
    id: provider.id,
    provider_key: provider.provider_key,
    supported_types: updated,
    enabled: provider.enabled,
    name: provider.name,
  });
  if (conflict) {
    showProviderEnablementConflict(conflict);
    return;
  }
  try {
    await adminAPI.payment.updateProvider(provider.id, {
      supported_types: updated,
    } as any);
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
  }
}

function confirmDeleteProvider(provider: ProviderInstance) {
  deletingProviderId.value = provider.id;
  showDeleteProviderDialog.value = true;
}

async function handleReorderProviders(
  updates: { id: number; sort_order: number }[],
) {
  try {
    await Promise.all(
      updates.map((u) =>
        adminAPI.payment.updateProvider(u.id, {
          sort_order: u.sort_order,
        } as Partial<ProviderInstance>),
      ),
    );
    await loadProviders();
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, "payment.errors", t("common.error")));
    loadProviders();
  }
}
</script>
