<template>
  <!-- Add Method (only for Anthropic OAuth-based type) -->
  <div v-if="show">
    <label class="input-label">{{ t('admin.accounts.addMethod') }}</label>
    <div class="mt-2 flex gap-4">
      <label class="flex cursor-pointer items-center">
        <input
          v-model="addMethod"
          type="radio"
          value="oauth"
          class="mr-2 text-accent focus:ring-accent"
        />
        <span class="text-sm text-foreground">{{ t('admin.accounts.types.oauth') }}</span>
      </label>
      <label class="flex cursor-pointer items-center">
        <input
          v-model="addMethod"
          type="radio"
          value="setup-token"
          class="mr-2 text-accent focus:ring-accent"
        />
        <span class="text-sm text-foreground">{{
          t('admin.accounts.setupTokenLongLived')
        }}</span>
      </label>
    </div>
  </div>
</template>

<script setup lang="ts">
// Anthropic OAuth-based "add method" radio group, lifted verbatim out of the
// host.
import { useI18n } from 'vue-i18n'
import type { AddMethod } from '@/composables/useAccountOAuth'

const { t } = useI18n()

defineProps<{
  show: boolean
}>()

const addMethod = defineModel<AddMethod>('addMethod', { required: true })
</script>
