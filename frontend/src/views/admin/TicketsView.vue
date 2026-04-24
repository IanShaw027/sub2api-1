<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="grid gap-4 rounded-2xl border bg-white p-5 dark:border-dark-700 dark:bg-dark-800 md:grid-cols-5">
        <div>
          <label class="input-label">{{ t('tickets.filters.search') }}</label>
          <input v-model="filters.search" class="input" @keyup.enter="applyFilters()" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.filters.user') }}</label>
          <input v-model="filters.user" class="input" :placeholder="t('tickets.filters.userPlaceholder')" @keyup.enter="applyFilters()" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.filters.category') }}</label>
          <Select v-model="filters.category" :options="categoryFilterOptions" :searchable="false" />
        </div>
        <div>
          <label class="input-label">{{ t('tickets.filters.status') }}</label>
          <Select v-model="filters.status" :options="statusFilterOptions" :searchable="false" />
        </div>
        <div>
          <label class="input-label">{{ t('dates.startDate') }}</label>
          <input v-model="filters.start_date" type="date" class="input" />
        </div>
        <div>
          <label class="input-label">{{ t('dates.endDate') }}</label>
          <input v-model="filters.end_date" type="date" class="input" />
        </div>
        <div class="md:col-span-5 flex justify-end gap-3">
          <button class="btn btn-secondary" @click="resetFilters">{{ t('common.reset') }}</button>
          <button class="btn btn-primary" @click="applyFilters">{{ t('common.search') }}</button>
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
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</td>
              </tr>
              <tr v-else-if="items.length === 0">
                <td colspan="7" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('tickets.empty') }}</td>
              </tr>
              <tr v-for="ticket in items" :key="ticket.id" class="hover:bg-gray-50 dark:hover:bg-dark-700/30">
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ t(`tickets.categories.${ticket.category}`) }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ ticket.title }}</td>
                <td class="px-4 py-4 text-sm text-gray-900 dark:text-white">{{ ticket.user_name }}</td>
                <td class="px-4 py-4"><span class="rounded-full px-2.5 py-1 text-xs font-medium" :class="getTicketStatusBadgeClass(ticket.status)">{{ t(`tickets.statuses.${ticket.status}`) }}</span></td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-gray-400">{{ formatRelativeWithDateTime(ticket.created_at) }}</td>
                <td class="px-4 py-4 text-sm text-gray-500 dark:text-gray-400">{{ formatRelativeWithDateTime(ticket.updated_at) }}</td>
                <td class="px-4 py-4"><button class="btn btn-secondary btn-sm" @click="router.push(`/admin/tickets/${ticket.id}`)">{{ t('tickets.actions.view') }}</button></td>
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
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import { useAppStore } from '@/stores'
import adminTicketsAPI from '@/api/adminTickets'
import { formatRelativeWithDateTime } from '@/utils/format'
import { getTicketStatusBadgeClass, ticketCategoryOptions, ticketStatusOptions } from '@/utils/tickets'
import type { SupportTicket, TicketCategory, TicketStatus } from '@/types'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const items = ref<SupportTicket[]>([])
const pagination = reactive({
  page: 1,
  page_size: 20,
  total: 0,
})
const filters = reactive<{ search: string; user: string; category: TicketCategory | ''; status: TicketStatus | ''; start_date: string; end_date: string }>({
  search: '',
  user: '',
  category: '',
  status: '',
  start_date: '',
  end_date: '',
})

const headers = computed(() => [
  t('tickets.table.category'),
  t('tickets.table.title'),
  t('tickets.table.user'),
  t('tickets.table.status'),
  t('tickets.table.createdAt'),
  t('tickets.table.updatedAt'),
  t('tickets.table.actions'),
])
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
    const data = await adminTicketsAPI.listAdminTickets({
      ...filters,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
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

function resetFilters() {
  filters.search = ''
  filters.user = ''
  filters.category = ''
  filters.status = ''
  filters.start_date = ''
  filters.end_date = ''
  pagination.page = 1
  loadTickets()
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

onMounted(loadTickets)
</script>
