import type {
  SkillPriceMode,
  SkillRunMode,
  SkillRunStatus,
  SkillStatus,
  SkillType,
  SkillVersionReviewStatus,
  SkillVersionStatus
} from '@/types/skills'

type Translator = (key: string, fallback: string) => string

function translateLabel(translator: Translator | undefined, key: string, fallback: string): string {
  return translator ? translator(key, fallback) : fallback
}

export function skillTypeLabel(type: SkillType, translator?: Translator): string {
  switch (type) {
    case 'prompt_image':
      return translateLabel(translator, 'skills.labels.typePromptImage', '图片提示词')
    case 'script':
      return translateLabel(translator, 'skills.labels.typeScript', '脚本')
    case 'prompt_chat':
    default:
      return translateLabel(translator, 'skills.labels.typePromptChat', '聊天提示词')
  }
}

export function skillStatusLabel(status: SkillStatus, translator?: Translator): string {
  switch (status) {
    case 'published':
      return translateLabel(translator, 'skills.labels.statusPublished', '已发布')
    case 'archived':
      return translateLabel(translator, 'skills.labels.statusArchived', '已归档')
    case 'hidden':
      return translateLabel(translator, 'skills.labels.statusHidden', '已隐藏')
    case 'draft':
    default:
      return translateLabel(translator, 'skills.labels.statusDraft', '草稿')
  }
}

export function skillVersionStatusLabel(status: SkillVersionStatus, translator?: Translator): string {
  switch (status) {
    case 'deprecated':
      return translateLabel(translator, 'skills.versions.statusDeprecated', '已废弃')
    case 'archived':
      return translateLabel(translator, 'skills.labels.statusArchived', '已归档')
    case 'published':
      return translateLabel(translator, 'skills.labels.statusPublished', '已发布')
    case 'draft':
    default:
      return translateLabel(translator, 'skills.labels.statusDraft', '草稿')
  }
}

export function skillRunStatusLabel(status: SkillRunStatus, translator?: Translator): string {
  switch (status) {
    case 'queued':
      return translateLabel(translator, 'skills.labels.runStatusQueued', '已排队')
    case 'running':
      return translateLabel(translator, 'skills.labels.runStatusRunning', '运行中')
    case 'failed':
      return translateLabel(translator, 'skills.labels.runStatusFailed', '失败')
    case 'cancelled':
      return translateLabel(translator, 'skills.labels.runStatusCancelled', '已取消')
    case 'succeeded':
    default:
      return translateLabel(translator, 'skills.labels.runStatusSucceeded', '成功')
  }
}

export function skillVersionReviewStatusLabel(status: SkillVersionReviewStatus, translator?: Translator): string {
  switch (status) {
    case 'approved':
      return translateLabel(translator, 'skills.labels.reviewStatusApproved', '已通过')
    case 'pending':
      return translateLabel(translator, 'skills.labels.reviewStatusPending', '待审核')
    case 'rejected':
      return translateLabel(translator, 'skills.labels.reviewStatusRejected', '已拒绝')
    case 'draft':
    default:
      return translateLabel(translator, 'skills.labels.statusDraft', '草稿')
  }
}

export function skillRunTriggerLabel(trigger: string | null, translator?: Translator): string {
  const normalized = (trigger || '').trim().toLowerCase()
  switch (normalized as SkillRunMode) {
    case 'test':
      return translateLabel(translator, 'skills.labels.triggerTest', '测试')
    case 'use':
      return translateLabel(translator, 'skills.labels.triggerUse', '使用')
    default:
      return trigger || '-'
  }
}

export function skillPriceModeLabel(mode: SkillPriceMode, translator?: Translator): string {
  return mode === 'paid'
    ? translateLabel(translator, 'skills.labels.priceModePaid', '付费')
    : translateLabel(translator, 'skills.labels.priceModeFree', '免费')
}

export function formatCurrency(amount: number, currency = 'CNY'): string {
  try {
    return new Intl.NumberFormat(undefined, {
      style: 'currency',
      currency,
      maximumFractionDigits: currency === 'JPY' ? 0 : 2
    }).format(amount)
  } catch {
    return `${currency} ${amount.toFixed(2)}`
  }
}

export function skillStatusBadgeClass(status: SkillStatus): string {
  switch (status) {
    case 'published':
      return 'bg-success-soft text-success dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'archived':
      return 'bg-warning-soft text-warning dark:bg-amber-900/20 dark:text-amber-200'
    case 'hidden':
      return 'bg-danger-soft text-danger dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-page text-ink-soft dark:bg-dark-800 dark:text-dark-200'
  }
}

export function skillVersionBadgeClass(status: SkillVersionStatus): string {
  switch (status) {
    case 'published':
      return 'bg-success-soft text-success dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'deprecated':
      return 'bg-warning-soft text-warning dark:bg-amber-900/20 dark:text-amber-200'
    case 'archived':
      return 'bg-danger-soft text-danger dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-page text-ink-soft dark:bg-dark-800 dark:text-dark-200'
  }
}

export function skillVersionReviewBadgeClass(status: SkillVersionReviewStatus): string {
  switch (status) {
    case 'approved':
      return 'bg-success-soft text-success dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'pending':
      return 'bg-warning-soft text-warning dark:bg-amber-900/20 dark:text-amber-200'
    case 'rejected':
      return 'bg-danger-soft text-danger dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-page text-ink-soft dark:bg-dark-800 dark:text-dark-200'
  }
}

export function skillRunBadgeClass(status: SkillRunStatus): string {
  switch (status) {
    case 'succeeded':
      return 'bg-success-soft text-success dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'failed':
      return 'bg-danger-soft text-danger dark:bg-rose-900/20 dark:text-rose-200'
    case 'running':
      return 'bg-accent-50 text-accent-700 dark:bg-accent-900/20 dark:text-accent-200'
    case 'queued':
      return 'bg-warning-soft text-warning dark:bg-amber-900/20 dark:text-amber-200'
    case 'cancelled':
    default:
      return 'bg-page text-ink-soft dark:bg-dark-800 dark:text-dark-200'
  }
}
