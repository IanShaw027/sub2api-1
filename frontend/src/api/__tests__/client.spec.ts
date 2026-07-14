import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import axios from 'axios'
import type { AxiosInstance } from 'axios'

// 需要在导入 client 之前设置 mock
vi.mock('@/i18n', () => ({
  getLocale: () => 'zh-CN',
}))

describe('API Client', () => {
  let apiClient: AxiosInstance

  beforeEach(async () => {
    localStorage.clear()
    window.history.replaceState({}, '', '/')
    // 每次测试重新导入以获取干净的模块状态
    vi.resetModules()
    const mod = await import('@/api/client')
    apiClient = mod.apiClient
  })

  afterEach(() => {
    vi.restoreAllMocks()
    vi.unstubAllEnvs()
  })

  // --- 请求拦截器 ---

  describe('请求拦截器', () => {
    it('规范化相对 API base，避免在回调页拼出相对 v1 路径', async () => {
      vi.resetModules()
      vi.stubEnv('VITE_API_BASE_URL', 'api/v1')

      const mod = await import('@/api/client')

      expect(mod.apiClient.defaults.baseURL).toBe('/api/v1')
      expect(mod.buildApiUrl('/auth/oauth/github/callback?code=abc')).toBe(
        '/api/v1/auth/oauth/github/callback?code=abc'
      )
    })

    it('构建网关 URL 时保留 API base 的部署路径前缀并去掉 /api/v1', async () => {
      vi.resetModules()
      vi.stubEnv('VITE_API_BASE_URL', '/edge/api/v1')

      const mod = await import('@/api/client')

      expect(mod.buildGatewayUrl('/v1/usage')).toBe(`${window.location.origin}/edge/v1/usage`)
    })

    it('自动附加 Authorization 头', async () => {
      localStorage.setItem('auth_token', 'my-jwt-token')

      // 拦截实际请求
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBe('Bearer my-jwt-token')
    })

    it('无 token 时不附加 Authorization 头', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBeFalsy()
    })

    it('公共支付恢复接口不附加 Authorization 头', async () => {
      localStorage.setItem('auth_token', 'stale-token')

      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/payment/public/orders/verify', { out_trade_no: 'legacy-order-no' })

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('Authorization')).toBeFalsy()
    })

    it('GET 请求自动附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/test')

      const config = adapter.mock.calls[0][0]
      expect(config.params).toHaveProperty('timezone')
    })

    it('POST 请求不附加 timezone 参数', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/test', { foo: 'bar' })

      const config = adapter.mock.calls[0][0]
      expect(config.params?.timezone).toBeUndefined()
    })

    it('请求默认带 withCredentials 以支持跨域 cookie', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.post('/auth/oauth/bind-token')

      const config = adapter.mock.calls[0][0]
      expect(config.withCredentials).toBe(true)
    })

    it('FormData 请求不保留默认 JSON Content-Type', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      const formData = new FormData()
      formData.append('file', new File(['zip-bytes'], 'accounts.zip', { type: 'application/zip' }))

      await apiClient.post('/admin/accounts/import/archive', formData)

      const config = adapter.mock.calls[0][0]
      expect(config.data).toBe(formData)
      expect(config.headers.get('Content-Type')).toBe('multipart/form-data')
    })

    it('Admin API 在进入管理页面前也带 Admin UI 标记', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/admin/users')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBe('1')
    })

    it('管理页面调用共享 API 时带 Admin UI 标记', async () => {
      window.history.replaceState({}, '', '/admin/dashboard')
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/groups/available')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBe('1')
    })

    it('普通用户页面调用共享 API 时不带 Admin UI 标记', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: {} },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await apiClient.get('/groups/available')

      const config = adapter.mock.calls[0][0]
      expect(config.headers.get('X-Admin-UI-Request')).toBeFalsy()
    })
  })

  // --- 响应拦截器 ---

  describe('响应拦截器', () => {
    it('code=0 时解包 data 字段', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 0, data: { name: 'test' }, message: 'ok' },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      const response = await apiClient.get('/test')
      expect(response.data).toEqual({ name: 'test' })
    })

    it('code!=0 时拒绝并返回结构化错误', async () => {
      const adapter = vi.fn().mockResolvedValue({
        status: 200,
        data: { code: 1001, message: '参数错误', data: null },
        headers: {},
        config: {},
        statusText: 'OK',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          code: 1001,
          message: '参数错误',
        })
      )
    })

    it('部署与运营合规未确认时广播事件且保留登录态', async () => {
      localStorage.setItem('auth_token', 'admin-token')
      const listener = vi.fn()
      window.addEventListener('admin-compliance-required', listener)

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 423,
          data: {
            code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
            message: 'administrator compliance acknowledgement is required',
            metadata: {
              version: 'v2026.06.10',
              document_path_zh: 'docs/legal/admin-compliance.zh.md',
              document_path_en: 'docs/legal/admin-compliance.en.md',
            },
          },
        },
        config: {
          url: '/admin/users',
          headers: { Authorization: 'Bearer admin-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/admin/users')).rejects.toEqual(
        expect.objectContaining({
          status: 423,
          code: 'ADMIN_COMPLIANCE_ACK_REQUIRED',
          metadata: expect.objectContaining({
            version: 'v2026.06.10',
          }),
        })
      )

      expect(listener).toHaveBeenCalledTimes(1)
      expect((listener.mock.calls[0][0] as CustomEvent).detail).toEqual(
        expect.objectContaining({
          version: 'v2026.06.10',
        })
      )
      expect(localStorage.getItem('auth_token')).toBe('admin-token')

      window.removeEventListener('admin-compliance-required', listener)
    })
  })

  // --- 401 Token 刷新 ---

  describe('401 Token 刷新', () => {
    it('无 refresh_token 时 401 清除 localStorage', async () => {
      localStorage.setItem('auth_token', 'expired-token')
      // 不设置 refresh_token

      // Mock window.location
      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/dashboard', href: '/dashboard' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/test',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toBeDefined()

      expect(localStorage.getItem('auth_token')).toBeNull()

      // 恢复 location
      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('payment result 页面上的支付恢复 401 不会重定向到 /login', async () => {
      localStorage.setItem('auth_token', 'expired-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/payment/result', href: '/payment/result' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/payment/orders/verify',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.post('/payment/orders/verify', { out_trade_no: 'legacy-order-no' })).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'TOKEN_EXPIRED',
        })
      )

      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(window.location.href).toBe('/payment/result')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('payment result 页面上的非支付恢复 401 仍然会重定向到 /login', async () => {
      localStorage.setItem('auth_token', 'expired-token')

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/payment/result', href: '/payment/result' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'TOKEN_EXPIRED', message: 'Token expired' },
        },
        config: {
          url: '/users/me',
          headers: { Authorization: 'Bearer expired-token' },
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/users/me')).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'TOKEN_EXPIRED',
        })
      )

      expect(localStorage.getItem('auth_token')).toBeNull()
      expect(window.location.href).toBe('/login')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })

    it('payment result 页面上的公共支付恢复 401 不会清空现有登录态', async () => {
      localStorage.setItem('auth_token', 'still-valid-token')
      localStorage.setItem('refresh_token', 'still-valid-refresh-token')
      localStorage.setItem('auth_user', JSON.stringify({ id: 1, email: 'user@example.com' }))

      const originalLocation = window.location
      Object.defineProperty(window, 'location', {
        value: { ...originalLocation, pathname: '/payment/result', href: '/payment/result' },
        writable: true,
      })

      const adapter = vi.fn().mockRejectedValue({
        response: {
          status: 401,
          data: { code: 'ORDER_NOT_FOUND', message: 'Order not found' },
        },
        config: {
          url: '/payment/public/orders/legacy-order-no',
          headers: {},
        },
        code: 'ERR_BAD_REQUEST',
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/payment/public/orders/legacy-order-no')).rejects.toEqual(
        expect.objectContaining({
          status: 401,
          code: 'ORDER_NOT_FOUND',
        })
      )

      expect(localStorage.getItem('auth_token')).toBe('still-valid-token')
      expect(localStorage.getItem('refresh_token')).toBe('still-valid-refresh-token')
      expect(localStorage.getItem('auth_user')).toBe(JSON.stringify({ id: 1, email: 'user@example.com' }))
      expect(window.location.href).toBe('/payment/result')

      Object.defineProperty(window, 'location', {
        value: originalLocation,
        writable: true,
      })
    })
  })

  // --- 网络错误 ---

  describe('网络错误', () => {
    it('网络错误返回 status 0 的错误', async () => {
      const adapter = vi.fn().mockRejectedValue({
        code: 'ERR_NETWORK',
        message: 'Network Error',
        config: { url: '/test' },
        // 没有 response
      })
      apiClient.defaults.adapter = adapter

      await expect(apiClient.get('/test')).rejects.toEqual(
        expect.objectContaining({
          status: 0,
          message: 'Network error. Please check your connection.',
        })
      )
    })
  })

  // --- 请求取消 ---

  describe('请求取消', () => {
    it('取消的请求保持原始取消错误', async () => {
      const source = axios.CancelToken.source()

      const adapter = vi.fn().mockRejectedValue(
        new axios.Cancel('Operation canceled')
      )
      apiClient.defaults.adapter = adapter

      await expect(
        apiClient.get('/test', { cancelToken: source.token })
      ).rejects.toBeDefined()
    })
  })
})
