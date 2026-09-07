<template>
  <!-- Custom Home Content: Full Page Mode -->
  <div v-if="hasHomeContent" class="min-h-screen">
    <!-- iframe mode -->
    <iframe
      v-if="isHomeContentUrl"
      :src="homeContent.trim()"
      class="h-screen w-full border-0"
      allowfullscreen
    ></iframe>
    <!-- HTML mode - SECURITY: homeContent is admin-only setting, XSS risk is acceptable -->
    <div v-else v-html="homeContent"></div>
  </div>

  <!-- Compact Home Page -->
  <div v-else-if="compactHomeEnabled" data-testid="compact-home" class="home-compact public-page">
    <header class="home-nav">
      <div class="home-nav-inner">
        <router-link to="/home" class="home-brand">
          <span class="brand-mark"><img :src="siteLogo || '/logo.svg'" alt="" /></span>
          <span class="home-brand-name">{{ siteName }}</span>
        </router-link>
        <div class="home-nav-actions">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="header-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="sm" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="header-icon-btn"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="sm" />
          </router-link>
          <button
            type="button"
            class="header-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary">
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </div>
    </header>

    <main class="home-compact-main">
      <div class="home-compact-card glass-card glass-ring">
        <span class="brand-mark brand-mark-xl home-compact-logo"><img :src="siteLogo || '/logo.svg'" alt="" /></span>
        <h1 class="home-compact-title">{{ siteName }}</h1>
        <p class="home-compact-subtitle">{{ siteSubtitle }}</p>
        <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary btn-hero">
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
          <Icon name="arrowRight" size="sm" />
        </router-link>
      </div>
    </main>

    <footer class="home-footer">
      <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
    </footer>
  </div>

  <!-- Default Home Page (design 01) -->
  <div v-else class="home-page public-page">
    <header class="home-nav">
      <div class="home-nav-inner">
        <router-link to="/home" class="home-brand">
          <span class="brand-mark"><img :src="siteLogo || '/logo.svg'" alt="" /></span>
          <span class="home-brand-name">{{ siteName }}</span>
        </router-link>

        <nav class="home-nav-links" aria-label="Primary">
          <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('home.nav.modelPlaza') }}</router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.nav.docs') }}</a>
          <router-link to="/key-usage">{{ t('home.nav.keyUsage') }}</router-link>
          <router-link v-if="channelMonitorEnabled" to="/monitor">{{ t('home.nav.status') }}</router-link>
        </nav>

        <div class="home-nav-actions">
          <LocaleSwitcher class="home-nav-locale" />
          <button
            type="button"
            class="header-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="sm" />
            <Icon v-else name="moon" size="sm" />
          </button>
          <router-link v-if="isAuthenticated" :to="dashboardPath" class="btn btn-secondary home-login-btn">
            <span class="home-user-dot">{{ userInitial }}</span>
            {{ t('home.dashboard') }}
          </router-link>
          <router-link v-else to="/login" class="btn btn-secondary home-login-btn">
            {{ t('home.login') }}
          </router-link>
          <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary home-start-btn">
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
          </router-link>
          <details class="home-mobile-menu">
            <summary class="header-icon-btn" :aria-label="t('common.toggleMenu')">
              <Icon name="menu" size="sm" />
            </summary>
            <div class="home-mobile-panel dropdown">
              <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="dropdown-item">{{ t('home.nav.modelPlaza') }}</router-link>
              <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer" class="dropdown-item">{{ t('home.nav.docs') }}</a>
              <router-link to="/key-usage" class="dropdown-item">{{ t('home.nav.keyUsage') }}</router-link>
              <router-link v-if="channelMonitorEnabled" to="/monitor" class="dropdown-item">{{ t('home.nav.status') }}</router-link>
              <a :href="githubUrl" target="_blank" rel="noopener noreferrer" class="dropdown-item">GitHub</a>
            </div>
          </details>
        </div>
      </div>
    </header>

    <main class="home-main">
      <!-- Hero -->
      <section class="home-hero">
        <div class="home-hero-copy">
          <div class="home-tags">
            <span class="chip chip-accent">{{ t('home.tags.subscriptionToApi') }}</span>
            <span class="chip">{{ t('home.tags.stickySession') }}</span>
            <span class="chip">{{ t('home.tags.realtimeBilling') }}</span>
          </div>
          <h1 class="home-hero-title">
            {{ t('home.heroTitle1') }}<br />
            <span class="text-gradient">{{ t('home.heroTitle2') }}</span>
          </h1>
          <p class="home-hero-desc">{{ heroDescription }}</p>
          <div class="home-hero-ctas">
            <router-link :to="isAuthenticated ? dashboardPath : '/login'" class="btn btn-primary btn-hero">
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="sm" :stroke-width="2" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn btn-secondary btn-hero home-cta-secondary"
            >
              {{ t('home.viewDocs') }}
            </a>
            <router-link v-else-if="showModelPlazaEntry" to="/model-plaza" class="btn btn-secondary btn-hero home-cta-secondary">
              {{ t('home.nav.modelPlaza') }}
            </router-link>
          </div>
          <div class="home-social">
            <span class="home-avatars" aria-hidden="true"><i></i><i></i><i></i></span>
            <span>{{ t('home.socialProof') }}</span>
          </div>
        </div>

        <div class="home-hero-visual">
          <ConsolePreview :api-base-url="apiBaseUrl" />
        </div>
      </section>

      <!-- Three steps + code -->
      <section class="home-steps">
        <div class="home-steps-copy">
          <div>
            <p class="section-kicker">{{ t('home.steps.kicker') }}</p>
            <h2 class="section-title">{{ t('home.steps.title') }}</h2>
          </div>
          <div class="home-step-list">
            <div v-for="(step, index) in steps" :key="step.title" class="home-step">
              <span class="home-step-num">{{ String(index + 1).padStart(2, '0') }}</span>
              <div>
                <h3>{{ step.title }}</h3>
                <p>{{ step.desc }}</p>
              </div>
            </div>
          </div>
          <div class="home-clients">
            <span v-for="c in clients" :key="c" class="home-client"><i></i>{{ c }}</span>
          </div>
        </div>
        <HomeCodeTabs :api-base-url="apiBaseUrl" />
      </section>

      <!-- Models & pricing -->
      <section v-if="showPricing" class="home-pricing">
        <div class="home-section-head">
          <div>
            <p class="section-kicker">{{ t('home.pricing.kicker') }}</p>
            <h2 class="section-title">{{ t('home.pricing.title') }}</h2>
          </div>
          <router-link v-if="showModelPlazaEntry" to="/model-plaza" class="home-section-link">
            {{ t('home.pricing.link') }}
            <Icon name="arrowRight" size="xs" :stroke-width="2" />
          </router-link>
        </div>
        <HomePricingTable :rows="pricingRows" />
      </section>

      <!-- Comparison -->
      <section class="home-compare">
        <div class="home-compare-copy">
          <p class="section-kicker">{{ t('home.comparison.title') }}</p>
          <h2 class="section-title">{{ t('home.comparison.heading') }}</h2>
          <p class="home-compare-desc">{{ t('home.comparison.headingDesc') }}</p>
        </div>
        <HomeCompareTable :rows="comparisonRows" />
      </section>

      <!-- CTA -->
      <HomeCtaBanner
        class="home-cta-slot"
        :to="isAuthenticated ? dashboardPath : '/register'"
        :label="isAuthenticated ? t('home.goToDashboard') : t('home.cta.button')"
      />
    </main>

    <HomeFooter
      :current-year="currentYear"
      :site-name="siteName"
      :legal-documents="legalDocuments"
      :doc-url="docUrl"
      :github-url="githubUrl"
      :contact-info="contactInfo"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import ConsolePreview from '@/components/home/ConsolePreview.vue'
import HomeCodeTabs from '@/components/home/HomeCodeTabs.vue'
import HomePricingTable, { type PricingRow } from '@/components/home/HomePricingTable.vue'
import HomeCompareTable from '@/components/home/HomeCompareTable.vue'
import HomeCtaBanner from '@/components/home/HomeCtaBanner.vue'
import HomeFooter from '@/components/home/HomeFooter.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useTheme } from '@/composables/useTheme'
import { getModelPlaza } from '@/api/modelPlaza'
import { platformFromModel, platformLabel } from '@/utils/platformTile'
import type { GroupPlatform } from '@/types'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const apiBaseUrl = computed(() => appStore.cachedPublicSettings?.api_base_url || '')
const contactInfo = computed(() => appStore.cachedPublicSettings?.contact_info || '')
const legalDocuments = computed(() => (appStore.cachedPublicSettings?.login_agreement_documents ?? []).filter((d) => d.title?.trim()))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))
const channelMonitorEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.channelMonitor))

const heroDescription = computed(() => {
  const custom = appStore.cachedPublicSettings?.site_subtitle
  const base = custom && custom !== siteName.value ? custom : t('home.heroDescription')
  return `${base.replace(/[。.]$/, '')}。${t('home.heroDescriptionExt')}`
})

// Check if homeContent is a URL (for iframe display)
const isHomeContentUrl = computed(() => {
  const content = homeContent.value.trim()
  return content.startsWith('http://') || content.startsWith('https://')
})

const { isDark, toggleTheme } = useTheme()

// GitHub URL
const githubUrl = 'https://github.com/Wei-Shaw/sub2api'

// Auth state
const isAuthenticated = computed(() => authStore.isAuthenticated)
const modelPlazaRequiresAuth = computed(
  () => appStore.cachedPublicSettings?.model_plaza_require_auth === true,
)
const showModelPlazaEntry = computed(
  () => modelPlazaEnabled.value && (isAuthenticated.value || !modelPlazaRequiresAuth.value),
)
const isAdmin = computed(() => authStore.isAdmin)
const dashboardPath = computed(() => isAdmin.value ? '/admin/dashboard' : '/dashboard')
const userInitial = computed(() => {
  const user = authStore.user
  if (!user || !user.email) return ''
  return user.email.charAt(0).toUpperCase()
})

// Current year for footer
const currentYear = computed(() => new Date().getFullYear())

const steps = computed(() => [
  { title: t('home.steps.items.create.title'), desc: t('home.steps.items.create.desc') },
  { title: t('home.steps.items.baseUrl.title'), desc: t('home.steps.items.baseUrl.desc') },
  { title: t('home.steps.items.client.title'), desc: t('home.steps.items.client.desc') }
])

const clients = ['Claude Code', 'Codex CLI', 'Cursor', 'Cline', 'Roo Code', 'OpenAI SDK', 'Anthropic SDK']

const comparisonRows = computed(() => [
  {
    feature: t('home.comparison.items.pricing.feature'),
    official: t('home.comparison.items.pricing.official'),
    us: t('home.comparison.items.pricing.us')
  },
  {
    feature: t('home.comparison.items.models.feature'),
    official: t('home.comparison.items.models.official'),
    us: t('home.comparison.items.models.us')
  },
  {
    feature: t('home.comparison.items.management.feature'),
    official: t('home.comparison.items.management.official'),
    us: t('home.comparison.items.management.us')
  },
  {
    feature: t('home.comparison.items.stability.feature'),
    official: t('home.comparison.items.stability.official'),
    us: t('home.comparison.items.stability.us')
  },
  {
    feature: t('home.comparison.items.control.feature'),
    official: t('home.comparison.items.control.official'),
    us: t('home.comparison.items.control.us')
  }
])

// ---- Public pricing (model plaza) ----
const pricingRows = ref<PricingRow[]>([])
const pricingLoaded = ref(false)
const showPricing = computed(() => showModelPlazaEntry.value && (pricingRows.value.length > 0 || !pricingLoaded.value))

function perMillion(price: number | null | undefined): string {
  if (price === null || price === undefined || !Number.isFinite(price)) return '—'
  const v = price * 1_000_000
  return `$${v >= 100 ? v.toFixed(0) : v.toFixed(2)}`
}

function contextLabel(model: string): string {
  const m = model.toLowerCase()
  if (m.startsWith('gemini')) return '1M'
  if (m.startsWith('gpt-5') || m.includes('codex')) return '400K'
  if (m.startsWith('grok')) return '256K'
  if (m.startsWith('claude')) return '200K'
  return '128K'
}

async function loadPublicPricing() {
  if (!showModelPlazaEntry.value) {
    pricingLoaded.value = true
    return
  }
  try {
    const data = await getModelPlaza()
    const seen = new Set<string>()
    const rows: PricingRow[] = []
    for (const group of data.groups ?? []) {
      for (const m of group.models ?? []) {
        if (seen.has(m.name) || rows.length >= 6) continue
        const pricing = m.pricing ?? m.official_pricing
        if (!pricing) continue
        seen.add(m.name)
        const platform = platformFromModel(m.name, m.platform) as GroupPlatform
        rows.push({
          model: m.name,
          vendor: platformLabel(platform),
          platform,
          input: perMillion(pricing.input_price),
          output: perMillion(pricing.output_price),
          context: contextLabel(m.name),
          limited: Boolean(m.time_pricing?.periods?.length)
        })
      }
      if (rows.length >= 6) break
    }
    pricingRows.value = rows
  } catch {
    pricingRows.value = []
  } finally {
    pricingLoaded.value = true
  }
}

onMounted(async () => {
  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    await appStore.fetchPublicSettings()
  }
  void loadPublicPricing()
})
</script>

<style scoped>
.home-page,
.home-compact {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background:
    linear-gradient(color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    linear-gradient(90deg, color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    var(--background);
}

.home-page {
  color: var(--foreground);
  overflow: hidden;
}

/* ---------- Nav · 68px ---------- */
.home-nav {
  height: 68px;
  display: flex;
  align-items: center;
  border-bottom: 1px solid color-mix(in oklch, var(--border) 72%, transparent);
  background: color-mix(in oklch, var(--surface) 58%, transparent);
  backdrop-filter: blur(18px);
  -webkit-backdrop-filter: blur(18px);
}

.home-nav-inner {
  width: 100%;
  max-width: 1440px;
  margin: 0 auto;
  padding: 0 80px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
}

.home-brand {
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--foreground);
  text-decoration: none;
}

.home-brand-name {
  font-size: 15px;
  font-weight: 700;
  letter-spacing: -0.01em;
}

.home-nav-links {
  display: flex;
  align-items: center;
  gap: 28px;
  font-size: 13.5px;
  font-weight: 500;
  color: var(--muted);
}

.home-nav-links a {
  color: inherit;
  text-decoration: none;
  transition: color 0.15s ease;
}

.home-nav-links a:hover,
.home-nav-links a.router-link-active {
  color: var(--foreground);
}

.home-nav-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

/* Home nav locale switcher: reshape shared component to a 34px icon button
   to match .header-icon-btn sizing used by the theme toggle (see deviations.md). */
.home-nav-locale :deep(button) {
  width: 34px;
  height: 34px;
  padding: 0;
  gap: 0;
  justify-content: center;
}

.home-nav-locale :deep(button span:not(:first-child)),
.home-nav-locale :deep(button svg) {
  display: none;
}

.home-user-dot {
  display: inline-flex;
  width: 18px;
  height: 18px;
  align-items: center;
  justify-content: center;
  border-radius: 999px;
  background: color-mix(in oklch, var(--accent) 18%, transparent);
  color: var(--accent);
  font-size: 10px;
  font-weight: 700;
}

.home-mobile-menu {
  display: none;
  position: relative;
}

.home-mobile-menu summary {
  list-style: none;
}

.home-mobile-menu summary::-webkit-details-marker {
  display: none;
}

.home-mobile-panel {
  right: 0;
  top: calc(100% + 8px);
  min-width: 200px;
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.home-mobile-panel .dropdown-item {
  height: 44px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 600;
}

/* ---------- Main ---------- */
.home-main {
  flex: 1;
  width: 100%;
  max-width: 1440px;
  margin: 0 auto;
  position: relative;
}

.home-hero {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 64px;
  align-items: center;
  padding: 76px 80px 72px;
  min-height: 520px;
}

.home-hero-copy {
  display: flex;
  flex-direction: column;
  gap: 20px;
  min-width: 0;
}

.home-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.home-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: clamp(44px, 4.2vw, 64px);
  line-height: 1.06;
  font-weight: 800;
  letter-spacing: -0.035em;
  text-wrap: balance;
}

.home-hero-copy::before {
  content: '';
  width: 56px;
  height: 4px;
  border-radius: 999px;
  background: var(--accent);
  margin-bottom: -8px;
}

.home-hero-desc {
  margin: 0;
  font-size: 16px;
  line-height: 1.65;
  color: var(--muted);
  max-width: 520px;
  text-wrap: pretty;
}

.home-hero-ctas {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
  margin-top: 6px;
}

.home-cta-secondary {
  padding: 0 18px;
}

.home-social {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 12.5px;
  color: var(--muted);
  margin-top: 8px;
}

.home-avatars {
  display: flex;
}

.home-avatars i {
  width: 22px;
  height: 22px;
  border-radius: 999px;
  border: 2px solid var(--background);
  background: var(--surface-tertiary);
}

.home-avatars i + i {
  margin-left: -8px;
}

.home-avatars i:nth-child(2) {
  background: color-mix(in oklch, var(--accent) 40%, var(--surface));
}

.home-avatars i:nth-child(3) {
  background: color-mix(in oklch, var(--success) 40%, var(--surface));
}

.home-hero-visual {
  position: relative;
  min-width: 0;
}

/* The shared .section-title class (src/style.css) ships line-height:1.15, tuned
   for other contexts; at this page's 30px section headings (steps/pricing/compare)
   the design reference renders ~1.47x, so we override locally per-section here
   instead of touching the global class. */
.home-steps .section-title,
.home-pricing .section-title,
.home-compare-copy .section-title {
  line-height: 1.467;
}

/* ---------- Steps ---------- */
.home-steps {
  padding: 32px 80px 76px;
  display: grid;
  grid-template-columns: 1fr 1.1fr;
  gap: 48px;
  align-items: center;
}

.home-steps-copy,
.home-pricing,
.home-compare {
  position: relative;
}

.home-pricing,
.home-compare {
  border-top: 1px solid color-mix(in oklch, var(--border) 72%, transparent);
  padding-top: 56px;
}

.home-steps-copy {
  display: flex;
  flex-direction: column;
  gap: 22px;
}

.section-kicker {
  margin: 0 0 8px;
}

.home-step-list {
  display: flex;
  flex-direction: column;
  gap: 14px;
}

.home-step {
  display: flex;
  gap: 14px;
}

.home-step-num {
  width: 28px;
  height: 28px;
  flex: none;
  border-radius: 8px;
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
  font-size: 12px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: var(--font-mono);
}

.home-step h3 {
  margin: 0;
  font-family: var(--font-body);
  font-size: 14.5px;
  font-weight: 600;
  letter-spacing: 0;
}

.home-step p {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--muted);
}

.home-clients {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.home-client {
  height: 28px;
  padding: 0 11px;
  border-radius: 999px;
  font-size: 12.5px;
  font-weight: 500;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  border: 1px solid var(--border);
  color: var(--foreground);
}

.home-client i {
  width: 6px;
  height: 6px;
  border-radius: 999px;
  background: var(--success);
}

/* ---------- Pricing ---------- */
.home-pricing {
  padding: 0 80px 64px;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.home-section-head {
  display: flex;
  align-items: flex-end;
  justify-content: space-between;
  gap: 16px;
}

.home-section-link {
  font-size: 13.5px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: var(--accent);
  white-space: nowrap;
}

.home-section-link:hover {
  text-decoration: underline;
}

/* ---------- Compare ---------- */
.home-compare {
  padding: 0 80px 64px;
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 48px;
  align-items: start;
}

.home-compare-copy {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.home-compare-copy .section-kicker {
  margin: 0;
}

.home-compare-desc {
  margin: 0;
  font-size: 14px;
  color: var(--muted);
  line-height: 1.65;
  max-width: 440px;
}

/* ---------- CTA ---------- */
/* .home-cta-slot: layout-only margin wrapper for the extracted HomeCtaBanner
   component (see components/home/HomeCtaBanner.vue for the banner's own styles). */
.home-cta-slot {
  margin: 8px 80px 72px;
}

/* ---------- Compact ---------- */
.home-compact-main {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  justify-content: center;
  padding: 64px 24px;
}

.home-compact-card {
  width: 100%;
  max-width: 520px;
  padding: 40px 36px;
  border-radius: 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  gap: 16px;
}

.home-compact-logo {
  margin-bottom: 4px;
}

.home-compact-title {
  margin: 0;
  font-family: var(--display);
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.03em;
  overflow-wrap: anywhere;
}

.home-compact-subtitle {
  margin: 0;
  font-size: 15px;
  line-height: 1.65;
  color: var(--muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

/* ---------- Responsive ---------- */
@media (max-width: 1180px) {
  .home-nav-inner {
    padding-left: 40px;
    padding-right: 40px;
  }

  .home-hero,
  .home-steps,
  .home-pricing,
  .home-compare {
    padding-left: 40px;
    padding-right: 40px;
  }

  .home-cta-slot {
    margin-left: 40px;
    margin-right: 40px;
  }

  .home-hero {
    gap: 40px;
  }

  .home-hero-title {
    font-size: 48px;
  }
}

@media (max-width: 1023px) {
  .home-nav-links,
  .home-start-btn,
  .home-nav-locale {
    display: none;
  }

  .home-mobile-menu {
    display: block;
  }

  .home-hero,
  .home-steps,
  .home-compare {
    grid-template-columns: 1fr;
    gap: 28px;
  }

  .home-hero {
    padding-top: 28px;
  }

  .home-hero-visual {
    padding: 20px 0 16px;
  }
}

@media (max-width: 767px) {
  .home-nav {
    height: 56px;
  }

  .home-nav-inner,
  .home-hero,
  .home-steps,
  .home-pricing,
  .home-compare {
    padding-left: 20px;
    padding-right: 20px;
  }

  .home-hero {
    padding-top: 10px;
    padding-bottom: 28px;
    gap: 18px;
  }

  .home-hero-title {
    font-size: 40px;
  }

  .home-hero-desc {
    font-size: 14.5px;
  }

  .home-hero-ctas .btn {
    flex: 1;
    height: 44px;
  }

  .home-social {
    display: none;
  }

  .home-hero-visual {
    padding: 24px 0 20px;
  }

  .home-steps,
  .home-pricing,
  .home-compare {
    padding-bottom: 40px;
  }

  .section-title {
    font-size: 24px;
  }

  .home-section-head {
    flex-direction: column;
    align-items: flex-start;
  }

  .home-cta-slot {
    margin: 0 20px 40px;
  }
}
</style>
