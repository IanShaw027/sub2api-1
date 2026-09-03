<template>
  <div class="oauth-section">
    <button type="button" :disabled="buttonDisabled" class="btn btn-secondary oauth-btn" @click="startLogin">
      <span class="oauth-mark oauth-mark-wechat" aria-hidden="true">
        <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor" aria-hidden="true">
          <path d="M9.2 3C5.2 3 2 5.8 2 9.2c0 1.9 1 3.6 2.7 4.8l-.7 2.1 2.4-1.2c.8.2 1.6.4 2.5.4h.5a5.9 5.9 0 0 1-.2-1.5c0-3.3 3.1-5.9 6.9-5.9h.6C16.1 5 12.9 3 9.2 3Zm-2.5 3.4a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8Zm5 0a.9.9 0 1 1 0 1.8.9.9 0 0 1 0-1.8ZM16.1 9.4c-3.3 0-6 2.2-6 4.9s2.7 4.9 6 4.9c.7 0 1.4-.1 2-.3l2 1-.6-1.8c1.4-.9 2.4-2.3 2.4-3.8 0-2.7-2.6-4.9-5.8-4.9Zm-2 2.6a.8.8 0 1 1 0 1.6.8.8 0 0 1 0-1.6Zm4.1 0a.8.8 0 1 1 0 1.6.8.8 0 0 1 0-1.6Z" />
        </svg>
      </span>
      <span class="oauth-btn-label">{{ t('auth.oidc.signIn', { providerName }) }}</span>
    </button>

    <p
      v-if="disabledHint"
      data-testid="wechat-oauth-hint"
      class="oauth-hint"
    >
      {{ disabledHint }}
    </p>

    <div v-if="showDivider" class="oauth-divider">
      <span>{{ t('auth.oauthOrContinue') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { resolveWeChatOAuthStart, type OAuthLoginStart } from '@/api/auth'
import { useAppStore } from '@/stores'
import { resolveAffiliateReferralCode, storeOAuthAffiliateCode } from '@/utils/oauthAffiliate'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

const props = withDefaults(defineProps<{
 disabled?: boolean
 affCode?: string
 showDivider?: boolean
}>(), {
 showDivider: true,
})
const emit = defineEmits<{
 start: [request: OAuthLoginStart]
}>()

const appStore = useAppStore()
const route = useRoute()
const { t, locale } = useI18n()
const providerName = computed(() => t('auth.wechatProviderName'))

function localizeWeChatHint(zh: string, en: string): string {
 return locale.value.startsWith('zh') ? zh : en
}

const resolvedStart = computed(() => resolveWeChatOAuthStart(appStore.cachedPublicSettings))
const buttonDisabled = computed(() => props.disabled || resolvedStart.value.mode === null)
const disabledHint = computed(() => {
 if (props.disabled) {
 return ''
 }
 switch (resolvedStart.value.unavailableReason) {
 case 'external_browser_required':
 return t('auth.oauthFlow.wechatSystemBrowserOnly')
 case 'wechat_browser_required':
 return t('auth.oauthFlow.wechatBrowserOnly')
 case 'native_app_required':
 return localizeWeChatHint(
 '当前仅配置微信移动应用登录，需要在原生 App 中通过微信 SDK 发起授权。',
 'This site only has WeChat mobile app login configured. Continue from the native app through the WeChat SDK.',
 )
 case 'not_configured':
 return t('auth.oauthFlow.wechatNotConfigured')
 default:
 return ''
 }
})

onMounted(() => {
 if (!appStore.cachedPublicSettings && !appStore.publicSettingsLoaded) {
 appStore.fetchPublicSettings()
 }
})

function startLogin(): void {
 if (buttonDisabled.value || !resolvedStart.value.mode) {
 return
 }
 const redirectTo = sanitizeAuthRedirect(route.query.redirect)
 storeOAuthAffiliateCode(resolveAffiliateReferralCode(props.affCode, route.query.aff, route.query.aff_code))
 const mode = resolvedStart.value.mode
 emit('start', {
 provider: 'wechat',
 params: { mode, redirect: redirectTo }
 })
}
</script>

<style scoped>
.oauth-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
}

.oauth-btn {
  width: 100%;
  height: 40px;
  border-radius: 12px;
  font-size: 13.5px;
  font-weight: 600;
  gap: 8px;
  min-width: 0;
}

.oauth-btn > span:last-child,
.oauth-btn-label {
  overflow: hidden;
  text-overflow: ellipsis;
}

.oauth-mark {
  width: 18px;
  height: 18px;
  flex: none;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border-radius: 5px;
  overflow: hidden;
  font-size: 10px;
  font-weight: 800;
  color: #fff;
  box-shadow: inset 0 0 0 1px rgba(255, 255, 255, 0.14);
}

.oauth-divider {
  display: flex;
  align-items: center;
  gap: 12px;
  font-size: 12px;
  color: var(--muted);
}

.oauth-divider::before,
.oauth-divider::after {
  content: '';
  flex: 1;
  height: 1px;
  background: var(--border);
}
.oauth-mark-wechat {
  background: #07c160;
  border-radius: 999px;
}

.oauth-hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--warning-text);
}
</style>
