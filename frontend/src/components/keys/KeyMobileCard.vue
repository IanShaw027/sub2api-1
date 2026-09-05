<template>
  <article class="glass-card keys-card">
    <div class="keys-card-top">
      <div class="keys-card-ident">
        <span class="keys-card-name">{{ row.name }}</span>
        <span class="keys-card-meta">#{{ row.id }} · {{ row.group?.name || t('keys.noGroup') }}</span>
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
  </article>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { ApiKey } from '@/types'
import { formatDate } from '@/utils/format'
import { maskApiKey } from '@/utils/maskApiKey'
import { formatCost, statusTone, expiryToneClass } from './keyUtils'

defineProps<{
  row: ApiKey
  revealed: boolean
  copied: boolean
  todayCost: number | null | undefined
  now: Date
}>()

defineEmits<{
  more: [event: MouseEvent]
  'toggle-reveal': []
  copy: []
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
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.keys-card-meta {
  font-size: 12px;
  color: var(--muted);
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
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100%;
}

.keys-tone-danger {
  color: var(--danger-text);
}
</style>
