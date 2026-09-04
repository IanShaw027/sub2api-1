<template>
  <div v-if="show" class="border-t border-line pt-4">
    <div class="flex items-center justify-between">
      <div class="flex-1 pr-4">
        <label
          id="bulk-edit-header-override-label"
          class="input-label mb-0"
          for="bulk-edit-header-override-enabled"
        >
          {{ t('admin.accounts.headerOverride.title') }}
        </label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.headerOverride.hint') }}
        </p>
      </div>
      <input
        v-model="enableHeaderOverride"
        id="bulk-edit-header-override-enabled"
        type="checkbox"
        aria-controls="bulk-edit-header-override-body"
        class="rounded border-line text-accent focus:ring-accent"
      />
    </div>
    <div v-if="enableHeaderOverride" id="bulk-edit-header-override-body" class="mt-3 space-y-3">
      <button
        type="button"
        :class="[
          'relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-accent focus:ring-offset-2',
          headerOverrideEnabled ? 'bg-accent' : 'bg-surface-3'
        ]"
        @click="headerOverrideEnabled = !headerOverrideEnabled"
      >
        <span
          :class="[
            'pointer-events-none inline-block h-5 w-5 transform rounded-full bg-[var(--thumb)] shadow ring-0 transition duration-200 ease-in-out',
            headerOverrideEnabled ? 'translate-x-5' : 'translate-x-0'
          ]"
        />
      </button>

      <div v-if="headerOverrideEnabled" class="space-y-3">
        <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3">
          <p class="text-xs text-accent">
            <Icon name="exclamationCircle" size="sm" class="mr-1 inline" :stroke-width="2" />
            {{ t('admin.accounts.headerOverride.info') }}
          </p>
        </div>

        <p class="text-xs text-warning-text">
          {{ t('admin.accounts.headerOverride.bulkReplaceHint') }}
        </p>

        <HeaderOverrideEditor
          :rows="headerOverrideRows"
          @update:rows="headerOverrideRows = $event"
        />
      </div>
      <p v-else class="text-xs text-muted">
        {{ t('admin.accounts.headerOverride.bulkDisableHint') }}
      </p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import HeaderOverrideEditor from '@/components/account/HeaderOverrideEditor.vue'
import type { HeaderOverrideRow } from '@/components/account/credentialsBuilder'

interface Props {
  show: boolean
}

defineProps<Props>()

const enableHeaderOverride = defineModel<boolean>('enableHeaderOverride', { required: true })
const headerOverrideEnabled = defineModel<boolean>('headerOverrideEnabled', { required: true })
const headerOverrideRows = defineModel<HeaderOverrideRow[]>('headerOverrideRows', { required: true })

const { t } = useI18n()
</script>
