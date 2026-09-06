<template>
 <AppLayout>
 <SettingsPageLayout>
 <template #header>
 <PageHeader :title="t('profile.title')" :description="t('profile.description')" />
 </template>

 <template #nav>
 <nav
 class="glass-card sticky top-[76px] flex flex-col gap-1 p-2"
 :aria-label="t('profile.title')"
 >
 <a
 v-for="item in navItems"
 :key="item.id"
 :href="`#${item.id}`"
 class="rounded-lg px-3 py-2 text-sm font-medium text-muted transition-colors hover:bg-surface-2 hover:text-foreground"
 >
 {{ item.label }}
 </a>
 </nav>
 </template>

 <template #content>
 <div
 data-testid="profile-shell"
 class="flex flex-col gap-3"
 >
 <div
 id="profile-section-profile"
 class="flex scroll-mt-20 flex-col gap-3"
 >
 <ProfileInfoCard
 :user="user"
 :oidc-provider-name="oidcOAuthProviderName"
 />

 <div
 v-if="contactInfo || supportQRCodes.length > 0"
 data-testid="profile-support-panel"
 class="glass-card border-[color-mix(in_oklch,var(--accent)_28%,transparent)] bg-[color-mix(in_oklch,var(--accent)_10%,transparent)] p-6"
 >
 <div class="flex items-start gap-4">
 <div class="rounded-xl bg-[color-mix(in_oklch,var(--accent)_14%,transparent)] p-3 text-accent">
 <Icon name="chat" size="lg" />
 </div>
 <div class="min-w-0 flex-1 space-y-4">
 <div>
 <h3 class="font-semibold text-accent">
 {{ t('common.contactSupport') }}
 </h3>
 <p v-if="contactInfo" class="text-sm font-medium">{{ contactInfo }}</p>
 </div>
 <div
 v-if="supportQRCodes.length > 0"
 data-testid="profile-support-qr-grid"
 class="grid gap-4 sm:grid-cols-2"
 >
 <div
 v-for="(qrCode, index) in supportQRCodes"
 :key="`${qrCode.image_url}-${index}`"
 class="overflow-hidden rounded-xl bg-surface p-3 "
 >
 <img
 :src="qrCode.image_url"
 :alt="qrCode.note || t('common.contactSupport')"
 class="aspect-square w-full object-contain"
 >
 <p
 v-if="qrCode.note"
 class="mt-2 text-center text-xs text-muted"
 >
 {{ qrCode.note }}
 </p>
 </div>
 </div>
 </div>
 </div>
 </div>
 </div>

 <SettingsSection
 id="profile-section-security"
 class="scroll-mt-20"
 :title="t('profile.securityTitle')"
 :description="t('profile.securityDescription')"
 >
 <ProfilePasswordForm embedded />
 <SettingRow
 :label="t('profile.totp.title')"
 :description="t('profile.totp.description')"
 >
 <ProfileTotpCard embedded />
 </SettingRow>
 <SettingRow
 :label="t('profile.passkey.title')"
 :description="t('profile.passkey.description')"
 >
 <ProfilePasskeyCard :enabled="passkeyEnabled" embedded />
 </SettingRow>
 </SettingsSection>

 <SettingsSection
 v-if="user && balanceLowNotifyEnabled"
 id="profile-section-notify"
 class="scroll-mt-20"
 :title="t('profile.balanceNotify.title')"
 :description="t('profile.balanceNotify.description')"
 >
 <ProfileBalanceNotifyCard
 :enabled="user.balance_notify_enabled ?? true"
 :threshold="user.balance_notify_threshold"
 :extra-emails="user.balance_notify_extra_emails ?? []"
 :system-default-threshold="systemDefaultThreshold"
 :user-email="user.email"
 embedded
 />
 </SettingsSection>

 <SettingsSection
 id="profile-section-bindings"
 class="scroll-mt-20"
 :title="t('profile.authBindings.title')"
 :description="t('profile.authBindings.description')"
 >
 <ProfileIdentityBindingsSection
 :user="user"
 :linuxdo-enabled="linuxdoOAuthEnabled"
 :dingtalk-enabled="dingtalkOAuthEnabled"
 :oidc-enabled="oidcOAuthEnabled"
 :oidc-provider-name="oidcOAuthProviderName"
 :wechat-enabled="wechatOAuthEnabled"
 :wechat-open-enabled="wechatOAuthOpenEnabled"
 :wechat-mp-enabled="wechatOAuthMPEnabled"
 embedded
 compact
 />
 </SettingsSection>

 <SettingsSection
 data-testid="profile-danger-zone"
 id="profile-section-danger"
 class="scroll-mt-20 border-danger-200"
 :title="t('profile.dangerZone.title')"
 :description="t('profile.dangerZone.description')"
 >
 <SettingRow :label="t('profile.dangerZone.title')">
 <p class="text-sm text-muted">
 {{ t('profile.dangerZone.noActionsAvailable') }}
 </p>
 </SettingRow>
 </SettingsSection>
 </div>
 </template>
 </SettingsPageLayout>
 </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Icon } from '@/components/icons'
import AppLayout from '@/components/layout/AppLayout.vue'
import SettingsPageLayout from '@/components/layout/SettingsPageLayout.vue'
import PageHeader from '@/components/ui/PageHeader.vue'
import SettingsSection from '@/components/ui/SettingsSection.vue'
import SettingRow from '@/components/ui/SettingRow.vue'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfileIdentityBindingsSection from '@/components/user/profile/ProfileIdentityBindingsSection.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import ProfilePasskeyCard from '@/components/user/profile/ProfilePasskeyCard.vue'
import { isWeChatWebOAuthEnabled } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { SupportQRCodeEntry } from '@/types'
import { sanitizeSupportQRUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()
const user = computed(() => authStore.user)

const contactInfo = ref('')
const supportQRCodes = ref<SupportQRCodeEntry[]>([])
const balanceLowNotifyEnabled = ref(false)
const systemDefaultThreshold = ref(0)
const linuxdoOAuthEnabled = ref(false)
const dingtalkOAuthEnabled = ref(false)
const wechatOAuthEnabled = ref(false)
const wechatOAuthOpenEnabled = ref<boolean | undefined>(undefined)
const wechatOAuthMPEnabled = ref<boolean | undefined>(undefined)
const oidcOAuthEnabled = ref(false)
const oidcOAuthProviderName = ref('OIDC')
const passkeyEnabled = ref(false)

const navItems = computed(() => {
  const items = [
    { id: 'profile-section-profile', label: t('profile.nav.profile') },
    { id: 'profile-section-security', label: t('profile.nav.security') }
  ]
  if (user.value && balanceLowNotifyEnabled.value) {
    items.push({ id: 'profile-section-notify', label: t('profile.nav.notify') })
  }
  items.push({ id: 'profile-section-bindings', label: t('profile.nav.bindings') })
  items.push({ id: 'profile-section-danger', label: t('profile.nav.danger') })
  return items
})

onMounted(async () => {
  const profileRefresh = authStore.refreshUser().catch((error) => {
    console.error('Failed to refresh profile:', error)
  })

  const settingsLoad = appStore.fetchPublicSettings()
    .then((settings) => {
      if (!settings) {
        return
      }
      contactInfo.value = settings.contact_info || ''
      supportQRCodes.value = Array.isArray(settings.support_qr_codes)
        ? settings.support_qr_codes
          .map((entry) => ({
            image_url: sanitizeSupportQRUrl(typeof entry?.image_url === 'string' ? entry.image_url : ''),
            note: entry?.note?.trim() || '',
          }))
          .filter((entry) => entry.image_url)
        : []
      balanceLowNotifyEnabled.value = settings.balance_low_notify_enabled ?? false
      systemDefaultThreshold.value = settings.balance_low_notify_threshold ?? 0
      linuxdoOAuthEnabled.value = settings.linuxdo_oauth_enabled ?? false
      dingtalkOAuthEnabled.value = settings.dingtalk_oauth_enabled ?? false
      wechatOAuthEnabled.value = isWeChatWebOAuthEnabled(settings)
      wechatOAuthOpenEnabled.value = typeof settings.wechat_oauth_open_enabled === 'boolean'
        ? settings.wechat_oauth_open_enabled
        : undefined
      wechatOAuthMPEnabled.value = typeof settings.wechat_oauth_mp_enabled === 'boolean'
        ? settings.wechat_oauth_mp_enabled
        : undefined
      oidcOAuthEnabled.value = settings.oidc_oauth_enabled ?? false
      oidcOAuthProviderName.value = settings.oidc_oauth_provider_name || 'OIDC'
      passkeyEnabled.value = settings.passkey_enabled === true
    })
    .catch((error) => {
      console.error('Failed to load settings:', error)
    })

  await Promise.all([profileRefresh, settingsLoad])
})
</script>

<style scoped>
/* SettingsPageLayout's own flex `gap: 14px` between #header and the nav/content
   grid already provides the header-to-content spacing; PageHeader's own
   default 14px margin-bottom would double it (28px). Zeroed here the same way
   admin/SettingsView.vue's settings.css does for its own PageHeader usage
   (see that file's comment) — this scoped rule reaches PageHeader's root
   `<header>` because Vue stamps a parent's scope id onto a slotted child's
   root element, even when passed through SettingsPageLayout's `#header` slot. */
.ui-page-header.ui-page-header {
  margin-bottom: 0;
}
</style>
