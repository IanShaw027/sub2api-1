<template>
  <div class="card border border-gray-200 p-5 dark:border-dark-700">
    <template v-if="item">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h3 class="text-base font-semibold text-gray-900 dark:text-white">{{ item.skill_name }}</h3>
            <span class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300">
              {{ item.version_name }}
            </span>
          </div>
          <p class="mt-2 text-sm text-gray-500 dark:text-gray-400">
            {{ item.skill_slug }}
            <span v-if="item.author_name"> · {{ t('skills.admin.review.authorLabel') }} {{ item.author_name }}</span>
            <span v-if="item.category"> · {{ item.category }}</span>
          </p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <SkillAdminStatusBadge :status="item.review_status" :label="reviewStatusLabel(item.review_status)" mode="review" />
          <SkillAdminStatusBadge :status="item.visibility" :label="visibilityLabel(item.visibility)" mode="visibility" />
          <span :class="riskClass(item.risk_level)" class="inline-flex items-center rounded-full px-2.5 py-1 text-xs font-semibold">
            {{ riskLabel(item.risk_level) }}
          </span>
        </div>
      </div>

      <div class="mt-5 grid gap-4 md:grid-cols-3">
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.review.metrics.requests24h') }}</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ item.requests_24h.toLocaleString() }}</p>
        </div>
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.review.metrics.revenue30d') }}</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(item.revenue_30d) }}</p>
        </div>
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">{{ t('skills.admin.review.metrics.publishedVersion') }}</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ item.latest_published_version || '-' }}</p>
        </div>
      </div>

      <div class="mt-5 space-y-4">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.review.summaryTitle') }}</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.summary || t('skills.admin.review.summaryEmpty') }}</p>
        </div>

        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.review.changelogTitle') }}</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.changelog || t('skills.admin.review.changelogEmpty') }}</p>
        </div>

        <div v-if="item.tags.length">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.review.tagsTitle') }}</p>
          <div class="mt-2 flex flex-wrap gap-2">
            <span
              v-for="tag in item.tags"
              :key="tag"
              class="rounded-full bg-gray-100 px-2.5 py-1 text-xs text-gray-600 dark:bg-dark-700 dark:text-gray-300"
            >
              {{ tag }}
            </span>
          </div>
        </div>

        <div v-if="item.review_note || item.rejection_reason" class="rounded-2xl border border-gray-200 p-4 dark:border-dark-700">
          <p class="text-sm font-medium text-gray-900 dark:text-white">{{ t('skills.admin.review.latestNoteTitle') }}</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.review_note || item.rejection_reason }}</p>
        </div>
      </div>

      <div class="mt-6 flex flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary btn-sm" @click="$emit('reject')">{{ t('skills.admin.review.rejectVersion') }}</button>
        <button type="button" class="btn btn-primary btn-sm" @click="$emit('approve')">{{ t('skills.admin.review.approveVersion') }}</button>
      </div>
    </template>

    <template v-else>
      <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
        {{ t('skills.admin.review.detailEmpty') }}
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { SkillReviewItem, SkillRiskLevel, SkillVisibility, SkillReviewStatus } from '@/api/admin/skills'
import SkillAdminStatusBadge from './SkillAdminStatusBadge.vue'

const { t, locale } = useI18n()

defineProps<{
  item: SkillReviewItem | null
}>()

defineEmits<{
  (e: 'approve'): void
  (e: 'reject'): void
}>()

function formatCurrency(value: number): string {
  return new Intl.NumberFormat(locale.value.startsWith('zh') ? 'zh-CN' : 'en-US', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2
  }).format(value)
}

function reviewStatusLabel(status: SkillReviewStatus): string {
  return {
    pending: t('skills.admin.review.labels.pending'),
    approved: t('skills.admin.review.labels.approved'),
    rejected: t('skills.admin.review.labels.rejected')
  }[status]
}

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: t('skills.admin.review.labels.public'),
    private: t('skills.admin.review.labels.private'),
    force_private: t('skills.admin.review.labels.forcePrivate')
  }[status]
}

function riskLabel(level: SkillRiskLevel): string {
  return {
    low: t('skills.admin.review.labels.low'),
    medium: t('skills.admin.review.labels.medium'),
    high: t('skills.admin.review.labels.high')
  }[level]
}

function riskClass(level: SkillRiskLevel): string {
  return {
    low: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-200',
    medium: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-200',
    high: 'bg-rose-50 text-rose-700 dark:bg-rose-900/30 dark:text-rose-200'
  }[level]
}
</script>
