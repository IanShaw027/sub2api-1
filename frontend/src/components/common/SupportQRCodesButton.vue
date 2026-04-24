<template>
  <div v-if="hasEntries">
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-gray-600 transition-colors hover:bg-gray-100 hover:text-gray-900 dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
      :aria-label="t('common.contactSupport')"
      @click="showDialog = true"
    >
      <Icon name="chat" size="sm" />
      <span class="hidden sm:inline">{{ t('common.contactSupport') }}</span>
    </button>

    <BaseDialog
      :show="showDialog"
      :title="t('common.contactSupport')"
      width="normal"
      :close-on-click-outside="true"
      @close="showDialog = false"
    >
      <div class="grid gap-4 sm:grid-cols-2">
        <div
          v-for="(entry, index) in normalizedEntries"
          :key="`${entry.image_url}-${index}`"
          class="rounded-2xl border border-gray-200 bg-gray-50/80 p-4 dark:border-dark-700 dark:bg-dark-900/60"
        >
          <div class="overflow-hidden rounded-2xl bg-white p-3 shadow-sm ring-1 ring-gray-100 dark:bg-dark-800 dark:ring-dark-700">
            <img
              :src="entry.image_url"
              :alt="entry.note || t('common.contactSupport')"
              class="aspect-square w-full object-contain"
            >
          </div>
          <p
            v-if="entry.note"
            class="mt-3 text-center text-sm text-gray-600 dark:text-gray-300"
          >
            {{ entry.note }}
          </p>
        </div>
      </div>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SupportQRCodeEntry } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'

const props = defineProps<{
  entries: SupportQRCodeEntry[]
}>()

const { t } = useI18n()
const showDialog = ref(false)

const normalizedEntries = computed(() =>
  (props.entries || [])
    .filter((entry) => typeof entry?.image_url === 'string' && entry.image_url.trim())
    .map((entry) => ({
      image_url: entry.image_url.trim(),
      note: entry.note?.trim() || '',
    }))
)

const hasEntries = computed(() => normalizedEntries.value.length > 0)
</script>
