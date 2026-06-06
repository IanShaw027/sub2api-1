/**
 * User Payment API endpoints
 * Handles payment operations for regular users
 */

import { apiClient } from './client'
import type {
  PaymentConfig,
  SubscriptionPlan,
  PaymentChannel,
  MethodLimitsResponse,
  CheckoutInfoResponse,
  CreateOrderRequest,
  CreateOrderResult,
  PaymentOrder,
  RefundPreview,
  InvoiceApplication,
  BatchApplyInvoiceResult
} from '@/types/payment'
import type { BasePaginationResponse } from '@/types'
import type { MediaDownloadURL } from '@/types'

export const paymentAPI = {
  /** Get payment configuration (enabled types, limits, etc.) */
  getConfig() {
    return apiClient.get<PaymentConfig>('/payment/config')
  },

  /** Get available subscription plans */
  getPlans() {
    return apiClient.get<SubscriptionPlan[]>('/payment/plans')
  },

  /** Get available payment channels */
  getChannels() {
    return apiClient.get<PaymentChannel[]>('/payment/channels')
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
    return apiClient.post<PaymentOrder>('/payment/public/orders/verify', { out_trade_no: outTradeNo })
  },

  /** Resolve an order from a signed resume token without auth */
  resolveOrderPublicByResumeToken(resumeToken: string) {
    return apiClient.post<PaymentOrder>('/payment/public/orders/resolve', { resume_token: resumeToken })
  },

  /** Request a refund for a completed order */
  requestRefund(id: number, data: { amount: number; reason: string }) {
    return apiClient.post(`/payment/orders/${id}/refund-request`, data)
  },

  /** Get refundable amount details for a completed order */
  getRefundPreview(id: number) {
    return apiClient.get<RefundPreview>(`/payment/orders/${id}/refund-preview`)
  },

  /** Get provider instance IDs that allow user refund */
  getRefundEligibleProviders() {
    return apiClient.get<{ provider_instance_ids: string[] }>('/payment/orders/refund-eligible-providers')
  },

  /** Get provider instance IDs that allow invoice applications */
  getInvoiceEligibleProviders() {
    return apiClient.get<{ provider_instance_ids: string[] }>('/payment/orders/invoice-eligible-providers')
  },

  /** Get invoice application by order ID */
  getOrderInvoice(id: number) {
    return apiClient.get<InvoiceApplication>(`/payment/orders/${id}/invoice`)
  },

  /** Apply invoice for a completed order */
  applyOrderInvoice(id: number, data: {
    title: string
    tax_number: string
    email: string
    contact_name?: string
    contact_phone?: string
    request_note?: string
  }) {
    return apiClient.post<InvoiceApplication>(`/payment/orders/${id}/invoice`, data)
  },

  /** Cancel invoice application for an order */
  cancelOrderInvoice(id: number) {
    return apiClient.post<InvoiceApplication>(`/payment/orders/${id}/invoice/cancel`)
  },

  /** Get invoice download URL */
  getOrderInvoiceDownloadURL(id: number) {
    return apiClient.get<MediaDownloadURL>(`/payment/orders/${id}/invoice/download`)
  },

  /** Batch apply invoice for multiple completed orders */
  batchApplyInvoice(data: {
    order_ids: number[]
    title: string
    tax_number: string
    email: string
    contact_name?: string
    contact_phone?: string
    request_note?: string
  }) {
    return apiClient.post<BatchApplyInvoiceResult>('/payment/orders/invoices/batch-apply', data)
  }
}
