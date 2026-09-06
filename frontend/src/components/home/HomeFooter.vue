<template>
  <footer class="home-footer">
    <span>&copy; {{ currentYear }} {{ siteName }}. {{ t('home.footer.allRightsReserved') }}</span>
    <div class="home-footer-links">
      <template v-if="legalDocuments.length">
        <router-link v-for="doc in legalDocuments" :key="doc.id" :to="`/legal/${doc.id}`">{{ doc.title }}</router-link>
      </template>
      <template v-else>
        <router-link to="/legal/terms">{{ t('home.footerLinks.terms') }}</router-link>
        <router-link to="/legal/privacy">{{ t('home.footerLinks.privacy') }}</router-link>
      </template>
      <a v-if="docUrl" :href="docUrl" target="_blank" rel="noopener noreferrer">{{ t('home.nav.docs') }}</a>
      <a :href="githubUrl" target="_blank" rel="noopener noreferrer">GitHub</a>
      <span v-if="contactInfo" :title="contactInfo">{{ t('home.footerLinks.contact') }}</span>
    </div>
  </footer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'

export interface LegalDocument {
  id: string
  title: string
}

defineProps<{
  currentYear: number
  siteName: string
  legalDocuments: LegalDocument[]
  docUrl?: string
  githubUrl: string
  contactInfo?: string
}>()
const { t } = useI18n()
</script>

<style scoped>
.home-footer {
  width: 100%;
  max-width: 1440px;
  margin: 0 auto;
  padding: 20px 80px 28px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  flex-wrap: wrap;
  font-size: 12.5px;
  color: var(--muted);
  border-top: 1px solid var(--border);
}

.home-footer-links {
  display: flex;
  flex-wrap: wrap;
  gap: 20px;
}

.home-footer-links a,
.home-footer-links span {
  color: inherit;
  text-decoration: none;
}

.home-footer-links a:hover {
  color: var(--foreground);
}

@media (max-width: 1180px) {
  .home-footer {
    padding-left: 40px;
    padding-right: 40px;
  }
}

@media (max-width: 767px) {
  .home-footer {
    padding-left: 20px;
    padding-right: 20px;
    flex-direction: column;
    align-items: flex-start;
  }
}
</style>
