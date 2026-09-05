<template>
  <div class="ptb">
    <!-- Row 1: 20px brand tile + platform name + `.tag` type chip -->
    <div class="ptb-row">
      <span class="ptb-tile" :style="{ background: platformTileBackground(platform) }" aria-hidden="true">
        <PlatformIcon :platform="platform" size="xs" />
      </span>
      <span class="ptb-name">{{ platformLabel }}</span>
      <span class="tag">
        <!-- OAuth icon -->
        <svg
          v-if="type === 'oauth'"
          class="h-3 w-3"
          fill="none"
          viewBox="0 0 24 24"
          stroke="currentColor"
          stroke-width="2"
        >
          <path
            stroke-linecap="round"
            stroke-linejoin="round"
            d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"
          />
        </svg>
        <!-- Setup Token icon -->
        <Icon v-else-if="type === 'setup-token'" name="shield" size="xs" />
        <!-- API Key icon -->
        <Icon v-else-if="type === 'service_account'" name="cloud" size="xs" />
        <Icon v-else name="key" size="xs" />
        <span>{{ typeLabel }}</span>
      </span>
    </div>
    <!-- Row 2: Plan type + Privacy mode (only if either exists) -->
    <div v-if="planLabel || privacyBadge" class="ptb-row">
      <span v-if="planLabel" :class="planBadgeClass">
        <GrokFreeIcon
          v-if="isGrokFreePlan"
          data-testid="grok-free-plan-icon"
        />
        <Icon
          v-else-if="planIconName"
          :name="planIconName"
          size="xs"
          data-testid="grok-plan-icon"
          aria-hidden="true"
        />
        <span>{{ planLabel }}</span>
      </span>
      <span
        v-if="privacyBadge"
        :class="privacyBadge.class"
        :title="privacyBadge.title"
      >
        <svg class="h-3 w-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
          <path stroke-linecap="round" stroke-linejoin="round" :d="privacyBadge.icon" />
        </svg>
        <span>{{ privacyBadge.label }}</span>
      </span>
    </div>
    <!-- Row 3: Subscription expiration (non-free paid accounts only) -->
    <div v-if="expiresLabel" class="ptb-expires" :title="subscriptionExpiresAt">
      {{ expiresLabel }}
    </div>
  </div>
</template>


<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AccountPlatform, AccountType } from '@/types'
import GrokFreeIcon from './GrokFreeIcon.vue'
import PlatformIcon from './PlatformIcon.vue'
import Icon from '@/components/icons/Icon.vue'
import { platformTileBackground } from '@/utils/platformTile'

const { t } = useI18n()

interface Props {
 platform: AccountPlatform
 type: AccountType
 authMode?: string
 planType?: string
 privacyMode?: string
 subscriptionExpiresAt?: string
}

const props = defineProps<Props>()

const platformLabel = computed(() => {
 if (props.platform === 'anthropic') return 'Anthropic'
 if (props.platform === 'openai') return 'OpenAI'
 if (props.platform === 'antigravity') return 'Antigravity'
 if (props.platform === 'grok') return 'Grok'
 if (props.platform === 'kiro') return 'Kiro'
 if (props.platform === 'kimi') return 'Kimi'
 if (props.platform === 'zhipu') return 'Zhipu GLM'
 if (props.platform === 'deepseek') return 'DeepSeek'
 return 'Gemini'
})

const normalizedAuthMode = computed(() =>
 (props.authMode || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
)

const typeLabel = computed(() => {
 if (props.platform === 'openai' && props.type === 'oauth') {
 if (normalizedAuthMode.value === 'agentidentity') return 'Agent Identity'
 if (normalizedAuthMode.value === 'personalaccesstoken') return 'PAT'
 }
 switch (props.type) {
 case 'oauth':
 return 'OAuth'
 case 'setup-token':
 return 'Token'
 case 'apikey':
 return 'Key'
 case 'bedrock':
 return 'AWS'
 case 'service_account':
 return 'Vertex'
 default:
 return props.type
 }
})

const normalizedPlanType = computed(() =>
 (props.planType || '').trim().toLowerCase().replace(/[\s_-]+/g, '')
)

const planLabel = computed(() => {
 if (!normalizedPlanType.value) return ''
 switch (normalizedPlanType.value) {
 case 'plus':
 return 'Plus'
 case 'team':
 return 'Team'
 case 'chatgptpro':
 case 'pro':
 return 'Pro'
 case 'free':
 case 'basic':
 return props.platform === 'grok' ? 'Grok Free' : 'Free'
 case 'supergrok':
 return 'SuperGrok'
 case 'supergroklite':
 return 'SuperGrok Lite'
 case 'supergrokplus':
 return 'SuperGrok Plus'
 case 'supergrokheavy':
 return 'SuperGrok Heavy'
 case 'heavy':
 return 'Heavy'
 case 'xbasic':
 return 'X Basic'
 case 'abnormal':
 return t('admin.accounts.subscriptionAbnormal')
 default:
 return props.planType
 }
})

const isGrokFreePlan = computed(() =>
 props.platform === 'grok' &&
 (normalizedPlanType.value === 'free' ||
 normalizedPlanType.value === 'basic' ||
 normalizedPlanType.value === 'xbasic')
)

const planIconName = computed<'bolt' | null>(() => {
 if (props.platform !== 'grok') return null
 // Paid Grok tiers (SuperGrok / Heavy) share the bolt mark; free uses GrokFreeIcon.
 if (
 normalizedPlanType.value === 'supergrok' ||
 normalizedPlanType.value === 'supergrokheavy' ||
 normalizedPlanType.value === 'heavy' ||
 normalizedPlanType.value.includes('heavy')
 ) {
 return 'bolt'
 }
 return null
})

const typeClass = computed(() => 'tag')

const planBadgeClass = computed(() => {
 if (normalizedPlanType.value === 'abnormal') {
 return 'tag tag-danger'
 }
 // Free stays muted gray; paid Grok tiers get distinct colors.
 if (
 normalizedPlanType.value === 'free' ||
 normalizedPlanType.value === 'basic' ||
 normalizedPlanType.value === 'xbasic'
 ) {
 return 'tag bg-surface-2 text-muted'
 }
 if (props.platform === 'grok' && normalizedPlanType.value) {
 // Heavy / SuperGrok Heavy → purple
 if (normalizedPlanType.value.includes('heavy')) {
 return 'tag bg-accent-100 text-accent-600'
 }
 // SuperGrok → cyan
 if (normalizedPlanType.value.includes('supergrok')) {
 return 'tag bg-accent-100 text-accent-700'
 }
 // Any other non-free Grok plan (future tiers) → amber so it still stands out
 return 'tag tag-warning'
 }
 // OpenAI / other paid plan labels: keep readable distinction from free gray
 if (normalizedPlanType.value === 'plus') {
 return 'tag bg-accent-100 text-accent-700'
 }
 if (normalizedPlanType.value === 'team') {
 return 'tag bg-accent-100 text-accent-700'
 }
 if (normalizedPlanType.value === 'pro' || normalizedPlanType.value === 'chatgptpro') {
 return 'tag bg-accent-100 text-accent-700'
 }
 return typeClass.value
})

// Subscription expiration label (non-free only)
const expiresLabel = computed(() => {
 if (!props.subscriptionExpiresAt || !props.planType) return ''
 if (
 normalizedPlanType.value === 'free' ||
 normalizedPlanType.value === 'basic' ||
 normalizedPlanType.value === 'xbasic'
 ) return ''
 try {
 const d = new Date(props.subscriptionExpiresAt)
 if (isNaN(d.getTime())) return ''
 const yyyy = d.getFullYear()
 const mm = String(d.getMonth() + 1).padStart(2, '0')
 const dd = String(d.getDate()).padStart(2, '0')
 return `${t('admin.accounts.subscriptionExpires')} ${yyyy}-${mm}-${dd}`
 } catch {
 return ''
 }
})

// Privacy badge — shows different states for OpenAI/Antigravity OAuth privacy setting
const privacyBadge = computed(() => {
 if (props.type !== 'oauth' || !props.privacyMode) return null
 // 支持 OpenAI 和 Antigravity 平台
 if (props.platform !== 'openai' && props.platform !== 'antigravity') return null

 const shieldCheck = 'M9 12.75L11.25 15 15 9.75m-3-7.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285z'
 const shieldX = 'M12 9v3.75m0-10.036A11.959 11.959 0 013.598 6 11.99 11.99 0 003 9.749c0 5.592 3.824 10.29 9 11.623 5.176-1.332 9-6.03 9-11.622 0-1.31-.21-2.571-.598-3.751h-.152c-3.196 0-6.1-1.248-8.25-3.285zM12 18h.008v.008H12V18z'
 switch (props.privacyMode) {
 // OpenAI states
 case 'training_off':
 return { label: 'Private', icon: shieldCheck, title: t('admin.accounts.privacyTrainingOff'), class: 'tag tag-success' }
 case 'training_set_cf_blocked':
 return { label: 'CF', icon: shieldX, title: t('admin.accounts.privacyCfBlocked'), class: 'tag tag-warning' }
 case 'training_set_failed':
 return { label: 'Fail', icon: shieldX, title: t('admin.accounts.privacyFailed'), class: 'tag tag-danger' }
 // Antigravity states
 case 'privacy_set':
 return { label: 'Private', icon: shieldCheck, title: t('admin.accounts.privacyAntigravitySet'), class: 'tag tag-success' }
 case 'privacy_set_failed':
 return { label: 'Fail', icon: shieldX, title: t('admin.accounts.privacyAntigravityFailed'), class: 'tag tag-danger' }
 default:
 return null
 }
})
</script>

<style scoped>
.ptb {
  display: inline-flex;
  flex-direction: column;
  gap: 4px;
}

.ptb-row {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

/* 20px 品牌底板 · 圆角 6，内描边 rgba(255,255,255,.14)，字形留白 */
.ptb-tile {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 20px;
  height: 20px;
  flex: none;
  border-radius: 6px;
  color: white;
  box-shadow: inset 0 0 0 1px color-mix(in oklch, white 14%, transparent);
}

.ptb-name {
  font-size: 12.5px;
  font-weight: 600;
  color: var(--foreground);
  white-space: nowrap;
}

.ptb-expires {
  font-size: 10.5px;
  line-height: 1.3;
  color: var(--muted);
}
</style>
