/**
 * Admin Settings API — SMTP test / send-test-email and email template
 * management endpoints.
 */

import { apiClient } from "../../client";

/**
 * Test SMTP connection request
 */
export interface TestSmtpRequest {
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  smtp_use_tls: boolean;
}

/**
 * Test SMTP connection with provided config
 * @param config - SMTP configuration to test
 * @returns Test result message
 */
export async function testSmtpConnection(
  config: TestSmtpRequest,
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    "/admin/settings/test-smtp",
    config,
  );
  return data;
}

/**
 * Send test email request
 */
export interface SendTestEmailRequest {
  email: string;
  smtp_host: string;
  smtp_port: number;
  smtp_username: string;
  smtp_password: string;
  smtp_from_email: string;
  smtp_from_name: string;
  smtp_use_tls: boolean;
}

/**
 * Send test email with provided SMTP config
 * @param request - Email address and SMTP config
 * @returns Test result message
 */
export async function sendTestEmail(
  request: SendTestEmailRequest,
): Promise<{ message: string }> {
  const { data } = await apiClient.post<{ message: string }>(
    "/admin/settings/send-test-email",
    request,
  );
  return data;
}

// ==================== Email Template Settings ====================

export interface EmailTemplateOption {
  value: string;
  label?: string;
  description?: string;
  category?: string;
  optional?: boolean;
}

export type EmailTemplateEventOption = string | EmailTemplateOption;

export interface EmailTemplateSummary {
  event: string;
  locale: string;
  subject: string;
  is_custom?: boolean;
  updated_at?: string;
}

export interface EmailTemplateListResponse {
  events: EmailTemplateEventOption[];
  locales: string[];
  templates?: EmailTemplateSummary[];
  placeholders?: string[];
}

export interface EmailTemplateDetail {
  event: string;
  locale: string;
  subject: string;
  html: string;
  is_custom?: boolean;
  updated_at?: string;
  placeholders?: string[];
}

export interface UpdateEmailTemplateRequest {
  subject: string;
  html: string;
}

export interface PreviewEmailTemplateRequest extends UpdateEmailTemplateRequest {
  event: string;
  locale: string;
}

export interface EmailTemplatePreviewResponse {
  subject: string;
  html: string;
}

export async function getEmailTemplates(): Promise<EmailTemplateListResponse> {
  const { data } = await apiClient.get<EmailTemplateListResponse>(
    "/admin/settings/email-templates",
  );
  return data;
}

export async function getEmailTemplate(
  event: string,
  locale: string,
): Promise<EmailTemplateDetail> {
  const { data } = await apiClient.get<EmailTemplateDetail>(
    `/admin/settings/email-templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`,
  );
  return data;
}

export async function updateEmailTemplate(
  event: string,
  locale: string,
  request: UpdateEmailTemplateRequest,
): Promise<EmailTemplateDetail> {
  const { data } = await apiClient.put<EmailTemplateDetail>(
    `/admin/settings/email-templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}`,
    request,
  );
  return data;
}

export async function restoreOfficialEmailTemplate(
  event: string,
  locale: string,
): Promise<EmailTemplateDetail> {
  const { data } = await apiClient.post<EmailTemplateDetail>(
    `/admin/settings/email-templates/${encodeURIComponent(event)}/${encodeURIComponent(locale)}/restore-official`,
  );
  return data;
}

export async function previewEmailTemplate(
  request: PreviewEmailTemplateRequest,
): Promise<EmailTemplatePreviewResponse> {
  const { data } = await apiClient.post<EmailTemplatePreviewResponse>(
    "/admin/settings/email-template-preview",
    request,
  );
  return data;
}
