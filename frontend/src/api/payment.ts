/**
 * User Payment API endpoints
 * Handles payment operations for regular users
 */

import { apiClient } from './client'
import type {
  PaymentConfig,
  SubscriptionPlan,
  MethodLimitsResponse,
  CheckoutInfoResponse,
  CreateOrderRequest,
  CreateOrderResult,
  PaymentOrder,
  Invoice
} from '@/types/payment'
import type { BasePaginationResponse } from '@/types'

export interface PublicOrderVerifyResult {
  out_trade_no: string
  status: string
  paid: boolean
  created_at: string
  expires_at: string
}

export const paymentAPI = {
  /** Get payment configuration (enabled types, limits, etc.) */
  getConfig() {
    return apiClient.get<PaymentConfig>('/payment/config')
  },

  /** Get available subscription plans */
  getPlans() {
    return apiClient.get<SubscriptionPlan[]>('/payment/plans')
  },

  /** Get all checkout page data in a single call */
  getCheckoutInfo() {
    return apiClient.get<CheckoutInfoResponse>('/payment/checkout-info')
  },

  /** Get payment method limits and fee rates */
  getLimits() {
    return apiClient.get<MethodLimitsResponse>('/payment/limits')
  },

  /** Create a new payment order */
  createOrder(data: CreateOrderRequest) {
    return apiClient.post<CreateOrderResult>('/payment/orders', data)
  },

  /** Get current user's orders */
  getMyOrders(params?: { page?: number; page_size?: number; status?: string }) {
    return apiClient.get<BasePaginationResponse<PaymentOrder>>('/payment/orders/my', { params })
  },

  /** Get a specific order by ID */
  getOrder(id: number) {
    return apiClient.get<PaymentOrder>(`/payment/orders/${id}`)
  },

  /** Cancel a pending order */
  cancelOrder(id: number) {
    return apiClient.post(`/payment/orders/${id}/cancel`)
  },

  /** Verify order payment status with upstream provider */
  verifyOrder(outTradeNo: string) {
    return apiClient.post<PaymentOrder>('/payment/orders/verify', { out_trade_no: outTradeNo })
  },

  /** Legacy-compatible public order lookup by out_trade_no */
  verifyOrderPublic(outTradeNo: string) {
    return apiClient.post<PublicOrderVerifyResult>('/payment/public/orders/verify', { out_trade_no: outTradeNo })
  },

  /** Resolve an order from a signed resume token without auth */
  resolveOrderPublicByResumeToken(resumeToken: string) {
    return apiClient.post<PublicOrderVerifyResult>('/payment/public/orders/resolve', { resume_token: resumeToken })
  },

  /** Request a refund for a completed order */
  requestRefund(id: number, data: { reason: string }) {
    return apiClient.post(`/payment/orders/${id}/refund-request`, data)
  },

  /** Get provider instance IDs that allow user refund */
  getRefundEligibleProviders() {
    return apiClient.get<{ provider_instance_ids: string[] }>('/payment/orders/refund-eligible-providers')
  },

  getInvoiceEligibleProviders() {
    return apiClient.get<{ provider_instance_ids: string[] }>('/payment/orders/invoice-eligible-providers')
  },

  applyInvoice(data: {
    order_ids: number[]
    title: string
    tax_number: string
    email: string
    contact_name?: string
    contact_phone?: string
    request_note?: string
  }) {
    return apiClient.post<Invoice>('/payment/invoices', data)
  },

  getMyInvoices(params?: { page?: number; page_size?: number; status?: string; keyword?: string }) {
    return apiClient.get<BasePaginationResponse<Invoice>>('/payment/invoices', { params })
  },

  getInvoice(id: number) {
    return apiClient.get<Invoice>(`/payment/invoices/${id}`)
  },

  cancelInvoice(id: number) {
    return apiClient.post<Invoice>(`/payment/invoices/${id}/cancel`)
  },

  getInvoiceDownloadGrant(id: number) {
    return apiClient.post<{ url: string; expires_at: number; ttl_minutes: number }>(`/payment/invoices/${id}/download-grant`)
  },

  downloadInvoiceFile(id: number) {
    return apiClient.get<Blob>(`/payment/invoices/${id}/file`, { responseType: 'blob' })
  },
}
