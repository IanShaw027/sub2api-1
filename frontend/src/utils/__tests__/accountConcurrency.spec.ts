import { describe, expect, it } from 'vitest'
import { resolveAccountConcurrency } from '@/utils/accountConcurrency'

describe('accountConcurrency', () => {
  it('normalizes concurrency without platform-specific limits', () => {
    expect(resolveAccountConcurrency(10)).toBe(10)
    expect(resolveAccountConcurrency(0)).toBe(1)
    expect(resolveAccountConcurrency(5)).toBe(5)
  })
})
