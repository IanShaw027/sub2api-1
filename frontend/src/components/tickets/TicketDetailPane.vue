<template>
  <div class="flex h-full min-h-0 flex-col overflow-hidden rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
    <div class="shrink-0 space-y-4">
      <div class="flex items-start justify-between gap-3">
        <div>
          <p class="text-xs font-medium uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ t('tickets.details.title') }}</p>
          <h2 class="mt-1 text-lg font-semibold text-gray-900 dark:text-white">{{ ticket.title }}</h2>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">#{{ ticket.ticket_no }}</p>
        </div>
        <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(ticket.status)">
          {{ t(`tickets.statuses.${ticket.status}`) }}
        </span>
      </div>

      <div v-if="showUserMeta" class="flex items-center gap-3 rounded-xl bg-gray-50 px-3 py-3 dark:bg-dark-700/40">
        <div class="flex h-11 w-11 shrink-0 items-center justify-center overflow-hidden rounded-full bg-gray-200 text-sm font-semibold text-gray-600 dark:bg-dark-600 dark:text-gray-200">
          <img
            v-if="safeImageUrl(ticket.user_avatar_url)"
            :src="safeImageUrl(ticket.user_avatar_url)"
            :alt="ticket.user_name"
            class="h-full w-full object-cover"
            referrerpolicy="no-referrer"
            loading="lazy"
          />
          <span v-else>{{ ticket.user_name.slice(0, 1).toUpperCase() }}</span>
        </div>
        <div class="min-w-0">
          <p class="font-medium text-gray-900 dark:text-white">{{ ticket.user_name }}</p>
          <p class="truncate text-sm text-gray-500 dark:text-gray-400">{{ ticket.user_email }}</p>
        </div>
      </div>

      <div class="grid gap-3 text-sm md:grid-cols-1 xl:grid-cols-2">
        <InfoItem :label="t('tickets.details.category')" :value="t(`tickets.categories.${ticket.category}`)" />
        <InfoItem :label="t('tickets.details.createdAt')" :value="formatDateTime(ticket.created_at)" />
        <InfoItem :label="t('tickets.details.updatedAt')" :value="formatDateTime(ticket.updated_at)" />
        <InfoItem v-if="showUserMeta" :label="t('tickets.details.user')" :value="ticket.user_name" />
      </div>
    </div>

    <div class="mt-4 min-h-0 flex-1 overflow-y-auto border-t border-gray-100 pt-4 dark:border-dark-700">
      <div class="space-y-3">
        <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('tickets.details.formData') }}</p>
        <TicketCategoryForm :category="ticket.category" :model-value="ticket.current_form_payload || {}" readonly />
      </div>
    </div>

    <div v-if="$slots.actions" class="mt-4 space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatDateTime } from '@/utils/format'
import { safeImageUrl } from '@/utils/safeImageUrl'
import { getTicketStatusBadgeClass } from '@/utils/tickets'
import type { SupportTicket } from '@/types'
import TicketCategoryForm from './TicketCategoryForm.vue'
import InfoItem from './TicketInfoItem.vue'

defineProps<{
  ticket: SupportTicket
  showUserMeta?: boolean
}>()

const { t } = useI18n()
</script>
