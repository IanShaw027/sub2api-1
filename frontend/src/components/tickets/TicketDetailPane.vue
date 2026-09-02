<template>
 <div class="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border bg-surface p-5">
 <div class="shrink-0 space-y-4">
 <div class="flex items-start justify-between gap-3">
 <div>
 <p class="text-xs font-medium uppercase tracking-wide text-muted">{{ t('tickets.details.title') }}</p>
 <h2 class="mt-1 text-lg font-semibold text-foreground">{{ ticket.title }}</h2>
 <p class="mt-1 text-xs text-muted">#{{ ticket.ticket_no }}</p>
 </div>
 <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(ticket.status)">
 {{ t(`tickets.statuses.${ticket.status}`) }}
 </span>
 </div>

 <div v-if="showUserMeta" class="flex items-center gap-3 rounded-xl bg-surface-2 px-3 py-3">
 <div class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-surface-2 text-sm font-semibold text-muted">
 <span>{{ displayName.slice(0, 1).toUpperCase() }}</span>
 </div>
 <div class="min-w-0">
 <p class="font-medium text-foreground">{{ displayName }}</p>
 <p class="truncate text-sm text-muted">{{ ticket.user_email }}</p>
 </div>
 </div>

 <div class="grid gap-3 text-sm md:grid-cols-1 xl:grid-cols-2">
 <InfoItem :label="t('tickets.details.category')" :value="t(`tickets.categories.${ticket.category}`)" />
 <InfoItem :label="t('tickets.details.createdAt')" :value="formatDateTime(ticket.created_at)" />
 <InfoItem :label="t('tickets.details.updatedAt')" :value="formatDateTime(ticket.updated_at)" />
 <InfoItem v-if="showUserMeta" :label="t('tickets.details.user')" :value="displayName" />
 </div>
 </div>

 <div class="mt-4 min-h-0 flex-1 overflow-y-auto border-t border-line pt-4">
 <div class="space-y-3">
 <p class="text-sm font-medium text-foreground">{{ t('tickets.details.formData') }}</p>
 <TicketCategoryForm :category="ticket.category" :model-value="ticket.current_form_payload || {}" readonly />
 </div>
 </div>

 <div v-if="$slots.actions" class="mt-4 space-y-3 border-t border-line pt-4">
 <slot name="actions" />
 </div>
 </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass } from '@/utils/tickets'
import type { SupportTicket } from '@/types/ticket'
import TicketCategoryForm from './TicketCategoryForm.vue'
import InfoItem from './TicketInfoItem.vue'

const props = defineProps<{
 ticket: SupportTicket
 showUserMeta?: boolean
}>()

const { t } = useI18n()
const displayName = computed(() => props.ticket.user_name || props.ticket.user_email || String(props.ticket.user_id || ''))
</script>
