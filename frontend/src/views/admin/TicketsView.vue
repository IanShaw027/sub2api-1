<template>
  <AppLayout>
    <div class="space-y-4">
      <div class="card flex flex-wrap items-center gap-3 p-4">
        <input v-model="keyword" type="text" class="input sm:max-w-64" :placeholder="t('tickets.search')" @input="debounceLoad" />
        <Select v-model="status" :options="statusFilters" class="w-40" @change="load" />
        <Select v-model="category" :options="categoryFilters" class="w-44" @change="load" />
        <label class="flex items-center gap-2 text-sm">
          <input v-model="unreadOnly" type="checkbox" @change="load" />
          {{ t('tickets.unreadOnly') }}
        </label>
        <button class="btn btn-secondary" @click="showTemplates = true">{{ t('tickets.templates') }}</button>
      </div>
      <DataTable :columns="columns" :data="tickets" :loading="loading">
        <template #cell-ticket_no="{ value, row }">
          <span class="font-mono text-sm">{{ value }}</span>
          <span v-if="row.unread_by_admin" class="ml-2 inline-block h-2 w-2 rounded-full bg-red-500" />
        </template>
        <template #cell-category="{ value }">{{ t('tickets.category.' + value, value) }}</template>
        <template #cell-status="{ value }">{{ t('tickets.status.' + value, value) }}</template>
        <template #cell-actions="{ row }">
          <button class="text-xs text-blue-600 hover:underline" @click="router.push(`/admin/tickets/${row.id}`)">{{ t('common.view') }}</button>
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
      <BaseDialog :show="showTemplates" :title="t('tickets.templates')" @close="showTemplates = false">
        <div class="space-y-3">
          <div v-for="item in templates" :key="item.id" class="flex items-start justify-between gap-2 rounded border p-2 text-sm">
            <div>
              <p class="font-medium">{{ item.title }}</p>
              <p class="text-gray-500">{{ item.content }}</p>
            </div>
            <button class="text-xs text-red-600" @click="removeTemplate(item.id)">{{ t('common.delete') }}</button>
          </div>
          <input v-model="templateTitle" class="input w-full" :placeholder="t('tickets.templateTitle')" />
          <textarea v-model="templateContent" rows="3" class="input w-full" :placeholder="t('tickets.templateContent')" />
        </div>
        <template #footer>
          <button class="btn btn-primary" @click="addTemplate">{{ t('common.add') }}</button>
        </template>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { adminTicketsAPI } from '@/api/admin/tickets'
import { extractI18nErrorMessage } from '@/utils/apiError'
import { useAppStore } from '@/stores'
import type { SupportTicket, TicketReplyTemplate } from '@/types/ticket'
import type { Column } from '@/components/common/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Select from '@/components/common/Select.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'

const { t } = useI18n()
const router = useRouter()
const appStore = useAppStore()
const loading = ref(false)
const tickets = ref<SupportTicket[]>([])
const templates = ref<TicketReplyTemplate[]>([])
const keyword = ref('')
const status = ref('')
const category = ref('')
const unreadOnly = ref(false)
const showTemplates = ref(false)
const templateTitle = ref('')
const templateContent = ref('')
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
let timer: number | undefined

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
  { key: 'user_email', label: t('tickets.user') },
  { key: 'title', label: t('tickets.title') },
  { key: 'category', label: t('tickets.categoryLabel') },
  { key: 'status', label: t('tickets.statusLabel') },
  { key: 'actions', label: t('common.actions') },
])

async function load() {
  loading.value = true
  try {
    const res = await adminTicketsAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      status: status.value || undefined,
      category: category.value || undefined,
      keyword: keyword.value || undefined,
      unread_only: unreadOnly.value || undefined,
    })
    tickets.value = res.data.items || []
    pagination.total = res.data.total || 0
  } catch (err: unknown) {
    appStore.showError(extractI18nErrorMessage(err, t, 'tickets.errors', t('common.error')))
  } finally {
    loading.value = false
  }
}

function debounceLoad() {
  window.clearTimeout(timer)
  timer = window.setTimeout(load, 300)
}

async function loadTemplates() {
  const res = await adminTicketsAPI.listTemplates()
  templates.value = res.data || []
}

async function addTemplate() {
  if (!templateTitle.value.trim() || !templateContent.value.trim()) return
  await adminTicketsAPI.createTemplate({ title: templateTitle.value.trim(), content: templateContent.value.trim() })
  templateTitle.value = ''
  templateContent.value = ''
  await loadTemplates()
}

async function removeTemplate(id: number) {
  await adminTicketsAPI.deleteTemplate(id)
  await loadTemplates()
}

onMounted(async () => {
  await load()
  await loadTemplates()
})
</script>
