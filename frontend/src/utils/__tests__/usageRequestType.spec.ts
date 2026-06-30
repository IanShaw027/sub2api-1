import { describe, expect, it } from 'vitest'

import {
  isUsageRequestType,
  requestTypeToLegacyStream,
  resolveUsageRequestType
} from '../usageRequestType'

describe('usageRequestType', () => {
  it('recognizes video request_type and keeps it ahead of legacy stream fields', () => {
    expect(isUsageRequestType('video')).toBe(true)
    expect(resolveUsageRequestType({ request_type: 'video', stream: true, openai_ws_mode: true })).toBe('video')
  })

  it('maps video to non-stream legacy filters', () => {
    expect(requestTypeToLegacyStream('video')).toBe(false)
  })
})
