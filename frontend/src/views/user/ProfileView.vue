<template>
  <AppLayout>
    <div
      data-testid="profile-shell"
      class="mx-auto max-w-[950px] space-y-6"
    >
      <ProfileInfoCard
        :user="user"
        :linuxdo-enabled="linuxdoOAuthEnabled"
        :dingtalk-enabled="dingtalkOAuthEnabled"
        :oidc-enabled="oidcOAuthEnabled"
        :oidc-provider-name="oidcOAuthProviderName"
        :contact-info="profileContactInfo"
        :support-q-r-codes="profileSupportQRCodes"
        :wechat-enabled="wechatOAuthEnabled"
        :wechat-open-enabled="wechatOAuthOpenEnabled"
        :wechat-mp-enabled="wechatOAuthMPEnabled"
        @balance-history="showBalanceHistory = true"
      />

      <ProfilePasswordForm />

      <ProfileBalanceNotifyCard
        v-if="user && balanceLowNotifyEnabled"
        :enabled="user.balance_notify_enabled ?? true"
        :threshold="user.balance_notify_threshold"
        :extra-emails="user.balance_notify_extra_emails ?? []"
        :system-default-threshold="systemDefaultThreshold"
        :user-email="user.email"
      />

      <ProfileTotpCard />

      <UserBalanceHistoryModal
        :show="showBalanceHistory"
        :email="user?.email"
        :balance="user?.balance || 0"
        @close="showBalanceHistory = false"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProfileBalanceNotifyCard from '@/components/user/profile/ProfileBalanceNotifyCard.vue'
import ProfileInfoCard from '@/components/user/profile/ProfileInfoCard.vue'
import ProfilePasswordForm from '@/components/user/profile/ProfilePasswordForm.vue'
import ProfileTotpCard from '@/components/user/profile/ProfileTotpCard.vue'
import UserBalanceHistoryModal from '@/components/user/UserBalanceHistoryModal.vue'
import { isWeChatWebOAuthEnabled } from '@/api/auth'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'
import type { PublicSettings, SupportQRCodeEntry } from '@/types'

const appStore = useAppStore()
const authStore = useAuthStore()
const user = computed(() => authStore.user)
const showBalanceHistory = ref(false)

type LegacyPublicSettings = PublicSettings & {
  oidc_connect_provider_name?: string
  oidc_connect_enabled?: boolean
}

const publicSettings = ref<LegacyPublicSettings | null>(appStore.cachedPublicSettings as LegacyPublicSettings | null)

const balanceLowNotifyEnabled = computed(() => publicSettings.value?.balance_low_notify_enabled ?? false)
const systemDefaultThreshold = computed(() => publicSettings.value?.balance_low_notify_threshold ?? 0)
const linuxdoOAuthEnabled = computed(() => publicSettings.value?.linuxdo_oauth_enabled ?? false)
const dingtalkOAuthEnabled = computed(() => publicSettings.value?.dingtalk_oauth_enabled ?? false)
const wechatOAuthEnabled = computed(() => {
  const settings = publicSettings.value
  return settings ? isWeChatWebOAuthEnabled(settings) : false
})
const wechatOAuthOpenEnabled = computed<boolean | undefined>(() => {
  const value = publicSettings.value?.wechat_oauth_open_enabled
  return typeof value === 'boolean' ? value : undefined
})
const wechatOAuthMPEnabled = computed<boolean | undefined>(() => {
  const value = publicSettings.value?.wechat_oauth_mp_enabled
  return typeof value === 'boolean' ? value : undefined
})
const oidcOAuthEnabled = computed(() => publicSettings.value?.oidc_oauth_enabled ?? publicSettings.value?.oidc_connect_enabled ?? false)
const oidcOAuthProviderName = computed(() => resolveOidcProviderName(publicSettings.value))
const profileContactInfo = computed(() => publicSettings.value?.contact_info?.trim() || '')
const profileSupportQRCodes = computed<SupportQRCodeEntry[]>(() => (
  Array.isArray(publicSettings.value?.support_qr_codes)
    ? [...(publicSettings.value?.support_qr_codes || [])]
    : []
))

onMounted(async () => {
  const profileRefresh = authStore.refreshUser({ touchActive: true }).catch((error) => {
    console.error('Failed to refresh profile:', error)
  })

  const settingsLoad = appStore.fetchPublicSettings()
    .then((settings) => {
      if (!settings) {
        return
      }
      publicSettings.value = settings as LegacyPublicSettings
    })
    .catch((error) => {
      console.error('Failed to load settings:', error)
    })

  await Promise.all([profileRefresh, settingsLoad])
})

function resolveOidcProviderName(settings: LegacyPublicSettings | null): string {
  const modernName = settings?.oidc_oauth_provider_name?.trim()
  if (modernName) {
    return modernName
  }

  const legacyName = settings?.oidc_connect_provider_name?.trim()
  if (legacyName) {
    return legacyName
  }

  return 'OIDC'
}
</script>
