<template>
  <teleport to="body">
    <transition name="modal">
      <div
        v-if="open"
        class="fixed inset-0 z-50 flex items-center justify-center p-4"
        @mousedown.self="$emit('close')"
      >
        <div class="fixed inset-0 glass-modal-scrim" @click="$emit('close')"></div>
        <div class="relative max-h-[85vh] w-full max-w-2xl overflow-y-auto glass-card-solid rounded-hero p-6">
          <button type="button" class="modal-close absolute right-4 top-4" @click="$emit('close')">
            <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" /></svg>
          </button>

          <h2 class="mb-4 text-lg font-bold text-foreground">{{ t('admin.subscriptions.guide.title') }}</h2>
          <p class="mb-5 text-sm text-muted">{{ t('admin.subscriptions.guide.subtitle') }}</p>

          <!-- Step 1 -->
          <div class="mb-5">
            <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">1</span>
              {{ t('admin.subscriptions.guide.step1.title') }}
            </h3>
            <ol class="ml-8 list-decimal space-y-1 text-sm text-muted">
              <li>{{ t('admin.subscriptions.guide.step1.line1') }}</li>
              <li>{{ t('admin.subscriptions.guide.step1.line2') }}</li>
              <li>{{ t('admin.subscriptions.guide.step1.line3') }}</li>
            </ol>
            <div class="ml-8 mt-2">
              <router-link
                to="/admin/groups"
                @click="$emit('close')"
                class="inline-flex items-center gap-1 text-sm font-medium text-accent hover:text-accent  "
              >
                {{ t('admin.subscriptions.guide.step1.link') }}
                <Icon name="arrowRight" size="xs" />
              </router-link>
            </div>
          </div>

          <!-- Step 2 -->
          <div class="mb-5">
            <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">2</span>
              {{ t('admin.subscriptions.guide.step2.title') }}
            </h3>
            <ol class="ml-8 list-decimal space-y-1 text-sm text-muted">
              <li>{{ t('admin.subscriptions.guide.step2.line1') }}</li>
              <li>{{ t('admin.subscriptions.guide.step2.line2') }}</li>
              <li>{{ t('admin.subscriptions.guide.step2.line3') }}</li>
            </ol>
          </div>

          <!-- Step 3 -->
          <div class="mb-5">
            <h3 class="mb-2 flex items-center gap-2 text-sm font-semibold text-foreground">
              <span class="flex h-6 w-6 items-center justify-center rounded-full bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] text-xs font-bold text-accent  ">3</span>
              {{ t('admin.subscriptions.guide.step3.title') }}
            </h3>
            <div class="ml-8 overflow-hidden rounded-lg border border-line">
              <table class="w-full text-sm">
                <tbody>
                  <tr v-for="(row, i) in guideActionRows" :key="i" class="border-b border-line last:border-0">
                    <td class="whitespace-nowrap bg-surface-2 px-3 py-2 font-medium text-foreground">{{ row.action }}</td>
                    <td class="px-3 py-2 text-muted">{{ row.desc }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <!-- Tip -->
          <div class="rounded-lg bg-[color-mix(in_oklch,var(--accent)_12%,transparent)] p-3 text-xs text-accent  ">
            {{ t('admin.subscriptions.guide.tip') }}
          </div>

          <div class="mt-4 text-right">
            <button type="button" class="btn-glass-primary text-sm" @click="$emit('close')">{{ t('common.close') }}</button>
          </div>
        </div>
      </div>
    </transition>
  </teleport>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  open: boolean
}>()

defineEmits<{
  close: []
}>()

const { t } = useI18n()

const guideActionRows = computed(() => [
  { action: t('admin.subscriptions.guide.actions.adjust'), desc: t('admin.subscriptions.guide.actions.adjustDesc') },
  { action: t('admin.subscriptions.guide.actions.resetQuota'), desc: t('admin.subscriptions.guide.actions.resetQuotaDesc') },
  { action: t('admin.subscriptions.guide.actions.revoke'), desc: t('admin.subscriptions.guide.actions.revokeDesc') }
])
</script>
