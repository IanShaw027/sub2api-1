<template>
  <PublicPageLayout>
    <div class="not-found-wrap">
      <div class="glass-card not-found-card">
        <p class="text-gradient text-display text-[64px] leading-none">404</p>
        <h1 class="mt-2 text-xl font-bold text-foreground">{{ t('errors.pageNotFound') }}</h1>
        <p class="mt-2 text-sm text-muted">{{ t('notFound.description') }}</p>

        <div class="not-found-actions">
          <button type="button" class="btn-glass-secondary" @click="goBack">
            <Icon name="arrowLeft" size="sm" />
            {{ t('notFound.goBack') }}
          </button>
          <Button variant="secondary" to="/home">
            <Icon name="home" size="sm" />
            {{ t('notFound.backHome') }}
          </Button>
        </div>
        <Button class="mt-2 w-full" :to="dashboardPath">
          <Icon name="chart" size="sm" />
          {{ t('home.goToDashboard') }}
        </Button>
      </div>
    </div>
  </PublicPageLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import PublicPageLayout from '@/components/layout/PublicPageLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Button from '@/components/ui/Button.vue'
import { useAuthStore } from '@/stores/auth'

const { t } = useI18n()
const router = useRouter()
const authStore = useAuthStore()
const dashboardPath = computed(() => authStore.isAdmin ? '/admin/dashboard' : '/dashboard')

function goBack(): void {
  router.back()
}
</script>

<style scoped>
.not-found-wrap {
  display: flex;
  width: 100%;
  min-height: calc(100vh - 64px);
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.not-found-card {
  display: flex;
  max-width: 26rem;
  width: 100%;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  padding: 48px 32px;
  text-align: center;
}

.not-found-actions {
  display: flex;
  flex-direction: column;
  gap: 12px;
  margin-top: 24px;
  width: 100%;
}

@media (min-width: 480px) {
  .not-found-actions {
    flex-direction: row;
    justify-content: center;
  }
}
</style>
