<template>
  <!-- 玻璃卡网格：每个渠道一张卡，卡内按平台分区展开分组 chip + 模型 chip + 价格表。
       不再使用响应式 table/card 双套 DOM——卡片本身天然在窄屏下堆叠，无需额外断点切换。 -->
  <div class="channel-list">
    <div v-if="loading" data-testid="channels-loading" class="py-10 text-center">
      <Icon name="refresh" size="lg" class="inline-block animate-spin text-muted" />
    </div>

    <EmptyState
      v-else-if="rows.length === 0"
      data-testid="channels-empty"
      :title="emptyLabel"
      size="lg"
    />

    <GlassCard
      v-else
      v-for="(channel, chIdx) in rows"
      :key="`${channel.name}-${chIdx}`"
      data-testid="channel-card"
      padding="lg"
      class="channel-card"
    >
      <header class="channel-card-header">
        <h3 class="channel-card-name">{{ channel.name }}</h3>
        <p v-if="channel.description" class="channel-card-description">
          {{ channel.description }}
        </p>
      </header>

      <div class="channel-card-body">
        <section
          v-for="section in channel.platforms"
          :key="`${channel.name}-${section.platform}`"
          class="platform-section"
        >
          <div class="platform-section-head">
            <span
              :class="[
                'inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-[11px] font-medium uppercase',
                platformBadgeClass(section.platform),
              ]"
            >
              <PlatformIcon :platform="section.platform as GroupPlatform" size="xs" />
              {{ section.platform }}
            </span>
          </div>

          <div class="platform-section-groups">
            <div class="mb-1 text-[11px] font-medium text-muted">{{ columns.groups }}</div>
            <div class="flex flex-col gap-1.5">
              <div
                v-if="exclusiveGroups(section).length > 0"
                class="flex min-w-0 flex-wrap items-center gap-1.5"
              >
                <span
                  class="inline-flex items-center gap-0.5 text-[10px] font-medium uppercase text-accent-600"
                  :title="t('availableChannels.exclusiveTooltip')"
                >
                  <Icon name="shield" size="xs" class="h-3 w-3" />
                  {{ t('availableChannels.exclusive') }}
                </span>
                <div
                  v-for="g in exclusiveGroups(section)"
                  :key="`ex-${g.id}`"
                  class="inline-flex max-w-full min-w-0 flex-wrap items-center gap-1"
                >
                  <GroupBadge
                    class="max-w-full"
                    :name="g.name"
                    :platform="g.platform as GroupPlatform"
                    :subscription-type="(g.subscription_type || 'standard') as SubscriptionType"
                    :rate-multiplier="g.rate_multiplier"
                    :user-rate-multiplier="userGroupRates[g.id] ?? null"
                    always-show-rate
                  />
                  <span
                    v-if="hasPeakRate(g)"
                    class="inline-flex items-center gap-1 rounded-md bg-warning-50 px-1.5 py-0.5 text-[10px] font-medium text-warning-text"
                    :title="peakRateTitle(g)"
                  >
                    <Icon name="clock" size="xs" class="h-3 w-3" />
                    {{ peakRateLabel(g) }}
                  </span>
                </div>
              </div>
              <div
                v-if="publicGroups(section).length > 0"
                class="flex min-w-0 flex-wrap items-center gap-1.5"
              >
                <span
                  class="inline-flex items-center gap-0.5 text-[10px] font-medium uppercase text-muted"
                  :title="t('availableChannels.publicTooltip')"
                >
                  <Icon name="globe" size="xs" class="h-3 w-3" />
                  {{ t('availableChannels.public') }}
                </span>
                <div
                  v-for="g in publicGroups(section)"
                  :key="`pub-${g.id}`"
                  class="inline-flex max-w-full min-w-0 flex-wrap items-center gap-1"
                >
                  <GroupBadge
                    class="max-w-full"
                    :name="g.name"
                    :platform="g.platform as GroupPlatform"
                    :subscription-type="(g.subscription_type || 'standard') as SubscriptionType"
                    :rate-multiplier="g.rate_multiplier"
                    :user-rate-multiplier="userGroupRates[g.id] ?? null"
                    always-show-rate
                  />
                  <span
                    v-if="hasPeakRate(g)"
                    class="inline-flex items-center gap-1 rounded-md bg-warning-50 px-1.5 py-0.5 text-[10px] font-medium text-warning-text"
                    :title="peakRateTitle(g)"
                  >
                    <Icon name="clock" size="xs" class="h-3 w-3" />
                    {{ peakRateLabel(g) }}
                  </span>
                </div>
              </div>
              <span v-if="section.groups.length === 0" class="text-xs text-muted">-</span>
            </div>
          </div>

          <div class="platform-section-models">
            <div class="mb-1 text-[11px] font-medium text-muted">{{ columns.supportedModels }}</div>
            <div class="flex min-w-0 flex-wrap gap-1">
              <SupportedModelChip
                v-for="m in section.supported_models"
                :key="`${section.platform}-${m.name}`"
                class="max-w-full [&>span]:max-w-full [&>span]:truncate"
                :model="m"
                :pricing-key-prefix="pricingKeyPrefix"
                :no-pricing-label="noPricingLabel"
                :show-platform="false"
                :platform-hint="section.platform"
              />
              <span v-if="section.supported_models.length === 0" class="text-xs text-muted">
                {{ noModelsLabel }}
              </span>
            </div>

            <!-- 阶梯/区间定价：折叠展示，避免撑大卡片；tabular mono 价格表用 PricingRow 逐行渲染。 -->
            <details
              v-for="m in tieredModels(section)"
              :key="`tier-${section.platform}-${m.name}`"
              class="pricing-details"
            >
              <summary class="pricing-details-summary">
                {{ m.name }} · {{ t(prefixKey('intervals')) }}
              </summary>
              <div class="pricing-details-body">
                <div
                  v-for="(iv, idx) in m.pricing?.intervals || []"
                  :key="idx"
                  class="pricing-tier"
                >
                  <div class="pricing-tier-label">
                    <template v-if="iv.tier_label">{{ iv.tier_label }}</template>
                    <template v-else>{{ formatRange(iv.min_tokens, iv.max_tokens) }}</template>
                  </div>
                  <template v-if="m.pricing?.billing_mode === BILLING_MODE_TOKEN">
                    <PricingRow
                      :label="t(prefixKey('inputPrice'))"
                      :value="iv.input_price"
                      :unit="t(prefixKey('unitPerMillion'))"
                      :scale="perMillionScale"
                    />
                    <PricingRow
                      :label="t(prefixKey('outputPrice'))"
                      :value="iv.output_price"
                      :unit="t(prefixKey('unitPerMillion'))"
                      :scale="perMillionScale"
                    />
                  </template>
                  <PricingRow
                    v-else
                    :label="t(prefixKey('perRequestPrice'))"
                    :value="iv.per_request_price"
                    :unit="t(prefixKey('unitPerRequest'))"
                    :scale="1"
                  />
                </div>
              </div>
            </details>
          </div>
        </section>
      </div>
    </GlassCard>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import GroupBadge from '@/components/common/GroupBadge.vue'
import GlassCard from '@/components/ui/GlassCard.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import SupportedModelChip from './SupportedModelChip.vue'
import PricingRow from './PricingRow.vue'
import { BILLING_MODE_TOKEN } from '@/constants/channel'
import type { UserAvailableChannel, UserAvailableGroup, UserChannelPlatformSection, UserSupportedModel } from '@/api/channels'
import type { GroupPlatform, SubscriptionType } from '@/types'
import { platformBadgeClass } from '@/utils/platformColors'
import { useAppStore } from '@/stores/app'
import { hasPeakRate as groupHasPeakRate, formatPeakRateWindow, serverTimezoneLabel } from '@/utils/peak-rate'

const props = defineProps<{
  columns: {
    name: string
    description: string
    platform: string
    groups: string
    supportedModels: string
  }
  rows: UserAvailableChannel[]
  loading: boolean
  pricingKeyPrefix: string
  noPricingLabel: string
  noModelsLabel: string
  emptyLabel: string
  /** 用户专属倍率（group_id → multiplier）；无专属时由 GroupBadge 仅显示默认倍率。 */
  userGroupRates: Record<number, number>
}>()

const { t } = useI18n()

function exclusiveGroups(section: UserChannelPlatformSection): UserAvailableGroup[] {
  return section.groups.filter((g) => g.is_exclusive)
}

function publicGroups(section: UserChannelPlatformSection): UserAvailableGroup[] {
  return section.groups.filter((g) => !g.is_exclusive)
}

/** 有阶梯/区间定价（intervals）的模型——用折叠详情表展示，其余模型只靠 SupportedModelChip 的悬浮价格。 */
function tieredModels(section: UserChannelPlatformSection): UserSupportedModel[] {
  return section.supported_models.filter((m) => (m.pricing?.intervals?.length ?? 0) > 0)
}

const perMillionScale = 1_000_000

function prefixKey(k: string): string {
  return `${props.pricingKeyPrefix}.${k}`
}

function formatRange(min: number, max: number | null): string {
  const maxLabel = max == null ? '∞' : String(max)
  return `(${min}, ${maxLabel}]`
}

const appStore = useAppStore()

function hasPeakRate(group: UserAvailableGroup): boolean {
  return groupHasPeakRate(group)
}

function peakRateLabel(group: UserAvailableGroup): string {
  return formatPeakRateWindow(group, serverTimezoneLabel(appStore.cachedPublicSettings?.server_utc_offset))
}

function peakRateTitle(group: UserAvailableGroup): string {
  return t('common.peakRateTooltip', { window: peakRateLabel(group) }) + t('common.peakRateImageNote')
}
</script>

<style scoped>
.channel-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
  min-width: 0;
}

.channel-card {
  border-radius: var(--radius-hero);
  min-width: 0;
}

.channel-card.ui-glass-card-pad-lg {
  padding: 24px;
}

.channel-card-header {
  border-bottom: 1px solid var(--border);
  padding-bottom: 12px;
  margin-bottom: 14px;
}

.channel-card-name {
  font-size: 15px;
  font-weight: 600;
  color: var(--foreground);
}

.channel-card-description {
  margin-top: 2px;
  font-size: 12.5px;
  color: var(--muted);
}

.channel-card-body {
  display: flex;
  flex-direction: column;
  gap: 16px;
}

.platform-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
  min-width: 0;
  padding-top: 12px;
  border-top: 1px solid var(--border);
}

.platform-section:first-child {
  padding-top: 0;
  border-top: none;
}

.platform-section-groups,
.platform-section-models {
  min-width: 0;
}

.pricing-details {
  margin-top: 8px;
  border: 1px solid var(--border);
  border-radius: var(--radius-field);
  padding: 6px 10px;
}

.pricing-details-summary {
  cursor: pointer;
  font-size: 11.5px;
  font-weight: 600;
  color: var(--muted);
  list-style: none;
}

.pricing-details-summary::-webkit-details-marker {
  display: none;
}

.pricing-details-body {
  margin-top: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: 100%;
  overflow-x: auto;
  font-size: 12px;
}

.pricing-tier {
  display: flex;
  flex-direction: column;
  gap: 2px;
  padding-bottom: 6px;
  border-bottom: 1px dashed var(--border);
}

.pricing-tier:last-child {
  border-bottom: none;
  padding-bottom: 0;
}

.pricing-tier-label {
  font-size: 11px;
  font-weight: 600;
  color: var(--foreground);
}
</style>
