/**
 * Vue Router configuration for Sub2API frontend
 * Defines all application routes with lazy loading and navigation guards
 */

import { createRouter, createWebHistory, type RouteLocationNormalized, type RouteRecordRaw } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { useAppStore } from '@/stores/app'
import { useAdminSettingsStore } from '@/stores/adminSettings'
import { useSkillsCenterStore } from '@/stores/skillsCenter'
import { useNavigationLoadingState } from '@/composables/useNavigationLoading'
import { useRoutePrefetch } from '@/composables/useRoutePrefetch'
import { skillPaths } from '@/components/skills/paths'
import { isSimpleModeRouteRestricted } from '@/navigation/simpleMode'
import {
  FeatureFlags,
  isChannelMonitorRouteEnabled,
  isFeatureFlagEnabled,
  isFeatureFlagResolved,
  type FeatureFlagDefinition,
} from '@/utils/featureFlags'
import { resolveDocumentTitle } from './title'

/**
 * Route definitions with lazy loading
 */
const routes: RouteRecordRaw[] = [
  // ==================== Setup Routes ====================
  {
    path: '/setup',
    name: 'Setup',
    component: () => import('@/views/setup/SetupWizardView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Setup'
    }
  },

  // ==================== Public Routes ====================
  {
    path: '/home',
    name: 'Home',
    component: () => import('@/views/HomeView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Home'
    }
  },
  {
    path: '/login',
    name: 'Login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Login',
      titleKey: 'home.login'
    }
  },
  {
    path: '/register',
    name: 'Register',
    component: () => import('@/views/auth/RegisterView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Register',
      titleKey: 'auth.createAccount'
    }
  },
  {
    path: '/email-verify',
    name: 'EmailVerify',
    component: () => import('@/views/auth/EmailVerifyView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Verify Email'
    }
  },
  {
    path: '/auth/callback',
    name: 'OAuthCallback',
    alias: '/auth/oauth/callback',
    component: () => import('@/views/auth/OAuthCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OAuth Callback',
      titleKey: 'auth.oauthCallbackPageTitle'
    }
  },
  {
    path: '/auth/linuxdo/callback',
    name: 'LinuxDoOAuthCallback',
    component: () => import('@/views/auth/LinuxDoCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'LinuxDo OAuth Callback',
      titleKey: 'auth.linuxdoCallbackPageTitle'
    }
  },
  {
    path: '/auth/dingtalk/callback',
    name: 'DingTalkOAuthCallback',
    component: () => import('@/views/auth/DingTalkCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'DingTalk OAuth Callback',
      titleKey: 'auth.dingtalk.callbackTitle'
    }
  },
  {
    path: '/auth/dingtalk/email-completion',
    name: 'DingTalkEmailCompletion',
    component: () => import('@/views/auth/DingTalkEmailCompletionView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Complete DingTalk Registration',
      titleKey: 'auth.dingtalk.createAccountTitle'
    }
  },
  {
    path: '/auth/wechat/callback',
    name: 'WeChatOAuthCallback',
    component: () => import('@/views/auth/WechatCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat OAuth Callback',
      titleKey: 'auth.wechatCallbackPageTitle'
    }
  },
  {
    path: '/auth/wechat/payment/callback',
    name: 'WeChatPaymentOAuthCallback',
    component: () => import('@/views/auth/WechatPaymentCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'WeChat Payment Callback',
      titleKey: 'auth.wechatPaymentCallbackPageTitle'
    }
  },
  {
    path: '/auth/oidc/callback',
    name: 'OIDCOAuthCallback',
    component: () => import('@/views/auth/OidcCallbackView.vue'),
    meta: {
      requiresAuth: false,
      title: 'OIDC OAuth Callback',
      titleKey: 'auth.oidcCallbackPageTitle'
    }
  },
  {
    path: '/forgot-password',
    name: 'ForgotPassword',
    component: () => import('@/views/auth/ForgotPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Forgot Password',
      titleKey: 'auth.forgotPasswordTitle'
    }
  },
  {
    path: '/reset-password',
    name: 'ResetPassword',
    component: () => import('@/views/auth/ResetPasswordView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Reset Password'
    }
  },
  {
    path: '/key-usage',
    name: 'KeyUsage',
    component: () => import('@/views/KeyUsageView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Key Usage',
    }
  },
  {
    path: '/legal/:documentId',
    name: 'LegalDocument',
    component: () => import('@/views/public/LegalDocumentView.vue'),
    meta: {
      requiresAuth: false,
      title: 'Legal Document'
    }
  },

  // ==================== User Routes ====================
  {
    path: '/',
    redirect: '/home'
  },
  {
    path: '/dashboard',
    name: 'Dashboard',
    component: () => import('@/views/user/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Dashboard',
      titleKey: 'dashboard.title',
      descriptionKey: 'dashboard.welcomeMessage'
    }
  },
  {
    path: '/ai',
    redirect: '/ai/chat'
  },
  {
    path: '/ai/chat',
    name: 'AIChat',
    component: () => import('@/views/user/AIChatView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'AI Chat',
      titleKey: 'ai.chat.title',
      descriptionKey: 'ai.chat.subtitle'
    }
  },
  {
    path: '/ai/image',
    name: 'AIImage',
    component: () => import('@/views/user/AIImageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'AI Image',
      titleKey: 'ai.image.title',
      descriptionKey: 'ai.image.subtitle'
    }
  },
  {
    path: '/ai/gallery',
    name: 'AIGallery',
    component: () => import('@/views/user/AIGalleryView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'AI Gallery',
      titleKey: 'ai.gallery.title',
      descriptionKey: 'ai.gallery.subtitle'
    }
  },
  {
    path: '/ai/prompts',
    name: 'AIPromptLibrary',
    component: () => import('@/views/user/AIPromptLibraryView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'AI Prompts',
      titleKey: 'ai.promptLibrary.title',
      descriptionKey: 'ai.promptLibrary.subtitle'
    }
  },
  {
    path: '/skills',
    redirect: '/skills/market',
    meta: {
      requiresAiStudio: true
    }
  },
  {
    path: '/skills/market',
    name: 'SkillMarket',
    component: () => import('@/views/user/SkillMarketView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'Skill Market',
      titleKey: 'skills.market.title',
      descriptionKey: 'skills.market.subtitle'
    }
  },
  {
    path: '/skills/installed',
    name: 'SkillInstalled',
    component: () => import('@/views/user/SkillMarketView.vue'),
    props: { installedOnly: true },
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'Installed Skills',
      titleKey: 'skills.installed.title',
      descriptionKey: 'skills.installed.subtitle'
    }
  },
  {
    path: '/skills/mine',
    name: 'SkillMine',
    component: () => import('@/views/user/SkillMySkillsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'My Skills',
      titleKey: 'skills.my.title',
      descriptionKey: 'skills.my.subtitle'
    }
  },
  {
    path: '/skills/new',
    name: 'SkillCreate',
    component: () => import('@/views/user/SkillEditorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'Create Skill',
      titleKey: 'skills.editor.create',
      descriptionKey: 'skills.editor.subtitle'
    }
  },
  {
    path: '/skills/:id/edit',
    name: 'SkillEdit',
    component: () => import('@/views/user/SkillEditorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      requiresSkillEditable: true,
      title: 'Edit Skill',
      titleKey: 'skills.editor.edit',
      descriptionKey: 'skills.editor.subtitle'
    }
  },
  {
    path: '/skills/:id/versions',
    name: 'SkillVersions',
    component: () => import('@/views/user/SkillVersionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      requiresSkillEditable: true,
      title: 'Skill Versions',
      titleKey: 'skills.versions.title',
      descriptionKey: 'skills.versions.subtitle'
    }
  },
  {
    path: '/skills/:id/runs',
    name: 'SkillRuns',
    component: () => import('@/views/user/SkillRunsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      requiresSkillOwned: true,
      title: 'Skill Runs',
      titleKey: 'skills.runs.title',
      descriptionKey: 'skills.runs.subtitle'
    }
  },
  {
    path: '/skills/:id/revenue',
    name: 'SkillRevenue',
    component: () => import('@/views/user/SkillRevenueView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      requiresSkillOwned: true,
      title: 'Skill Revenue',
      titleKey: 'skills.revenue.title',
      descriptionKey: 'skills.revenue.subtitle'
    }
  },
  {
    path: '/skills/:id',
    name: 'SkillDetail',
    component: () => import('@/views/user/SkillDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAiStudio: true,
      title: 'Skill Detail',
      titleKey: 'skills.detail.title',
      descriptionKey: 'skills.detail.subtitle'
    }
  },
  {
    path: '/keys',
    name: 'Keys',
    component: () => import('@/views/user/KeysView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'API Keys',
      titleKey: 'keys.title',
      descriptionKey: 'keys.description'
    }
  },
  {
    path: '/usage',
    name: 'Usage',
    component: () => import('@/views/user/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Usage Records',
      titleKey: 'usage.title',
      descriptionKey: 'usage.description'
    }
  },
  {
    path: '/redeem',
    name: 'Redeem',
    component: () => import('@/views/user/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Redeem Code',
      titleKey: 'redeem.title',
      descriptionKey: 'redeem.description'
    }
  },
  {
    path: '/affiliate',
    name: 'Affiliate',
    component: () => import('@/views/user/AffiliateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAffiliate: true,
      title: 'Affiliate',
      titleKey: 'affiliate.title',
      descriptionKey: 'affiliate.description'
    }
  },
  {
    path: '/available-channels',
    name: 'UserAvailableChannels',
    component: () => import('@/views/user/AvailableChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresAvailableChannels: true,
      title: 'Available Channels',
      titleKey: 'availableChannels.title',
      descriptionKey: 'availableChannels.description'
    }
  },
  {
    path: '/profile',
    name: 'Profile',
    component: () => import('@/views/user/ProfileView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Profile',
      titleKey: 'profile.title',
      descriptionKey: 'profile.description'
    }
  },
  {
    path: '/tickets',
    name: 'Tickets',
    component: () => import('@/views/user/TicketsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresTicket: true,
      title: 'Tickets',
      titleKey: 'tickets.title',
      descriptionKey: 'tickets.description'
    }
  },
  {
    path: '/tickets/create',
    name: 'TicketCreate',
    component: () => import('@/views/user/TicketCreateView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresTicket: true,
      title: 'Create Ticket',
      titleKey: 'tickets.create'
    }
  },
  {
    path: '/tickets/:id',
    name: 'TicketDetail',
    component: () => import('@/views/user/TicketDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresTicket: true,
      title: 'Ticket Detail',
      titleKey: 'tickets.detailTitle'
    }
  },
  {
    path: '/subscriptions',
    name: 'Subscriptions',
    component: () => import('@/views/user/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Subscriptions',
      titleKey: 'userSubscriptions.title',
      descriptionKey: 'userSubscriptions.description'
    }
  },
  {
    path: '/purchase',
    name: 'PurchaseSubscription',
    component: () => import('@/views/user/PaymentView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Purchase Subscription',
      titleKey: 'nav.buySubscription',
      descriptionKey: 'purchase.description',
      requiresPayment: true
    }
  },
  {
    path: '/orders',
    name: 'OrderList',
    component: () => import('@/views/user/UserOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Orders',
      titleKey: 'nav.myOrders',
      requiresPayment: true
    }
  },
  {
    path: '/orders/invoices',
    name: 'MyInvoices',
    component: () => import('@/views/user/UserInvoicesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'My Invoices',
      titleKey: 'nav.myInvoices',
      requiresPayment: true
    }
  },
  {
    path: '/orders/invoices/:id',
    name: 'MyInvoiceDetail',
    component: () => import('@/views/user/UserInvoiceDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Invoice Detail',
      titleKey: 'nav.invoiceDetail',
      requiresPayment: true
    }
  },
  {
    path: '/payment/qrcode',
    name: 'PaymentQRCode',
    component: () => import('@/views/user/PaymentQRCodeView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Payment',
      titleKey: 'payment.qr.scanToPay',
      requiresPayment: true
    }
  },
  {
    path: '/payment/result',
    name: 'PaymentResult',
    component: () => import('@/views/user/PaymentResultView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment Result',
      titleKey: 'payment.result.success',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe',
    name: 'StripePayment',
    component: () => import('@/views/user/StripePaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Stripe Payment',
      titleKey: 'payment.stripePay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/airwallex',
    name: 'AirwallexPayment',
    component: () => import('@/views/user/AirwallexPaymentView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Airwallex Payment',
      titleKey: 'payment.airwallexPay',
      requiresPayment: false
    }
  },
  {
    path: '/payment/stripe-popup',
    name: 'StripePopup',
    component: () => import('@/views/user/StripePopupView.vue'),
    meta: {
      requiresAuth: false,
      requiresAdmin: false,
      title: 'Payment',
      requiresPayment: false
    }
  },
  {
    path: '/custom/:id',
    name: 'CustomPage',
    component: () => import('@/views/user/CustomPageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      title: 'Custom Page',
      titleKey: 'customPage.title',
    }
  },

  // ==================== Admin Routes ====================
  {
    path: '/admin',
    redirect: '/admin/dashboard'
  },
  {
    path: '/admin/ai',
    redirect: '/admin/ai/prompts'
  },
  {
    path: '/admin/skills',
    redirect: '/admin/skills/governance',
    meta: {
      requiresAiStudio: true
    }
  },
  {
    path: '/admin/dashboard',
    name: 'AdminDashboard',
    component: () => import('@/views/admin/DashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Admin Dashboard',
      titleKey: 'admin.dashboard.title',
      descriptionKey: 'admin.dashboard.description'
    }
  },
  {
    path: '/admin/ops',
    name: 'AdminOps',
    component: () => import('@/views/admin/ops/OpsDashboard.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Ops Monitoring',
      titleKey: 'admin.ops.title',
      descriptionKey: 'admin.ops.description'
    }
  },
  {
    path: '/admin/users',
    name: 'AdminUsers',
    component: () => import('@/views/admin/UsersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'User Management',
      titleKey: 'admin.users.title',
      descriptionKey: 'admin.users.description'
    }
  },
  {
    path: '/admin/groups',
    name: 'AdminGroups',
    component: () => import('@/views/admin/GroupsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Group Management',
      titleKey: 'admin.groups.title',
      descriptionKey: 'admin.groups.description'
    }
  },
  {
    path: '/admin/channels',
    redirect: '/admin/channels/pricing'
  },
  {
    path: '/admin/channels/pricing',
    name: 'AdminChannels',
    component: () => import('@/views/admin/ChannelsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Channel Management',
      titleKey: 'admin.channels.title',
      descriptionKey: 'admin.channels.description'
    }
  },
  {
    path: '/admin/channels/monitor',
    name: 'AdminChannelMonitor',
    component: () => import('@/views/admin/ChannelMonitorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresChannelMonitor: true,
      title: 'Channel Monitor',
      titleKey: 'admin.channelMonitor.title',
      descriptionKey: 'admin.channelMonitor.description'
    }
  },
  {
    path: '/monitor',
    name: 'ChannelStatus',
    component: () => import('@/views/user/ChannelStatusView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: false,
      requiresChannelMonitor: true,
      title: 'Channel Status',
      titleKey: 'nav.channelStatus'
    }
  },
  {
    path: '/admin/subscriptions',
    name: 'AdminSubscriptions',
    component: () => import('@/views/admin/SubscriptionsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Management',
      titleKey: 'admin.subscriptions.title',
      descriptionKey: 'admin.subscriptions.description'
    }
  },
  {
    path: '/admin/accounts',
    name: 'AdminAccounts',
    component: () => import('@/views/admin/AccountsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Account Management',
      titleKey: 'admin.accounts.title',
      descriptionKey: 'admin.accounts.description'
    }
  },
  {
    path: '/admin/announcements',
    name: 'AdminAnnouncements',
    component: () => import('@/views/admin/AnnouncementsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Announcements',
      titleKey: 'admin.announcements.title',
      descriptionKey: 'admin.announcements.description'
    }
  },
  {
    path: '/admin/tickets',
    name: 'AdminTickets',
    component: () => import('@/views/admin/TicketsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresTicket: true,
      title: 'Ticket Management',
      titleKey: 'admin.tickets.title',
      descriptionKey: 'admin.tickets.description'
    }
  },
  {
    path: '/admin/tickets/:id',
    name: 'AdminTicketDetail',
    component: () => import('@/views/admin/TicketDetailView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresTicket: true,
      title: 'Ticket Detail',
      titleKey: 'tickets.detailTitle'
    }
  },
  {
    path: '/admin/proxies',
    name: 'AdminProxies',
    component: () => import('@/views/admin/ProxiesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Proxy Management',
      titleKey: 'admin.proxies.title',
      descriptionKey: 'admin.proxies.description'
    }
  },
  {
    path: '/admin/redeem',
    name: 'AdminRedeem',
    component: () => import('@/views/admin/RedeemView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Redeem Code Management',
      titleKey: 'admin.redeem.title',
      descriptionKey: 'admin.redeem.description'
    }
  },
  {
    path: '/admin/promo-codes',
    name: 'AdminPromoCodes',
    component: () => import('@/views/admin/PromoCodesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Promo Code Management',
      titleKey: 'admin.promo.title',
      descriptionKey: 'admin.promo.description'
    }
  },
  {
    path: '/admin/settings',
    name: 'AdminSettings',
    component: () => import('@/views/admin/SettingsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'System Settings',
      titleKey: 'admin.settings.title',
      descriptionKey: 'admin.settings.description'
    }
  },
  {
    path: '/admin/risk-control',
    name: 'AdminRiskControl',
    component: () => import('@/views/admin/RiskControlView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Risk Control',
      titleKey: 'admin.riskControl.title',
      descriptionKey: 'admin.riskControl.description',
      requiresRiskControl: true
    }
  },
  {
    path: '/admin/usage',
    name: 'AdminUsage',
    component: () => import('@/views/admin/UsageView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Usage Records',
      titleKey: 'admin.usage.title',
      descriptionKey: 'admin.usage.description'
    }
  },
  {
    path: '/admin/ai/prompts',
    name: 'AdminAIPrompts',
    component: () => import('@/views/admin/AIPromptGovernanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'AI Prompt Governance',
      titleKey: 'ai.promptGovernance.title',
      descriptionKey: 'ai.promptGovernance.subtitle'
    }
  },
  {
    path: '/admin/ai/artworks',
    name: 'AdminAIArtworks',
    component: () => import('@/views/admin/AIArtworkGovernanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'AI Artwork Governance',
      titleKey: 'ai.artworkGovernance.title',
      descriptionKey: 'ai.artworkGovernance.subtitle'
    }
  },
  {
    path: '/admin/skills/review',
    name: 'AdminSkillReview',
    component: () => import('@/views/admin/SkillReviewView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'Skill Review',
      titleKey: 'skills.admin.review.title',
      descriptionKey: 'skills.admin.review.emptyDesc'
    }
  },
  {
    path: '/admin/skills/governance',
    name: 'AdminSkillGovernance',
    component: () => import('@/views/admin/SkillGovernanceView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'Skill Governance',
      titleKey: 'skills.admin.governance.title',
      descriptionKey: 'skills.market.subtitle'
    }
  },
  {
    path: '/admin/skills/runtime',
    name: 'AdminSkillRuntime',
    component: () => import('@/views/admin/SkillRuntimeMonitorView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'Skill Runtime Monitor',
      titleKey: 'skills.admin.runtime.title',
      descriptionKey: 'skills.admin.runtime.subtitle'
    }
  },
  {
    path: '/admin/skills/settlements',
    name: 'AdminSkillSettlements',
    component: () => import('@/views/admin/SkillSettlementView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAiStudio: true,
      title: 'Skill Settlements',
      titleKey: 'skills.admin.settlement.title',
      descriptionKey: 'skills.revenue.subtitle'
    }
  },
  {
    path: '/admin/affiliates',
    redirect: '/admin/affiliates/invites',
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAffiliate: true
    }
  },
  {
    path: '/admin/affiliates/invites',
    name: 'AdminAffiliateInvites',
    component: () => import('@/views/admin/affiliates/AdminAffiliateInvitesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAffiliate: true,
      title: 'Affiliate Invite Records',
      titleKey: 'nav.affiliateInviteRecords',
      descriptionKey: 'admin.affiliates.invitesDescription'
    }
  },
  {
    path: '/admin/affiliates/rebates',
    name: 'AdminAffiliateRebates',
    component: () => import('@/views/admin/affiliates/AdminAffiliateRebatesView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAffiliate: true,
      title: 'Affiliate Rebate Records',
      titleKey: 'nav.affiliateRebateRecords',
      descriptionKey: 'admin.affiliates.rebatesDescription'
    }
  },
  {
    path: '/admin/affiliates/transfers',
    name: 'AdminAffiliateTransfers',
    component: () => import('@/views/admin/affiliates/AdminAffiliateTransfersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      requiresAffiliate: true,
      title: 'Affiliate Transfer Records',
      titleKey: 'nav.affiliateTransferRecords',
      descriptionKey: 'admin.affiliates.transfersDescription'
    }
  },


  // ==================== Payment Admin Routes ====================
  {
    path: '/admin/orders/dashboard',
    name: 'AdminPaymentDashboard',
    component: () => import('@/views/admin/orders/AdminPaymentDashboardView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Payment Dashboard',
      titleKey: 'nav.paymentDashboard',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders',
    name: 'AdminOrders',
    component: () => import('@/views/admin/orders/AdminOrdersView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Order Management',
      titleKey: 'nav.orderManagement',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders/plans',
    name: 'AdminPaymentPlans',
    component: () => import('@/views/admin/orders/AdminPaymentPlansView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Subscription Plans',
      titleKey: 'nav.paymentPlans',
      requiresPayment: true
    }
  },
  {
    path: '/admin/orders/invoices',
    name: 'AdminInvoiceApplications',
    component: () => import('@/views/admin/orders/AdminInvoiceApplicationsView.vue'),
    meta: {
      requiresAuth: true,
      requiresAdmin: true,
      title: 'Invoice Applications',
      titleKey: 'nav.invoiceApplications',
      requiresPayment: true
    }
  },

  // ==================== 404 Not Found ====================
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    component: () => import('@/views/NotFoundView.vue'),
    meta: {
      title: '404 Not Found'
    }
  }
]

function featureFlagsRequiredByRoute(to: RouteLocationNormalized): FeatureFlagDefinition[] {
  const flags: FeatureFlagDefinition[] = []
  if (to.meta?.requiresPayment === true) flags.push(FeatureFlags.payment)
  if (to.meta?.requiresTicket === true) flags.push(FeatureFlags.ticket)
  if (to.meta?.requiresAffiliate === true) flags.push(FeatureFlags.affiliate)
  if (to.meta?.requiresAiStudio === true) flags.push(FeatureFlags.aiStudio)
  if (to.meta?.requiresChannelMonitor === true) flags.push(FeatureFlags.channelMonitor)
  if (to.meta?.requiresAvailableChannels === true) flags.push(FeatureFlags.availableChannels)
  if (to.meta?.requiresRiskControl === true) flags.push(FeatureFlags.riskControl)
  return flags
}

async function ensurePublicSettingsForFeatureRoute(to: RouteLocationNormalized): Promise<void> {
  const requiredFlags = featureFlagsRequiredByRoute(to)
  if (requiredFlags.length === 0 || requiredFlags.every((flag) => isFeatureFlagResolved(flag))) {
    return
  }
  const appStore = useAppStore()
  await appStore.fetchPublicSettings()
}

async function ensureSkillRouteAccess(to: RouteLocationNormalized): Promise<string | null> {
  const requiresSkillEditable = to.meta.requiresSkillEditable === true
  const requiresSkillOwned = to.meta.requiresSkillOwned === true

  if (!requiresSkillEditable && !requiresSkillOwned) {
    return null
  }

  const rawSkillId = Array.isArray(to.params.id) ? to.params.id[0] : to.params.id
  const skillId = Number(rawSkillId)
  if (!Number.isFinite(skillId) || skillId <= 0) {
    return skillPaths.market
  }

  const skillsStore = useSkillsCenterStore()
  try {
    const skill = await skillsStore.loadSkillDetail(skillId, true)
    if (requiresSkillEditable && !skill.editable) {
      return skillPaths.detail(skillId)
    }
    if (requiresSkillOwned && !skill.owned) {
      return skillPaths.detail(skillId)
    }
  } catch {
    return '/dashboard'
  }

  return null
}

/**
 * Create router instance
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes,
  scrollBehavior(_to, _from, savedPosition) {
    // Scroll to saved position when using browser back/forward
    if (savedPosition) {
      return savedPosition
    }
    // Scroll to top for new routes
    return { top: 0 }
  }
})

/**
 * Navigation guard: Authentication check
 */
let authInitialized = false

// 初始化导航加载状态和预加载
const navigationLoading = useNavigationLoadingState()
// 延迟初始化预加载，传入 router 实例
let routePrefetch: ReturnType<typeof useRoutePrefetch> | null = null
export const BACKEND_MODE_ALLOWED_PATHS = [
  '/login',
  '/key-usage',
  '/setup',
  '/payment/result',
  '/payment/stripe',
  '/payment/stripe-popup',
  '/payment/airwallex',
  '/legal',
]
export const BACKEND_MODE_CALLBACK_PATHS = [
  '/auth/callback',
  '/auth/oauth/callback',
  '/auth/dingtalk/callback',
  '/auth/linuxdo/callback',
  '/auth/oidc/callback',
  '/auth/wechat/callback',
  '/auth/wechat/payment/callback',
]
export const BACKEND_MODE_PENDING_AUTH_PATHS = ['/register', '/email-verify', '/auth/dingtalk/email-completion']

export function isBackendModePublicRouteAllowed(path: string, hasPendingAuthSession: boolean): boolean {
  if (BACKEND_MODE_ALLOWED_PATHS.some((allowedPath) => path === allowedPath || path.startsWith(allowedPath))) {
    return true
  }

  if (BACKEND_MODE_CALLBACK_PATHS.some((callbackPath) => path === callbackPath)) {
    return true
  }

  if (hasPendingAuthSession && BACKEND_MODE_PENDING_AUTH_PATHS.some((allowedPath) => path === allowedPath)) {
    return true
  }

  return false
}

router.beforeEach(async (to, _from, next) => {
  // 开始导航加载状态
  navigationLoading.startNavigation()

  const authStore = useAuthStore()

  // Restore auth state from localStorage on first navigation (page refresh)
  if (!authInitialized) {
    authStore.checkAuth()
    authInitialized = true
  }

  // Set page title
  const appStore = useAppStore()
  // For custom pages, use menu item label as document title
  if (to.name === 'CustomPage') {
    const id = to.params.id as string
    const publicItems = appStore.cachedPublicSettings?.custom_menu_items ?? []
    const adminSettingsStore = useAdminSettingsStore()
    const menuItem = publicItems.find((item) => item.id === id)
      ?? (authStore.isAdmin ? adminSettingsStore.customMenuItems.find((item) => item.id === id) : undefined)
    if (menuItem?.label) {
      const siteName = appStore.siteName || 'Sub2API'
      document.title = `${menuItem.label} - ${siteName}`
    } else {
      document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string)
    }
  } else {
    document.title = resolveDocumentTitle(to.meta.title, appStore.siteName, to.meta.titleKey as string)
  }

  // Check if route requires authentication
  const requiresAuth = to.meta.requiresAuth !== false // Default to true
  const requiresAdmin = to.meta.requiresAdmin === true

  // If route doesn't require auth, allow access
  if (!requiresAuth) {
    // If already authenticated and trying to access login/register, redirect to appropriate dashboard
    if (authStore.isAuthenticated && (to.path === '/login' || to.path === '/register')) {
      // In backend mode, non-admin users should NOT be redirected away from login
      // (they are blocked from all protected routes, so redirecting would cause a loop)
      if (appStore.backendModeEnabled && !authStore.isAdmin) {
        next()
        return
      }
      // Admin users go to admin dashboard, regular users go to user dashboard
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
    // Backend mode: block public pages for unauthenticated users (except login, key-usage, setup)
    if (appStore.backendModeEnabled && !authStore.isAuthenticated) {
      const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
      if (!isAllowed) {
        next('/login')
        return
      }
    }
    next()
    return
  }

  // Route requires authentication
  if (!authStore.isAuthenticated) {
    // Not authenticated, redirect to login
    next({
      path: '/login',
      query: { redirect: to.fullPath } // Save intended destination
    })
    return
  }

  // Check admin requirement
  if (requiresAdmin && !authStore.isAdmin) {
    // User is authenticated but not admin, redirect to user dashboard
    next('/dashboard')
    return
  }

  await ensurePublicSettingsForFeatureRoute(to)

  // Check payment requirement (internal payment system only)
  if (to.meta.requiresPayment === true && !isFeatureFlagEnabled(FeatureFlags.payment)) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  // Check ticket module requirement
  if (to.meta.requiresTicket === true && !isFeatureFlagEnabled(FeatureFlags.ticket)) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  // Check affiliate module requirement
  if (to.meta.requiresAffiliate === true && !isFeatureFlagEnabled(FeatureFlags.affiliate)) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  if (to.meta.requiresAiStudio === true && !isFeatureFlagEnabled(FeatureFlags.aiStudio)) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  const skillRouteRedirect = await ensureSkillRouteAccess(to)
  if (skillRouteRedirect) {
    next(skillRouteRedirect)
    return
  }

  if (to.meta.requiresChannelMonitor === true && !isChannelMonitorRouteEnabled()) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  if (to.meta.requiresAvailableChannels === true && !isFeatureFlagEnabled(FeatureFlags.availableChannels)) {
    next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
    return
  }

  if (to.meta.requiresRiskControl === true && !isFeatureFlagEnabled(FeatureFlags.riskControl)) {
    next(authStore.isAdmin ? '/admin/settings' : '/dashboard')
    return
  }

  // 简易模式下限制访问某些页面
  if (authStore.isSimpleMode) {
    if (isSimpleModeRouteRestricted(to.path)) {
      // 简易模式下访问受限页面,重定向到仪表板
      next(authStore.isAdmin ? '/admin/dashboard' : '/dashboard')
      return
    }
  }

  // Backend mode: admin gets full access, non-admin blocked
  if (appStore.backendModeEnabled) {
    if (authStore.isAuthenticated && authStore.isAdmin) {
      next()
      return
    }
    const isAllowed = isBackendModePublicRouteAllowed(to.path, authStore.hasPendingAuthSession)
    if (!isAllowed) {
      next('/login')
      return
    }
  }

  // All checks passed, allow navigation
  next()
})

/**
 * Navigation guard: End loading and trigger prefetch
 */
router.afterEach((to) => {
  // 结束导航加载状态
  navigationLoading.endNavigation()

  // 懒初始化预加载（首次导航时创建，传入 router 实例）
  if (!routePrefetch) {
    routePrefetch = useRoutePrefetch(router)
  }
  // 触发路由预加载（在浏览器空闲时执行）
  routePrefetch.triggerPrefetch(to)
})

/**
 * Navigation guard: Error handling
 * Handles dynamic import failures caused by deployment updates
 */
router.onError((error) => {
  console.error('Router error:', error)
  if (typeof window === 'undefined' || typeof sessionStorage === 'undefined') {
    return
  }

  // Check if this is a dynamic import failure (chunk loading error)
  const isChunkLoadError =
    error.message?.includes('Failed to fetch dynamically imported module') ||
    error.message?.includes('Loading chunk') ||
    error.message?.includes('Loading CSS chunk') ||
    error.name === 'ChunkLoadError'

  if (isChunkLoadError) {
    // Avoid infinite reload loop by checking sessionStorage
    const reloadKey = 'chunk_reload_attempted'
    const lastReload = sessionStorage.getItem(reloadKey)
    const now = Date.now()

    // Allow reload if never attempted or more than 10 seconds ago
    if (!lastReload || now - parseInt(lastReload) > 10000) {
      sessionStorage.setItem(reloadKey, now.toString())
      console.warn('Chunk load error detected, reloading page to fetch latest version...')
      window.location.reload()
    } else {
      console.error('Chunk load error persists after reload. Please clear browser cache.')
    }
  }
})

export default router
