/**
 * Admin Settings API endpoints
 * Handles system settings management for administrators
 *
 * This file is a re-export barrel. Domain-specific settings API modules now
 * live under src/api/admin/settings/ (split for maintainability); every
 * existing `@/api/admin/settings` import (named or default) keeps working
 * unchanged.
 */

export * from "./settings/general";
export * from "./settings/security";
export * from "./settings/email";
export * from "./settings/gateway";

import {
  getSettings,
  updateSettings,
} from "./settings/general";
import {
  testSmtpConnection,
  sendTestEmail,
  getEmailTemplates,
  getEmailTemplate,
  updateEmailTemplate,
  restoreOfficialEmailTemplate,
  previewEmailTemplate,
} from "./settings/email";
import {
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey,
  getPanelRateLimitSettings,
  updatePanelRateLimitSettings,
} from "./settings/security";
import {
  getOverloadCooldownSettings,
  updateOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
  resetWebSearchUsage,
} from "./settings/gateway";

export const settingsAPI = {
  getSettings,
  updateSettings,
  testSmtpConnection,
  sendTestEmail,
  getEmailTemplates,
  getEmailTemplate,
  updateEmailTemplate,
  restoreOfficialEmailTemplate,
  previewEmailTemplate,
  getAdminApiKey,
  regenerateAdminApiKey,
  deleteAdminApiKey,
  getOverloadCooldownSettings,
  updateOverloadCooldownSettings,
  getRateLimit429CooldownSettings,
  updateRateLimit429CooldownSettings,
  getPanelRateLimitSettings,
  updatePanelRateLimitSettings,
  getStreamTimeoutSettings,
  updateStreamTimeoutSettings,
  getRectifierSettings,
  updateRectifierSettings,
  getBetaPolicySettings,
  updateBetaPolicySettings,
  getWebSearchEmulationConfig,
  updateWebSearchEmulationConfig,
  testWebSearchEmulation,
  resetWebSearchUsage,
};

export default settingsAPI;
