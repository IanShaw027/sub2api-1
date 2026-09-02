<template>
 <div class="auth-page">
 <GlassCard class="auth-brand" padding="lg">
 <div class="auth-brand-inner">
 <div class="auth-brand-mark">
 <img :src="siteLogo || '/logo.svg'" alt="" />
 <span>{{ siteName }}</span>
 </div>
 <div class="auth-brand-copy">
 <h1>{{ siteName }}</h1>
 <p>{{ siteSubtitle }}</p>
 </div>
 </div>
 </GlassCard>

 <div class="auth-form-wrap">
 <GlassCard class="auth-card" padding="lg">
 <slot />
 </GlassCard>
 <div v-if="$slots.footer" class="auth-footer">
 <slot name="footer" />
 </div>
 <p class="auth-copy">&copy; {{ currentYear }} {{ siteName }}. All rights reserved.</p>
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import GlassCard from '@/components/ui/GlassCard.vue'

const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'Subscription to API Conversion Platform')

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
 appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-page {
 min-height: 100vh;
 display: grid;
 grid-template-columns: 1.1fr 1fr;
 background:
 linear-gradient(color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
 linear-gradient(90deg, color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
 radial-gradient(circle at 0% 0%, color-mix(in oklch, var(--accent) 22%, transparent) 0%, transparent 30rem),
 radial-gradient(circle at 100% 0%, color-mix(in oklch, var(--success) 14%, transparent) 0%, transparent 24rem),
 var(--background);
}

.auth-brand {
 margin: 20px 0 20px 20px;
 border-radius: 20px;
 position: relative;
 overflow: hidden;
}

.auth-brand-inner {
 min-height: calc(100vh - 40px);
 display: flex;
 flex-direction: column;
 justify-content: space-between;
 gap: 24px;
 padding: 28px 36px;
}

.auth-brand-mark {
 display: flex;
 align-items: center;
 gap: 10px;
 font-size: 16px;
 font-weight: 700;
}

.auth-brand-mark img {
 width: 32px;
 height: 32px;
 border-radius: 9px;
 object-fit: contain;
}

.auth-brand-copy h1 {
 margin: 0;
 font-family: var(--display);
 font-size: 46px;
 line-height: 1.1;
 font-weight: 800;
 letter-spacing: -0.035em;
 color: var(--foreground);
}

.auth-brand-copy p {
 color: var(--muted);
 font-size: 15px;
 line-height: 1.65;
}

.auth-form-wrap {
 display: flex;
 flex-direction: column;
 align-items: center;
 justify-content: center;
 padding: 40px;
}

.auth-card {
 width: 440px;
 max-width: 100%;
 border-radius: 20px;
}

.auth-footer {
 margin-top: 16px;
 text-align: center;
 font-size: 12.5px;
 color: var(--muted);
 width: 440px;
 max-width: 100%;
}

.auth-copy {
 margin: 16px 0 0;
 text-align: center;
 font-size: 11px;
 color: var(--muted);
}

@media (max-width: 900px) {
 .auth-page {
 grid-template-columns: 1fr;
 }

 .auth-brand {
 display: none;
 }

 .auth-form-wrap {
 padding: 24px 16px;
 }
}
</style>
