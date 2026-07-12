/**
 * Account concurrency helpers shared by Create/Edit account modals.
 *
 * Grok personal OAuth subscriptions are sensitive to multi-session load.
 * Backend rejects concurrency > 1 unless XAI_GROK_UNSAFE_ALLOW_CONCURRENCY_GT_ONE is set.
 * UI clamps to 1 for that account type so operators don't hit unexpected 400s.
 */

export function isGrokOAuthConcurrency(platform: string, type: string): boolean {
  return platform === 'grok' && type === 'oauth'
}

export function resolveAccountConcurrency(
  platform: string,
  type: string,
  concurrency: number,
): number {
  const n = Math.max(1, Number(concurrency) || 1)
  if (isGrokOAuthConcurrency(platform, type)) {
    return 1
  }
  return n
}
