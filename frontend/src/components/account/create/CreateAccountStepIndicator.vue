<template>
  <!-- Step Indicator for OAuth accounts -->
  <div v-if="isOAuthFlow" class="mb-6 flex items-center justify-center">
    <div class="flex items-center space-x-4">
      <div class="flex items-center">
        <div
          :class="[
 'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
 step >= 1 ? 'bg-accent text-white' : 'bg-surface-3 text-muted'
 ]"
        >
          1
        </div>
        <span class="ml-2 text-sm font-medium text-foreground">{{
          t('admin.accounts.oauth.authMethod')
        }}</span>
      </div>
      <div class="h-0.5 w-8 bg-surface-3" />
      <div class="flex items-center">
        <div
          :class="[
 'flex h-8 w-8 items-center justify-center rounded-full text-sm font-semibold',
 step >= 2 ? 'bg-accent text-white' : 'bg-surface-3 text-muted'
 ]"
        >
          2
        </div>
        <span class="ml-2 text-sm font-medium text-foreground">{{
          oauthStepTitle
        }}</span>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
// OAuth two-step progress indicator, lifted verbatim out of the host.
import { useI18n } from 'vue-i18n'

const { t } = useI18n()

defineProps<{
  isOAuthFlow: boolean
  step: number
  oauthStepTitle: string
}>()
</script>
