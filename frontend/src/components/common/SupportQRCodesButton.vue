<template>
  <div v-if="hasEntries || hasLegacyContactInfo">
    <button
      type="button"
      class="flex items-center gap-1.5 rounded-lg px-2.5 py-1.5 text-sm font-medium text-ink-soft transition-colors hover:bg-page hover:text-ink dark:text-dark-400 dark:hover:bg-dark-800 dark:hover:text-white"
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
      <div v-if="hasEntries" :class="['grid gap-4', normalizedEntries.length > 1 ? 'sm:grid-cols-2' : '']">
        <div
          v-for="(entry, index) in normalizedEntries"
          :key="`${entry.image_url}-${index}`"
          class="rounded-card border border-line bg-page/80 p-4 dark:border-dark-700 dark:bg-dark-900/60"
        >
          <div class="overflow-hidden rounded-card bg-card p-3 shadow-xs ring-1 ring-line dark:bg-dark-800 dark:ring-dark-700">
            <img
              :src="entry.image_url"
              :alt="entry.note || t('common.contactSupport')"
              class="aspect-square w-full object-contain"
            >
          </div>
          <p
            v-if="entry.note"
            class="mt-3 text-center text-sm text-ink-soft"
          >
            {{ entry.note }}
          </p>
        </div>
      </div>
      <p
        v-else
        class="rounded-card border border-line bg-page/80 p-4 text-sm leading-6 text-ink-body whitespace-pre-line break-words dark:border-dark-700 dark:bg-dark-900/60 dark:text-dark-200"
      >
        {{ normalizedLegacyContactInfo }}
      </p>
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
  legacyContactInfo?: string
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
const normalizedLegacyContactInfo = computed(() => props.legacyContactInfo?.trim() || '')
const hasLegacyContactInfo = computed(() => !hasEntries.value && normalizedLegacyContactInfo.value.length > 0)
</script>
