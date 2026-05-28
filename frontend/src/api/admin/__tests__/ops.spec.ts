import { describe, expect, it } from 'vitest'

import { buildOpsWebSocketURL } from '../ops'

describe('ops websocket URL handling', () => {
  it('accepts a bare host and preserves the current page protocol', () => {
    expect(buildOpsWebSocketURL('https:', 'api.example.com').toString()).toBe(
      'wss://api.example.com/api/v1/admin/ops/ws/qps'
    )
  })

  it('normalizes explicit http and websocket base URLs', () => {
    expect(buildOpsWebSocketURL('http:', 'https://api.example.com/base/').toString()).toBe(
      'wss://api.example.com/base/api/v1/admin/ops/ws/qps'
    )
    expect(buildOpsWebSocketURL('https:', 'ws://127.0.0.1:8080').toString()).toBe(
      'ws://127.0.0.1:8080/api/v1/admin/ops/ws/qps'
    )
  })

  it('supports relative websocket bases behind the current origin', () => {
    expect(buildOpsWebSocketURL('https:', '/ops-ws', 'admin.example.com').toString()).toBe(
      'wss://admin.example.com/ops-ws/api/v1/admin/ops/ws/qps'
    )
  })
})
