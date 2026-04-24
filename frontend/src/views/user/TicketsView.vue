<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-col gap-4 rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800 lg:flex-row lg:items-end lg:justify-between">
        <div class="grid gap-4 md:grid-cols-3">
          <div>
            <label class="input-label">{{ t('tickets.filters.search') }}</label>
            <input v-model="filters.search" class="input" @keyup.enter="applyFilters()" />
          </div>
        <div>
          <label class="input-label">{{ t('tickets.filters.category') }}</label>
          <Select v-model="filters.category" :options="categoryFilterOptions" :searchable="false" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.filters.status') }}</label>
          <Select v-model="filters.status" :options="statusFilterOptions" :searchable="false" />
        </div>
        </div>
        <div class="flex gap-3">
          <button class="btn btn-secondary" @click="applyFilters()">{{ t('common.search') }}</button>
          <button class="btn btn-primary" @click="openCreateDialog">{{ t('tickets.create') }}</button>
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
                    <button v-if="ticket.status === 'withdrawn'" class="btn btn-secondary btn-sm" @click="openEdit(ticket)">{{ t('tickets.actions.edit') }}</button>
                    <button class="btn btn-secondary btn-sm" :disabled="ticket.status === 'closed'" @click="closeTicketItem(ticket.id)">{{ t('tickets.actions.close') }}</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />

      <TicketCreateDialog
        :show="showCreateDialog"
        :submitting="creating"
        :user-concurrency="authStore.user?.concurrency ?? null"
        :available-groups="availableGroups"
        :user-group-rates="userGroupRates"
        @close="closeCreateDialog"
        @submit="createTicket"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import TicketCreateDialog from '@/components/tickets/TicketCreateDialog.vue'
import { useAppStore, useAuthStore } from '@/stores'
import ticketsAPI from '@/api/tickets'
import userGroupsAPI from '@/api/groups'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions, validateTicketPayload } from '@/utils/tickets'
import type { Group, SupportTicket, TicketCategory, TicketStatus } from '@/types'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()
const appStore = useAppStore()
const authStore = useAuthStore()
const loading = ref(false)
const creating = ref(false)
const showCreateDialog = ref(false)
const items = ref<SupportTicket[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})
const availableGroups = ref<Group[]>([])
const userGroupRates = ref<Record<number, number>>({})
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
const isCreateRoute = computed(() => route.name === 'TicketCreate')
const categoryFilterOptions = computed(() => [
  { value: '', label: t('tickets.filters.allCategories') },
  ...ticketCategoryOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
])
const statusFilterOptions = computed(() => [
  { value: '', label: t('tickets.filters.allStatuses') },
  ...ticketStatusOptions.map((option) => ({ value: option.value, label: t(option.labelKey) })),
])

async function loadTickets() {
  try {
    loading.value = true
    const data = await ticketsAPI.listTickets({
      ...filters,
      page: pagination.page,
      page_size: pagination.page_size,
    })
    items.value = data.items
    pagination.total = data.total
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    loading.value = false
  }
}

function applyFilters() {
  pagination.page = 1
  loadTickets()
}

async function loadTicketContext() {
  try {
    const [groups, rates] = await Promise.all([
      userGroupsAPI.getAvailable(),
      userGroupsAPI.getUserGroupRates(),
    ])
    availableGroups.value = groups
    userGroupRates.value = rates
  } catch (error) {
    console.error('Failed to load ticket context:', error)
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

function handlePageChange(page: number) {
  pagination.page = page
  loadTickets()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  loadTickets()
}

function openCreateDialog() {
  if (isCreateRoute.value) {
    showCreateDialog.value = true
    return
  }
  router.push({ name: 'TicketCreate' })
}

function closeCreateDialog() {
  showCreateDialog.value = false
  if (isCreateRoute.value) {
    router.replace({ name: 'Tickets' })
  }
}

function openEdit(ticket: SupportTicket) {
  if (ticket.status === 'withdrawn') {
    router.push({ path: `/tickets/${ticket.id}`, query: { edit: '1' } })
    return
  }
  router.push(`/tickets/${ticket.id}`)
}

async function createTicket(form: { category: TicketCategory; title: string; form_payload: Record<string, unknown> }) {
  const validationKey = validateTicketPayload(form.category, form.title, form.form_payload)
  if (validationKey) {
    appStore.showError(t(validationKey))
    return
  }
  try {
    creating.value = true
    const created = await ticketsAPI.createTicket(form)
    showCreateDialog.value = false
    appStore.showSuccess(t('tickets.messages.created'))
    router.push(`/tickets/${created.id}`)
  } catch (err: any) {
    appStore.showError(err?.message || t('common.unknownError'))
  } finally {
    creating.value = false
  }
}

onMounted(async () => {
  await Promise.all([loadTickets(), loadTicketContext()])
})

watch(isCreateRoute, (active) => {
  showCreateDialog.value = active
}, { immediate: true })
</script>
