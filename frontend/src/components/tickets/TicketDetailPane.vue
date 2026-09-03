<template>
 <div class="glass-card detail-pane">
 <div class="card-header detail-pane-header">
 <div class="min-w-0">
 <p class="card-subtitle">{{ t('tickets.details.title') }}</p>
 <h2 class="card-title mt-1 truncate">{{ ticket.title }}</h2>
 <p class="mt-1 font-mono text-xs text-muted">#{{ ticket.ticket_no }}</p>
 </div>
 <StatusBadge dot :tone="statusTone" :label="t(`tickets.statuses.${ticket.status}`)" />
 </div>

 <div class="detail-pane-body">
 <div v-if="showUserMeta" class="user-meta">
 <div class="user-meta-avatar">
 <span>{{ displayName.slice(0, 1).toUpperCase() }}</span>
 </div>
 <div class="min-w-0">
 <p class="user-meta-name">{{ displayName }}</p>
 <p class="user-meta-email truncate">{{ ticket.user_email }}</p>
 </div>
 </div>

 <div class="meta-rows">
 <div class="meta-row">
 <span class="meta-label">{{ t('tickets.details.category') }}</span>
 <span class="meta-value">{{ t(`tickets.categories.${ticket.category}`) }}</span>
 </div>
 <div class="meta-row">
 <span class="meta-label">{{ t('tickets.details.createdAt') }}</span>
 <span class="meta-value font-mono">{{ formatDateTime(ticket.created_at) }}</span>
 </div>
 <div class="meta-row">
 <span class="meta-label">{{ t('tickets.details.updatedAt') }}</span>
 <span class="meta-value font-mono">{{ formatDateTime(ticket.updated_at) }}</span>
 </div>
 <div v-if="showUserMeta" class="meta-row">
 <span class="meta-label">{{ t('tickets.details.user') }}</span>
 <span class="meta-value truncate">{{ displayName }}</span>
 </div>
 </div>

 <div class="form-data">
 <p class="form-data-title">{{ t('tickets.details.formData') }}</p>
 <TicketCategoryForm :category="ticket.category" :model-value="ticket.current_form_payload || {}" readonly />
 </div>
 </div>

 <div v-if="$slots.actions" class="detail-pane-actions">
 <slot name="actions" />
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import StatusBadge from '@/components/ui/StatusBadge.vue'
import type { StatusBadgeTone } from '@/components/ui/types'
import type { SupportTicket, TicketStatus } from '@/types/ticket'
import TicketCategoryForm from './TicketCategoryForm.vue'

const props = defineProps<{
 ticket: SupportTicket
 showUserMeta?: boolean
}>()

const { t } = useI18n()
const displayName = computed(() => props.ticket.user_name || props.ticket.user_email || String(props.ticket.user_id || ''))

const statusToneMap: Record<TicketStatus, StatusBadgeTone> = {
 submitted: 'accent',
 processing: 'warning',
 waiting_user: 'warning',
 waiting_admin: 'accent',
 resolved: 'success',
 closed: 'muted',
 withdrawn: 'danger',
}

const statusTone = computed<StatusBadgeTone>(() => statusToneMap[props.ticket.status] || 'muted')
</script>

<style scoped>
.detail-pane {
 display: flex;
 height: 100%;
 min-height: 0;
 flex-direction: column;
 overflow: hidden;
 padding: 0;
}

.detail-pane-header {
 display: flex;
 flex-direction: row;
 align-items: flex-start;
 justify-content: space-between;
 gap: 12px;
}

.detail-pane-body {
 min-height: 0;
 flex: 1;
 overflow-y: auto;
 padding: 16px 18px;
 display: flex;
 flex-direction: column;
 gap: 16px;
}

.user-meta {
 display: flex;
 align-items: center;
 gap: 12px;
 border-radius: var(--radius-field);
 background: var(--surface-secondary);
 padding: 10px 12px;
}

.user-meta-avatar {
 display: flex;
 height: 36px;
 width: 36px;
 flex: none;
 align-items: center;
 justify-content: center;
 border-radius: 999px;
 background: color-mix(in oklch, var(--accent) 18%, transparent);
 color: var(--accent);
 font-size: 13px;
 font-weight: 700;
}

.user-meta-name {
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
}

.user-meta-email {
 font-size: 12px;
 color: var(--muted);
}

.meta-rows {
 display: flex;
 flex-direction: column;
}

.meta-row {
 display: flex;
 align-items: center;
 justify-content: space-between;
 gap: 12px;
 padding: 10px 0;
 border-bottom: 1px solid var(--border);
}

.meta-row:last-child {
 border-bottom: 0;
}

.meta-label {
 font-size: 12.5px;
 color: var(--muted);
}

.meta-value {
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
 text-align: right;
}

.form-data {
 border-top: 1px solid var(--border);
 padding-top: 16px;
}

.form-data-title {
 margin-bottom: 10px;
 font-size: 13px;
 font-weight: 600;
 color: var(--foreground);
}

.detail-pane-actions {
 border-top: 1px solid var(--border);
 padding: 14px 18px;
 display: flex;
 flex-wrap: wrap;
 gap: 10px;
}
</style>
