<template>
 <GlassCard padding="md" class="dash-qa-card">
 <div class="card-header">
 <span class="card-title">{{ t('dashboard.quickActions') }}</span>
 </div>
 <div class="dash-qa-list">
 <button type="button" class="dash-action" @click="router.push('/keys')">
 <span class="dash-action-icon"><Icon name="key" size="lg" /></span>
 <span class="min-w-0 flex-1 text-left">
 <span class="dash-action-title">{{ t('dashboard.createApiKey') }}</span>
 <span class="dash-action-desc">{{ t('dashboard.generateNewKey') }}</span>
 </span>
 <Icon name="chevronRight" size="md" class="text-muted" />
 </button>

 <button type="button" class="dash-action" @click="router.push('/usage')">
 <span class="dash-action-icon"><Icon name="chart" size="lg" /></span>
 <span class="min-w-0 flex-1 text-left">
 <span class="dash-action-title">{{ t('dashboard.viewUsage') }}</span>
 <span class="dash-action-desc">{{ t('dashboard.checkDetailedLogs') }}</span>
 </span>
 <Icon name="chevronRight" size="md" class="text-muted" />
 </button>

 <button v-if="canUseBatchImage" type="button" class="dash-action" @click="router.push('/batch-image')">
 <span class="dash-action-icon"><Icon name="sparkles" size="lg" /></span>
 <span class="min-w-0 flex-1 text-left">
 <span class="dash-action-title">{{ t('dashboard.batchImageAgent') }}</span>
 <span class="dash-action-desc">{{ t('dashboard.batchImageAgentDesc') }}</span>
 </span>
 <Icon name="chevronRight" size="md" class="text-muted" />
 </button>

 <button type="button" class="dash-action" @click="router.push('/redeem')">
 <span class="dash-action-icon"><Icon name="gift" size="lg" /></span>
 <span class="min-w-0 flex-1 text-left">
 <span class="dash-action-title">{{ t('dashboard.redeemCode') }}</span>
 <span class="dash-action-desc">{{ t('dashboard.addBalanceWithCode') }}</span>
 </span>
 <Icon name="chevronRight" size="md" class="text-muted" />
 </button>
 </div>
 </GlassCard>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import { useBatchImageAccess } from '@/composables/useBatchImageAccess'
const router = useRouter()
const { t } = useI18n()
const { canUseBatchImage, refreshBatchImageAccess } = useBatchImageAccess()

onMounted(() => {
 void refreshBatchImageAccess()
})
</script>
<style scoped>
.dash-qa-card {
 padding: 0 !important;
}
.dash-qa-list {
 display: flex;
 flex-direction: column;
 gap: 8px;
 padding: 12px 16px 16px;
}
.dash-action {
 display: flex;
 align-items: center;
 gap: 12px;
 width: 100%;
 min-height: 44px;
 padding: 10px 12px;
 border-radius: 12px;
 background: color-mix(in oklch, var(--surface-secondary) 70%, transparent);
 border: 0;
 cursor: pointer;
 text-align: left;
 transition: background 0.15s ease;
}
.dash-action:hover {
 background: color-mix(in oklch, var(--surface-secondary) 100%, transparent);
}
.dash-action-icon {
 width: 40px;
 height: 40px;
 border-radius: 10px;
 display: inline-flex;
 align-items: center;
 justify-content: center;
 background: color-mix(in oklch, var(--accent) 12%, transparent);
 color: var(--accent);
 flex: none;
}
.dash-action-title { display: block; font-size: 14px; font-weight: 600; color: var(--foreground); }
.dash-action-desc { display: block; font-size: 12px; color: var(--muted); }
</style>
