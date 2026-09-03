import { refreshAuthTokens } from './tokenRefresh'

export interface AuthenticatedFetchInit extends RequestInit {
  /** Internal: prevents infinite refresh loops. */
  _retry?: boolean
}

function readBearerToken(headers: HeadersInit | undefined): string | null {
  if (!headers) return null
  const record = headers instanceof Headers ? Object.fromEntries(headers.entries()) : headers
  const auth = record.Authorization ?? record.authorization
  if (typeof auth !== 'string' || !auth.startsWith('Bearer ')) return null
  return auth.slice('Bearer '.length)
}

function withBearerToken(headers: HeadersInit | undefined, token: string): Headers {
  const next = new Headers(headers)
  next.set('Authorization', `Bearer ${token}`)
  return next
}

function clearAuthAndRedirect(): void {
  localStorage.removeItem('auth_token')
  localStorage.removeItem('refresh_token')
  localStorage.removeItem('auth_user')
  localStorage.removeItem('token_expires_at')
  sessionStorage.setItem('auth_expired', '1')

  if (!window.location.pathname.includes('/login')) {
    window.location.href = '/login'
  }
}

/**
 * fetch wrapper that mirrors apiClient's 401 → refresh → retry behaviour for
 * endpoints that cannot use axios (SSE streams, raw Response bodies).
 */
export async function authenticatedFetch(
  input: RequestInfo | URL,
  init: AuthenticatedFetchInit = {},
): Promise<Response> {
  const response = await fetch(input, init)

  if (response.status !== 401 || init._retry) {
    return response
  }

  const refreshToken = localStorage.getItem('refresh_token')
  if (!refreshToken) {
    return response
  }

  const refreshSessionUser = localStorage.getItem('auth_user')
  const failedAccessToken = readBearerToken(init.headers)

  try {
    const tokens = await refreshAuthTokens({ failedAccessToken })
    return authenticatedFetch(input, {
      ...init,
      headers: withBearerToken(init.headers, tokens.access_token),
      _retry: true,
    })
  } catch {
    const sessionChanged =
      localStorage.getItem('refresh_token') !== refreshToken ||
      localStorage.getItem('auth_user') !== refreshSessionUser

    if (!sessionChanged) {
      clearAuthAndRedirect()
    }

    return response
  }
}
