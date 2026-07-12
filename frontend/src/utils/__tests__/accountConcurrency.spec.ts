import { describe, expect, it } from 'vitest'
import { isGrokOAuthConcurrency, resolveAccountConcurrency } from '@/utils/accountConcurrency'

describe('accountConcurrency', () => {
  it('detects grok oauth', () => {
    expect(isGrokOAuthConcurrency('grok', 'oauth')).toBe(true)
    expect(isGrokOAuthConcurrency('grok', 'apikey')).toBe(false)
    expect(isGrokOAuthConcurrency('openai', 'oauth')).toBe(false)
  })

  it('clamps grok oauth concurrency to 1', () => {
    expect(resolveAccountConcurrency('grok', 'oauth', 10)).toBe(1)
    expect(resolveAccountConcurrency('grok', 'oauth', 0)).toBe(1)
    expect(resolveAccountConcurrency('openai', 'oauth', 10)).toBe(10)
    expect(resolveAccountConcurrency('grok', 'apikey', 5)).toBe(5)
  })
})
