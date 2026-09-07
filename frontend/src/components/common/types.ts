/**
 * Common component types
 */

export interface Column {
  key: string
  label: string
  sortable?: boolean
  class?: string
  /** Maximum content width; long values wrap without hiding actions or data. */
  maxWidth?: number | string
  formatter?: (value: any, row: any) => string
}
