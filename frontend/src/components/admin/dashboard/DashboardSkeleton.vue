<template>
  <div class="dashboard-skeleton" role="status" :aria-label="t('common.loading')" aria-busy="true">
    <Skeleton v-if="hero" :height="219" aria-hidden="true" />
    <div class="dashboard-skeleton-stats" aria-hidden="true">
      <Skeleton v-for="index in 4" :key="index" :height="126" />
    </div>
    <div class="dashboard-skeleton-charts" aria-hidden="true">
      <Skeleton :height="300" />
      <Skeleton :height="300" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Skeleton from '@/components/common/Skeleton.vue'

defineProps<{ hero?: boolean }>()
const { t } = useI18n()
</script>

<style scoped>
.dashboard-skeleton {
  display: grid;
  gap: 16px;
  min-width: 0;
}
.dashboard-skeleton-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12px;
}
.dashboard-skeleton-charts {
  display: grid;
  grid-template-columns: 2fr 1fr;
  gap: 16px;
}
@media (max-width: 1023px) {
  .dashboard-skeleton-stats {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
  .dashboard-skeleton-charts {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 639px) {
  .dashboard-skeleton-stats {
    grid-template-columns: 1fr;
  }
}
</style>
