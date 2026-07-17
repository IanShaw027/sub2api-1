<template>
  <!-- Native Clomio API Platform landing — always native Vue (home_content ignored) -->
  <div class="relative flex min-h-screen flex-col overflow-hidden bg-page text-ink-body ambient-canvas">
    <!-- TopNav -->
    <header class="relative z-20 border-b border-line/60 bg-page/70 px-6 py-3.5 backdrop-blur-md dark:border-dark-700/60 dark:bg-page/60">
      <nav class="mx-auto flex max-w-6xl items-center justify-between gap-4">
        <!-- Brand -->
        <router-link to="/home" class="flex min-w-0 items-center gap-2.5 no-underline">
          <img
            v-if="siteLogo"
            :src="siteLogo"
            alt=""
            class="h-9 w-9 shrink-0 rounded-xl object-contain shadow-xs"
          />
          <BrandLogo v-else :size="32" :wordmark="siteName" />
          <span
            v-if="siteLogo"
            class="truncate text-lg font-[650] tracking-[-0.01em] text-ink dark:text-ink"
          >
            {{ siteName }}
          </span>
        </router-link>

        <!-- Actions -->
        <div class="flex shrink-0 items-center gap-2 sm:gap-3">
          <LocaleSwitcher />

          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="rounded-lg p-2 text-ink-soft transition-colors hover:bg-page hover:text-ink dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>

          <button
            type="button"
            class="rounded-lg p-2 text-ink-soft transition-colors hover:bg-page hover:text-ink dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>

          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="inline-flex items-center gap-1.5 rounded-full bg-ink py-1 pl-1 pr-2.5 text-white transition-opacity hover:opacity-90 dark:bg-dark-700"
          >
            <span
              class="flex h-5 w-5 items-center justify-center rounded-full bg-brand-gradient text-[10px] font-semibold text-white"
            >
              {{ userInitial }}
            </span>
            <span class="text-xs font-medium">{{ t('home.dashboard') }}</span>
            <Icon name="arrowRight" size="xs" class="text-white/50" />
          </router-link>
          <router-link
            v-else
            to="/login"
            class="inline-flex items-center rounded-full bg-ink px-3 py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90 dark:bg-dark-700"
          >
            {{ t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="relative z-10 flex-1">
      <!-- Hero -->
      <section class="px-6 pb-12 pt-12 sm:pt-16 lg:pb-16 lg:pt-20">
        <div class="mx-auto grid max-w-6xl items-center gap-10 lg:grid-cols-2 lg:gap-14">
          <!-- Left: copy + CTAs -->
          <div class="text-center lg:text-left">
            <span
              class="mb-4 inline-flex items-center rounded-full border border-brand/20 bg-brand/10 px-3 py-1 text-[12px] font-medium text-brand-700 dark:border-brand/30 dark:bg-brand/15 dark:text-brand-300"
            >
              {{ t('home.eyebrow') }}
            </span>

            <h1
              class="mb-4 text-[32px] font-[650] leading-[1.15] tracking-[-0.02em] text-ink md:text-[40px] dark:text-white"
            >
              {{ t('home.heroTitle') }}
            </h1>

            <p class="mb-8 text-base leading-relaxed text-ink-body md:text-lg dark:text-dark-300">
              {{ heroSubtitleText }}
            </p>

            <div class="flex flex-wrap items-center justify-center gap-3 lg:justify-start">
              <router-link
                :to="isAuthenticated ? dashboardPath : '/login'"
                class="btn btn-brand btn-lg inline-flex items-center gap-2 px-7 shadow-md shadow-brand/25"
              >
                {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
                <Icon name="arrowRight" size="md" :stroke-width="2" />
              </router-link>

              <a
                v-if="docUrl"
                :href="docUrl"
                target="_blank"
                rel="noopener noreferrer"
                class="btn btn-secondary btn-lg inline-flex items-center gap-2 px-6"
              >
                <Icon name="book" size="md" />
                {{ t('home.viewDocs') }}
              </a>
            </div>
          </div>

          <!-- Right: CodeCard -->
          <div class="flex justify-center lg:justify-end">
            <div class="w-full max-w-lg">
              <CodeCard :api-base-url="apiBaseUrl" />
            </div>
          </div>
        </div>
      </section>

      <!-- Trust strip: 4 capability facts (no fake logos) -->
      <section class="border-y border-line/70 bg-card/60 px-6 py-6 dark:border-dark-700/60 dark:bg-dark-900/40">
        <div class="mx-auto grid max-w-6xl gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div
            v-for="item in trustItems"
            :key="item.key"
            class="flex items-center gap-3 rounded-xl px-2 py-1"
          >
            <div
              class="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-brand/10 text-brand dark:bg-brand/15"
            >
              <Icon :name="item.icon" size="sm" />
            </div>
            <div class="min-w-0">
              <p class="text-sm font-semibold text-ink dark:text-white">{{ item.title }}</p>
              <p class="text-xs text-ink-soft dark:text-dark-400">{{ item.desc }}</p>
            </div>
          </div>
        </div>
      </section>

      <!-- Capabilities 3×2 -->
      <section class="px-6 py-14 lg:py-16">
        <div class="mx-auto max-w-6xl">
          <div class="mb-8 text-center">
            <h2 class="text-2xl font-semibold tracking-tight text-ink dark:text-white">
              {{ t('home.capabilities.title') }}
            </h2>
            <p class="mt-2 text-sm text-ink-soft dark:text-dark-400">
              {{ t('home.capabilities.subtitle') }}
            </p>
          </div>

          <div class="grid gap-5 sm:grid-cols-2 lg:grid-cols-3">
            <article
              v-for="cap in capabilities"
              :key="cap.key"
              class="group rounded-card border border-line bg-card p-6 shadow-xs transition-all duration-150 hover:border-brand/30 hover:shadow-md dark:border-dark-700 dark:bg-dark-800/60"
            >
              <div
                class="mb-4 flex h-11 w-11 items-center justify-center rounded-xl bg-brand-gradient text-white shadow-md shadow-brand/20 transition-transform group-hover:scale-105"
              >
                <Icon :name="cap.icon" size="md" class="text-white" />
              </div>
              <h3 class="mb-1.5 text-base font-semibold text-ink dark:text-white">
                {{ cap.title }}
              </h3>
              <p class="text-sm leading-relaxed text-ink-soft dark:text-dark-400">
                {{ cap.desc }}
              </p>
            </article>
          </div>
        </div>
      </section>

      <!-- How it works -->
      <section class="border-t border-line/70 bg-card/40 px-6 py-14 dark:border-dark-700/60 dark:bg-dark-900/30 lg:py-16">
        <div class="mx-auto max-w-6xl">
          <div class="mb-10 text-center">
            <h2 class="text-2xl font-semibold tracking-tight text-ink dark:text-white">
              {{ t('home.howItWorks.title') }}
            </h2>
            <p class="mt-2 text-sm text-ink-soft dark:text-dark-400">
              {{ t('home.howItWorks.subtitle') }}
            </p>
          </div>

          <ol class="grid gap-6 md:grid-cols-3">
            <li
              v-for="(step, index) in howSteps"
              :key="step.key"
              class="relative rounded-card border border-line bg-card p-6 dark:border-dark-700 dark:bg-dark-800/50"
            >
              <span
                class="mb-4 inline-flex h-8 w-8 items-center justify-center rounded-full bg-brand-gradient text-sm font-semibold text-white shadow-xs"
              >
                {{ index + 1 }}
              </span>
              <h3 class="mb-1.5 text-base font-semibold text-ink dark:text-white">
                {{ step.title }}
              </h3>
              <p class="text-sm leading-relaxed text-ink-soft dark:text-dark-400">
                {{ step.desc }}
              </p>
            </li>
          </ol>
        </div>
      </section>
    </main>

    <!-- Footer -->
    <footer class="relative z-10 border-t border-line/70 bg-page/80 px-6 py-8 dark:border-dark-700/60">
      <div
        class="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 text-center sm:flex-row sm:text-left"
      >
        <p class="text-sm text-ink-soft dark:text-dark-400">
          &copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}
        </p>
        <div class="flex flex-wrap items-center justify-center gap-4">
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="text-sm text-ink-soft transition-colors hover:text-ink dark:text-dark-400 dark:hover:text-white"
          >
            {{ t('home.docs') }}
          </a>
          <router-link
            v-for="doc in legalLinks"
            :key="doc.id"
            :to="`/legal/${doc.id}`"
            class="text-sm text-ink-soft transition-colors hover:text-ink dark:text-dark-400 dark:hover:text-white"
          >
            {{ doc.title }}
          </router-link>
        </div>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import BrandLogo from '@/components/brand/BrandLogo.vue'
import CodeCard from '@/components/home/CodeCard.vue'
import { sanitizeUrl } from '@/utils/url'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings — read-only from public settings / app store
const siteName = computed(
  () => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Clomio'
)
const siteLogo = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', {
    allowRelative: true,
    allowDataUrl: true
  })
)
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || '')
const docUrl = computed(() =>
  sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || '')
)
const apiBaseUrl = computed(
  () => appStore.cachedPublicSettings?.api_base_url || appStore.apiBaseUrl || ''
)

// Prefer configured subtitle when present; otherwise i18n hero description
const heroSubtitleText = computed(
  () => siteSubtitle.value || t('home.heroDescription')
)

// Legal links from login agreement documents (if configured)
const legalLinks = computed(() => {
  const docs = appStore.cachedPublicSettings?.login_agreement_documents
  if (!docs?.length) return []
  return docs
    .filter((d) => d?.id && d?.title)
    .map((d) => ({ id: d.id, title: d.title }))
})

// Theme
const isDark = ref(document.documentElement.classList.contains('dark'))

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => (isAdmin.value ? '/admin/dashboard' : '/dashboard'))
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

const currentYear = computed(() => new Date().getFullYear())

// Trust strip — capability facts only (no fake customer logos)
const trustItems = computed(() => [
  {
    key: 'billing',
    icon: 'dollar' as const,
    title: t('home.trust.billing'),
    desc: t('home.trust.billingDesc')
  },
  {
    key: 'keys',
    icon: 'key' as const,
    title: t('home.trust.keys'),
    desc: t('home.trust.keysDesc')
  },
  {
    key: 'usage',
    icon: 'chart' as const,
    title: t('home.trust.usage'),
    desc: t('home.trust.usageDesc')
  },
  {
    key: 'routing',
    icon: 'swap' as const,
    title: t('home.trust.routing'),
    desc: t('home.trust.routingDesc')
  }
])

// Capability cards — map to real sub2api modules
const capabilities = computed(() => [
  {
    key: 'gateway',
    icon: 'server' as const,
    title: t('home.capabilities.gateway'),
    desc: t('home.capabilities.gatewayDesc')
  },
  {
    key: 'keys',
    icon: 'key' as const,
    title: t('home.capabilities.apiKeys'),
    desc: t('home.capabilities.apiKeysDesc')
  },
  {
    key: 'usage',
    icon: 'chartBar' as const,
    title: t('home.capabilities.usageBilling'),
    desc: t('home.capabilities.usageBillingDesc')
  },
  {
    key: 'subscriptions',
    icon: 'creditCard' as const,
    title: t('home.capabilities.subscriptions'),
    desc: t('home.capabilities.subscriptionsDesc')
  },
  {
    key: 'reliability',
    icon: 'shield' as const,
    title: t('home.capabilities.reliability'),
    desc: t('home.capabilities.reliabilityDesc')
  },
  {
    key: 'support',
    icon: 'chat' as const,
    title: t('home.capabilities.support'),
    desc: t('home.capabilities.supportDesc')
  }
])

// How it works — 3 steps
const howSteps = computed(() => [
  {
    key: 'register',
    title: t('home.howItWorks.step1Title'),
    desc: t('home.howItWorks.step1Desc')
  },
  {
    key: 'key',
    title: t('home.howItWorks.step2Title'),
    desc: t('home.howItWorks.step2Desc')
  },
  {
    key: 'call',
    title: t('home.howItWorks.step3Title'),
    desc: t('home.howItWorks.step3Desc')
  }
])

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function initTheme() {
  const savedTheme = localStorage.getItem('theme')
  if (
    savedTheme === 'dark' ||
    (!savedTheme && window.matchMedia('(prefers-color-scheme: dark)').matches)
  ) {
    isDark.value = true
    document.documentElement.classList.add('dark')
  }
}

onMounted(() => {
  initTheme()
  authStore.checkAuth()
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>
