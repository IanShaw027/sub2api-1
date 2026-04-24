<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800 lg:flex-row lg:items-end lg:justify-between">
        <div class="grid gap-4 md:grid-cols-3">
          <div>
            <label class="input-label">{{ t('tickets.filters.search') }}</label>
            <input v-model="filters.search" class="input" @keyup.enter="loadTickets()" />
          </div>
          <div>
            <label class="input-label">{{ t('tickets.filters.category') }}</label>
            <select v-model="filters.category" class="input">
              <option value="">{{ t('tickets.filters.allCategories') }}</option>
              <option v-for="option in ticketCategoryOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </div>
          <div>
            <label class="input-label">{{ t('tickets.filters.status') }}</label>
            <select v-model="filters.status" class="input">
              <option value="">{{ t('tickets.filters.allStatuses') }}</option>
              <option v-for="option in ticketStatusOptions" :key="option.value" :value="option.value">
                {{ t(option.labelKey) }}
              </option>
            </select>
          </div>
        </div>
        <div class="flex gap-3">
          <button class="btn btn-secondary" @click="loadTickets()">{{ t('common.search') }}</button>
          <button class="btn btn-primary" @click="router.push('/tickets/create')">{{ t('tickets.create') }}</button>
        </div>
      </div>

      <div class="overflow-hidden rounded-2xl border bg-white dark:border-dark-700 dark:bg-dark-800">
        <div class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-100 dark:divide-dark-700">
            <thead class="bg-gray-50 dark:bg-dark-700/50">
              <tr>
                <th v-for="header in headers" :key="header" class="px-4 py-3 text-left text-xs font-semibold uppercase tracking-wide text-gray-500 dark:text-gray-400">{{ header }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-if="loading">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="items.length === 0">
                <td colspan="6" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('tickets.empty') }}</td>
              </tr>
              <tr v-for="ticket in items" :key="ticket.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/30">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ t(`tickets.categories.${ticket.category}`) }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ ticket.title }}</td>
                <td class="px-4 py-4">
                  <span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(ticket.status)">
                    {{ t(`tickets.statuses.${ticket.status}`) }}
                  </span>
                </td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-gray-400">{{ formatRelativeWithDateTime(ticket.created_at) }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-gray-400">{{ formatRelativeWithDateTime(ticket.updated_at) }}</td>
                <td class="px-4 py-4">
                  <div class="flex gap-2">
                    <button class="btn btn-secondary btn-sm" @click="router.push(`/tickets/${ticket.id}`)">{{ t('tickets.actions.view') }}</button>
                    <button class="btn btn-secondary btn-sm" @click="openEdit(ticket)">{{ t('tickets.actions.edit') }}</button>
                    <button class="btn btn-secondary btn-sm" :disabled="ticket.status === 'closed'" @click="closeTicketItem(ticket.id)">{{ t('tickets.actions.close') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useAppStore } from '@/stores'
import ticketsAPI from '@/api/tickets'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions } from '@/utils/tickets'
import type { SupportTicket, TicketCategory, TicketStatus } from '@/types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const items = ref<SupportTicket[]>([])
const filters = reactive<{ search: string; category: TicketCategory | ''; status: TicketStatus | '' }>({
  search: '',
  category: '',
  status: '',
})

const headers = computed(() => [
  t('tickets.table.category'),
  t('tickets.table.title'),
  t('tickets.table.status'),
  t('tickets.table.createdAt'),
  t('tickets.table.updatedAt'),
  t('tickets.table.actions'),
])

async function loadTickets() {
  try {
    loading.value = true
    const data = await ticketsAPI.listTickets(filters)
    items.value = data.items
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

async function closeTicketItem(id: number) {
  try {
    await ticketsAPI.closeTicket(id)
    appStore.showSuccess(t('tickets.messages.closed'))
    await loadTickets()
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  }
}

function openEdit(ticket: SupportTicket) {
  if (ticket.status === 'withdrawn') {
    router.push({ path: `/tickets/${ticket.id}`, query: { edit: '1' } })
    return
  }
  router.push(`/tickets/${ticket.id}`)
}

onMounted(loadTickets)
</script>
