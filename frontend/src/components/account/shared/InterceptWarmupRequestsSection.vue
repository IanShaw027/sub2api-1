<template>
  <!-- Intercept Warmup Requests (Anthropic/Antigravity) -->
  <div v-if="show" class="border-t border-line pt-4">
    <div class="flex items-center justify-between">
      <div>
        <label class="input-label mb-0">{{
          t('admin.accounts.interceptWarmupRequests')
        }}</label>
        <p class="mt-1 text-xs text-muted">
          {{ t('admin.accounts.interceptWarmupRequestsDesc') }}
        </p>
      </div>
      <InlineToggleSwitch v-model="interceptWarmupRequests" />
    </div>
  </div>
</template>

<script setup lang="ts">
// Shared between Create/Edit: identical markup, only the outer v-if platform
// condition differs between hosts (passed in as the `show` prop).
import { useI18n } from 'vue-i18n'
import InlineToggleSwitch from '@/components/account/shared/InlineToggleSwitch.vue'

const { t } = useI18n()

defineProps<{
  show: boolean
}>()

const interceptWarmupRequests = defineModel<boolean>('interceptWarmupRequests', { required: true })
</script>
