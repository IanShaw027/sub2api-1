/**
 * API Client for Sub2API Backend
 * Central export point for all API modules
 */

// Re-export the HTTP client
export { apiClient } from './client'

// Auth API
export { authAPI, isTotp2FARequired, type LoginResponse } from './auth'

// User APIs
export { keysAPI } from './keys'
export { usageAPI } from './usage'
export { userAPI } from './user'
export { redeemAPI, type RedeemHistoryItem } from './redeem'
export { paymentAPI } from './payment'
export { userGroupsAPI } from './groups'
export { userChannelsAPI } from './channels'
export { default as aiAPI, listPrompts as listAIPrompts, createPrompt as createAIPrompt, updatePrompt as updateAIPrompt, deletePrompt as deleteAIPrompt, clonePrompt as cloneAIPrompt, listArtworks as listAIArtworks, createArtwork as createAIArtwork, editArtwork as editAIArtwork, updateArtwork as updateAIArtwork, deleteArtwork as deleteAIArtwork, chat as aiChat, loadRuntimeLines as loadAIRuntimeLines } from './ai'
export { default as skillsAPI, listSkillMarket, listMySkills, getSkillDetail, createSkill, updateSkill, installSkill, uninstallSkill, listSkillVersions, createSkillVersion, updateSkillVersion, listSkillRuns, getSkillRevenue } from './skills'
export { totpAPI } from './totp'
export { default as announcementsAPI } from './announcements'
export { default as ticketsAPI } from './tickets'
export { default as adminTicketsAPI } from './adminTickets'
export { channelMonitorUserAPI } from './channelMonitor'

// Admin APIs
export { adminAPI } from './admin'

// Default export
export { default } from './client'
