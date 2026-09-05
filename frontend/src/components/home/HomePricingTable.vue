<template>
  <div class="pricing-table glass-card">
    <div class="pricing-head">
      <span>{{ t('home.pricing.headers.model') }}</span>
      <span>{{ t('home.pricing.headers.vendor') }}</span>
      <span>{{ t('home.pricing.headers.input') }}</span>
      <span>{{ t('home.pricing.headers.output') }}</span>
      <span>{{ t('home.pricing.headers.context') }}</span>
      <span>{{ t('home.pricing.headers.status') }}</span>
    </div>
    <template v-if="rows.length">
      <div v-for="m in rows" :key="m.model" class="pricing-row">
        <span class="pricing-model">{{ m.model }}</span>
        <span class="pricing-vendor">
          <span class="platform-tile" :style="{ background: platformTileBackground(m.platform) }">
            <PlatformIcon :platform="m.platform" size="xs" />
          </span>
          {{ m.vendor }}
        </span>
        <span class="num">{{ m.input }}</span>
        <span class="num">{{ m.output }}</span>
        <span class="pricing-ctx">{{ m.context }}</span>
        <StatusBadge :tone="m.limited ? 'warning' : 'success'" dot :label="m.limited ? t('home.pricing.limited') : t('home.pricing.normal')" />
      </div>
    </template>
    <div v-else class="pricing-empty">{{ t('home.pricing.empty') }}</div>
    <div class="pricing-foot">{{ t('home.pricing.footnote') }}</div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import { platformTileBackground } from '@/utils/platformTile'
import type { GroupPlatform } from '@/types'

export interface PricingRow {
  model: string
  vendor: string
  platform: GroupPlatform
  input: string
  output: string
  context: string
  limited?: boolean
}

defineProps<{ rows: PricingRow[] }>()
const { t } = useI18n()
</script>

<style scoped>
.pricing-table {
  border-radius: 16px;
  overflow: hidden;
}

.pricing-head,
.pricing-row {
  display: grid;
  grid-template-columns: 2fr 1fr 1fr 1fr 1fr 110px;
  gap: 16px;
  padding: 12px 22px;
  align-items: center;
  border-bottom: 1px solid var(--border);
}

.pricing-head {
  font-size: 11px;
  font-weight: 600;
  color: var(--muted);
  letter-spacing: 0.06em;
  background: color-mix(in oklch, var(--surface-secondary) 45%, transparent);
}

.pricing-row {
  padding: 13px 22px;
  font-size: 13.5px;
  transition: background 0.15s ease, box-shadow 0.15s ease;
}

.pricing-row:hover {
  background: color-mix(in oklch, var(--accent) 5%, transparent);
  box-shadow: inset 3px 0 0 var(--accent);
}

.pricing-model {
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 500;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.pricing-vendor {
  display: flex;
  align-items: center;
  gap: 8px;
}

.platform-tile {
  width: 20px;
  height: 20px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  color: white;
  box-shadow: inset 0 0 0 1px color-mix(in oklch, white 14%, transparent);
  flex: none;
}

.pricing-ctx {
  color: var(--muted);
}

.pricing-foot {
  padding: 12px 22px;
  font-size: 12.5px;
  color: var(--muted);
}

.pricing-empty {
  padding: 28px 22px;
  text-align: center;
  font-size: 13px;
  color: var(--muted);
  border-bottom: 1px solid var(--border);
}

@media (max-width: 860px) {
  .pricing-head {
    display: none;
  }

  .pricing-row {
    grid-template-columns: 1fr auto;
    grid-template-areas:
      'model status'
      'vendor vendor'
      'input output'
      'ctx ctx';
    gap: 6px 12px;
    padding: 14px 16px;
  }

  .pricing-row > :nth-child(1) { grid-area: model; }
  .pricing-row > :nth-child(2) { grid-area: vendor; font-size: 12.5px; color: var(--muted); }
  .pricing-row > :nth-child(3) { grid-area: input; }
  .pricing-row > :nth-child(4) { grid-area: output; }
  .pricing-row > :nth-child(5) { grid-area: ctx; font-size: 12px; }
  .pricing-row > :nth-child(6) { grid-area: status; justify-self: end; }
}
</style>
