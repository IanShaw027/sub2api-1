<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card flex flex-wrap items-center justify-between gap-3 p-4">
        <div class="flex flex-wrap items-center gap-3">
          <Select v-model="status" :options="statusFilters" class="w-40" @change="load" />
          <Select v-model="category" :options="categoryFilters" class="w-44" @change="load" />
        </div>
        <button class="btn btn-primary" @click="router.push('/tickets/new')">{{ t('tickets.create') }}</button>
      </div>
      <DataTable :columns="columns" :data="tickets" :loading="loading">
        <template #cell-ticket_no="{ value, row }">
          <span class="font-mono text-sm">{{ value }}</span>
          <span v-if="row.unread_by_user" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500" />
        </template>
        <template #cell-category="{ value }">{{ t('tickets.category.' + value, value) }}</template>
        <template #cell-status="{ value }">{{ t('tickets.status.' + value, value) }}</template>
        <template #cell-actions="{ row }">
          <button class="text-xs text-blue-600 hover:underline" @click="router.push(`/tickets/${row.id}`)">{{ t('common.view') }}</button>
        </template>
      </DataTable>
      <Pagination
        v-if="pagination.total > 0"
        :page="pagination.page"
        :total="pagination.total"
        :page-size="pagination.page_size"
        @update:page="(p: number) => { pagination.page = p; load() }"
        @update:pageSize="(s: number) => { pagination.page_size = s; pagination.page = 1; load() }"
      />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ticketsAPI } from '@/api/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { SupportTicket } from '@/types/ticket'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const tickets = ref<SupportTicket[]>([])
const status = ref('')
const category = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })

const statusFilters = computed(() => [
  { value: '', label: t('common.all') },
  ...['submitted', 'processing', 'waiting_user', 'waiting_admin', 'resolved', 'closed', 'withdrawn'].map((value) => ({
    value, label: t('tickets.status.' + value),
  })),
])
const categoryFilters = computed(() => [
  { value: '', label: t('common.all') },
  ...['consult', 'refund', 'concurrency_apply', 'rate_apply', 'other'].map((value) => ({
    value, label: t('tickets.category.' + value),
  })),
])
const columns = computed((): Column[] => [
  { key: 'ticket_no', label: t('tickets.ticketNo') },
  { key: 'title', label: t('tickets.title') },
  { key: 'category', label: t('tickets.categoryLabel') },
  { key: 'status', label: t('tickets.statusLabel') },
  { key: 'actions', label: t('common.actions') },
])

async function load() {
  loading.value = true
  try {
    const res = await ticketsAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      status: status.value || undefined,
      category: category.value || undefined,
    })
    tickets.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

onMounted(load)
</script>
