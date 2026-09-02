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
  <div
    v-else-if="compactHomeEnabled"
    data-testid="compact-home"
    class="home-compact"
  >
    <header class="home-compact-header">
      <nav class="home-compact-nav">
        <div class="flex min-w-0 flex-1 items-center gap-3">
          <img
            :src="siteLogo || '/logo.svg'"
            alt="Logo"
            class="h-9 w-9 shrink-0 rounded-lg object-contain"
          />
          <span class="min-w-0 truncate text-base font-semibold text-foreground">{{ siteName }}</span>
        </div>
        <div class="flex max-w-full shrink-0 flex-wrap items-center justify-end gap-2">
          <LocaleSwitcher />
          <a
            v-if="docUrl"
            :href="docUrl"
            target="_blank"
            rel="noopener noreferrer"
            class="home-icon-btn"
            :title="t('home.viewDocs')"
          >
            <Icon name="book" size="md" />
          </a>
          <router-link
            v-if="showModelPlazaEntry"
            to="/model-plaza"
            class="home-icon-btn home-icon-btn-wide"
            :title="t('nav.modelPlaza')"
          >
            <Icon name="grid" size="md" />
            <span class="hidden sm:inline">{{ t('nav.modelPlaza') }}</span>
          </router-link>
          <button
            class="home-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="btn-glass-primary home-compact-cta"
          >
            {{ isAuthenticated ? t('home.dashboard') : t('home.login') }}
          </router-link>
        </div>
      </nav>
    </header>

    <main class="home-compact-main">
      <div class="min-w-0 max-w-2xl text-center">
        <img
          :src="siteLogo || '/logo.svg'"
          alt="Logo"
          class="mx-auto mb-6 h-20 w-20 rounded-2xl object-contain"
        />
        <h1 class="home-compact-title">{{ siteName }}</h1>
        <p class="home-compact-subtitle">{{ siteSubtitle }}</p>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="btn-glass-primary mt-8 inline-flex min-h-10 items-center justify-center px-5 text-sm"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.login') }}
        </router-link>
      </div>
    </main>

    <footer class="home-compact-footer">
      &copy; {{ currentYear }} {{ siteName }}
    </footer>
  </div>

  <!-- Default Home Page -->
  <div v-else class="home-page">
    <header class="home-nav">
      <div class="home-nav-inner">
        <div class="home-brand">
          <div class="home-logo">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" />
          </div>
          <span class="home-brand-name">{{ siteName }}</span>
        </div>

        <nav class="home-nav-links" aria-label="Primary">
          <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
          <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
          <router-link to="/key-usage">{{ t('keyUsage.title') }}</router-link>
        </nav>

        <div class="home-nav-actions">
          <LocaleSwitcher />
          <button
            class="home-icon-btn"
            :title="isDark ? t('home.switchToLight') : t('home.switchToDark')"
            @click="toggleTheme"
          >
            <Icon v-if="isDark" name="sun" size="md" />
            <Icon v-else name="moon" size="md" />
          </button>
          <router-link
            v-if="isAuthenticated"
            :to="dashboardPath"
            class="btn-glass-secondary home-login-btn"
          >
            <span class="home-user-dot">{{ userInitial }}</span>
            {{ t('home.dashboard') }}
          </router-link>
          <router-link v-else to="/login" class="btn-glass-secondary home-login-btn">
            {{ t('home.login') }}
          </router-link>
          <router-link
            :to="isAuthenticated ? dashboardPath : '/login'"
            class="btn-glass-primary home-start-btn"
          >
            {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
          </router-link>
          <details class="home-mobile-menu">
            <summary class="home-icon-btn" :aria-label="t('common.toggleMenu')">
              <Icon name="menu" size="md" />
            </summary>
            <div class="home-mobile-panel">
              <router-link v-if="showModelPlazaEntry" to="/model-plaza">{{ t('nav.modelPlaza') }}</router-link>
              <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
              <router-link to="/key-usage">{{ t('keyUsage.title') }}</router-link>
              <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
            </div>
          </details>
        </div>
      </div>
    </header>

    <main class="home-main">
      <section class="home-hero">
        <div class="home-hero-copy">
          <div class="home-tags">
            <span class="home-tag home-tag-accent">{{ t('home.tags.subscriptionToApi') }}</span>
            <span class="home-tag">{{ t('home.tags.stickySession') }}</span>
            <span class="home-tag">{{ t('home.tags.realtimeBilling') }}</span>
          </div>
          <h1 class="home-hero-title">
            {{ siteName }}
            <span class="home-hero-gradient">{{ t('home.heroSubtitle') }}</span>
          </h1>
          <p class="home-hero-desc">{{ siteSubtitle || t('home.heroDescription') }}</p>
          <div class="home-hero-ctas">
            <router-link
              :to="isAuthenticated ? dashboardPath : '/login'"
              class="btn-glass-primary home-cta-primary"
            >
              {{ isAuthenticated ? t('home.goToDashboard') : t('home.getStarted') }}
              <Icon name="arrowRight" size="md" />
            </router-link>
            <a
              v-if="docUrl"
              :href="docUrl"
              target="_blank"
              rel="noopener noreferrer"
              class="btn-glass-secondary home-cta-secondary"
            >
              {{ t('home.viewDocs') }}
            </a>
          </div>
        </div>

        <div class="home-hero-visual">
          <div class="terminal-container">
            <div class="terminal-window">
              <div class="terminal-header">
                <div class="terminal-buttons">
                  <span class="btn-close"></span>
                  <span class="btn-minimize"></span>
                  <span class="btn-maximize"></span>
                </div>
                <span class="terminal-title">terminal</span>
              </div>
              <div class="terminal-body">
                <div class="code-line line-1">
                  <span class="code-prompt">$</span>
                  <span class="code-cmd">curl</span>
                  <span class="code-flag">-X POST</span>
                  <span class="code-url">/v1/messages</span>
                </div>
                <div class="code-line line-2">
                  <span class="code-comment"># Routing to upstream...</span>
                </div>
                <div class="code-line line-3">
                  <span class="code-success">200 OK</span>
                  <span class="code-response">{ "content": "Hello!" }</span>
                </div>
                <div class="code-line line-4">
                  <span class="code-prompt">$</span>
                  <span class="cursor"></span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      <section class="home-steps">
        <div>
          <p class="home-kicker">{{ t('home.solutions.subtitle') }}</p>
          <h2 class="home-section-title">{{ t('home.solutions.title') }}</h2>
          <div class="home-step-list">
            <GlassCard v-for="(feature, index) in featureCards" :key="feature.title" padding="md" hover>
              <div class="home-step">
                <span class="home-step-num">{{ String(index + 1).padStart(2, '0') }}</span>
                <div>
                  <h3>{{ feature.title }}</h3>
                  <p>{{ feature.desc }}</p>
                </div>
              </div>
            </GlassCard>
          </div>
        </div>
      </section>

      <section class="home-providers">
        <div class="home-section-head">
          <h2 class="home-section-title">{{ t('home.providers.title') }}</h2>
          <p class="home-section-desc">{{ t('home.providers.description') }}</p>
        </div>
        <div class="home-provider-row">
          <GlassCard v-for="provider in providers" :key="provider.name" padding="sm" class="home-provider-card">
            <div class="home-provider">
              <span class="home-provider-mark" :style="{ background: provider.color }">{{ provider.mark }}</span>
              <span>{{ provider.name }}</span>
              <StatusBadge :tone="provider.soon ? 'muted' : 'success'" :label="provider.soon ? t('home.providers.soon') : t('home.providers.supported')" />
            </div>
          </GlassCard>
        </div>
      </section>

      <section class="home-compare">
        <div>
          <p class="home-kicker">{{ t('home.comparison.title') }}</p>
          <h2 class="home-section-title">{{ t('home.features.unifiedGateway') }}</h2>
          <p class="home-hero-desc">{{ t('home.features.unifiedGatewayDesc') }}</p>
        </div>
        <GlassCard padding="sm" class="home-compare-table">
          <div class="home-compare-head">
            <span>{{ t('home.comparison.headers.feature') }}</span>
            <span>{{ t('home.comparison.headers.official') }}</span>
            <span>{{ t('home.comparison.headers.us') }}</span>
          </div>
          <div v-for="row in comparisonRows" :key="row.feature" class="home-compare-row">
            <span>{{ row.feature }}</span>
            <span class="text-muted">{{ row.official }}</span>
            <span>{{ row.us }}</span>
          </div>
        </GlassCard>
      </section>

      <section class="home-cta-banner">
        <div>
          <h2>{{ t('home.cta.title') }}</h2>
          <p>{{ t('home.cta.description') }}</p>
        </div>
        <router-link
          :to="isAuthenticated ? dashboardPath : '/login'"
          class="btn-glass-primary home-cta-primary"
        >
          {{ isAuthenticated ? t('home.goToDashboard') : t('home.cta.button') }}
        </router-link>
      </section>
    </main>

    <footer class="home-footer">
      <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
      <div class="home-footer-links">
        <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.docs') }}</a>
        <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAuthStore, useAppStore } from '@/stores'
import LocaleSwitcher from '@/components/common/LocaleSwitcher.vue'
import Icon from '@/components/icons/Icon.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { sanitizeUrl } from '@/utils/url'
import { FeatureFlags, isFeatureFlagEnabled } from '@/utils/featureFlags'
import { useTheme } from '@/composables/useTheme'

const { t } = useI18n()

const authStore = useAuthStore()
const appStore = useAppStore()

// Site settings - directly from appStore (already initialized from injected config)
const siteName = computed(() => appStore.cachedPublicSettings?.site_name || appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const siteSubtitle = computed(() => appStore.cachedPublicSettings?.site_subtitle || 'AI API Gateway Platform')
const docUrl = computed(() => sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl || ''))
const homeContent = computed(() => appStore.cachedPublicSettings?.home_content || '')
const hasHomeContent = computed(() => homeContent.value.trim().length > 0)
const compactHomeEnabled = computed(() => appStore.cachedPublicSettings?.compact_home_enabled === true)
const modelPlazaEnabled = computed(() => isFeatureFlagEnabled(FeatureFlags.modelPlaza))

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

const featureCards = computed(() => [
  { title: t('home.features.unifiedGateway'), desc: t('home.features.unifiedGatewayDesc') },
  { title: t('home.features.multiAccount'), desc: t('home.features.multiAccountDesc') },
  { title: t('home.features.balanceQuota'), desc: t('home.features.balanceQuotaDesc') }
])

const providers = computed(() => [
  { name: t('home.providers.claude'), mark: 'C', color: 'oklch(70% 0.18 55)', soon: false },
  { name: 'GPT', mark: 'G', color: 'oklch(62% 0.17 155)', soon: false },
  { name: t('home.providers.gemini'), mark: 'G', color: 'oklch(62% 0.18 240)', soon: false },
  { name: t('home.providers.antigravity'), mark: 'A', color: 'oklch(62% 0.2 350)', soon: false },
  { name: t('home.providers.more'), mark: '+', color: 'var(--muted)', soon: true }
])

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

onMounted(() => {
  // Check auth state
  authStore.checkAuth()

  // Ensure public settings are loaded (will use cache if already loaded from injected config)
  if (!appStore.publicSettingsLoaded) {
    appStore.fetchPublicSettings()
  }
})
</script>

<style scoped>
.home-page {
  position: relative;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  color: var(--foreground);
  background:
    linear-gradient(color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    linear-gradient(90deg, color-mix(in oklch, var(--foreground) 2.5%, transparent) 1px, transparent 1px) 0 0 / 48px 48px,
    radial-gradient(circle at 0% 0%, color-mix(in oklch, var(--accent) 22%, transparent) 0%, transparent 30rem),
    radial-gradient(circle at 18% 100%, color-mix(in oklch, var(--accent) 10%, transparent) 0%, transparent 30rem),
    radial-gradient(circle at 100% 0%, color-mix(in oklch, var(--success) 14%, transparent) 0%, transparent 24rem),
    var(--background);
}

.home-compact {
  display: flex;
  min-height: 100vh;
  flex-direction: column;
  background: var(--background);
  color: var(--foreground);
}

.home-compact-header {
  border-bottom: 1px solid var(--border);
  padding: 16px 24px;
}

.home-compact-nav {
  margin: 0 auto;
  display: flex;
  max-width: 64rem;
  flex-wrap: wrap;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.home-compact-main {
  display: flex;
  flex: 1;
  min-width: 0;
  align-items: center;
  justify-content: center;
  padding: 64px 24px;
}

.home-compact-title {
  font-family: var(--display);
  font-size: 32px;
  font-weight: 800;
  letter-spacing: -0.03em;
  overflow-wrap: anywhere;
}

.home-compact-subtitle {
  margin-top: 16px;
  font-size: 16px;
  color: var(--muted);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

.home-compact-footer {
  border-top: 1px solid var(--border);
  padding: 20px 24px;
  text-align: center;
  font-size: 14px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.home-compact-cta {
  min-height: 40px;
  padding: 0 16px;
}

.home-icon-btn {
  display: inline-flex;
  height: 40px;
  width: 40px;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  color: var(--muted);
  background: transparent;
  border: 0;
  cursor: pointer;
  text-decoration: none;
}

.home-icon-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.home-icon-btn-wide {
  width: auto;
  gap: 6px;
  padding: 0 10px;
  font-size: 14px;
  font-weight: 600;
}

.home-nav {
  height: 68px;
  display: flex;
  align-items: center;
}

.home-nav-inner {
  width: 100%;
  max-width: 1280px;
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
}

.home-logo {
  width: 30px;
  height: 30px;
  border-radius: 9px;
  overflow: hidden;
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.45), 0 6px 14px -6px var(--accent);
}

.home-logo img {
  width: 100%;
  height: 100%;
  object-fit: contain;
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
}

.home-nav-links a.router-link-active {
  color: var(--foreground);
}

.home-nav-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.home-login-btn,
.home-start-btn {
  height: 34px;
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
  position: absolute;
  right: 0;
  top: calc(100% + 8px);
  z-index: 30;
  display: flex;
  min-width: 180px;
  flex-direction: column;
  gap: 4px;
  padding: 8px;
  border-radius: 12px;
  background: color-mix(in oklch, var(--surface) 88%, transparent);
  border: 1px solid var(--border);
  backdrop-filter: blur(20px);
  box-shadow: var(--shadow);
}

.home-mobile-panel a {
  height: 44px;
  display: flex;
  align-items: center;
  padding: 0 12px;
  border-radius: 10px;
  color: var(--foreground);
  text-decoration: none;
  font-size: 14px;
  font-weight: 600;
}

.home-main {
  flex: 1;
  max-width: 1280px;
  width: 100%;
  margin: 0 auto;
  padding: 0 80px 56px;
}

.home-hero {
  display: grid;
  grid-template-columns: 1.15fr 1fr;
  gap: 64px;
  align-items: center;
  padding: 64px 0 56px;
}

.home-hero-copy {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.home-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.home-tag {
  height: 24px;
  padding: 0 10px;
  border-radius: 999px;
  font-size: 12px;
  font-weight: 600;
  display: inline-flex;
  align-items: center;
  background: color-mix(in oklch, var(--surface) 80%, transparent);
  border: 1px solid var(--border);
  color: var(--muted);
}

.home-tag-accent {
  background: color-mix(in oklch, var(--accent) 12%, transparent);
  color: var(--accent);
  border-color: transparent;
}

.home-hero-title {
  margin: 0;
  font-family: var(--display);
  font-size: 60px;
  line-height: 1.06;
  font-weight: 800;
  letter-spacing: -0.035em;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.home-hero-gradient {
  background: linear-gradient(92deg, var(--foreground) 0%, var(--accent) 70%, color-mix(in oklch, var(--accent) 70%, var(--success)) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}

.home-hero-desc {
  margin: 0;
  font-size: 16px;
  line-height: 1.65;
  color: var(--muted);
  max-width: 520px;
}

.home-hero-ctas {
  display: flex;
  flex-wrap: wrap;
  gap: 10px;
}

.home-cta-primary,
.home-cta-secondary {
  height: 42px;
  padding: 0 18px;
}

.home-cta-primary {
  flex: 1 1 auto;
}

.home-kicker {
  font-size: 12px;
  letter-spacing: 0.08em;
  text-transform: uppercase;
  color: var(--accent);
  font-weight: 700;
  margin: 0 0 8px;
}

.home-section-title {
  margin: 0;
  font-family: var(--display);
  font-size: 30px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.home-section-desc {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 14px;
}

.home-steps,
.home-providers,
.home-compare {
  padding: 0 0 64px;
}

.home-step-list {
  margin-top: 22px;
  display: grid;
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
  font-size: 14.5px;
  font-weight: 600;
}

.home-step p {
  margin: 2px 0 0;
  font-size: 13px;
  color: var(--muted);
}

.home-section-head {
  text-align: center;
  margin-bottom: 20px;
}

.home-provider-row {
  display: flex;
  flex-wrap: wrap;
  justify-content: center;
  gap: 12px;
}

.home-provider {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
}

.home-provider-mark {
  width: 24px;
  height: 24px;
  border-radius: 7px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: #fff;
  font-size: 10px;
  font-weight: 700;
}

.home-compare {
  display: grid;
  grid-template-columns: 1fr 1.1fr;
  gap: 48px;
  align-items: start;
}

.home-compare-head,
.home-compare-row {
  display: grid;
  grid-template-columns: 110px 1fr 1fr;
  gap: 16px;
  padding: 12px 22px;
  font-size: 13px;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.home-compare-head {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  letter-spacing: 0.06em;
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
}

.home-cta-banner {
  margin-bottom: 56px;
  padding: 40px 48px;
  border-radius: 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  background:
    linear-gradient(120deg, color-mix(in oklch, var(--accent) 16%, transparent), color-mix(in oklch, var(--success) 10%, transparent)),
    color-mix(in oklch, var(--surface) 70%, transparent);
  border: 1px solid color-mix(in oklch, var(--accent) 35%, transparent);
  backdrop-filter: blur(20px);
  box-shadow: var(--shadow), 0 30px 60px -40px var(--accent);
}

.home-cta-banner h2 {
  margin: 0;
  font-family: var(--display);
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.home-cta-banner p {
  margin: 8px 0 0;
  font-size: 14.5px;
  color: var(--muted);
}

.home-footer {
  padding: 20px 80px 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  font-size: 12.5px;
  color: var(--muted);
  border-top: 1px solid var(--border);
}

.home-footer-links {
  display: flex;
  gap: 20px;
}

.home-footer-links a {
  color: inherit;
  text-decoration: none;
}

.terminal-container {
  position: relative;
  display: inline-block;
}

.terminal-window {
  width: 420px;
  background: color-mix(in oklch, var(--surface) 76%, transparent);
  border-radius: 18px;
  border: 1px solid color-mix(in oklch, var(--border) 85%, transparent);
  backdrop-filter: blur(24px);
  box-shadow: var(--shadow), 0 50px 100px -50px color-mix(in oklch, var(--accent) 55%, transparent);
  overflow: hidden;
}

.terminal-header {
  display: flex;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}

.terminal-buttons {
  display: flex;
  gap: 8px;
}

.terminal-buttons span {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: var(--surface-tertiary);
}

.terminal-title {
  flex: 1;
  text-align: center;
  font-size: 12px;
  font-family: var(--font-mono);
  color: var(--muted);
  margin-right: 52px;
}

.terminal-body {
  padding: 20px 24px;
  font-family: var(--font-mono);
  font-size: 14px;
  line-height: 2;
}

.code-line {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
  opacity: 0;
  animation: line-appear 0.5s ease forwards;
}

.line-1 { animation-delay: 0.3s; }
.line-2 { animation-delay: 1s; }
.line-3 { animation-delay: 1.8s; }
.line-4 { animation-delay: 2.5s; }

@keyframes line-appear {
  from { opacity: 0; transform: translateY(5px); }
  to { opacity: 1; transform: translateY(0); }
}

.code-prompt { color: var(--success-text); font-weight: bold; }
.code-cmd { color: var(--accent); }
.code-flag { color: var(--muted); }
.code-url { color: var(--foreground); }
.code-comment { color: var(--muted); font-style: italic; }
.code-success {
  color: var(--success-text);
  background: color-mix(in oklch, var(--success) 16%, transparent);
  padding: 2px 8px;
  border-radius: 4px;
  font-weight: 600;
}
.code-response { color: var(--warning-text); }

.cursor {
  display: inline-block;
  width: 8px;
  height: 16px;
  background: var(--success);
  animation: blink 1s step-end infinite;
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

@media (max-width: 1100px) {
  .home-nav-links,
  .home-start-btn {
    display: none;
  }

  .home-mobile-menu {
    display: block;
  }

  .home-nav-inner,
  .home-main,
  .home-footer {
    padding-left: 20px;
    padding-right: 20px;
  }

  .home-hero,
  .home-compare {
    grid-template-columns: 1fr;
    gap: 28px;
    padding-top: 28px;
  }

  .home-hero-title {
    font-size: 40px;
  }

  .home-cta-banner {
    flex-direction: column;
    align-items: stretch;
    padding: 28px 20px;
  }

  .home-cta-primary {
    width: 100%;
    justify-content: center;
  }

  .terminal-window {
    width: 100%;
    max-width: 420px;
  }

  .home-nav {
    height: 56px;
  }
}
</style>
