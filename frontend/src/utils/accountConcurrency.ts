/**
 * Account concurrency helper shared by Create/Edit account modals.
 */

export function resolveAccountConcurrency(concurrency: number): number {
  return Math.max(1, Number(concurrency) || 1)
}
