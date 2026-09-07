<template>
  <SettingsSection
    :title="t('admin.settings.dingtalk.title')"
    :description="t('admin.settings.dingtalk.description')"
  >
    <div class="settings-rows">
      <SettingRow
        :label="t('admin.settings.dingtalk.enable')"
        :description="t('admin.settings.dingtalk.enableHint')"
      >
        <Toggle v-model="form.dingtalk_connect_enabled" />
      </SettingRow>
      <div v-if="form.dingtalk_connect_enabled" class="settings-rows">
        <div class="settings-rows">
          <SettingRow
            :label="t('admin.settings.dingtalk.clientId')"
            :description="t('admin.settings.dingtalk.clientIdHint')"
          >
            <input
              v-model="form.dingtalk_connect_client_id"
              type="text"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.dingtalk.clientIdPlaceholder')"
            /> </SettingRow
          ><SettingRow
            :label="t('admin.settings.dingtalk.clientSecret')"
            :description="
              form.dingtalk_connect_client_secret_configured
                ? t('admin.settings.dingtalk.clientSecretConfiguredHint')
                : t('admin.settings.dingtalk.clientSecretHint')
            "
          >
            <input
              v-model="form.dingtalk_connect_client_secret"
              type="password"
              class="input font-mono text-sm"
              :placeholder="
                form.dingtalk_connect_client_secret_configured
                  ? t(
                      'admin.settings.dingtalk.clientSecretConfiguredPlaceholder',
                    )
                  : t('admin.settings.dingtalk.clientSecretPlaceholder')
              "
            /> </SettingRow
          ><SettingRow
            :label="t('admin.settings.dingtalk.redirectUrl')"
            :description="t('admin.settings.dingtalk.redirectUrlHint')"
          >
            <input
              v-model="form.dingtalk_connect_redirect_url"
              type="url"
              class="input font-mono text-sm"
              :placeholder="t('admin.settings.dingtalk.redirectUrlPlaceholder')"
            /> </SettingRow
          ><!-- Corp Restriction Policy --><SettingRow
            :label="t('admin.settings.dingtalk.corpPolicy.label')"
          >
            <p class="mb-3 text-xs text-muted">
              {{ t("admin.settings.dingtalk.corpPolicy.hint") }}
            </p>
            <div class="space-y-2">
              <label
                class="settings-flex-row flex cursor-pointer items-center gap-3"
              >
                <input
                  v-model="form.dingtalk_connect_corp_restriction_policy"
                  type="radio"
                  value="none"
                  class="h-4 w-4 text-accent"
                />
                <span class="text-sm text-foreground">
                  {{ t("admin.settings.dingtalk.corpPolicy.none") }}
                </span>
              </label>
              <label
                class="settings-flex-row flex cursor-pointer items-center gap-3"
              >
                <input
                  v-model="form.dingtalk_connect_corp_restriction_policy"
                  type="radio"
                  value="internal_only"
                  class="h-4 w-4 text-accent"
                />
                <span class="text-sm text-foreground">
                  {{ t("admin.settings.dingtalk.corpPolicy.internalOnly") }}
                </span>
              </label>
            </div> </SettingRow
          ><!-- bypass_registration toggle（仅 internal_only 模式下可见可用） --><SettingRow
            v-if="
              form.dingtalk_connect_corp_restriction_policy === 'internal_only'
            "
            :label="t('admin.settings.dingtalk.bypassRegistration')"
            :description="t('admin.settings.dingtalk.bypassRegistrationHint')"
          >
            <Toggle
              v-model="form.dingtalk_connect_bypass_registration"
            /> </SettingRow
          ><!-- 身份同步开关（仅 internal_only 模式下可见） -->
          <div
            v-if="
              form.dingtalk_connect_corp_restriction_policy === 'internal_only'
            "
            class="pt-4 border-t border-line space-y-2 settings-block"
          >
            <SettingRow
              :label="t('admin.settings.dingtalk.syncDisplayName')"
              :description="t('admin.settings.dingtalk.syncDisplayNameHint')"
            >
              <Toggle v-model="form.dingtalk_connect_sync_display_name" />
            </SettingRow>
            <div
              v-if="form.dingtalk_connect_sync_display_name"
              class="space-y-2"
            >
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncDisplayNameTarget") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_display_name_attr_key"
                  type="text"
                  placeholder="dingtalk_name"
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncAttrDisplayName") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_display_name_attr_name"
                  type="text"
                  :placeholder="localText('钉钉姓名', 'DingTalk Name')"
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
            </div>
            <p
              v-if="form.dingtalk_connect_sync_display_name"
              class="text-xs text-muted"
            >
              {{ t("admin.settings.dingtalk.syncDisplayNameTargetHint") }}
            </p>
          </div>
          <div
            v-if="
              form.dingtalk_connect_corp_restriction_policy === 'internal_only'
            "
            class="pt-4 border-t border-line space-y-2 settings-block"
          >
            <SettingRow :label="t('admin.settings.dingtalk.syncCorpEmail')">
              <template #description
                ><p class="text-sm text-muted">
                  {{ t("admin.settings.dingtalk.syncCorpEmailHint") }}
                </p>
                <p class="text-xs text-warning-text mt-1">
                  {{ t("admin.settings.dingtalk.syncCorpEmailPermissionHint") }}
                </p></template
              >
              <Toggle v-model="form.dingtalk_connect_sync_corp_email" />
            </SettingRow>
            <div v-if="form.dingtalk_connect_sync_corp_email" class="space-y-2">
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncCorpEmailTarget") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_corp_email_attr_key"
                  type="text"
                  placeholder="dingtalk_email"
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncAttrDisplayName") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_corp_email_attr_name"
                  type="text"
                  :placeholder="
                    localText('钉钉企业邮箱', 'DingTalk Corporate Email')
                  "
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
            </div>
            <p
              v-if="form.dingtalk_connect_sync_corp_email"
              class="text-xs text-muted"
            >
              {{ t("admin.settings.dingtalk.syncCorpEmailTargetHint") }}
            </p>
          </div>
          <div
            v-if="
              form.dingtalk_connect_corp_restriction_policy === 'internal_only'
            "
            class="pt-4 border-t border-line space-y-2 settings-block"
          >
            <SettingRow :label="t('admin.settings.dingtalk.syncDept')">
              <template #description
                ><p class="text-sm text-muted">
                  {{ t("admin.settings.dingtalk.syncDeptHint") }}
                </p>
                <p class="text-xs text-warning-text mt-1">
                  {{ t("admin.settings.dingtalk.syncDeptPermissionHint") }}
                </p></template
              >
              <Toggle v-model="form.dingtalk_connect_sync_dept" />
            </SettingRow>
            <div v-if="form.dingtalk_connect_sync_dept" class="space-y-2">
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncDeptTarget") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_dept_attr_key"
                  type="text"
                  placeholder="dingtalk_department"
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
              <div class="settings-flex-row flex items-center gap-2">
                <label
                  class="text-sm text-muted whitespace-nowrap min-w-[5rem]"
                >
                  {{ t("admin.settings.dingtalk.syncAttrDisplayName") }}
                </label>
                <input
                  v-model="form.dingtalk_connect_sync_dept_attr_name"
                  type="text"
                  :placeholder="localText('钉钉部门', 'DingTalk Department')"
                  class="input text-sm flex-1 max-w-xs"
                />
              </div>
            </div>
            <p
              v-if="form.dingtalk_connect_sync_dept"
              class="text-xs text-muted"
            >
              {{ t("admin.settings.dingtalk.syncDeptTargetHint") }}
            </p>
          </div>
        </div>
      </div>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";

const settingsForm = inject(SettingsFormKey)!;
const { t, form, localText } = settingsForm;
</script>
