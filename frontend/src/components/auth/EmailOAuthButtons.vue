<template>
  <div v-if="hasProviders" class="oauth-section">
    <div v-if="showDivider" class="oauth-divider">
      <span>{{ t('auth.oauthOrContinue') }}</span>
    </div>

    <div :class="providerGridClass">
      <button
        v-for="provider in visibleProviders"
        :key="provider"
        type="button"
        :disabled="disabled"
        class="btn btn-secondary oauth-btn"
        @click="startLogin(provider)"
      >
        <span class="oauth-mark oauth-mark-plain" aria-hidden="true">
          <GitHubMark v-if="provider === 'github'" class="oauth-mark-svg" />
          <GoogleMark v-else class="oauth-mark-svg" />
        </span>
        <span class="oauth-btn-label">{{ providerLabel(provider) }}</span>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import GitHubMark from './GitHubMark.vue'
import GoogleMark from './GoogleMark.vue'
import type { OAuthLoginStart } from '@/api/auth'
import { resolveAffiliateReferralCode, storeOAuthAffiliateCode } from '@/utils/oauthAffiliate'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

type EmailOAuthProvider = 'github' | 'google'
const EMAIL_OAUTH_PENDING_PROVIDER_KEY = 'email_oauth_pending_provider'

const props = withDefaults(defineProps<{
 disabled?: boolean
 affCode?: string
 promoCode?: string
 githubEnabled?: boolean
 googleEnabled?: boolean
 showDivider?: boolean
}>(), {
 showDivider: true
})
const emit = defineEmits<{
 start: [request: OAuthLoginStart]
}>()

const route = useRoute()
const { t } = useI18n()

const visibleProviders = computed<EmailOAuthProvider[]>(() => {
 const providers: EmailOAuthProvider[] = []
 if (props.githubEnabled) providers.push('github')
 if (props.googleEnabled) providers.push('google')
 return providers
})

const hasProviders = computed(() => visibleProviders.value.length > 0)
const hasMultipleProviders = computed(() => visibleProviders.value.length > 1)
const providerGridClass = computed(() => [
  'grid',
  'oauth-grid',
  'grid-cols-1',
  hasMultipleProviders.value ? 'sm:grid-cols-2' : ''
])

function providerLabel(provider: EmailOAuthProvider): string {
 const name = provider === 'github' ? 'GitHub' : 'Google'
 return hasMultipleProviders.value ? name : t('auth.emailOAuth.signIn', { providerName: name })
}

function startLogin(provider: EmailOAuthProvider): void {
 const redirectTo = sanitizeAuthRedirect(route.query.redirect)
 const affiliateCode = resolveAffiliateReferralCode(props.affCode, route.query.aff, route.query.aff_code)
 storeOAuthAffiliateCode(affiliateCode)
 window.sessionStorage.setItem(EMAIL_OAUTH_PENDING_PROVIDER_KEY, provider)
 const params: Record<string, string> = { redirect: redirectTo }
 if (affiliateCode) {
 params.aff_code = affiliateCode
 }
 const promoCode = props.promoCode?.trim()
 if (promoCode) {
 params.promo_code = promoCode
 }
 emit('start', { provider, params })
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
.oauth-grid {
  display: grid;
  gap: 10px;
}

.oauth-mark-plain {
  background: transparent;
  box-shadow: none;
  color: var(--foreground);
}

.oauth-mark-svg {
  width: 18px;
  height: 18px;
}
</style>
