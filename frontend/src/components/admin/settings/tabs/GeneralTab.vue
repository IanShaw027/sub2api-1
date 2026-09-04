<template>
 <div v-show="activeTab === 'general'" class="settings-stack">
 <!-- Site Settings -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.site.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.site.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Backend Mode -->
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.site.backendMode") }}
 </label>
 <div class="flex items-center gap-3">
 <Toggle v-model="form.backend_mode_enabled" />
 <span class="settings-warning-hint">
 <svg
 width="14"
 height="14"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="2"
 stroke-linecap="round"
 stroke-linejoin="round"
 aria-hidden="true"
 >
 <path
 d="M10.3 3.9 1.8 18a2 2 0 0 0 1.7 3h17a2 2 0 0 0 1.7-3L13.7 3.9a2 2 0 0 0-3.4 0zM12 9v4M12 17h.01"
 />
 </svg>
 {{ t("admin.settings.site.backendModeWarning") }}
 </span>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.site.backendModeDescription") }}
 </p>
 </div>

	 <div class="settings-row-group">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.siteName") }}
 </label>
 <input
 v-model="form.site_name"
 type="text"
 class="input"
 :placeholder="t('admin.settings.site.siteNamePlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.siteNameHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.siteSubtitle") }}
 </label>
 <input
 v-model="form.site_subtitle"
 type="text"
 class="input"
 :placeholder="
 t('admin.settings.site.siteSubtitlePlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.siteSubtitleHint") }}
 </p>
 </div>
 </div>

 <!-- API Base URL -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.apiBaseUrl") }}
 </label>
 <input
 v-model="form.api_base_url"
 type="text"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.site.apiBaseUrlPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.apiBaseUrlHint") }}
 </p>
 </div>


 <!-- Contact Info -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.contactInfo") }}
 </label>
 <input
 v-model="form.contact_info"
 type="text"
 class="input"
 :placeholder="t('admin.settings.site.contactInfoPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.contactInfoHint") }}
 </p>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.supportQRCodes") }}
 </label>
 <div class="space-y-4">
 <div
 v-if="form.support_qr_codes.length > 0"
 class="settings-row-group"
 :class="form.support_qr_codes.length > 1 ? 'lg:grid-cols-2' : ''"
 >
 <div
 v-for="(item, index) in form.support_qr_codes"
 :key="`support-qr-${index}`"
 class="rounded-2xl border border-line bg-surface-2/80 p-4 "
 >
 <div class="flex items-start justify-between gap-3">
 <div class="min-w-0 flex-1 space-y-4">
 <ImageUpload
 v-model="item.image_url"
 mode="image"
 :upload-label="t('admin.settings.site.uploadQRCode')"
 :remove-label="t('admin.settings.site.remove')"
 :hint="t('admin.settings.site.supportQRCodeImageHint')"
 :max-size="500 * 1024"
 />
 <div class="settings-row">
 <label class="settings-row-label">
 {{ t("admin.settings.site.supportQRCodeNote") }}
 </label>
 <input
 v-model="item.note"
 type="text"
 maxlength="80"
 class="input"
 :placeholder="t('admin.settings.site.supportQRCodeNotePlaceholder')"
 />
 </div>
 </div>
 <button
 type="button"
 class="btn-glass-secondary shrink-0 text-red-600 hover:text-red-700 "
 @click="removeSupportQRCode(index)"
 >
 {{ t("admin.settings.site.remove") }}
 </button>
 </div>
 </div>
 </div>

 <button
 v-if="form.support_qr_codes.length < 8"
 type="button"
 class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-line px-4 py-2.5 text-sm text-muted transition-colors hover:border-accent hover:text-accent "
 @click="addSupportQRCode"
 >
 {{ t("admin.settings.site.addSupportQRCode") }}
 </button>
 </div>
 <p class="settings-row-hint">
 {{ t("admin.settings.site.supportQRCodesHint") }}
 </p>
 </div>

 <!-- Doc URL -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.docUrl") }}
 </label>
 <input
 v-model="form.doc_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.site.docUrlPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.docUrlHint") }}
 </p>
 </div>

 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.downloadToolsUrl") }}
 </label>
 <input
 v-model="form.download_tools_url"
 type="url"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.site.downloadToolsUrlPlaceholder')"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.downloadToolsUrlHint") }}
 </p>
 </div>

 <!-- Site Logo Upload -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.siteLogo") }}
 </label>
 <ImageUpload
 v-model="form.site_logo"
 mode="image"
 :upload-label="t('admin.settings.site.uploadImage')"
 :remove-label="t('admin.settings.site.remove')"
 :hint="t('admin.settings.site.logoHint')"
 :max-size="300 * 1024"
 />
 </div>

 <!-- Home Content -->
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.homeContent") }}
 </label>
 <textarea
 v-model="form.home_content"
 rows="6"
 class="input font-mono text-sm"
 :placeholder="t('admin.settings.site.homeContentPlaceholder')"
 ></textarea>
 <p class="settings-row-hint">
 {{ t("admin.settings.site.homeContentHint") }}
 </p>
 <!-- iframe CSP Warning -->
 <p class="mt-2 text-xs text-amber-600 ">
 {{ t("admin.settings.site.homeContentIframeWarning") }}
 </p>
 </div>

 <!-- Compact Home Page -->
 <div class="flex items-center justify-between border-t border-line pt-4 ">
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.site.compactHome")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.site.compactHomeHint") }}
 </p>
 </div>
 <Toggle v-model="form.compact_home_enabled" data-testid="compact-home-toggle" />
 </div>

 <!-- Hide CCS Import Button -->
 <div
 class="flex items-center justify-between border-t border-line pt-4 "
 >
 <div>
 <label class="font-medium text-foreground ">{{
 t("admin.settings.site.hideCcsImportButton")
 }}</label>
 <p class="text-sm text-muted ">
 {{ t("admin.settings.site.hideCcsImportButtonHint") }}
 </p>
 </div>
 <Toggle v-model="form.hide_ccs_import_button" />
 </div>
 </div>
 </div>

 <!-- Custom Endpoints -->
 <div class="glass-card settings-card">
 <div class="settings-card-head settings-list-head">
 <div class="settings-list-head-text">
 <h2 class="settings-card-title">
 {{ t("admin.settings.site.customEndpoints.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.site.customEndpoints.description") }}
 </p>
 </div>
 <button
 type="button"
 class="btn-glass-secondary settings-btn-sm"
 @click="addEndpoint"
 >
 <svg
 width="14"
 height="14"
 viewBox="0 0 24 24"
 fill="none"
 stroke="currentColor"
 stroke-width="2.2"
 stroke-linecap="round"
 stroke-linejoin="round"
 aria-hidden="true"
 >
 <path d="M12 5v14M5 12h14" />
 </svg>
 {{ t("admin.settings.site.customEndpoints.add") }}
 </button>
 </div>
 <div class="settings-card-body">
 <div
 v-for="(ep, index) in form.custom_endpoints"
 :key="index"
 class="settings-list-row"
 >
 <input
 v-model="ep.name"
 type="text"
 class="input"
 :aria-label="t('admin.settings.site.customEndpoints.name')"
 :placeholder="
 t('admin.settings.site.customEndpoints.namePlaceholder')
 "
 />
 <input
 v-model="ep.endpoint"
 type="url"
 class="input font-mono"
 :aria-label="t('admin.settings.site.customEndpoints.endpointUrl')"
 :placeholder="
 t('admin.settings.site.customEndpoints.endpointUrlPlaceholder')
 "
 />
 <input
 v-model="ep.description"
 type="text"
 class="input"
 :aria-label="
 t('admin.settings.site.customEndpoints.descriptionLabel')
 "
 :placeholder="
 t('admin.settings.site.customEndpoints.descriptionPlaceholder')
 "
 />
 <button
 type="button"
 class="settings-icon-danger"
 :title="t('common.delete')"
 :aria-label="t('common.delete')"
 @click="removeEndpoint(index)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
 />
 </svg>
 </button>
 </div>
 <p v-if="form.custom_endpoints.length === 0" class="settings-empty-hint">
 {{ t("common.noData") }}
 </p>
 </div>
 </div>

 <!-- Global Table Preferences -->
 <div class="glass-card settings-card">
 <div class="settings-card-head">
 <h2 class="settings-card-title">
 {{ t("admin.settings.site.tablePreferencesTitle") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.site.tablePreferencesDescription") }}
 </p>
 </div>
 <div class="settings-card-body">
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.tableDefaultPageSize") }}
 </label>
 <input
 v-model.number="form.table_default_page_size"
 type="number"
 min="5"
 max="1000"
 step="1"
 class="input w-40"
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.tableDefaultPageSizeHint") }}
 </p>
 </div>
 <div class="settings-row">
 <label
 class="settings-row-label"
 >
 {{ t("admin.settings.site.tablePageSizeOptions") }}
 </label>
 <input
 v-model="tablePageSizeOptionsInput"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.site.tablePageSizeOptionsPlaceholder')
 "
 />
 <p class="settings-row-hint">
 {{ t("admin.settings.site.tablePageSizeOptionsHint") }}
 </p>
 </div>
 </div>
 </div>

 <!-- Custom Menu Items -->
 <div class="glass-card settings-card">
 <div
 class="settings-card-head"
 >
 <h2 class="settings-card-title">
 {{ t("admin.settings.customMenu.title") }}
 </h2>
 <p class="settings-card-desc">
 {{ t("admin.settings.customMenu.description") }}
 </p>
 </div>
 <div class="settings-card-body">
 <!-- Existing menu items -->
 <div
 v-for="(item, index) in form.custom_menu_items"
 :key="item.id || index"
 class="rounded-lg border border-line p-4 "
 >
 <div class="mb-3 flex items-center justify-between">
 <span
 class="text-sm font-medium text-foreground "
 >
 {{
 t("admin.settings.customMenu.itemLabel", { n: index + 1 })
 }}
 </span>
 <div class="flex items-center gap-2">
 <!-- Move up -->
 <button
 v-if="index > 0"
 type="button"
 class="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground "
 :title="t('admin.settings.customMenu.moveUp')"
 @click="moveMenuItem(index, -1)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M5 15l7-7 7 7"
 />
 </svg>
 </button>
 <!-- Move down -->
 <button
 v-if="index < form.custom_menu_items.length - 1"
 type="button"
 class="rounded p-1 text-muted hover:bg-surface-2 hover:text-foreground "
 :title="t('admin.settings.customMenu.moveDown')"
 @click="moveMenuItem(index, 1)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M19 9l-7 7-7-7"
 />
 </svg>
 </button>
 <!-- Delete -->
 <button
 type="button"
 class="rounded p-1 text-red-400 hover:bg-red-50 hover:text-red-600 "
 :title="t('admin.settings.customMenu.remove')"
 @click="removeMenuItem(index)"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
 />
 </svg>
 </button>
 </div>
 </div>

 <div class="settings-row-group">
 <!-- Label -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.customMenu.name") }}
 </label>
 <input
 v-model="item.label"
 type="text"
 class="input text-sm"
 :placeholder="
 t('admin.settings.customMenu.namePlaceholder')
 "
 />
 </div>

 <!-- Visibility -->
 <div>
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.customMenu.visibility") }}
 </label>
 <select v-model="item.visibility" class="input text-sm">
 <option value="user">
 {{ t("admin.settings.customMenu.visibilityUser") }}
 </option>
 <option value="admin">
 {{ t("admin.settings.customMenu.visibilityAdmin") }}
 </option>
 </select>
 </div>

 <!-- URL (full width) -->
 <div class="sm:col-span-2">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.customMenu.url") }}
 </label>
 <input
 v-model="item.url"
 type="text"
 class="input font-mono text-sm"
 :placeholder="
 t('admin.settings.customMenu.urlPlaceholder')
 "
 />
 <p class="mt-1 text-xs text-muted ">
 {{ t("admin.settings.customMenu.urlHint") }}
 </p>
 </div>

 <!-- SVG Icon (full width) -->
 <div class="sm:col-span-2">
 <label
 class="settings-sub-label"
 >
 {{ t("admin.settings.customMenu.iconSvg") }}
 </label>
 <ImageUpload
 :model-value="item.icon_svg"
 mode="svg"
 size="sm"
 :upload-label="t('admin.settings.customMenu.uploadSvg')"
 :remove-label="t('admin.settings.customMenu.removeSvg')"
 @update:model-value="(v: string) => (item.icon_svg = v)"
 />
 </div>
 </div>
 </div>

 <!-- Add button -->
 <button
 type="button"
 class="flex w-full items-center justify-center gap-2 rounded-lg border-2 border-dashed border-line py-3 text-sm text-muted transition-colors hover:border-accent hover:text-accent "
 @click="addMenuItem"
 >
 <svg
 class="h-4 w-4"
 fill="none"
 viewBox="0 0 24 24"
 stroke="currentColor"
 stroke-width="2"
 >
 <path
 stroke-linecap="round"
 stroke-linejoin="round"
 d="M12 4v16m8-8H4"
 />
 </svg>
 {{ t("admin.settings.customMenu.add") }}
 </button>
 </div>
 </div>
	 </div>
</template>

<script setup lang="ts">
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";
import ImageUpload from "@/components/common/ImageUpload.vue";

const settingsForm = inject(SettingsFormKey)!;
const {
  t,
  activeTab,
  tablePageSizeOptionsInput,
  form,
} = settingsForm;

function addMenuItem() {
  form.custom_menu_items.push({
    id: "",
    label: "",
    icon_svg: "",
    url: "",
    visibility: "user",
    sort_order: form.custom_menu_items.length,
  });
}

function removeMenuItem(index: number) {
  form.custom_menu_items.splice(index, 1);
  // Re-index sort_order
  form.custom_menu_items.forEach((item, i) => {
    item.sort_order = i;
  });
}

function moveMenuItem(index: number, direction: -1 | 1) {
  const targetIndex = index + direction;
  if (targetIndex < 0 || targetIndex >= form.custom_menu_items.length) return;
  const items = form.custom_menu_items;
  const temp = items[index];
  items[index] = items[targetIndex];
  items[targetIndex] = temp;
  // Re-index sort_order
  items.forEach((item, i) => {
    item.sort_order = i;
  });
}

function addEndpoint() {
  form.custom_endpoints.push({ name: "", endpoint: "", description: "" });
}

function removeEndpoint(index: number) {
  form.custom_endpoints.splice(index, 1);
}

function addSupportQRCode() {
  if (form.support_qr_codes.length >= 8) return;
  form.support_qr_codes.push({ image_url: "", note: "" });
}

function removeSupportQRCode(index: number) {
  form.support_qr_codes.splice(index, 1);
}
</script>
