<template>
 <header
 class="glass sticky top-0 z-30 border-b border-line/50"
 >
 <div class="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3.5 sm:px-6">
 <!-- 左:站点 logo + 名称 -->
 <div class="flex min-w-0 items-center gap-3">
 <template v-if="settings">
 <span
 class="flex h-9 w-9 flex-shrink-0 items-center justify-center overflow-hidden rounded-xl bg-surface shadow-sm ring-1 ring-line"
 >
 <img :src="siteLogo || '/logo.svg'" alt="Logo" class="h-full w-full object-contain" />
 </span>
 <span class="truncate text-base font-semibold text-foreground">
 {{ siteName }}
 </span>
 </template>
 <template v-else>
 <span class="h-9 w-9 flex-shrink-0 animate-pulse rounded-xl bg-surface-2" aria-hidden="true"></span>
 <span class="h-5 w-28 animate-pulse rounded bg-surface-2" aria-hidden="true"></span>
 </template>
 </div>

 <!-- 右:登录 / 回到后台 -->
 <RouterLink
 v-if="isAuthenticated"
 :to="backTarget"
 class="inline-flex flex-shrink-0 items-center justify-center gap-1.5 rounded-xl bg-gradient-to-r from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)] px-4 py-2 text-sm font-semibold text-white shadow-md shadow-[color-mix(in_oklch,var(--accent)_25%,transparent)] transition-all duration-200 hover:opacity-90 hover:shadow-lg hover:shadow-[color-mix(in_oklch,var(--accent)_30%,transparent)] active:scale-[0.98]"
 >
 {{ t('modelPlaza.nav.backToDashboard') }}
 </RouterLink>
 <RouterLink
 v-else
 :to="{ path: '/login', query: { redirect: '/model-plaza' } }"
 class="inline-flex flex-shrink-0 items-center justify-center rounded-xl bg-gradient-to-r from-[var(--accent)] to-[color-mix(in_oklch,var(--accent)_80%,black)] px-4 py-2 text-sm font-semibold text-white shadow-md shadow-[color-mix(in_oklch,var(--accent)_25%,transparent)] transition-all duration-200 hover:opacity-90 hover:shadow-lg hover:shadow-[color-mix(in_oklch,var(--accent)_30%,transparent)] active:scale-[0.98]"
 >
 {{ t('modelPlaza.nav.login') }}
 </RouterLink>
 </div>
 </header>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { sanitizeUrl } from '@/utils/url'
import { useAppStore } from '@/stores/app'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const appStore = useAppStore()
const authStore = useAuthStore()

const settings = computed(() => appStore.cachedPublicSettings)
const siteName = computed(() => settings.value?.site_name || 'Sub2API')
const siteLogo = computed(() =>
 sanitizeUrl(settings.value?.site_logo || '', { allowRelative: true, allowDataUrl: true })
)
const isAuthenticated = computed(() => authStore.isAuthenticated)
const backTarget = computed(() => (authStore.isAdmin ? '/admin/dashboard' : '/dashboard'))
</script>
