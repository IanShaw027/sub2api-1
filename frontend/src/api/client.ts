/**
 * Axios HTTP Client Configuration
 * Base client with interceptors for authentication, token refresh, and error handling
 */

import axios, { AxiosInstance, AxiosError, InternalAxiosRequestConfig, AxiosResponse } from 'axios'
import type { ApiResponse } from '@/types'
import { getLocale } from '@/i18n'
import { ADMIN_UI_REQUEST_HEADER, shouldMarkAdminUIRequest } from './adminUIRequest'
import { getAPIBaseURL } from './url'
import {
  clearAccessToken,
  clearLegacyAuthStorage,
  getAccessToken,
  setAccessToken,
  setAccessTokenExpiresIn,
} from '@/utils/authSession'
export { buildApiUrl, buildGatewayUrl } from './url'

export const REFRESH_COOKIE_MODE_HEADER = 'X-Sub2API-Refresh'

// ==================== Axios Instance Configuration ====================

export const apiClient: AxiosInstance = axios.create({
  baseURL: getAPIBaseURL(),
  withCredentials: true,
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

// ==================== Token Refresh State ====================

let refreshPromise: Promise<RefreshSessionResponse> | null = null

export interface RefreshSessionResponse {
  access_token: string
  refresh_token?: string
  expires_in: number
  token_type: string
}

function normalizeRequestUrl(url: string): string {
  if (!url) {
    return ''
  }

  try {
    const base = typeof window !== 'undefined' ? window.location.origin : 'http://localhost'
    return new URL(url, base).pathname
  } catch {
    const stripped = url.split('?')[0]?.split('#')[0] ?? url
    if (!stripped) {
      return ''
    }
    return stripped.startsWith('/') ? stripped : `/${stripped.replace(/^\/+/, '')}`
  }
}

function isPublicPaymentRecoveryPath(path: string): boolean {
  return path.startsWith('/payment/public/orders/')
}

function isPublicPaymentRecoveryEndpoint(url: string): boolean {
  return isPublicPaymentRecoveryPath(normalizeRequestUrl(url))
}

function isPaymentResultRecoveryRequest(url: string): boolean {
  if (typeof window === 'undefined' || window.location.pathname !== '/payment/result') {
    return false
  }

  const path = normalizeRequestUrl(url)
  return path === '/payment/orders/verify' ||
    /^\/payment\/orders\/\d+$/.test(path) ||
    isPublicPaymentRecoveryPath(path)
}

async function withCrossTabRefreshLock<T>(task: () => Promise<T>): Promise<T> {
  const locks = typeof navigator !== 'undefined'
    ? (navigator as Navigator & {
        locks?: { request: <R>(name: string, callback: () => Promise<R>) => Promise<R> }
      }).locks
    : undefined
  if (locks?.request) {
    return locks.request('sub2api-auth-refresh', task)
  }

  if (typeof localStorage === 'undefined') {
    return task()
  }
  const lockKey = 'sub2api_auth_refresh_lock'
  const owner = `${Date.now()}:${Math.random()}`
  const deadline = Date.now() + 5000
  while (Date.now() < deadline) {
    const now = Date.now()
    const current = localStorage.getItem(lockKey)
    const expiresAt = Number(current?.split(':').at(-1) || 0)
    if (!current || !Number.isFinite(expiresAt) || expiresAt <= now) {
      localStorage.setItem(lockKey, `${owner}:${now + 4000}`)
      if (localStorage.getItem(lockKey)?.startsWith(owner)) {
        try {
          return await task()
        } finally {
          if (localStorage.getItem(lockKey)?.startsWith(owner)) {
            localStorage.removeItem(lockKey)
          }
        }
      }
    }
    await new Promise((resolve) => window.setTimeout(resolve, 50))
  }
  return task()
}

export function refreshSession(legacyRefreshToken?: string | null): Promise<RefreshSessionResponse> {
  if (refreshPromise) return refreshPromise

  refreshPromise = withCrossTabRefreshLock(async () => {
    const response = await axios.post(
      `${getAPIBaseURL()}/auth/refresh`,
      legacyRefreshToken ? { refresh_token: legacyRefreshToken } : {},
      {
        withCredentials: true,
        headers: {
          'Content-Type': 'application/json',
          [REFRESH_COOKIE_MODE_HEADER]: '1',
        },
        timeout: 30000,
      }
    )
    const envelope = response.data as ApiResponse<RefreshSessionResponse>
    if (!envelope || envelope.code !== 0 || !envelope.data?.access_token) {
      throw new Error(envelope?.message || 'Token refresh failed')
    }
    setAccessToken(envelope.data.access_token)
    setAccessTokenExpiresIn(envelope.data.expires_in)
    return envelope.data
  }).finally(() => {
    refreshPromise = null
  })
  return refreshPromise
}

// ==================== Request Interceptor ====================

// Get user's timezone
const getUserTimezone = (): string => {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone
  } catch {
    return 'UTC'
  }
}

apiClient.interceptors.request.use(
  (config: InternalAxiosRequestConfig) => {
    const isFormDataPayload = typeof FormData !== 'undefined' && config.data instanceof FormData
    if (isFormDataPayload && config.headers) {
      // Prevent the instance default application/json header from serializing files as {"file":{}}.
      // Axios' browser adapters remove the bare multipart header and let the browser add the boundary.
      config.headers.setContentType('multipart/form-data')
    }

    // Access tokens are process-memory only; refresh credentials stay HttpOnly.
    const token = getAccessToken()
    const url = String(config.url || '')
    if (token && config.headers && !isPublicPaymentRecoveryEndpoint(url)) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // Attach locale for backend translations
    if (config.headers) {
      config.headers['Accept-Language'] = getLocale()
    }

    // Attach timezone for all GET requests (backend may use it for default date ranges)
    if (config.method === 'get') {
      if (!config.params) {
        config.params = {}
      }
      config.params.timezone = getUserTimezone()
    }

    if (config.headers && shouldMarkAdminUIRequest(String(config.url || ''))) {
      config.headers[ADMIN_UI_REQUEST_HEADER] = '1'
    }
    const normalizedPath = normalizeRequestUrl(String(config.url || ''))
    if (
      config.headers &&
      (
        normalizedPath === '/auth/login' ||
        normalizedPath === '/auth/login/2fa' ||
        normalizedPath === '/auth/register' ||
        normalizedPath === '/auth/refresh' ||
        normalizedPath === '/auth/logout' ||
        normalizedPath.startsWith('/auth/oauth/')
      )
    ) {
      config.headers[REFRESH_COOKIE_MODE_HEADER] = '1'
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// ==================== Response Interceptor ====================

apiClient.interceptors.response.use(
  (response: AxiosResponse) => {
    // Unwrap standard API response format { code, message, data }
    const apiResponse = response.data as ApiResponse<unknown>
    if (apiResponse && typeof apiResponse === 'object' && 'code' in apiResponse) {
      if (apiResponse.code === 0) {
        // Success - return the data portion
        response.data = apiResponse.data
      } else {
        // API error
        const resp = apiResponse as unknown as Record<string, unknown>
        return Promise.reject({
          status: response.status,
          code: apiResponse.code,
          message: apiResponse.message || 'Unknown error',
          reason: resp.reason,
          metadata: resp.metadata,
        })
      }
    }
    return response
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    // Request cancellation: keep the original axios cancellation error so callers can ignore it.
    // Otherwise we'd misclassify it as a generic "network error".
    if (error.code === 'ERR_CANCELED' || axios.isCancel(error)) {
      return Promise.reject(error)
    }

    const originalRequest = error.config as InternalAxiosRequestConfig & { _retry?: boolean }

    // Handle common errors
    if (error.response) {
      const { status, data } = error.response
      const url = String(error.config?.url || '')
      const normalizedUrl = normalizeRequestUrl(url)
      const suppressAuthRedirect = isPaymentResultRecoveryRequest(normalizedUrl)
      const isPublicPaymentRecovery = isPublicPaymentRecoveryPath(normalizedUrl)
      const preserveAuthState = isPublicPaymentRecovery

      // Validate `data` shape to avoid HTML error pages breaking our error handling.
      const apiData = (typeof data === 'object' && data !== null ? data : {}) as Record<string, any>

      // Ops monitoring disabled: treat as feature-flagged 404, and proactively redirect away
      // from ops pages to avoid broken UI states.
      if (status === 404 && apiData.message === 'Ops monitoring is disabled') {
        try {
          localStorage.setItem('ops_monitoring_enabled_cached', 'false')
        } catch {
          // ignore localStorage failures
        }
        try {
          window.dispatchEvent(new CustomEvent('ops-monitoring-disabled'))
        } catch {
          // ignore event failures
        }

        if (window.location.pathname.startsWith('/admin/ops')) {
          window.location.href = '/admin/settings'
        }

        return Promise.reject({
          status,
          code: 'OPS_DISABLED',
          message: apiData.message || error.message,
          url
        })
      }

      if (status === 423 && apiData.code === 'ADMIN_COMPLIANCE_ACK_REQUIRED') {
        try {
          window.dispatchEvent(new CustomEvent('admin-compliance-required', {
            detail: apiData.metadata || {}
          }))
        } catch {
          // ignore event failures
        }

        return Promise.reject({
          status,
          code: apiData.code,
          message: apiData.message || error.message,
          metadata: apiData.metadata,
        })
      }

      // 401: Try the HttpOnly refresh cookie once for authenticated requests.
      // This handles TOKEN_EXPIRED, INVALID_TOKEN, TOKEN_REVOKED, etc.
      if (status === 401 && !originalRequest._retry) {
        const isAuthEndpoint =
          url.includes('/auth/login') || url.includes('/auth/register') || url.includes('/auth/refresh')
        const currentToken = getAccessToken()

        if (currentToken && !isAuthEndpoint && !isPublicPaymentRecovery) {
          originalRequest._retry = true

          try {
            const refreshed = await refreshSession()
            if (originalRequest.headers) {
              originalRequest.headers.Authorization = `Bearer ${refreshed.access_token}`
            }
            return apiClient(originalRequest)
          } catch {
            clearAccessToken()
            clearLegacyAuthStorage()
            if (!preserveAuthState) {
              sessionStorage.setItem('auth_expired', '1')
            }

            if (!suppressAuthRedirect && !window.location.pathname.includes('/login')) {
              window.location.href = '/login'
            }

            return Promise.reject({
              status: 401,
              code: 'TOKEN_REFRESH_FAILED',
              message: 'Session expired. Please log in again.'
            })
          }
        }

        // No refresh token or is auth endpoint - clear auth and redirect
        const hasToken = !!getAccessToken()
        const headers = error.config?.headers as Record<string, unknown> | undefined
        const authHeader = headers?.Authorization ?? headers?.authorization
        const sentAuth =
          typeof authHeader === 'string'
            ? authHeader.trim() !== ''
            : Array.isArray(authHeader)
              ? authHeader.length > 0
              : !!authHeader

        if (!preserveAuthState) {
          clearAccessToken()
          clearLegacyAuthStorage()
        }
        if (!preserveAuthState && (hasToken || sentAuth) && !isAuthEndpoint) {
          sessionStorage.setItem('auth_expired', '1')
        }
        // Only redirect if not already on login page
        if (!suppressAuthRedirect && !window.location.pathname.includes('/login')) {
          window.location.href = '/login'
        }
      }

      // Return structured error
      return Promise.reject({
        status,
        code: apiData.code,
        reason: apiData.reason,
        error: apiData.error,
        message: apiData.message || apiData.detail || error.message,
        metadata: apiData.metadata,
      })
    }

    // Network error
    return Promise.reject({
      status: 0,
      message: 'Network error. Please check your connection.'
    })
  }
)

export default apiClient
