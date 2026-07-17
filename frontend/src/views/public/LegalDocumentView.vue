<template>
  <div class="min-h-screen bg-page text-ink-body dark:bg-page dark:text-ink-body">
    <header class="border-b border-line/60 bg-page/70 backdrop-blur-md dark:border-line/60 dark:bg-page/60">
      <div class="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-4 sm:px-6">
        <RouterLink to="/home" class="flex min-w-0 items-center gap-2.5 no-underline">
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
        </RouterLink>
        <RouterLink
          to="/login"
          class="inline-flex flex-shrink-0 items-center justify-center rounded-full bg-ink px-3 py-1.5 text-xs font-medium text-white transition-opacity hover:opacity-90 dark:bg-dark-700"
        >
          {{ t('home.login') }}
        </RouterLink>
      </div>
    </header>

    <main class="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:py-10">
      <div v-if="loading" class="flex min-h-[320px] items-center justify-center">
        <div class="h-8 w-8 animate-spin rounded-full border-b-2 border-brand"></div>
      </div>

      <section
        v-else-if="loadError"
        class="rounded-card border border-danger/30 bg-danger-soft p-6 text-danger dark:border-red-500/30 dark:bg-red-500/10 dark:text-red-200"
      >
        <h1 class="text-lg font-semibold">{{ t('legal.loadFailed') }}</h1>
        <p class="mt-2 text-sm">{{ t('legal.retryLater') }}</p>
      </section>

      <section
        v-else-if="!currentDocument"
        class="rounded-card border border-line bg-card p-6 shadow-xs dark:border-line dark:bg-card"
      >
        <div class="flex items-start gap-3">
          <span class="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-xl bg-page text-ink-soft dark:bg-card dark:text-ink-soft">
            <Icon name="document" size="sm" />
          </span>
          <div>
            <h1 class="text-lg font-semibold text-ink dark:text-ink">{{ t('legal.notFound') }}</h1>
            <p class="mt-2 text-sm leading-6 text-ink-soft dark:text-ink-soft">
              {{ t('legal.notFoundDescription') }}
            </p>
          </div>
        </div>
      </section>

      <article v-else>
        <div class="mb-8 border-b border-line pb-6 dark:border-line">
          <div class="flex items-start gap-4">
            <span class="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl bg-brand/10 text-brand-700 dark:bg-brand/15 dark:text-brand-300">
              <Icon :name="documentIcon" size="md" />
            </span>
            <div class="min-w-0">
              <p class="text-sm font-medium text-brand-700 dark:text-brand-300">{{ documentTypeLabel }}</p>
              <h1 class="mt-2 break-words text-2xl font-semibold tracking-tight text-ink dark:text-ink sm:text-3xl">
                {{ currentDocument.title }}
              </h1>
              <p v-if="updatedAt" class="mt-3 text-sm text-ink-soft dark:text-ink-soft">
                {{ t('legal.updatedAt', { date: updatedAt }) }}
              </p>
            </div>
          </div>
        </div>

        <div
          v-if="hasContent"
          class="legal-document-content"
          v-html="renderedHtml"
        ></div>
        <div
          v-else
          class="rounded-card border border-dashed border-line bg-card px-6 py-14 text-center text-sm text-ink-soft dark:border-line dark:bg-card dark:text-ink-soft"
        >
          {{ t('legal.empty') }}
        </div>
      </article>
    </main>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { BrandLogo } from '@/components/brand'
import { getPublicSettings } from '@/api/auth'
import { getLocale } from '@/i18n'
import { sanitizeUrl } from '@/utils/url'
import type { LoginAgreementDocument, PublicSettings } from '@/types'
import zhAdminCompliance from '../../../../docs/legal/admin-compliance.zh.md?raw'
import enAdminCompliance from '../../../../docs/legal/admin-compliance.en.md?raw'

type LegalDocumentIcon = 'document' | 'shield' | 'globe' | 'cog'

const route = useRoute()
const { t } = useI18n()
const settings = ref<PublicSettings | null>(null)
const loading = ref(true)
const loadError = ref(false)

marked.setOptions({
  breaks: true,
  gfm: true,
})

const documentId = computed(() => String(route.params.documentId || ''))
const isAdminComplianceDocument = computed(() => documentId.value === 'admin-compliance')
const documents = computed(() => settings.value?.login_agreement_documents ?? [])
const siteName = computed(() => settings.value?.site_name || 'Clomio')
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
  loading.value = true
  loadError.value = false
  try {
    settings.value = await getPublicSettings()
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
})
</script>

<style scoped>
.legal-document-content {
  line-height: 1.75;
  overflow-wrap: anywhere;
  color: inherit;
}

.legal-document-content :deep(h1) {
  @apply mb-4 mt-8 border-b border-line pb-3 text-3xl font-bold dark:border-line;
}

.legal-document-content :deep(h2) {
  @apply mb-3 mt-7 text-2xl font-bold text-ink dark:text-ink;
}

.legal-document-content :deep(h3) {
  @apply mb-2 mt-6 text-xl font-semibold text-ink dark:text-ink;
}

.legal-document-content :deep(h4) {
  @apply mb-2 mt-5 text-lg font-semibold text-ink dark:text-ink;
}

.legal-document-content :deep(p) {
  @apply mb-4 text-ink-body dark:text-ink-body;
}

.legal-document-content :deep(a) {
  @apply text-accent underline underline-offset-4 hover:text-accent-600 dark:text-accent-300 dark:hover:text-accent-200;
}

.legal-document-content :deep(ul) {
  @apply mb-4 list-disc pl-6;
}

.legal-document-content :deep(ol) {
  @apply mb-4 list-decimal pl-6;
}

.legal-document-content :deep(li) {
  @apply mb-1 text-ink-body dark:text-ink-body;
}

.legal-document-content :deep(blockquote) {
  @apply my-5 border-l-4 border-line pl-4 text-ink-soft dark:border-line dark:text-ink-soft;
}

.legal-document-content :deep(code) {
  @apply rounded bg-page px-1.5 py-0.5 font-mono text-sm dark:bg-card;
}

.legal-document-content :deep(pre) {
  @apply my-5 overflow-x-auto rounded-card p-4;
  background: rgb(var(--code-bg));
  color: rgb(var(--code-fg));
}

.legal-document-content :deep(pre code) {
  @apply bg-transparent p-0 text-inherit;
}

.legal-document-content :deep(table) {
  @apply my-5 block w-full overflow-x-auto border-collapse;
}

.legal-document-content :deep(th) {
  @apply border border-line bg-page px-3 py-2 text-left font-semibold dark:border-line dark:bg-card;
}

.legal-document-content :deep(td) {
  @apply border border-line px-3 py-2 dark:border-line;
}

.legal-document-content :deep(img) {
  @apply my-5 h-auto max-w-full rounded-card;
}

.legal-document-content :deep(hr) {
  @apply my-7 border-line dark:border-line;
}
</style>
