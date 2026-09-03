<template>
  <section class="auth-brand">
    <div class="auth-brand-texture" aria-hidden="true"></div>
    <div class="auth-brand-orb" aria-hidden="true"></div>

    <div class="auth-brand-top">
      <span class="brand-mark brand-mark-lg">
        <img v-if="siteLogo" :src="siteLogo" alt="" />
        <template v-else>{{ siteInitial }}</template>
      </span>
      <span class="auth-brand-name">{{ siteName }}</span>
    </div>

    <div class="auth-brand-copy">
      <slot>
        <h1>{{ brandTitle }}</h1>
        <p>{{ brandDescription }}</p>
        <div class="auth-brand-features">
          <div v-for="feature in brandFeatures" :key="feature" class="auth-brand-feature">
            <span class="auth-brand-check" aria-hidden="true">
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round">
                <path d="M20 6L9 17l-5-5" />
              </svg>
            </span>
            <span>{{ feature }}</span>
          </div>
        </div>
      </slot>
    </div>

    <div class="auth-brand-foot">
      <StatusBadge tone="success" dot :label="t('auth.brand.serviceNormal')" />
      <span>{{ providerLine }}</span>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import StatusBadge from '@/components/ui/StatusBadge.vue'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(
  () => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API'
)
const siteInitial = computed(() => siteName.value.trim().charAt(0).toUpperCase() || 'S')
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true
  })
)
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')

const brandTitle = computed(() => t('home.heroSubtitle'))
const brandDescription = computed(() => siteSubtitle.value || t('home.heroDescription'))
const brandFeatures = computed(() => [
  `${t('home.features.unifiedGateway')} · ${t('home.features.unifiedGatewayDesc')}`,
  `${t('home.features.multiAccount')} · ${t('home.features.multiAccountDesc')}`,
  `${t('home.features.balanceQuota')} · ${t('home.features.balanceQuotaDesc')}`
])

/**
 * Provider line: derive from public settings when the backend exposes an
 * enabled-platform list, otherwise fall back to the static i18n copy.
 */
const providerLine = computed(() => {
  const settings = appStore.cachedPublicSettings as Record<string, unknown> | null | undefined
  const raw = settings?.enabled_platforms ?? settings?.supported_platforms
  const list = Array.isArray(raw)
    ? raw.filter((item): item is string => typeof item === 'string' && item.trim().length > 0)
    : []
  if (list.length > 0) {
    return `${t('home.providers.supported')} ${list.join(' · ')}`
  }
  return `${t('home.providers.supported')} ${t('home.providers.line')}`
})

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-brand {
  position: relative;
  overflow: hidden;
  margin: 20px 0 20px 20px;
  padding: 48px 56px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  background: color-mix(in oklch, var(--surface) 55%, transparent);
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow: var(--shadow);
}

.auth-brand-texture {
  position: absolute;
  inset: 0;
  background: radial-gradient(color-mix(in oklch, var(--foreground) 9%, transparent) 1px, transparent 1.3px) 0 0 / 18px 18px;
  mask-image: linear-gradient(200deg, transparent 30%, black 100%);
  -webkit-mask-image: linear-gradient(200deg, transparent 30%, black 100%);
  pointer-events: none;
}

.auth-brand-orb {
  position: absolute;
  right: -140px;
  bottom: -160px;
  width: 520px;
  height: 520px;
  border-radius: 50%;
  background: radial-gradient(
    circle at 35% 35%,
    color-mix(in oklch, var(--accent) 45%, white) 0%,
    color-mix(in oklch, var(--accent) 35%, transparent) 45%,
    transparent 70%
  );
  pointer-events: none;
}

.auth-brand-top {
  position: relative;
  display: flex;
  align-items: center;
  gap: 10px;
}

.auth-brand-name {
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.auth-brand-copy {
  position: relative;
  display: flex;
  flex-direction: column;
  gap: 22px;
  max-width: 520px;
}

.auth-brand-copy :deep(h1) {
  margin: 0;
  font-family: var(--display);
  font-size: 46px;
  line-height: 1.1;
  font-weight: 800;
  letter-spacing: -0.035em;
  text-wrap: balance;
}

.auth-brand-copy :deep(p) {
  margin: 0;
  font-size: 15px;
  line-height: 1.65;
  color: var(--muted);
}

.auth-brand-features {
  display: flex;
  flex-direction: column;
  gap: 12px;
  font-size: 14px;
}

.auth-brand-feature {
  display: flex;
  align-items: center;
  gap: 10px;
}

.auth-brand-check {
  width: 22px;
  height: 22px;
  flex: none;
  border-radius: 999px;
  background: color-mix(in oklch, var(--success) 18%, transparent);
  color: var(--success-text);
  display: inline-flex;
  align-items: center;
  justify-content: center;
}

.auth-brand-foot {
  position: relative;
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12.5px;
  color: var(--muted);
}
</style>
