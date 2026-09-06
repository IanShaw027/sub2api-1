<template>
  <div class="oauth-section">
    <button type="button" :disabled="disabled" class="btn btn-secondary oauth-btn" @click="startLogin">
      <span class="oauth-mark oauth-mark-dingtalk" aria-hidden="true"></span>
      {{ t('auth.dingtalk.signIn') }}
    </button>

    <div v-if="showDivider" class="oauth-divider">
      <span>{{ t('auth.oauthOrContinue') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import type { OAuthLoginStart } from '@/api/auth'
import { resolveAffiliateReferralCode, storeOAuthAffiliateCode } from '@/utils/oauthAffiliate'
import { sanitizeAuthRedirect } from '@/utils/authRedirect'

const props = withDefaults(defineProps<{
 disabled?: boolean
 affCode?: string
 showDivider?: boolean
}>(), {
 showDivider: true
})
const emit = defineEmits<{
 start: [request: OAuthLoginStart]
}>()

const route = useRoute()
const { t } = useI18n()

function startLogin(): void {
 const redirectTo = sanitizeAuthRedirect(route.query.redirect)
 storeOAuthAffiliateCode(resolveAffiliateReferralCode(props.affCode, route.query.aff, route.query.aff_code))
 emit('start', { provider: 'dingtalk', params: { redirect: redirectTo } })
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
.oauth-mark-dingtalk {
  border-radius: 999px;
  background: oklch(62% 0.18 240);
}
</style>
