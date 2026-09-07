<template>
  <SettingsSection
    :title="localText('邮箱快捷登录', 'Email OAuth Sign-in')"
    :description="
      localText(
        '开启 GitHub 或 Google 邮箱授权登录后，系统会读取已验证邮箱，存在则直接登录，不存在则自动注册。',
        'After GitHub or Google email OAuth is enabled, the system reads a verified email, signs in matching users, and auto-registers missing users.',
      )
    "
  >
    <div class="settings-rows">
      <div class="settings-rows">
        <div class="rounded-lg border border-line p-4 settings-block">
          <SettingRow
            :description="
              localText(
                'GitHub OAuth App 需要 read:user user:email 权限，回调地址填写下方后端地址。',
                'GitHub OAuth App needs read:user user:email scopes. Use the backend callback URL below.',
              )
            "
          >
            <template #label
              ><h3 class="font-medium text-foreground">GitHub</h3></template
            >
            <Toggle v-model="form.github_oauth_enabled" />
          </SettingRow>
          <div v-if="form.github_oauth_enabled" class="mt-4 space-y-4">
            <div class="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">
              <template v-if="isZhLocale">
                开通引导：GitHub Settings → Developer settings →
                <a
                  data-testid="github-oauth-apps-guide-link"
                  href="https://github.com/settings/developers"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="font-medium text-accent hover:underline"
                  >OAuth Apps</a
                >
                → New OAuth App；Homepage URL 填站点域名，Authorization callback
                URL 填下面的后端回调地址。
              </template>
              <template v-else>
                Setup guide: GitHub Settings → Developer settings →
                <a
                  data-testid="github-oauth-apps-guide-link"
                  href="https://github.com/settings/developers"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="font-medium text-accent hover:underline"
                  >OAuth Apps</a
                >
                → New OAuth App. Use your site origin as Homepage URL and the
                backend callback URL below as Authorization callback URL.
              </template>
            </div>

            <div class="settings-rows">
              <SettingRow>
                <template #label
                  ><label class="settings-row-label">Client ID</label></template
                >
                <input
                  v-model="form.github_oauth_client_id"
                  type="text"
                  class="input font-mono text-sm"
                  placeholder="GitHub OAuth Client ID"
                />
              </SettingRow>
              <SettingRow>
                <template #label
                  ><label class="settings-row-label"
                    >Client Secret</label
                  ></template
                >
                <input
                  v-model="form.github_oauth_client_secret"
                  type="password"
                  class="input font-mono text-sm"
                  :placeholder="
                    form.github_oauth_client_secret_configured
                      ? localText(
                          '密钥已配置，留空以保留当前值。',
                          'Secret configured. Leave empty to keep the current value.',
                        )
                      : 'GitHub OAuth Client Secret'
                  "
                />
              </SettingRow>
            </div>

            <SettingRow
              :label="localText('后端回调地址', 'Backend Callback URL')"
            >
              <input
                v-model="form.github_oauth_redirect_url"
                type="url"
                class="input font-mono text-sm"
                placeholder="https://your-domain.com/api/v1/auth/oauth/github/callback"
              />
              <div
                class="settings-flex-row mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
              >
                <button
                  type="button"
                  class="btn-glass-secondary w-fit"
                  @click="setAndCopyEmailOAuthRedirectUrl('github')"
                >
                  {{ localText("生成并复制", "Generate and copy") }}
                </button>
                <code
                  v-if="githubOAuthRedirectUrlSuggestion"
                  class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted"
                >
                  {{ githubOAuthRedirectUrlSuggestion }}
                </code>
              </div>
            </SettingRow>

            <SettingRow
              :label="localText('前端回跳地址', 'Frontend Callback URL')"
            >
              <input
                v-model="form.github_oauth_frontend_redirect_url"
                type="text"
                class="input font-mono text-sm"
                placeholder="/auth/oauth/callback"
              />
            </SettingRow>
          </div>
        </div>
        <div class="rounded-lg border border-line p-4 settings-block">
          <SettingRow
            :description="
              localText(
                'Google OAuth 客户端需要 openid email profile 范围，并在凭据里登记后端回调地址。',
                'Google OAuth client needs openid email profile scopes and the backend callback URL registered in credentials.',
              )
            "
          >
            <template #label
              ><h3 class="font-medium text-foreground">Google</h3></template
            >
            <Toggle v-model="form.google_oauth_enabled" />
          </SettingRow>
          <div v-if="form.google_oauth_enabled" class="mt-4 space-y-4">
            <div class="rounded-lg bg-surface-2 px-3 py-2 text-xs text-muted">
              {{
                localText(
                  "开通引导：Google Cloud Console → APIs & Services → OAuth consent screen 完成同意屏幕；Credentials → Create Credentials → OAuth client ID，类型选择 Web application，并把下面地址加入 Authorized redirect URIs。",
                  "Setup guide: Google Cloud Console → APIs & Services → OAuth consent screen, then Credentials → Create Credentials → OAuth client ID, choose Web application, and add the URL below to Authorized redirect URIs.",
                )
              }}
            </div>

            <div class="settings-rows">
              <SettingRow>
                <template #label
                  ><label class="settings-row-label">Client ID</label></template
                >
                <input
                  v-model="form.google_oauth_client_id"
                  type="text"
                  class="input font-mono text-sm"
                  placeholder="Google OAuth Client ID"
                />
              </SettingRow>
              <SettingRow>
                <template #label
                  ><label class="settings-row-label"
                    >Client Secret</label
                  ></template
                >
                <input
                  v-model="form.google_oauth_client_secret"
                  type="password"
                  class="input font-mono text-sm"
                  :placeholder="
                    form.google_oauth_client_secret_configured
                      ? localText(
                          '密钥已配置，留空以保留当前值。',
                          'Secret configured. Leave empty to keep the current value.',
                        )
                      : 'Google OAuth Client Secret'
                  "
                />
              </SettingRow>
            </div>

            <SettingRow
              :label="localText('后端回调地址', 'Backend Callback URL')"
            >
              <input
                v-model="form.google_oauth_redirect_url"
                type="url"
                class="input font-mono text-sm"
                placeholder="https://your-domain.com/api/v1/auth/oauth/google/callback"
              />
              <div
                class="settings-flex-row mt-2 flex flex-col gap-2 sm:flex-row sm:items-center sm:gap-3"
              >
                <button
                  type="button"
                  class="btn-glass-secondary w-fit"
                  @click="setAndCopyEmailOAuthRedirectUrl('google')"
                >
                  {{ localText("生成并复制", "Generate and copy") }}
                </button>
                <code
                  v-if="googleOAuthRedirectUrlSuggestion"
                  class="select-all break-all rounded bg-surface-2 px-2 py-1 font-mono text-xs text-muted"
                >
                  {{ googleOAuthRedirectUrlSuggestion }}
                </code>
              </div>
            </SettingRow>

            <SettingRow
              :label="localText('前端回跳地址', 'Frontend Callback URL')"
            >
              <input
                v-model="form.google_oauth_frontend_redirect_url"
                type="text"
                class="input font-mono text-sm"
                placeholder="/auth/oauth/callback"
              />
            </SettingRow>
          </div>
        </div>
      </div>
    </div>
  </SettingsSection>
</template>

<script setup lang="ts">
import SettingRow from "@/components/ui/SettingRow.vue";
import SettingsSection from "@/components/ui/SettingsSection.vue";
import { inject, computed } from "vue";
import { SettingsFormKey } from "../useSettingsForm";
import Toggle from "@/components/common/Toggle.vue";
import { useClipboard } from "@/composables/useClipboard";
import { buildApiCallbackUrl } from "./oauthCallbackUrl";

type EmailOAuthProvider = "github" | "google";

const settingsForm = inject(SettingsFormKey)!;
const { form, isZhLocale, localText, currentOrigin } = settingsForm;

const { copyToClipboard } = useClipboard();

const githubOAuthRedirectUrlSuggestion = computed(() => {
  return buildApiCallbackUrl(
    form.api_base_url,
    currentOrigin,
    "/auth/oauth/github/callback",
  );
});

const googleOAuthRedirectUrlSuggestion = computed(() => {
  return buildApiCallbackUrl(
    form.api_base_url,
    currentOrigin,
    "/auth/oauth/google/callback",
  );
});

async function setAndCopyEmailOAuthRedirectUrl(provider: EmailOAuthProvider) {
  const url =
    provider === "github"
      ? githubOAuthRedirectUrlSuggestion.value
      : googleOAuthRedirectUrlSuggestion.value;
  if (!url) return;

  if (provider === "github") {
    form.github_oauth_redirect_url = url;
  } else {
    form.google_oauth_redirect_url = url;
  }
  await copyToClipboard(
    url,
    localText("回调地址已写入并复制。", "Callback URL set and copied."),
  );
}
</script>
