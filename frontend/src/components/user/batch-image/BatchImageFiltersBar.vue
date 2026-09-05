<template>
  <div class="flex flex-col gap-3">
    <div class="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
      <div class="grid w-full grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-[260px_160px_144px_152px] lg:w-auto">
        <div class="min-w-0">
          <SearchInput
            v-model="filters.taskName"
            :placeholder="t('batchImage.filters.searchTaskName')"
            class="w-full"
            @search="$emit('apply')"
          />
        </div>
        <Select v-model="filters.apiKeyId" :options="apiKeyFilterOptions" class="w-full" @change="$emit('apply')" />
        <Select v-model="filters.status" :options="statusFilterOptions" class="w-full" @change="$emit('apply')" />
        <Select v-model="filters.downloaded" :options="downloadFilterOptions" class="w-full" @change="$emit('apply')" />
      </div>
      <div class="flex flex-wrap items-center justify-start gap-2 sm:justify-end lg:flex-shrink-0">
        <button type="button" class="btn-glass-secondary" :disabled="loadingJobs" @click="$emit('reset')">
          {{ t('common.reset') }}
        </button>
        <button type="button" class="btn-glass-secondary" :disabled="loadingKeys || loadingJobs" :title="t('common.refresh')" @click="$emit('refresh')">
          <Icon name="refresh" size="md" :class="loadingKeys || loadingJobs ? 'animate-spin' : ''" />
        </button>
        <button type="button" class="btn-glass-secondary" @click="$emit('open-guide')">
          <Icon name="book" size="md" class="mr-2" />
          {{ t('batchImage.actions.usageGuide') }}
        </button>
        <button type="button" class="btn-glass-primary" @click="$emit('open-create')">
          <Icon name="plus" size="md" class="mr-2" />
          {{ t('batchImage.actions.createJob') }}
        </button>
      </div>
    </div>

    <div
      v-if="selectedCount"
      class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-line bg-surface px-3 py-2 shadow-sm "
    >
      <i18n-t
        keypath="batchImage.list.selectedJobs"
        tag="span"
        scope="global"
        :plural="selectedCount"
        class="text-sm text-muted "
      >
        <template #count>
          <span class="font-medium text-foreground">{{ selectedCount }}</span>
        </template>
      </i18n-t>
      <div class="flex flex-wrap items-center gap-2">
        <button
          type="button"
          class="btn-glass-secondary btn-sm"
          :disabled="bulkDownloading || selectedDownloadableCount === 0"
          @click="$emit('download-selected')"
        >
          <Icon :name="bulkDownloading ? 'refresh' : 'download'" size="sm" class="mr-1.5" :class="bulkDownloading ? 'animate-spin' : ''" />
          {{ t('batchImage.actions.downloadSelected') }}
        </button>
        <button
          type="button"
          class="btn-glass-secondary btn-sm text-danger-text hover:text-danger-text "
          :disabled="bulkDeleting"
          @click="$emit('delete-selected')"
        >
          <Icon :name="bulkDeleting ? 'refresh' : 'trash'" size="sm" class="mr-1.5" :class="bulkDeleting ? 'animate-spin' : ''" />
          {{ t('batchImage.actions.deleteRecords') }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import SearchInput from '@/components/common/SearchInput.vue'
import Icon from '@/components/icons/Icon.vue'
import type { SelectOption } from '@/components/common/Select.vue'

const filters = defineModel<{ taskName: string; apiKeyId: string; status: string; downloaded: string }>('filters', { required: true })

defineProps<{
  apiKeyFilterOptions: SelectOption[]
  statusFilterOptions: SelectOption[]
  downloadFilterOptions: SelectOption[]
  loadingJobs: boolean
  loadingKeys: boolean
  selectedCount: number
  selectedDownloadableCount: number
  bulkDownloading: boolean
  bulkDeleting: boolean
}>()

defineEmits<{
  apply: []
  reset: []
  refresh: []
  'open-guide': []
  'open-create': []
  'download-selected': []
  'delete-selected': []
}>()

const { t } = useI18n()
</script>
