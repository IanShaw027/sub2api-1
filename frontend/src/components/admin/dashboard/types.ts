/** Shared shapes for the admin dashboard sparkline widgets. */
export interface SparkPath {
  line: string
  area: string
}

export interface BreakdownItem {
  key: string
  label: string
  value: number
  textClass: string
}
export interface DistributionRow {
  key: string
  name: string
  cost: string
  pct: number
  requests: number
  tokens: number
  userId?: number
  model?: string
  standardCost?: number
  accountCost?: number
}
