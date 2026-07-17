<template>
  <!-- Clomio-style split auth shell: left navy brand panel (lg+), right form card -->
  <div class="flex min-h-screen">
    <!-- Left brand panel (desktop only; fixed deep navy, not theme-toggled) -->
    <div
      class="auth-brand-panel relative hidden w-[45%] flex-col justify-between overflow-hidden p-12 text-white lg:flex"
    >
      <!-- Subtle brand gradient wash -->
      <div
        class="pointer-events-none absolute inset-0 bg-brand-gradient opacity-[0.07]"
        aria-hidden="true"
      />
      <!-- Large cloud watermark (bottom-right) -->
      <BrandLogo
        :size="240"
        icon-only
        class="pointer-events-none absolute -bottom-8 -right-8 opacity-[0.06]"
        aria-hidden="true"
      />

      <!-- Decorative sparkles / dots (CSS-only, no image assets) -->
      <svg
        class="pointer-events-none absolute right-16 top-24 z-[1] text-gold"
        width="30"
        height="30"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        stroke-width="1.5"
        aria-hidden="true"
      >
        <path
          stroke-linecap="round"
          stroke-linejoin="round"
          d="M9.813 15.904L9 18.75l-.813-2.846a4.5 4.5 0 00-3.09-3.09L2.25 12l2.846-.813a4.5 4.5 0 003.09-3.09L9 5.25l.813 2.846a4.5 4.5 0 003.09 3.09L15.75 12l-2.846.813a4.5 4.5 0 00-3.09 3.09zM18.259 8.715L18 9.75l-.259-1.035a3.375 3.375 0 00-2.455-2.456L14.25 6l1.036-.259a3.375 3.375 0 002.455-2.456L18 2.25l.259 1.035a3.375 3.375 0 002.456 2.456L21.75 6l-1.035.259a3.375 3.375 0 00-2.456 2.456z"
        />
      </svg>
      <svg
        class="pointer-events-none absolute bottom-40 left-12 z-[1] text-gold/60"
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill="currentColor"
        aria-hidden="true"
      >
        <path d="M12 2l1.2 3.6L17 7l-3.8 1.4L12 12l-1.2-3.6L7 7l3.8-1.4L12 2z" />
      </svg>
      <span
        class="pointer-events-none absolute left-1/3 top-16 z-[1] h-1.5 w-1.5 rounded-full bg-white/50"
        aria-hidden="true"
      />
      <span
        class="pointer-events-none absolute right-1/4 top-1/2 z-[1] h-1 w-1 rounded-full bg-brand-300/70"
        aria-hidden="true"
      />
      <span
        class="pointer-events-none absolute bottom-24 right-24 z-[1] h-1.5 w-1.5 rounded-full bg-white/30"
        aria-hidden="true"
      />

      <div v-if="settingsLoaded" class="relative z-[1] flex items-center gap-3">
        <div
          v-if="siteLogo"
          class="flex h-10 w-10 items-center justify-center overflow-hidden rounded-xl bg-white/10"
        >
          <img :src="siteLogo" :alt="t('common.logoAlt')" class="h-full w-full object-contain" />
        </div>
        <BrandLogo
          v-else
          :size="40"
          :wordmark="siteName"
          class="auth-brand-logo"
        />
        <span v-if="siteLogo" class="text-xl font-semibold tracking-tight text-white">
          {{ siteName }}
        </span>
      </div>
      <div v-else class="relative z-[1] h-10" />

      <div class="relative z-[1] space-y-5">
        <h1 class="text-3xl font-semibold leading-tight tracking-tight">
          {{ t('auth.brandHeadline', '稳定可靠的 API 网关') }}
        </h1>
        <p class="max-w-sm text-base leading-relaxed text-white/75">
          {{ siteSubtitle }}
        </p>
        <div class="flex flex-wrap gap-2.5 pt-1">
          <span
            v-for="chip in brandChips"
            :key="chip"
            class="inline-flex items-center gap-1.5 rounded-full border border-white/15 bg-white/10 px-3 py-1.5 text-xs text-white/85"
          >
            <span class="h-1.5 w-1.5 rounded-full bg-brand-300" aria-hidden="true" />
            {{ chip }}
          </span>
        </div>
      </div>

      <div class="relative z-[1] text-sm text-white/60">
        &copy; {{ currentYear }} {{ siteName }}
      </div>
    </div>

    <!-- Right form panel -->
    <div class="relative flex flex-1 items-center justify-center overflow-hidden bg-page px-4 py-12 dark:bg-page">
      <span
        class="pointer-events-none absolute -right-24 -top-24 h-96 w-96 rounded-full bg-brand/10 blur-3xl"
        aria-hidden="true"
      />
      <span
        class="pointer-events-none absolute -bottom-32 -left-16 h-72 w-72 rounded-full bg-accent/10 blur-3xl"
        aria-hidden="true"
      />

      <div class="relative z-10 w-full max-w-[360px]">
        <!-- Mobile / form-card brand header -->
        <div v-if="settingsLoaded" class="mb-6 flex flex-col items-center text-center lg:items-start lg:text-left">
          <div
            v-if="siteLogo"
            class="mb-3 inline-flex h-12 w-12 items-center justify-center overflow-hidden rounded-card shadow-md shadow-brand/20"
          >
            <img :src="siteLogo" :alt="t('common.logoAlt')" class="h-full w-full object-contain" />
          </div>
          <BrandLogo
            v-else
            :size="36"
            :wordmark="siteName"
            class="mb-3"
          />
          <p class="text-sm text-ink-soft dark:text-ink-soft lg:hidden">
            {{ siteSubtitle }}
          </p>
        </div>

        <!-- Card Container (slot API unchanged) -->
        <div class="rounded-hero border border-line bg-card px-8 py-9 shadow-lg dark:border-line dark:bg-card">
          <slot />
        </div>

        <!-- Footer Links -->
        <div class="mt-6 text-center text-sm text-ink-soft">
          <slot name="footer" />
        </div>

        <!-- Copyright (mobile; desktop shows on left panel) -->
        <div class="mt-8 text-center text-xs text-ink-faint lg:hidden">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { BrandLogo } from '@/components/brand'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Clomio')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || t('home.siteSubtitleFallback'))
const settingsLoaded = computed(() => appStore.publicSettingsLoaded)

const brandChips = computed(() => [
  t('auth.brandChipKeys', 'API 密钥'),
  t('auth.brandChipBilling', '用量计费'),
  t('auth.brandChipChannels', '多渠道')
])

const currentYear = computed(() => new Date().getFullYear())

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped>
.auth-brand-panel {
  background-color: #0a1a33;
  background-image:
    radial-gradient(closest-side at 18% 22%, rgba(52, 217, 195, 0.3), transparent 70%),
    radial-gradient(closest-side at 88% 72%, rgba(31, 162, 214, 0.32), transparent 72%),
    radial-gradient(closest-side at 62% 42%, rgba(52, 217, 195, 0.1), transparent 70%);
}

/* BrandLogo wordmark on navy panel */
.auth-brand-logo :deep(span) {
  color: #ffffff;
  font-size: 1.25rem;
  line-height: 1.75rem;
}
</style>
