import { describe, expect, it } from 'vitest'

import { collectGeminiTierMetadataSources, stripStaleGeminiExtra } from '../geminiExtra'

describe('geminiExtra', () => {
  it('does not treat extra.subscription_type as a Gemini tier source', () => {
    expect(
      collectGeminiTierMetadataSources(
        {},
        {
          subscription_type: 'Gemini Code Assist in Google One AI Pro',
          keep_flag: true
        }
      )
    ).toEqual([])
  })

  it('removes stale Gemini tier metadata while preserving runtime usage snapshots', () => {
    expect(
      stripStaleGeminiExtra({
        subscription_type: 'Gemini Code Assist in Google One AI Pro',
        plan_name: 'Gemini Code Assist in Google One AI Pro',
        gemini_paid_tier_id: 'g1-pro-tier',
        gemini_current_tier_name: 'Gemini Code Assist in Google One AI Pro',
        gemini_usage_raw: { shared: true },
        gemini_status: 'forbidden',
        gemini_status_reason: 'quota denied',
        quota_query_last_error: 'forbidden',
        quota_query_last_error_at: '2026-05-08T00:00:00Z',
        usage_updated_at: '2026-05-08T00:00:00Z',
        keep_flag: true
      })
    ).toEqual({
      gemini_usage_raw: { shared: true },
      gemini_status: 'forbidden',
      gemini_status_reason: 'quota denied',
      quota_query_last_error: 'forbidden',
      quota_query_last_error_at: '2026-05-08T00:00:00Z',
      usage_updated_at: '2026-05-08T00:00:00Z',
      keep_flag: true
    })
  })
})
