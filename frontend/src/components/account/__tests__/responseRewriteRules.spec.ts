import { describe, expect, it } from 'vitest'

import { buildResponseRewriteRules, loadResponseRewriteRules } from '../responseRewriteRules'

describe('responseRewriteRules helpers', () => {
  it('builds response rewrite rules in order and skips invalid entries', () => {
    const rules = buildResponseRewriteRules([
      {
        status_code: 503,
        keywords: '欠费, insufficient_quota',
        match_mode: 'all',
        response_message: 'Service temporarily unavailable',
        description: 'quota mask'
      },
      {
        status_code: null,
        keywords: '',
        match_mode: 'all',
        response_message: ''
      },
      {
        status_code: 429,
        keywords: 'too many requests',
        match_mode: 'bogus' as 'any',
        response_message: 'skip invalid mode'
      },
      {
        status_code: null,
        keywords: 'rate limit',
        match_mode: 'any',
        response_message: 'Retry later',
        description: 'keyword only'
      }
    ])

    expect(rules).toEqual([
      {
        status_code: 503,
        keywords: ['欠费', 'insufficient_quota'],
        match_mode: 'all',
        response_message: 'Service temporarily unavailable',
        description: 'quota mask'
      },
      {
        keywords: ['rate limit'],
        match_mode: 'any',
        response_message: 'Retry later',
        description: 'keyword only'
      }
    ])
  })

  it('loads stored response rewrite rules into form state', () => {
    const rules = loadResponseRewriteRules({
      response_rewrite_rules: [
        {
          status_code: 401,
          keywords: ['token_invalidated', ' auth '],
          match_mode: 'all',
          response_message: 'Service temporarily unavailable',
          description: 'masked auth'
        },
        {
          keywords: ['Too Many Requests'],
          match_mode: 'invalid',
          response_message: 'Retry later'
        }
      ]
    })

    expect(rules).toEqual([
      {
        status_code: 401,
        keywords: 'token_invalidated, auth',
        match_mode: 'all',
        response_message: 'Service temporarily unavailable',
        description: 'masked auth'
      },
      {
        status_code: null,
        keywords: 'Too Many Requests',
        match_mode: 'any',
        response_message: 'Retry later',
        description: ''
      }
    ])
  })
})
