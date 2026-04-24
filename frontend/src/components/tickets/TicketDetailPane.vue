<template>
  <div class="space-y-4 rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800">
    <div class="space-y-3">
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
      <div class="grid gap-3 text-sm md:grid-cols-2">
        <div>
          <p class="text-gray-500 dark:text-gray-400">{{ t('tickets.details.category') }}</p>
          <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ t(`tickets.categories.${ticket.category}`) }}</p>
        </div>
        <div>
          <p class="text-gray-500 dark:text-gray-400">{{ t('tickets.details.createdAt') }}</p>
          <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ formatRelativeWithDateTime(ticket.created_at) }}</p>
        </div>
        <div>
          <p class="text-gray-500 dark:text-gray-400">{{ t('tickets.details.updatedAt') }}</p>
          <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ formatRelativeWithDateTime(ticket.updated_at) }}</p>
        </div>
        <div v-if="showUserMeta">
          <p class="text-gray-500 dark:text-gray-400">{{ t('tickets.details.user') }}</p>
          <p class="mt-1 font-medium text-gray-900 dark:text-white">{{ ticket.user_name }}</p>
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ ticket.user_email }}</p>
        </div>
      </div>
    </div>

    <div class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
      <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('tickets.details.formData') }}</p>
      <TicketCategoryForm :category="ticket.category" :model-value="ticket.current_form_payload || {}" readonly />
    </div>

    <div v-if="$slots.actions" class="space-y-3 border-t border-gray-100 pt-4 dark:border-dark-700">
      <slot name="actions" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass } from '@/utils/tickets'
import type { SupportTicket } from '@/types'
import TicketCategoryForm from './TicketCategoryForm.vue'

defineProps<{
  ticket: SupportTicket
  showUserMeta?: boolean
}>()

const { t } = useI18n()
</script>
