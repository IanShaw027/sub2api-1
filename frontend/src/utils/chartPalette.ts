/** Clomio chart color sequence — brand first, gold as emphasis */
export const chartPalette = [
  '#17b8a6', // brand
  '#1fa2d6', // brand-cyan
  '#2f7bf6', // accent
  '#f5b93f', // gold
  '#16a34a', // success
  '#d97706', // warning
  '#dc2626', // danger
  '#0b8578', // brand-700
  '#1f63db', // accent-600
  '#64748b', // ink-soft / slate for "other"
] as const

export function chartColor(i: number): string {
  return chartPalette[i % chartPalette.length]
}
