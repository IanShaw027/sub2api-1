<template>
  <article class="glass-card keys-card">
    <div class="keys-card-top">
      <div class="keys-card-ident">
        <span class="keys-card-name">{{ row.name }}</span>
        <span class="keys-card-meta">#{{ row.id }}</span>
        <div class="group/dropdown">
          <button
            :ref="(el) => setGroupButtonRef?.(row.id, el)"
            type="button"
            class="keys-card-group"
            :aria-label="t('keys.group')"
            @click="$emit('open-group-selector')"
          >{{ row.group?.name || t('keys.noGroup') }}<Icon name="chevronDown" size="xs" /></button>
        </div>
      </div>
      <div class="keys-card-top-right">
        <StatusBadge :tone="statusTone(row.status)" :label="t('keys.status.' + row.status)" />
        <button
          type="button"
          class="icon-btn keys-card-more"
          :title="t('keys.moreActions')"
          :aria-label="t('keys.moreActions')"
          @click.stop="$emit('more', $event)"
        >
          <Icon name="more" size="sm" :stroke-width="2.4" />
        </button>
      </div>
    </div>
    <div class="keys-card-key">
      <span class="keys-card-key-text">{{ revealed ? row.key : maskApiKey(row.key) }}</span>
      <button
        type="button"
        class="keys-card-key-btn"
        :title="revealed ? t('keys.hideKey') : t('keys.showKey')"
        :aria-label="revealed ? t('keys.hideKey') : t('keys.showKey')"
        @click.stop="$emit('toggle-reveal')"
      >
        <Icon :name="revealed ? 'eyeOff' : 'eye'" size="sm" />
      </button>
      <button
        type="button"
        class="keys-card-key-btn"
        :title="copied ? t('keys.copied') : t('keys.copyToClipboard')"
        :aria-label="t('keys.copyToClipboard')"
        @click.stop="$emit('copy')"
      >
        <Icon :name="copied ? 'check' : 'clipboard'" size="sm" />
      </button>
    </div>
    <div class="keys-card-stats">
      <div class="keys-card-stat">
        <span>{{ t('keys.today') }}</span>
        <b>{{ formatCost(todayCost) }}</b>
      </div>
      <div class="keys-card-stat">
        <span>{{ t('common.total') }}</span>
        <b>{{ formatCost(totalCost, 4) }}</b>
      </div>
      <div class="keys-card-stat">
        <span>{{ t('keys.quotaUsed') }}</span>
        <b>{{ formatCost(row.quota_used) }}<span v-if="row.quota > 0"> / {{ formatCost(row.quota) }}</span></b>
      </div>
      <div class="keys-card-stat">
        <span>{{ t('keys.currentConcurrency') }}</span>
        <b>{{ row.current_concurrency ?? 0 }}</b>
      </div>
      <div class="keys-card-stat">
        <span>{{ t('keys.expiresAt') }}</span>
        <b :class="expiryToneClass(row.expires_at, now)">
          {{ row.expires_at ? formatDate(row.expires_at) : t('keys.noExpiration') }}
        </b>
      </div>
      <div class="keys-card-stat is-end">
        <span>{{ t('keys.lastUsedAt') }}</span>
        <b>{{ row.last_used_at ? formatDate(row.last_used_at) : '—' }}</b>
      </div>
    </div>
    <div class="keys-card-details">
      <div><span>{{ t('keys.lastUsedIP') }}</span><b class="keys-card-mono">{{ row.last_used_ip || '—' }}</b></div>
      <div><span>{{ t('keys.created') }}</span><b>{{ formatDate(row.created_at) }}</b></div>
    </div>
    <div v-if="activeRateLimitWindows(row).length" class="keys-card-limits">
      <span>{{ t('keys.rateLimitUsage') }}</span>
      <div v-for="window in activeRateLimitWindows(row)" :key="window.key" class="keys-card-limit">
        <b>{{ window.shortLabel }}: {{ formatCost(row[window.usageField], 4) }} / {{ formatCost(row[window.limitField], 4) }}</b>
        <span v-if="row[window.resetField]">{{ t('dashboard.platformQuota.resetsAt', { time: formatDate(row[window.resetField]!) }) }}</span>
      </div>
      <button
        v-if="row.usage_5h > 0 || row.usage_1d > 0 || row.usage_7d > 0"
        type="button"
        class="icon-btn"
        :title="t('keys.resetRateLimitUsage')"
        :aria-label="t('keys.resetRateLimitUsage')"
        @click.stop="$emit('reset-rate-limit')"
      ><Icon name="refresh" size="sm" /></button>
    </div>
    <KeyInlineActions
      :row="row"
      :hide-ccs-import="hideCcsImport"
      @use="$emit('use')"
      @import-ccs="$emit('import-ccs')"
      @toggle-status="$emit('toggle-status')"
      @edit="$emit('edit')"
      @delete="$emit('delete')"
    />
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { ComponentPublicInstance } from 'vue'
import KeyInlineActions from './KeyInlineActions.vue'
import Icon from '@/components/icons/Icon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { ApiKey } from '@/types'
import { formatDate } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { formatCost, statusTone, expiryToneClass, activeRateLimitWindows } from './keyUtils'

defineProps<{
  row: ApiKey
  revealed: boolean
  copied: boolean
  todayCost: number | null | undefined
  totalCost?: number | null
  now: Date
  hideCcsImport?: boolean
  setGroupButtonRef?: (id: number, el: Element | ComponentPublicInstance | null) => void
}>()

defineEmits<{
  more: [event: MouseEvent]
  'toggle-reveal': []
  copy: []
  use: []
  'import-ccs': []
  'toggle-status': []
  edit: []
  delete: []
  'reset-rate-limit': []
  'open-group-selector': []
}>()

const { t } = useI18n()
</script>

<style scoped>
.keys-card {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 14px;
  border-radius: 14px;
}

.keys-card-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
}

.keys-card-ident {
  display: flex;
  flex-direction: column;
  gap: 3px;
  min-width: 0;
}

.keys-card-name {
  font-size: 15px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.keys-card-meta {
  font-size: 12px;
  color: var(--muted);
  overflow-wrap: anywhere;
}

.keys-card-group {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  min-height: 44px;
  max-width: 100%;
  color: var(--info-text);
  font-size: 12px;
  overflow-wrap: anywhere;
  text-align: left;
}

.keys-card-top-right {
  display: flex;
  align-items: center;
  gap: 4px;
  flex: none;
}

.keys-card-more {
  /* touch target: keep the icon compact but ensure a >=44px hit area on mobile */
  width: 44px;
  height: 44px;
}

.keys-card-key {
  display: flex;
  align-items: center;
  gap: 4px;
  height: 44px;
  padding: 0 4px 0 12px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: color-mix(in oklch, var(--foreground) 4%, transparent);
}

.keys-card-key-text {
  flex: 1;
  min-width: 0;
  font-family: var(--font-mono);
  font-size: 13px;
  letter-spacing: 0.01em;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-card-key-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  height: 44px;
  flex: none;
  border: 0;
  border-radius: 8px;
  background: transparent;
  color: var(--muted);
  cursor: pointer;
}

.keys-card-key-btn:hover {
  background: color-mix(in oklch, var(--foreground) 6%, transparent);
  color: var(--foreground);
}

.keys-card-stats {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 6px;
  font-size: 11px;
  color: var(--muted);
}

.keys-card-details {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 8px;
  padding-top: 10px;
  border-top: 1px solid var(--border);
  font-size: 11px;
  color: var(--muted);
}

.keys-card-details > div {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.keys-card-details b {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--foreground);
  font-size: 12px;
  font-weight: 600;
}

.keys-card-mono {
  font-family: var(--font-mono);
}

@media (max-width: 430px) {
  .keys-card-stats { grid-template-columns: repeat(2, minmax(0, 1fr)); row-gap: 8px; }
  .keys-card-stat.is-end { align-items: flex-start; text-align: left; }
  .keys-card-details { grid-template-columns: 1fr 1fr; }
}

.keys-card-stat {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}

.keys-card-stat.is-end {
  align-items: flex-end;
  text-align: right;
}

.keys-card-stat b {
  font-size: 13px;
  font-weight: 600;
  color: var(--foreground);
  font-variant-numeric: tabular-nums;
  overflow-wrap: anywhere;
  max-width: 100%;
}

.keys-card-limits,
.keys-card-limit {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
  overflow-wrap: anywhere;
}

.keys-card-limits {
  border-top: 1px solid var(--border);
  padding-top: 10px;
  font-size: 12px;
  color: var(--muted);
}

.keys-card-limit b { color: var(--foreground); }

.keys-tone-danger {
  color: var(--danger-text);
}
</style>
