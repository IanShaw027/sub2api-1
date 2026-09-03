<template>
  <div class="plaza-nav glass">
    <div class="plaza-nav-inner">
      <!-- 左:站点 logo + 名称 -->
      <div class="plaza-nav-brand">
        <template v-if="settings">
          <span class="brand-mark">
            <img :src="siteLogo || '/logo.svg'" alt="Logo" />
          </span>
          <span class="plaza-nav-name">{{ siteName }}</span>
        </template>
        <template v-else>
          <span class="brand-mark skeleton" aria-hidden="true"></span>
          <span class="skeleton plaza-nav-name-skeleton" aria-hidden="true"></span>
        </template>
      </div>

      <!-- 右:登录 / 回到后台 -->
      <Button v-if="isAuthenticated" :to="backTarget" class="plaza-nav-cta">
        {{ t('modelPlaza.nav.backToDashboard') }}
      </Button>
      <Button v-else :to="{ path: '/login', query: { redirect: '/model-plaza' } }" class="plaza-nav-cta">
        {{ t('modelPlaza.nav.login') }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Button from '@/components/ui/Button.vue'
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

<style scoped>
.plaza-nav {
  position: sticky;
  top: 0;
  z-index: 30;
  border-bottom: 1px solid var(--border);
}

.plaza-nav-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  max-width: 1280px;
  margin: 0 auto;
  padding: 12px 24px;
}

.plaza-nav-brand {
  display: flex;
  min-width: 0;
  align-items: center;
  gap: 10px;
}

.plaza-nav-brand img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}

.plaza-nav-name {
  overflow: hidden;
  font-size: 15px;
  font-weight: 600;
  text-overflow: ellipsis;
  white-space: nowrap;
  color: var(--foreground);
}

.plaza-nav-name-skeleton {
  width: 112px;
  height: 20px;
  border-radius: 5px;
}

.plaza-nav-cta {
  flex: none;
}
</style>
