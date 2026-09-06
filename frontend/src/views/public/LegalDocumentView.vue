<template>
  <PublicPageLayout>
    <template #nav>
      <header class="legal-nav glass">
        <nav class="legal-nav-inner">
          <RouterLink to="/home" class="legal-brand">
            <template v-if="settings">
              <span class="brand-mark">
                <img :src="siteLogo || '/logo.svg'" alt="Logo" />
              </span>
              <span class="legal-brand-name">{{ siteName }}</span>
            </template>
            <template v-else>
              <span class="brand-mark skeleton" aria-hidden="true"></span>
              <span class="skeleton legal-brand-name-skeleton" aria-hidden="true"></span>
            </template>
          </RouterLink>
          <RouterLink to="/login" class="btn-glass-primary">
            {{ t('home.login') }}
          </RouterLink>
        </nav>
      </header>
    </template>

    <main class="legal-main">
      <div v-if="loading" class="legal-loading">
        <div class="spinner text-accent"></div>
      </div>

      <div v-else-if="loadError" class="notice notice-danger">
        <p class="legal-notice-title">{{ t('legal.loadFailed') }}</p>
        <p class="legal-notice-desc">{{ t('legal.retryLater') }}</p>
      </div>

      <div v-else-if="!currentDocument" class="empty-state legal-empty">
        <Icon name="document" size="sm" class="empty-state-icon" />
        <p class="empty-state-title">{{ t('legal.notFound') }}</p>
        <p class="empty-state-description">{{ t('legal.notFoundDescription') }}</p>
      </div>

      <article v-else class="glass-card legal-card">
        <div class="card-header legal-card-header">
          <span class="legal-doc-icon">
            <Icon :name="documentIcon" size="md" />
          </span>
          <div class="legal-doc-heading">
            <p class="legal-doc-type">{{ documentTypeLabel }}</p>
            <h1 class="section-title legal-doc-title">{{ currentDocument.title }}</h1>
            <p v-if="updatedAt" class="legal-doc-updated">
              {{ t('legal.updatedAt', { date: updatedAt }) }}
            </p>
          </div>
        </div>

        <div v-if="hasContent" class="card-body markdown-body legal-doc-content" v-html="renderedHtml"></div>
        <div v-else class="card-body">
          <p class="empty-state-description legal-doc-empty">{{ t('legal.empty') }}</p>
        </div>
      </article>
    </main>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import { getLocale } from '@/i18n'
import { sanitizeUrl } from '@/utils/url'
import { useAppStore } from '@/stores/app'
import type { LoginAgreementDocument } from '@/types'
import '@/styles/announcement-markdown.css'
import zhAdminCompliance from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enAdminCompliance from '../../../../docs/legal/admin-compliance.en.md?raw'

type LegalDocumentIcon = 'document' | 'shield' | 'globe' | 'cog'

const route = useRoute()
const { t } = useI18n()
const appStore = useAppStore()
const settings = computed(() => appStore.cachedPublicSettings)
const loading = ref(!settings.value)
const loadError = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const documentId = computed(() => String(route.params.documentId || ''))
const isAdminComplianceDocument = computed(() => documentId.value === 'admin-compliance')
const documents = computed(() => settings.value?.login_agreement_documents ?? [])
const siteName = computed(() => settings.value?.site_name || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(settings.value?.site_logo || '', {
  allowRelative: true,
  allowDataUrl: true,
}))
const updatedAt = computed(() =>
  isAdminComplianceDocument.value ? '' : settings.value?.login_agreement_updated_at || ''
)
const documentTypeLabel = computed(() =>
  isAdminComplianceDocument.value ? t('legal.adminCompliance') : t('legal.loginAgreement')
)

const currentDocument = computed<LoginAgreementDocument | null>(() => {
  if (isAdminComplianceDocument.value) {
    return {
      id: 'admin-compliance',
      title: t('adminCompliance.title'),
      content_md: getLocale() === 'zh' ? zhAdminCompliance : enAdminCompliance
    }
  }
  const id = documentId.value
  if (!id) {
    return null
  }
  return documents.value.find((doc) => doc.id === id) ?? null
})

const hasContent = computed(() => Boolean(currentDocument.value?.content_md?.trim()))

const renderedHtml = computed(() => {
  const content = currentDocument.value?.content_md?.trim() || ''
  if (!content) {
    return ''
  }
  const html = marked.parse(content) as string
  return DOMPurify.sanitize(html)
})

const documentIcon = computed<LegalDocumentIcon>(() => {
  const title = currentDocument.value?.title || ''
  if (title.includes('政策') || title.includes('隐私')) {
    return 'shield'
  }
  if (title.includes('国家') || title.includes('地区')) {
    return 'globe'
  }
  if (title.includes('特定')) {
    return 'cog'
  }
  return 'document'
})

onMounted(async () => {
  loadError.value = false
  const loadedSettings = await appStore.fetchPublicSettings()
  if (!loadedSettings) {
    loadError.value = true
  }
  loading.value = false
})
</script>

<style scoped>
.legal-nav {
  position: sticky;
  top: 0;
  z-index: 30;
  border-bottom: 1px solid var(--border);
}

.legal-nav-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  max-width: 1024px;
  margin: 0 auto;
  padding: 12px 24px;
}

.legal-brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
  text-decoration: none;
}

.legal-brand img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.legal-brand-name {
  overflow: hidden;
  font-size: var(--fs-15);
  font-weight: var(--fw-semibold);
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--foreground);
}

.legal-brand-name-skeleton {
  width: 112px;
  height: 20px;
  border-radius: var(--radius-5);
}

.legal-main {
  width: 100%;
  max-width: 800px;
  margin: 0 auto;
  padding: 48px 24px;
}

.legal-loading {
  display: flex;
  min-height: 320px;
  align-items: center;
  justify-content: center;
}

.legal-notice-desc {
  margin-top: 8px;
}

.legal-empty {
  padding: 56px 16px;
}

.legal-card-header {
  flex-direction: row;
  align-items: flex-start;
  gap: 16px;
}

.legal-doc-icon {
  display: inline-flex;
  flex: none;
  align-items: center;
  justify-content: center;
  width: 48px;
  height: 48px;
  border-radius: var(--radius-field);
  background: color-mix(in oklch, var(--accent) 10%, transparent);
  color: var(--accent);
}

.legal-doc-heading {
  min-width: 0;
}

.legal-doc-type {
  font-size: var(--fs-13);
  font-weight: var(--fw-semibold);
  color: var(--accent);
}

.legal-doc-title {
  margin-top: 8px;
  overflow-wrap: anywhere;
}

.legal-doc-updated {
  margin-top: 10px;
  font-size: var(--fs-13);
  color: var(--muted);
}

.legal-doc-content {
  overflow-wrap: anywhere;
}

.legal-doc-empty {
  padding: 40px 0;
  text-align: center;
}

@media (max-width: 640px) {
  .legal-main {
    padding: 32px 16px;
  }
}
</style>
