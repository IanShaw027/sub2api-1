import type { TicketCategory, TicketStatus } from '@/types'

export const ticketCategoryOptions: Array<{ value: TicketCategory; labelKey: string }> = [
  { value: 'consult', labelKey: 'tickets.categories.consult' },
  { value: 'refund', labelKey: 'tickets.categories.refund' },
  { value: 'concurrency_apply', labelKey: 'tickets.categories.concurrency_apply' },
  { value: 'rate_apply', labelKey: 'tickets.categories.rate_apply' },
  { value: 'other', labelKey: 'tickets.categories.other' },
]

export const ticketStatusOptions: Array<{ value: TicketStatus; labelKey: string }> = [
  { value: 'submitted', labelKey: 'tickets.statuses.submitted' },
  { value: 'processing', labelKey: 'tickets.statuses.processing' },
  { value: 'waiting_user', labelKey: 'tickets.statuses.waiting_user' },
  { value: 'waiting_admin', labelKey: 'tickets.statuses.waiting_admin' },
  { value: 'resolved', labelKey: 'tickets.statuses.resolved' },
  { value: 'closed', labelKey: 'tickets.statuses.closed' },
  { value: 'withdrawn', labelKey: 'tickets.statuses.withdrawn' },
]

export function getTicketStatusBadgeClass(status: TicketStatus): string {
  switch (status) {
    case 'submitted':
      return 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-300'
    case 'processing':
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/40 dark:text-amber-300'
    case 'waiting_user':
      return 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-300'
    case 'waiting_admin':
      return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-300'
    case 'resolved':
      return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/40 dark:text-emerald-300'
    case 'closed':
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-ink-body'
    case 'withdrawn':
      return 'bg-rose-100 text-rose-700 dark:bg-rose-900/40 dark:text-rose-300'
    default:
      return 'bg-gray-100 text-gray-700 dark:bg-dark-700 dark:text-ink-body'
  }
}

const requiredFieldsByCategory: Record<TicketCategory, string[]> = {
  consult: ['question'],
  refund: ['order_no', 'reason'],
  concurrency_apply: ['current_concurrency', 'target_concurrency', 'usage_scenario'],
  rate_apply: ['target_rate', 'usage_scenario'],
  other: ['details'],
}

function isFiniteNumberString(value: unknown, { allowZero = false }: { allowZero?: boolean } = {}): boolean {
  const normalized = String(value ?? '').trim()
  if (!normalized) return false
  const parsed = Number(normalized)
  if (!Number.isFinite(parsed)) return false
  return allowZero ? parsed >= 0 : parsed > 0
}

export function validateTicketPayload(category: TicketCategory, title: string, payload: Record<string, unknown>) {
  if (!title.trim()) {
    return 'tickets.validation.titleRequired'
  }
  if (category === 'concurrency_apply') {
    if (!isFiniteNumberString(payload?.current_concurrency, { allowZero: true })) {
      return 'tickets.validation.formIncomplete'
    }
    if (!isFiniteNumberString(payload?.target_concurrency)) {
      return 'tickets.validation.formIncomplete'
    }
  }
  if (category === 'rate_apply') {
    const groupIDs = Array.isArray(payload?.group_ids)
      ? payload.group_ids.map((item) => Number(item)).filter((item) => Number.isFinite(item) && item > 0)
      : []
    if (groupIDs.length === 0) {
      return 'tickets.validation.formIncomplete'
    }
    if (!isFiniteNumberString(payload?.target_rate)) {
      return 'tickets.validation.formIncomplete'
    }
  }
  const requiredFields = requiredFieldsByCategory[category] || []
  for (const field of requiredFields) {
    const value = String(payload?.[field] ?? '').trim()
    if (!value) {
      return 'tickets.validation.formIncomplete'
    }
  }
  return ''
}
