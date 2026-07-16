import type { User } from '@/types'

let accessToken: string | null = null
let accessTokenExpiresAt: number | null = null
let currentUser: User | null = null
const authEventStorageKey = 'sub2api_auth_event'

export type AuthSessionEvent = 'logout' | 'session-available'

let authChannel: BroadcastChannel | null = null
const authSessionListeners = new Set<(type: AuthSessionEvent) => void>()
if (typeof BroadcastChannel !== 'undefined') {
  authChannel = new BroadcastChannel('sub2api-auth')
}

function dispatchAuthSessionEvent(type: AuthSessionEvent): void {
  authSessionListeners.forEach((listener) => listener(type))
}

authChannel?.addEventListener('message', (event: MessageEvent<{ type?: AuthSessionEvent }>) => {
  if (event.data?.type) dispatchAuthSessionEvent(event.data.type)
})
if (typeof window !== 'undefined') {
  window.addEventListener('storage', (event: StorageEvent) => {
    if (event.key !== authEventStorageKey || !event.newValue) return
    try {
      const type = JSON.parse(event.newValue)?.type as AuthSessionEvent | undefined
      if (type) dispatchAuthSessionEvent(type)
    } catch {
      // Ignore malformed non-secret coordination data.
    }
  })
}

export function getAccessToken(): string | null {
  return accessToken
}

export function setAccessToken(token: string | null): void {
  accessToken = token?.trim() || null
}

export function clearAccessToken(): void {
  accessToken = null
  accessTokenExpiresAt = null
  currentUser = null
}

export function getSessionUser(): User | null {
  return currentUser
}

export function setSessionUser(user: User | null): void {
  currentUser = user
}

export function getAccessTokenExpiresAt(): number | null {
  return accessTokenExpiresAt
}

export function setAccessTokenExpiresIn(expiresInSeconds?: number): number | null {
  if (!expiresInSeconds || expiresInSeconds <= 0) {
    accessTokenExpiresAt = null
    return null
  }
  accessTokenExpiresAt = Date.now() + expiresInSeconds * 1000
  return accessTokenExpiresAt
}

export function clearLegacyAuthStorage(): void {
  if (typeof localStorage === 'undefined') return
  localStorage.removeItem('auth_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('auth_user')
  localStorage.removeItem('token_expires_at')
}

export function readLegacyRefreshToken(): string | null {
  if (typeof localStorage === 'undefined') return null
  return localStorage.getItem('refresh_token')
}

export function publishAuthSessionEvent(type: AuthSessionEvent): void {
  authChannel?.postMessage({ type })
  if (typeof localStorage !== 'undefined') {
    try {
      localStorage.setItem(authEventStorageKey, JSON.stringify({ type, at: Date.now() }))
      localStorage.removeItem(authEventStorageKey)
    } catch {
      // BroadcastChannel remains the primary path.
    }
  }
}

export function subscribeAuthSessionEvents(listener: (type: AuthSessionEvent) => void): () => void {
  authSessionListeners.add(listener)
  return () => {
    authSessionListeners.delete(listener)
  }
}
