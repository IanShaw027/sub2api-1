<template>
  <BaseDialog :show="show" :title="t('admin.accounts.cyber.title', { name: account.name })" width="wide" @close="close">
    <div class="space-y-4 p-1">
      <div class="flex items-center justify-between gap-4 border-b border-gray-100 pb-3 dark:border-dark-700">
        <div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.cyber.total', { count: total }) }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.cyber.deduplicatedHint') }}
          </div>
        </div>
        <button class="btn btn-secondary inline-flex h-9 w-9 items-center justify-center p-0" :title="t('common.refresh')" @click="load">
          <Icon name="refresh" size="sm" />
        </button>
      </div>

      <div v-if="loading" class="flex justify-center py-12">
        <div class="h-7 w-7 animate-spin rounded-full border-2 border-gray-200 border-t-primary-600"></div>
      </div>
      <div v-else-if="error" class="py-10 text-center text-sm text-red-600 dark:text-red-400">{{ error }}</div>
      <div v-else-if="events.length === 0" class="py-10 text-center text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.cyber.empty') }}
      </div>
      <div v-else class="overflow-hidden rounded-md border border-gray-200 dark:border-dark-700">
        <div
          v-for="event in events"
          :key="event.error_id ?? `${event.created_at}:${event.request_id}`"
          class="grid grid-cols-[minmax(0,1fr)_auto] gap-3 border-b border-gray-100 p-3 last:border-b-0 dark:border-dark-700"
        >
          <div class="min-w-0 space-y-1">
            <div class="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-gray-500 dark:text-gray-400">
              <span>{{ formatDateTime(event.created_at) }}</span>
              <span v-if="event.model" class="font-mono text-gray-700 dark:text-gray-300">{{ event.model }}</span>
              <span v-if="event.status_code != null" class="font-mono">HTTP {{ event.status_code }}</span>
            </div>
            <div class="break-words text-sm text-gray-800 dark:text-gray-200">{{ event.message || '-' }}</div>
            <div v-if="event.request_id" class="break-all font-mono text-[11px] text-gray-400">{{ event.request_id }}</div>
          </div>
          <button
            v-if="event.error_id != null"
            class="inline-flex h-8 w-8 items-center justify-center rounded-md text-gray-500 hover:bg-gray-100 hover:text-primary-600 dark:hover:bg-dark-700 dark:hover:text-primary-400"
            :title="t('admin.accounts.cyber.viewDetail')"
            @click="emit('open-detail', event.error_id)"
          >
            <Icon name="eye" size="sm" />
          </button>
        </div>
      </div>

      <Pagination
        v-if="total > pageSize"
        :page="page"
        :total="total"
        :page-size="pageSize"
        @update:page="changePage"
        @update:pageSize="changePageSize"
      />
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type { Account, AccountCyberEvent } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'

const props = defineProps<{ show: boolean; account: Account }>()
const emit = defineEmits<{
  (event: 'close'): void
  (event: 'open-detail', errorId: number): void
}>()
const { t } = useI18n()
const events = ref<AccountCyberEvent[]>([])
const total = ref(0)
const page = ref(1)
const pageSize = ref(20)
const loading = ref(false)
const error = ref('')
let requestSequence = 0

const close = () => emit('close')

const load = async () => {
  const sequence = ++requestSequence
  loading.value = true
  error.value = ''
  try {
    const result = await adminAPI.accounts.getCyberEvents(props.account.id, page.value, pageSize.value)
    if (sequence !== requestSequence) return
    events.value = result.events ?? []
    total.value = result.total ?? 0
  } catch (cause) {
    if (sequence !== requestSequence) return
    events.value = []
    error.value = extractApiErrorMessage(cause)
  } finally {
    if (sequence === requestSequence) loading.value = false
  }
}

const changePage = (value: number) => {
  page.value = value
  void load()
}

const changePageSize = (value: number) => {
  pageSize.value = value
  page.value = 1
  void load()
}

watch(() => [props.show, props.account.id] as const, ([show]) => {
  if (!show) return
  page.value = 1
  void load()
}, { immediate: true })
</script>
