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
            <span v-if="item.author_name"> · 提交者 {{ item.author_name }}</span>
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
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">24h 调用</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ item.requests_24h.toLocaleString() }}</p>
        </div>
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">30d 收入</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ formatCurrency(item.revenue_30d) }}</p>
        </div>
        <div class="rounded-2xl bg-gray-50 p-4 dark:bg-dark-900/60">
          <p class="text-xs uppercase tracking-[0.14em] text-gray-500 dark:text-gray-400">公开版本</p>
          <p class="mt-2 text-xl font-semibold text-gray-900 dark:text-white">{{ item.latest_published_version || '-' }}</p>
        </div>
      </div>

      <div class="mt-5 space-y-4">
        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">版本摘要</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.summary || '提交方未填写版本摘要。' }}</p>
        </div>

        <div>
          <p class="text-sm font-medium text-gray-900 dark:text-white">变更说明</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.changelog || '当前版本未附带变更说明。' }}</p>
        </div>

        <div v-if="item.tags.length">
          <p class="text-sm font-medium text-gray-900 dark:text-white">标签</p>
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
          <p class="text-sm font-medium text-gray-900 dark:text-white">最近处理意见</p>
          <p class="mt-2 text-sm leading-6 text-gray-600 dark:text-gray-400">{{ item.review_note || item.rejection_reason }}</p>
        </div>
      </div>

      <div class="mt-6 flex flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary btn-sm" @click="$emit('reject')">拒绝版本</button>
        <button type="button" class="btn btn-primary btn-sm" @click="$emit('approve')">通过版本</button>
      </div>
    </template>

    <template v-else>
      <div class="rounded-2xl border border-dashed border-gray-200 px-4 py-10 text-center text-sm text-gray-500 dark:border-dark-700 dark:text-gray-400">
        选择一条待审记录后，这里会展示版本摘要、风险等级与操作入口。
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import type { SkillReviewItem, SkillRiskLevel, SkillVisibility, SkillReviewStatus } from '@/api/admin/skills'
import SkillAdminStatusBadge from './SkillAdminStatusBadge.vue'

defineProps<{
  item: SkillReviewItem | null
}>()

defineEmits<{
  (e: 'approve'): void
  (e: 'reject'): void
}>()

function formatCurrency(value: number): string {
  return new Intl.NumberFormat('zh-CN', {
    style: 'currency',
    currency: 'CNY',
    minimumFractionDigits: 2
  }).format(value)
}

function reviewStatusLabel(status: SkillReviewStatus): string {
  return {
    pending: '待审核',
    approved: '已通过',
    rejected: '已拒绝',
    changes_requested: '待修改'
  }[status]
}

function visibilityLabel(status: SkillVisibility): string {
  return {
    public: '公开',
    private: '私有',
    force_private: '强制私有'
  }[status]
}

function riskLabel(level: SkillRiskLevel): string {
  return {
    low: '低风险',
    medium: '中风险',
    high: '高风险'
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
