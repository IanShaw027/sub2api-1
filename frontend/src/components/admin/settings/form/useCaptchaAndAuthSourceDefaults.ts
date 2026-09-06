// 从 useSettingsForm.ts 拆分而来（frontend-health-cleanup 16）：验证码提供方选择状态与
// 认证来源默认值（authSourceDefaults）元信息。逐字保留原实现，t/localText/form 由调用方
// 传入以复用同一实例。
import { computed, reactive, ref } from "vue";
import { useI18n } from "vue-i18n";
import {
  buildAuthSourceDefaultsState,
  type AuthSourceDefaultsState,
  type AuthSourceType,
} from "@/api/admin/settings";
import type { CaptchaProviderSelection, SettingsForm } from "../useSettingsForm";

export function useCaptchaAndAuthSourceDefaults(
  form: SettingsForm,
  t: ReturnType<typeof useI18n>["t"],
  localText: (zh: string, en: string) => string,
) {
  const captchaProviderSelection = ref<CaptchaProviderSelection>("turnstile");

  function syncCaptchaProviderSelection(): void {
    if (form.tencent_captcha_enabled) {
      captchaProviderSelection.value = "tencent";
    } else if (form.aliyun_captcha_enabled) {
      captchaProviderSelection.value = "aliyun";
    } else if (form.turnstile_enabled) {
      captchaProviderSelection.value = "turnstile";
    }
  }

  const authSourceDefaults = reactive<AuthSourceDefaultsState>(
    buildAuthSourceDefaultsState({}),
  );

  const authSourceDefaultsMeta = computed(() => [
    {
      source: "email" as AuthSourceType,
      title: t("admin.settings.authSourceDefaults.sources.email.title"),
      description: t("admin.settings.authSourceDefaults.sources.email.description"),
    },
    {
      source: "linuxdo" as AuthSourceType,
      title: t("admin.settings.authSourceDefaults.sources.linuxdo.title"),
      description: t("admin.settings.authSourceDefaults.sources.linuxdo.description"),
    },
    {
      source: "oidc" as AuthSourceType,
      title: t("admin.settings.authSourceDefaults.sources.oidc.title"),
      description: t("admin.settings.authSourceDefaults.sources.oidc.description"),
    },
    {
      source: "wechat" as AuthSourceType,
      title: t("admin.settings.authSourceDefaults.sources.wechat.title"),
      description: t("admin.settings.authSourceDefaults.sources.wechat.description"),
    },
    {
      source: "github" as AuthSourceType,
      title: "GitHub",
      description: localText(
        "通过 GitHub 已验证邮箱首次注册或首次绑定时应用。",
        "Applied on first signup or first bind through a verified GitHub email.",
      ),
    },
    {
      source: "google" as AuthSourceType,
      title: "Google",
      description: localText(
        "通过 Google 已验证邮箱首次注册或首次绑定时应用。",
        "Applied on first signup or first bind through a verified Google email.",
      ),
    },
    {
      source: "dingtalk" as AuthSourceType,
      title: t("auth.dingtalkProviderName"),
      description: localText(
        "通过钉钉首次注册或首次绑定时应用。",
        "Applied on first signup or first bind through DingTalk.",
      ),
    },
  ]);

  return {
    captchaProviderSelection,
    syncCaptchaProviderSelection,
    authSourceDefaults,
    authSourceDefaultsMeta,
  };
}
