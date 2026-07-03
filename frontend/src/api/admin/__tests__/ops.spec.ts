import { afterEach, describe, expect, it, vi } from 'vitest'

import { buildOpsWebSocketURL } from '../ops'

describe('ops websocket URL handling', () => {
  afterEach(() => {
    vi.unstubAllEnvs()
    vi.restoreAllMocks()
  })

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

  it('uses the configured API base URL when VITE_WS_BASE_URL is not set', async () => {
    vi.resetModules()
    vi.stubEnv('VITE_API_BASE_URL', 'https://api.example.com/proxy/api/v1')
    vi.stubEnv('VITE_WS_BASE_URL', '')

    const createdURLs: string[] = []
    class FakeWebSocket {
      static CONNECTING = 0
      static OPEN = 1
      static CLOSING = 2
      static CLOSED = 3

      readyState = FakeWebSocket.CONNECTING
      onopen: ((event: Event) => void) | null = null
      onmessage: ((event: MessageEvent) => void) | null = null
      onerror: ((event: Event) => void) | null = null
      onclose: ((event: CloseEvent) => void) | null = null

      constructor(url: string | URL) {
        createdURLs.push(String(url))
      }

      close() {
        this.readyState = FakeWebSocket.CLOSED
      }
    }

    vi.stubGlobal('WebSocket', FakeWebSocket)
    const { subscribeQPS } = await import('../ops')

    const unsubscribe = subscribeQPS(() => {}, { token: 'admin-token', maxReconnectAttempts: 0 })
    unsubscribe()

    expect(createdURLs[0]).toBe('wss://api.example.com/proxy/api/v1/admin/ops/ws/qps')
  })
})
