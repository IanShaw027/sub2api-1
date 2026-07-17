<template>
  <div
    class="relative flex min-h-screen flex-col items-center justify-center overflow-hidden bg-page px-4 text-ink-body ambient-canvas dark:bg-page"
  >
    <div class="relative z-10 w-full max-w-md text-center">
      <!-- Brand -->
      <div class="mb-10 flex justify-center">
        <router-link to="/home" class="inline-flex no-underline" :aria-label="siteName">
          <BrandLogo :size="36" :wordmark="siteName" />
        </router-link>
      </div>

      <!-- Calm 404 mark -->
      <div class="mb-8">
        <div
          class="mx-auto mb-6 flex h-20 w-20 items-center justify-center rounded-card border border-line bg-card shadow-xs dark:border-line dark:bg-card"
        >
          <span
            class="text-2xl font-[650] tracking-tight text-ink-soft dark:text-ink-soft"
            aria-hidden="true"
          >404</span>
        </div>
        <h1 class="mb-3 text-2xl font-semibold tracking-tight text-ink dark:text-ink">
          {{ t('errors.pageNotFound') }}
        </h1>
        <p class="text-sm leading-relaxed text-ink-soft dark:text-ink-soft">
          {{ t('errors.pageNotFoundDescription') }}
        </p>
      </div>

      <!-- Actions: home + login -->
      <div class="flex flex-col justify-center gap-3 sm:flex-row">
        <router-link to="/home" class="btn btn-primary">
          <Icon name="home" size="md" class="mr-2" />
          {{ t('home.getStarted') }}
        </router-link>
        <router-link to="/login" class="btn btn-secondary">
          {{ t('home.login') }}
        </router-link>
      </div>

      <button
        type="button"
        class="mt-6 text-sm text-ink-faint transition-colors hover:text-ink-soft dark:hover:text-ink-soft"
        @click="goBack"
      >
        ← {{ t('common.back') }}
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { useRouter } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { BrandLogo } from '@/components/brand'

const { t } = useI18n()
const appStore = useAppStore()
const siteName = computed(() => appStore.siteName || 'Clomio')
const router = useRouter()

function goBack(): void {
  router.back()
}
</script>
