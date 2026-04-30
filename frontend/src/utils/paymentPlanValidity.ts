export type PaymentPlanValidityUnit = 'day' | 'week' | 'month' | 'year'

export function normalizePaymentPlanValidityUnit(unit: string | null | undefined): PaymentPlanValidityUnit {
  switch ((unit || '').trim().toLowerCase()) {
    case 'week':
    case 'weeks':
      return 'week'
    case 'month':
    case 'months':
      return 'month'
    case 'year':
    case 'years':
      return 'year'
    default:
      return 'day'
  }
}
