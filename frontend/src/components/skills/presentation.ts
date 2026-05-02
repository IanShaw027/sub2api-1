import type {
  SkillPriceMode,
  SkillRunMode,
  SkillRunStatus,
  SkillStatus,
  SkillType,
  SkillVersionReviewStatus,
  SkillVersionStatus
} from '@/types/skills'

export function skillTypeLabel(type: SkillType): string {
  switch (type) {
    case 'prompt_image':
      return 'Prompt Image'
    case 'script':
      return 'Script'
    case 'prompt_chat':
    default:
      return 'Prompt Chat'
  }
}

export function skillStatusLabel(status: SkillStatus): string {
  switch (status) {
    case 'published':
      return 'Published'
    case 'archived':
      return 'Archived'
    case 'hidden':
      return 'Hidden'
    case 'draft':
    default:
      return 'Draft'
  }
}

export function skillVersionStatusLabel(status: SkillVersionStatus): string {
  switch (status) {
    case 'deprecated':
      return 'Deprecated'
    case 'archived':
      return 'Archived'
    case 'published':
      return 'Published'
    case 'draft':
    default:
      return 'Draft'
  }
}

export function skillRunStatusLabel(status: SkillRunStatus): string {
  switch (status) {
    case 'queued':
      return 'Queued'
    case 'running':
      return 'Running'
    case 'failed':
      return 'Failed'
    case 'cancelled':
      return 'Cancelled'
    case 'succeeded':
    default:
      return 'Succeeded'
  }
}

export function skillVersionReviewStatusLabel(status: SkillVersionReviewStatus): string {
  switch (status) {
    case 'approved':
      return 'Approved'
    case 'pending':
      return 'Pending Review'
    case 'rejected':
      return 'Rejected'
    case 'draft':
    default:
      return 'Draft'
  }
}

export function skillRunTriggerLabel(trigger: string | null): string {
  const normalized = (trigger || '').trim().toLowerCase()
  switch (normalized as SkillRunMode) {
    case 'test':
      return 'Test'
    case 'use':
      return 'Use'
    default:
      return trigger || '-'
  }
}

export function skillPriceModeLabel(mode: SkillPriceMode): string {
  return mode === 'paid' ? 'Paid' : 'Free'
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
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'archived':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-200'
    case 'hidden':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-dark-800 dark:text-slate-200'
  }
}

export function skillVersionBadgeClass(status: SkillVersionStatus): string {
  switch (status) {
    case 'published':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'deprecated':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-200'
    case 'archived':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-dark-800 dark:text-slate-200'
  }
}

export function skillVersionReviewBadgeClass(status: SkillVersionReviewStatus): string {
  switch (status) {
    case 'approved':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'pending':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-200'
    case 'rejected':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-200'
    case 'draft':
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-dark-800 dark:text-slate-200'
  }
}

export function skillRunBadgeClass(status: SkillRunStatus): string {
  switch (status) {
    case 'succeeded':
      return 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300'
    case 'failed':
      return 'bg-rose-50 text-rose-700 dark:bg-rose-900/20 dark:text-rose-200'
    case 'running':
      return 'bg-sky-50 text-sky-700 dark:bg-sky-900/20 dark:text-sky-200'
    case 'queued':
      return 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-200'
    case 'cancelled':
    default:
      return 'bg-slate-100 text-slate-700 dark:bg-dark-800 dark:text-slate-200'
  }
}
