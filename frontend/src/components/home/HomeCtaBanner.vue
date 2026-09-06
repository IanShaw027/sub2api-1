<template>
  <section class="cta-banner">
    <div>
      <h2>{{ t('home.cta.title') }}</h2>
      <p>{{ t('home.cta.description') }}</p>
    </div>
    <router-link :to="to" class="btn btn-primary cta-btn">
      {{ label }}
    </router-link>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { RouteLocationRaw } from 'vue-router'

defineProps<{ to: RouteLocationRaw; label: string }>()
const { t } = useI18n()
</script>

<style scoped>
.cta-banner {
  padding: 40px 48px;
  border-radius: 20px;
  position: relative;
  overflow: hidden;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  color: var(--foreground);
  background:
    linear-gradient(120deg, color-mix(in oklch, var(--accent) 16%, transparent), color-mix(in oklch, var(--success) 10%, transparent)),
    color-mix(in oklch, var(--surface) 70%, transparent);
  border: 1px solid color-mix(in oklch, var(--accent) 35%, transparent);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  box-shadow: var(--shadow), 0 30px 60px -40px var(--accent);
}

.cta-banner h2 {
  margin: 0;
  font-family: var(--display);
  font-size: 28px;
  font-weight: 800;
  letter-spacing: -0.03em;
}

.cta-banner p {
  margin: 8px 0 0;
  font-size: 14.5px;
  color: var(--muted);
}

.cta-btn {
  height: 44px;
  padding: 0 22px;
  border-radius: 12px;
  font-size: 14px;
  font-weight: 700;
  box-shadow: 0 12px 28px -12px var(--accent);
  flex: none;
}

@media (max-width: 767px) {
  .cta-banner {
    padding: 28px 20px;
    flex-direction: column;
    align-items: stretch;
  }

  .cta-btn {
    width: 100%;
  }
}
</style>
